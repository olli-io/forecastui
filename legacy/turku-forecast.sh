#!/usr/bin/env bash
# Forecast from FMI open data (OGC API-EDR, JSON, no API key), drawn as braille
# bars: per column a temperature bar and a rain bar side by side.
# Uses "pal_skandinavia" = FMI's own edited forecast (what ilmatieteenlaitos.fi shows).
#
# Usage: ./turku-forecast.sh [hours|week] [lon lat]   defaults: 24 h, Turku
#   up to 48 h  ->  one column per hour, hour labels, one-column gaps
#   beyond that ->  five columns per day at 09 12 15 18 21 local, each a 3 h
#                   rolling mean, with the day printed under 09 (max ~9 days)
set -euo pipefail

case "${1:-24}" in
  week|w) HOURS=168 ;;
  *)      HOURS="${1:-24}" ;;
esac
LON="${2:-22.2666}"
LAT="${3:-60.4518}"

case "$HOURS" in
  ''|*[!0-9]*) echo "usage: $(basename "$0") [hours|week] [lon lat]" >&2; exit 2 ;;
esac
[ "$HOURS" -ge 1 ] || { echo "hours must be at least 1" >&2; exit 2; }

# Only the default coordinates are Turku; anywhere else is labelled by position.
if [ "$LON" = "22.2666" ] && [ "$LAT" = "60.4518" ]; then
  PLACE="Turku"
else
  PLACE="$LAT, $LON"
fi

SLOTS=0
[ "$HOURS" -gt 48 ] && SLOTS=1

FROM=$(date -u +%Y-%m-%dT%H:00:00Z)
# HOURS-1: the range is inclusive of both ends, so asking for 24 gives 24 bars
# and keeps the graph inside an 80-column terminal.
TO=$(date -u -d "+$((HOURS - 1)) hours" +%Y-%m-%dT%H:00:00Z)

PARAMS='temperature,windspeedms,hourlymaximumgust,winddirection,precipitation1h,pop,totalcloudcover,weathersymbol3'

# Rendering lives in python: braille cells are built from bitmasks, which is
# more than bash and jq want to do. Kept in a variable rather than a heredoc so
# the curl output still reaches it on stdin.
RENDER=$(cat <<'PY'
import json, os, sys
from datetime import datetime, timezone

SLOTS  = len(sys.argv) > 1 and sys.argv[1] == "1"
PLACE  = sys.argv[2] if len(sys.argv) > 2 else "Turku"
CELL_H = 6                          # braille cells tall; 4 dot rows each
DOTS   = CELL_H * 4
SLOT_HOURS = tuple(range(0, 24, 3))   # 00 03 06 ... 21
# Every bar pair is followed by one blank column; between days that column
# carries a divider instead. A week is 35 slots, so the graph runs wider than
# 80 columns - the hourly view still fits.
STEP   = 3
LEFT   = (0x01, 0x02, 0x04, 0x40)   # dot rows top->bottom, left column
RIGHT  = (0x08, 0x10, 0x20, 0x80)   # ... and right column

COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None
def c(rgb, s):
    if not COLOR or s.strip("⠀ ") == "": return s
    return "\x1b[38;2;%d;%d;%dm%s\x1b[0m" % (rgb[0], rgb[1], rgb[2], s)

# gruvbox material (dark, medium) accents, matching the bar and terminal theme
RED, ORANGE, YELLOW = (234,105,98), (231,138,78), (216,166,87)   # ea6962 e78a4e d8a657
GREEN, AQUA         = (169,182,101), (137,180,130)               # a9b665 89b482
BLUE, PURPLE        = (125,174,163), (211,134,155)               # 7daea3 d3869b
GREY, FG            = (124,111,100), (213,196,161)               # 7c6f64 d5c4a1
RAIN = BLUE

# The usual cold-to-hot weather ramp, in palette hues. Blue belongs to rain, so
# the cold end runs purple -> aqua instead of passing through it - otherwise a
# freezing hour and a wet hour would be the same colour side by side.
TEMP_STOPS = ((-10, PURPLE), (0, AQUA), (8, GREEN), (16, YELLOW), (24, ORANGE))

def temp_color(t):
    if t is None: return GREY
    for limit, col in TEMP_STOPS:
        if t < limit: return col
    return RED

def cells(dots, row):
    """Braille bits for a bar `dots` tall at cell row `row` (0 = top)."""
    base = 4 * (CELL_H - 1 - row)
    bits = 0
    for s in range(4):
        if base + 3 - s < dots:
            bits |= LEFT[s] | RIGHT[s]
    return bits

def local(t):
    return datetime.strptime(t, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc).astimezone()

d = json.load(sys.stdin)
rows = {}
for f in d.get("features", []):
    p = f["properties"]; ts = p["time"]
    for k, vals in p.items():
        if k == "time": continue
        for i, t in enumerate(ts):
            rows.setdefault(t, {})[k] = vals[i]

times = sorted(rows)
if not times:
    sys.exit("no forecast data returned")

seq = [(local(t), rows[t].get("temperature"), rows[t].get("precipitation1h") or 0.0)
       for t in times]

# Columns are (temperature, rain, label, starts-a-new-day).
cols, prev_day = [], None
if SLOTS:
    for i, (dt, _, _) in enumerate(seq):
        if dt.hour not in SLOT_HOURS: continue
        # centred 3 h rolling mean, so a single spiky hour cannot carry a column
        win = [seq[j] for j in (i - 1, i, i + 1) if 0 <= j < len(seq)]
        tv = [w[1] for w in win if w[1] is not None]
        cols.append((sum(tv) / len(tv) if tv else None,
                     sum(w[2] for w in win) / len(win),
                     dt, dt.date() != prev_day))
        prev_day = dt.date()
else:
    for dt, tv, rv in seq:
        cols.append((tv, rv, dt, False))

if not cols:
    sys.exit("no forecast hours fell on 09/12/15/18/21 - try a longer range")

known = [t for t, _, _, _ in cols if t is not None]
if not known:
    sys.exit("forecast contained no temperatures")

hi, lo = max(known), min(known)
if hi == lo: hi = lo + 1.0
floor = lo - 0.10 * (hi - lo) - 0.2      # keeps the coldest column a visible stub
rmax  = max(r for _, r, _, _ in cols)

def scale(v):
    return (v - floor) / (hi - floor) * DOTS

def temp_dots(v):
    return 0 if v is None else max(1, round(scale(v)))

def rain_dots(v):
    if not v or rmax <= 0: return 0
    return max(1, round(v / rmax * DOTS))

VERT, CORNER, HORIZ, TICK, DAYEND, RCORNER = "│", "└", "─", "┼", "┴", "┘"

# Where 0 C falls on the dot grid. Bars are drawn against the graph floor, not
# against freezing, so this is a reference line: it goes in the gaps between
# columns and through any cell a bar does not reach, never over a bar itself.
zero_dots = scale(0.0)
zero_row = zero_sub = None
if 0 <= zero_dots <= DOTS - 1:
    idx = int(round(zero_dots))
    zero_row, zero_sub = CELL_H - 1 - idx // 4, 3 - idx % 4

def zero_bits(row):
    return 0 if row != zero_row else LEFT[zero_sub] | RIGHT[zero_sub]

first, last = seq[0][0], seq[-1][0]
fmt = "%d %b" if SLOTS else "%H:%M"
span = "%s %s → %s %s" % (first.strftime("%a"), first.strftime(fmt),
                          last.strftime("%a"), last.strftime(fmt))
out = ["  %s  %s" % (c(FG, PLACE), c(GREY, span + ", local time")), ""]

if rmax > 0:
    rain_top, rain_bottom = "%.1f mm/h" % rmax, "0"
else:
    rain_top, rain_bottom = "no rain", ""

def draw(block):
    """One graph. Blocks share the scales above, so they stay comparable."""
    # A day ends where the next one starts; that column's gap becomes the divider.
    starts = [entry[3] for entry in block]
    ends = [i + 1 < len(block) and starts[i + 1] for i in range(len(block))]

    for r in range(CELL_H):
        if   r == 0:          label = "%5.1f°" % hi
        elif r == CELL_H - 1: label = "%5.1f°" % lo
        elif r == zero_row:   label = "    0°"
        else:                 label = " " * 6
        line = [c(GREY, label) + c(GREY, TICK if r == zero_row else VERT)]
        z = zero_bits(r)
        for i, (t, rn, _, _) in enumerate(block):
            for bits, col in ((cells(temp_dots(t), r), temp_color(t)),
                              (cells(rain_dots(rn), r), RAIN)):
                # an empty cell is free to carry the line; a bar keeps its own colour
                line.append(c(col, chr(0x2800 + bits)) if bits else c(GREY, chr(0x2800 + z)))
            # Two rows only: the divider rises from the axis as a tick rather
            # than walling off the bars.
            if ends[i] and r >= CELL_H - 2:
                line.append(c(GREY, VERT))
            else:
                line.append(c(GREY, chr(0x2800 + z)) if z else " ")
        # rain axis, mirroring the temperature axis on the left
        if   r == 0:          right = rain_top
        elif r == CELL_H - 1: right = rain_bottom
        else:                 right = ""
        line.append(c(GREY, VERT + right))
        out.append("".join(line))

    # Axis rule, meeting each day divider with a tick.
    rule = "".join(HORIZ * 2 + (DAYEND if ends[i] else HORIZ) for i in range(len(block)))
    out.append(c(GREY, " " * 6 + CORNER + rule + RCORNER))

    # Hours under every bar pair, and in slot mode the day under its first column.
    width = STEP * len(block)
    def place(items, style):
        row = [" "] * width
        for at, text in items:
            row[at:at + len(text)] = list(text[:width - at])
        out.append(" " * 7 + c(style, "".join(row).rstrip()))

    place([(i * STEP, dt.strftime("%H")) for i, (_, _, dt, _) in enumerate(block)], GREY)
    if SLOTS:
        place([(i * STEP, dt.strftime("%a %d"))
               for i, (_, _, dt, first_of_day) in enumerate(block) if first_of_day], FG)

# Eight slots a day is too wide for a week on one row, so days are dealt out
# four to a graph and stacked - a week becomes four days over three.
if SLOTS:
    days = []
    for entry in cols:
        if entry[3] or not days: days.append([])
        days[-1].append(entry)
    blocks = [[e for day in days[i:i + 4] for e in day] for i in range(0, len(days), 4)]
else:
    blocks = [cols]

for n, block in enumerate(blocks):
    if n: out.extend(["", ""])   # two blank rows, so stacked graphs stay distinct
    draw(block)
out.append("")

ramp = "".join(c(col, "█") for col in (PURPLE, AQUA, GREEN, YELLOW, ORANGE, RED))
rain_note = c(GREY, "none forecast") if rmax <= 0 else "peak %.1f mm/h, %.1f mm total" % (
    rmax, sum(r for _, r, _, _ in cols) * (3 if SLOTS else 1))
out.append("  %s temperature %.1f…%.1f °C    %s rain %s" % (ramp, lo, hi, c(RAIN, "█"), rain_note))
if SLOTS:
    out.append(c(GREY, "  each column is a 3 h rolling mean, every 3 h from 00 to 21 local"))
print("\n".join(out))
PY
)

curl -sS --get "https://opendata.fmi.fi/edr/collections/pal_skandinavia/position" \
  --data-urlencode "coords=POINT($LON $LAT)" \
  --data-urlencode "parameter-name=$PARAMS" \
  --data-urlencode "datetime=$FROM/$TO" \
  --data-urlencode "f=GeoJSON" \
| python3 -c "$RENDER" "$SLOTS" "$PLACE"
