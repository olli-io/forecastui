package ui

// No terminal will say what font it is using, and a missing glyph is drawn as
// tofu rather than reported. What this file can establish is narrower:
//
//   - what the reader said, through --glyphs or the environment;
//   - terminals with no font to patch at all, the Linux console among them;
//   - whether the glyph advances the cursor by one cell, since a double-width
//     fallback shears the braille grid;
//   - whether any font on the machine covers the codepoint, per fontconfig.
//
// None of those proves a Nerd Font is loaded; together they catch the cases
// where one certainly is not. Anything unsettled draws the glyphs.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/olli-io/forecastui/internal/fmi"
)

// GlyphMode is what --glyphs settles: the reader's word, or the checks below.
type GlyphMode int

const (
	GlyphAuto GlyphMode = iota
	GlyphNerd
	GlyphPlain
)

// glyphEnv says the same thing as the flag.
const glyphEnv = "FORECASTUI_GLYPHS"

// probeWait is how long the terminal is given to answer; it only matters for
// one that never will, and it is paid once at startup.
const probeWait = 100 * time.Millisecond

// fontWait bounds the fontconfig lookup, which reads the font cache.
const fontWait = 500 * time.Millisecond

// ttyPath is the terminal itself, whatever the standard streams point at.
// Windows has no such file, and the probe is skipped there.
const ttyPath = "/dev/tty"

// ParseGlyphMode reads the flag or the environment variable.
func ParseGlyphMode(s string) (GlyphMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return GlyphAuto, nil
	case "nerd", "on", "yes", "1", "true":
		return GlyphNerd, nil
	case "plain", "ascii", "off", "no", "0", "false":
		return GlyphPlain, nil
	}
	return GlyphAuto, fmt.Errorf("unknown glyph mode %q: want auto, nerd or plain", s)
}

// NerdFont reports whether the weather symbols should be drawn as Nerd Font
// glyphs. Settle it once at startup: it costs a round trip and a subprocess.
func NerdFont(mode GlyphMode) bool {
	switch mode {
	case GlyphNerd:
		return true
	case GlyphPlain:
		return false
	}
	if env, err := ParseGlyphMode(os.Getenv(glyphEnv)); err == nil && env != GlyphAuto {
		return env == GlyphNerd
	}
	// A console with a built-in font cannot be given a patched one.
	switch strings.ToLower(os.Getenv("TERM")) {
	case "", "dumb", "linux", "cons25", "vt100", "vt220":
		return false
	}
	advance, font := probeTerminal()
	// A two-cell glyph shears the braille grid, whatever font draws it.
	if advance > 0 && advance != 1 {
		return false
	}
	if font != "" {
		return nerdName(font)
	}
	if has, known := fontHasGlyph(fmi.SampleGlyph); known {
		return has
	}
	return true
}

// nerdName reads a font name the way the patcher writes it: "Nerd Font", the
// "NF" abbreviations, or the symbols-only fallback font.
func nerdName(name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "nerd") || strings.Contains(lower, "symbols") {
		return true
	}
	for _, f := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == '-' || r == ':' || r == ','
	}) {
		switch f {
		case "nf", "nfm", "nfp":
			return true
		}
	}
	return false
}

// probeTerminal asks two things in one round trip: what font is in use, which
// almost no terminal will say, and where the cursor lands after a glyph, which
// all of them will — so the cursor report doubles as the end of the reply.
//
// It goes through /dev/tty, not the standard streams: the runtime can poll that
// file, where blocking os.Stdin takes no read deadline, and it keeps the probe
// out of a redirected stdout.
func probeTerminal() (advance int, font string) {
	tty, err := os.OpenFile(ttyPath, os.O_RDWR, 0)
	if err != nil {
		return 0, ""
	}
	defer tty.Close() //nolint:errcheck // read-only conversation
	restore, ok := rawMode(tty)
	if !ok {
		return 0, ""
	}
	defer restore()
	if err := tty.SetReadDeadline(time.Now().Add(probeWait)); err != nil {
		return 0, ""
	}

	// Written from the start of the line so the reported column is its width;
	// the line is wiped afterwards.
	fmt.Fprintf(tty, "\r\x1b]50;?\x1b\\%c\x1b[6n", fmi.SampleGlyph)
	reply := readReply(tty)
	fmt.Fprint(tty, "\r\x1b[2K")

	if col, ok := cursorCol(reply); ok {
		advance = col - 1
	}
	return advance, fontReply(reply)
}

// rawMode turns off echo and line buffering for the probe. It reaches the
// descriptor through SyscallConn: File.Fd would take the file out of the
// runtime's poller, and the read deadline with it.
func rawMode(tty *os.File) (restore func(), ok bool) {
	conn, err := tty.SyscallConn()
	if err != nil {
		return nil, false
	}
	var state *term.State
	if err := conn.Control(func(fd uintptr) {
		state, err = term.MakeRaw(fd)
	}); err != nil || state == nil {
		return nil, false
	}
	return func() {
		conn.Control(func(fd uintptr) { term.Restore(fd, state) }) //nolint:errcheck // best effort
	}, true
}

// readReply collects the answer up to the cursor report, or to the deadline.
func readReply(in *os.File) string {
	var b []byte
	buf := make([]byte, 64)
	for len(b) < 512 {
		n, err := in.Read(buf)
		b = append(b, buf[:n]...)
		if err != nil || bytes.IndexByte(buf[:n], 'R') >= 0 {
			break
		}
	}
	return string(b)
}

// cursorCol pulls the column out of a cursor position report, ESC [ row ; col R.
func cursorCol(reply string) (int, bool) {
	i := strings.LastIndex(reply, "\x1b[")
	if i < 0 {
		return 0, false
	}
	body, _, ok := strings.Cut(reply[i+2:], "R")
	if !ok {
		return 0, false
	}
	_, col, ok := strings.Cut(body, ";")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(col))
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// fontReply pulls the font name out of an OSC 50 answer, ESC ] 50 ; name ST.
func fontReply(reply string) string {
	i := strings.Index(reply, "\x1b]50;")
	if i < 0 {
		return ""
	}
	rest := reply[i+len("\x1b]50;"):]
	end := strings.IndexAny(rest, "\x07\x1b")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// fontHasGlyph asks fontconfig whether any installed font covers the
// codepoint; terminals that lay out through it fall back across every font it
// lists. Elsewhere a missing fc-list means "unknown" rather than "no".
func fontHasGlyph(r rune) (has, known bool) {
	if runtime.GOOS == "windows" {
		return false, false
	}
	if _, err := exec.LookPath("fc-list"); err != nil {
		return false, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), fontWait)
	defer cancel()
	out, err := exec.CommandContext(ctx, "fc-list", fmt.Sprintf(":charset=%x", r), "family").Output()
	if err != nil {
		return false, false
	}
	return len(bytes.TrimSpace(out)) > 0, true
}
