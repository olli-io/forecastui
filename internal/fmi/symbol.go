package fmi

// Symbol describes an FMI weathersymbol3 code in two forms: the Nerd Font
// glyph and a one-cell ASCII stand-in.
type Symbol struct {
	Glyph rune
	Plain rune
	Desc  string
}

// Rune picks the form the terminal can draw; both are one cell wide.
func (s Symbol) Rune(nerd bool) rune {
	if nerd {
		return s.Glyph
	}
	return s.Plain
}

// Nerd Fonts' Material Design weather set (nf-md-weather-*), single-width in
// the Mono and Propo builds — a double-width icon would shear the strip.
const (
	nfSunny            = '\U000F0599'
	nfNight            = '\U000F0594'
	nfPartlyCloudy     = '\U000F0595'
	nfNightPartlyCloud = '\U000F0F31'
	nfCloudy           = '\U000F0590'
	nfRainy            = '\U000F0597'
	nfPouring          = '\U000F0596'
	nfSnowy            = '\U000F0598'
	nfSnowyHeavy       = '\U000F0F36'
	nfSnowyRainy       = '\U000F067F'
	nfLightningRainy   = '\U000F067E'
	nfLightning        = '\U000F0593'
	nfHazy             = '\U000F0F30'
	nfFog              = '\U000F0591'
)

// ASCII stand-ins are mnemonic: lower case for the ordinary fall, upper case
// for the heavy one.
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
	ascThunderNo = 'T' // thunder without rain
	ascMist      = '-'
	ascFog       = '#'
)

// SampleGlyph probes whether a terminal can draw the set.
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

// afterDark holds the glyphs that change once the sun is down. Only codes
// describing the sky itself have one.
var afterDark = map[int]Symbol{
	1: {Glyph: nfNight, Plain: ascNight},
	2: {Glyph: nfNightPartlyCloud, Plain: ascPartly},
}

// Moonlit reports whether Describe would draw this code as a moon. Narrower
// than "it is dark": a rainy hour at midnight is not moonlit.
func Moonlit(code int, night bool) bool {
	if !night {
		return false
	}
	_, ok := afterDark[code]
	return ok
}

// Describe returns the symbol for a weathersymbol3 code, in its night form
// where it has one. Unknown codes render as a blank.
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
