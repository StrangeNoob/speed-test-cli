package output

import "strings"

// barGlyphs are the eight fractional fill levels (1/8 .. 8/8 of a cell).
var barGlyphs = []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

// renderBar returns a bar of exactly `width` runes representing value/max.
// Filled cells use '█', the final partial cell uses a fractional glyph, and
// empty cells use a space. value>max clamps to full; max<=0 renders all spaces.
func renderBar(value, max float64, width int) string {
	if width <= 0 {
		return ""
	}
	if max <= 0 {
		return strings.Repeat(" ", width)
	}
	frac := value / max
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}

	// Total eighths of a cell to fill across the whole bar.
	totalEighths := int(frac * float64(width) * 8)
	full := totalEighths / 8
	rem := totalEighths % 8

	var b strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i < full:
			b.WriteRune('█')
		case i == full && rem > 0:
			b.WriteRune(barGlyphs[rem-1])
		default:
			b.WriteRune(' ')
		}
	}
	return b.String()
}
