package fmi

// Symbol describes an FMI weathersymbol3 code. It carries two forms of the
// same weather: the Nerd Font glyph, and a one-cell ASCII stand-in for a
// terminal whose font cannot draw it.
type Symbol struct {
	Glyph rune
	Plain rune
	Desc  string
}

// Rune picks the form the terminal can actually draw. Both are one cell wide,
// so the chart's grid holds either way.
func (s Symbol) Rune(nerd bool) rune {
	if nerd {
		return s.Glyph
	}
	return s.Plain
}

// The glyphs are Nerd Fonts' Material Design weather set (nf-md-weather-*).
// They are single-width in the Mono and Propo builds, which the chart depends
// on: each hour owns four columns of a shared grid, and a double-width icon
// would shear the whole strip. A terminal without a Nerd Font patched in will
// draw them as tofu.
const (
	nfSunny            = '\U000F0599' // nf-md-weather_sunny
	nfNight            = '\U000F0594' // nf-md-weather_night
	nfPartlyCloudy     = '\U000F0595' // nf-md-weather_partly_cloudy
	nfNightPartlyCloud = '\U000F0F31' // nf-md-weather_night_partly_cloudy
	nfCloudy           = '\U000F0590' // nf-md-weather_cloudy
	nfRainy            = '\U000F0597' // nf-md-weather_rainy
	nfPouring          = '\U000F0596' // nf-md-weather_pouring
	nfSnowy            = '\U000F0598' // nf-md-weather_snowy
	nfSnowyHeavy       = '\U000F0F36' // nf-md-weather_snowy_heavy
	nfSnowyRainy       = '\U000F067F' // nf-md-weather_snowy_rainy
	nfLightningRainy   = '\U000F067E' // nf-md-weather_lightning_rainy
	nfLightning        = '\U000F0593' // nf-md-weather_lightning
	nfHazy             = '\U000F0F30' // nf-md-weather_hazy
	nfFog              = '\U000F0591' // nf-md-weather_fog
)

// The ASCII stand-ins are mnemonic rather than pictorial: at one cell there is
// no drawing a cloud, so the row reads as a code — lower case for the ordinary
// fall of it, upper case for the heavy one. Colour still carries the rest, the
// same way it does under the glyphs.
const (
	ascSunny     = '*'
	ascNight     = ')'
	ascPartly    = '%'
	ascCloudy    = '='
	ascRain      = 'r'
	ascPouring   = 'R'
	ascSnow      = 's'
	ascSnowHeavy = 'S'
	ascSleet     = 'x'
	ascThunder   = 't'
	ascThunderNo = 'T' // thunder without the rain under it
	ascMist      = '-'
	ascFog       = '#'
)

// SampleGlyph is one glyph out of the set, for asking a terminal what it makes
// of them. Any of them would do; the sun is the one most fonts patch first.
const SampleGlyph = nfSunny

var symbols = map[int]Symbol{
	1:  {nfSunny, ascSunny, "clear"},
	2:  {nfPartlyCloudy, ascPartly, "partly cloudy"},
	3:  {nfCloudy, ascCloudy, "cloudy"},
	21: {nfRainy, ascRain, "light showers"},
	22: {nfRainy, ascRain, "showers"},
	23: {nfPouring, ascPouring, "heavy showers"},
	31: {nfRainy, ascRain, "light rain"},
	32: {nfRainy, ascRain, "rain"},
	33: {nfPouring, ascPouring, "heavy rain"},
	41: {nfSnowy, ascSnow, "light snow showers"},
	42: {nfSnowy, ascSnow, "snow showers"},
	43: {nfSnowyHeavy, ascSnowHeavy, "heavy snow showers"},
	51: {nfSnowy, ascSnow, "light snowfall"},
	52: {nfSnowy, ascSnow, "snowfall"},
	53: {nfSnowyHeavy, ascSnowHeavy, "heavy snowfall"},
	61: {nfLightningRainy, ascThunder, "thundershowers"},
	62: {nfLightningRainy, ascThunder, "heavy thundershowers"},
	63: {nfLightning, ascThunderNo, "thunder"},
	64: {nfLightning, ascThunderNo, "heavy thunder"},
	71: {nfSnowyRainy, ascSleet, "light sleet showers"},
	72: {nfSnowyRainy, ascSleet, "sleet showers"},
	73: {nfSnowyRainy, ascSleet, "heavy sleet showers"},
	81: {nfSnowyRainy, ascSleet, "light sleet"},
	82: {nfSnowyRainy, ascSleet, "sleet"},
	83: {nfSnowyRainy, ascSleet, "heavy sleet"},
	91: {nfHazy, ascMist, "mist"},
	92: {nfFog, ascFog, "fog"},
}

// afterDark holds the glyphs that change once the sun is down. Only the codes
// that describe the sky itself have one: rain falls the same in the dark, and
// the set has no moonlit shower to draw it with anyway.
var afterDark = map[int]Symbol{
	1: {Glyph: nfNight, Plain: ascNight},
	2: {Glyph: nfNightPartlyCloud, Plain: ascPartly},
}

// Moonlit reports whether Describe would draw this code as a moon. Only the
// codes for the sky itself have a night form, so this is narrower than "it is
// dark": a rainy hour at midnight is not moonlit.
func Moonlit(code int, night bool) bool {
	if !night {
		return false
	}
	_, ok := afterDark[code]
	return ok
}

// Describe returns the symbol for a weathersymbol3 code, in its night form
// where it has one. Unknown and absent codes render as a blank rather than a
// placeholder, so the strip stays quiet.
func Describe(code int, night bool) Symbol {
	s, ok := symbols[code]
	if !ok {
		return Symbol{' ', ' ', ""}
	}
	if night {
		if n, ok := afterDark[code]; ok {
			s.Glyph, s.Plain = n.Glyph, n.Plain
		}
	}
	return s
}
