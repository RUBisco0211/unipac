package views

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// The 16 standard terminal colors, as editors such as One Dark and One Light
// draw them. Using a pair of palettes rather than the theme's semantic colors
// keeps what a tool meant (a red error, a green success) distinct from what
// UniPac itself uses those colors for.
var (
	ansiDark = [16]ui.Color{
		ui.Hex("#5C6370"), ui.Hex("#E06C75"), ui.Hex("#98C379"), ui.Hex("#E5C07B"),
		ui.Hex("#61AFEF"), ui.Hex("#C678DD"), ui.Hex("#56B6C2"), ui.Hex("#ABB2BF"),
		ui.Hex("#7F848E"), ui.Hex("#FF7B86"), ui.Hex("#B5E890"), ui.Hex("#FFD68A"),
		ui.Hex("#7CC4FF"), ui.Hex("#E29AF0"), ui.Hex("#6FD6E0"), ui.Hex("#FFFFFF"),
	}
	ansiLight = [16]ui.Color{
		ui.Hex("#383A42"), ui.Hex("#E45649"), ui.Hex("#50A14F"), ui.Hex("#C18401"),
		ui.Hex("#4078F2"), ui.Hex("#A626A4"), ui.Hex("#0184BC"), ui.Hex("#696C77"),
		ui.Hex("#4F525E"), ui.Hex("#E45649"), ui.Hex("#50A14F"), ui.Hex("#C18401"),
		ui.Hex("#4078F2"), ui.Hex("#A626A4"), ui.Hex("#0184BC"), ui.Hex("#1F2328"),
	}
)

// ansiStyle is the terminal state one line's SGR sequences have set.
type ansiStyle struct {
	fg    ui.Color
	fgSet bool
	// fgIndex is the standard color's index, or -1 for an extended color and
	// for the terminal's default. Bold brightens a standard foreground.
	fgIndex   int
	bg        ui.Color
	bgSet     bool
	bold      bool
	dim       bool
	italic    bool
	underline bool
	strike    bool
	reverse   bool
}

// ansiSpans turns one line of terminal output into styled spans. Escape
// sequences that only move the cursor or clear the screen are dropped; SGR
// colors, weights and decorations are kept.
func ansiSpans(line string, t *ui.Theme) []ui.Span {
	if !strings.ContainsRune(line, 0x1b) {
		return []ui.Span{{Text: line}}
	}

	style := ansiStyle{fgIndex: -1}
	spans := make([]ui.Span, 0, 2)
	var text strings.Builder
	flush := func() {
		if text.Len() == 0 {
			return
		}
		spans = append(spans, style.span(text.String(), t))
		text.Reset()
	}

	for i := 0; i < len(line); {
		if line[i] != 0x1b {
			text.WriteByte(line[i])
			i++
			continue
		}
		flush()
		next, sgr, ok := ansiEscape(line, i)
		if ok {
			style = applySGR(style, sgr, t)
		}
		i = next
	}
	flush()

	if len(spans) == 0 {
		// A line that held only escapes still occupies a row.
		return []ui.Span{{Text: ""}}
	}
	return spans
}

// ansiEscape reads the escape sequence at i and returns where it ends, plus the
// parameters of an SGR sequence.
func ansiEscape(line string, i int) (next int, sgr string, ok bool) {
	if i+1 >= len(line) {
		return len(line), "", false
	}
	switch line[i+1] {
	case '[':
		j := i + 2
		for j < len(line) && !(line[j] >= 0x40 && line[j] <= 0x7e) {
			j++
		}
		if j >= len(line) {
			return len(line), "", false
		}
		if line[j] == 'm' {
			return j + 1, line[i+2 : j], true
		}
		return j + 1, "", false
	case ']':
		// An operating-system command, such as a hyperlink; ends with BEL or
		// ESC \.
		j := i + 2
		for j < len(line) {
			if line[j] == 0x07 {
				return j + 1, "", false
			}
			if line[j] == 0x1b && j+1 < len(line) && line[j+1] == '\\' {
				return j + 2, "", false
			}
			j++
		}
		return len(line), "", false
	default:
		// A two-byte escape such as ESC ( B skips its intermediate bytes and
		// one final rune; a final UTF-8 rune must be consumed whole.
		j := i + 1
		for j < len(line) && line[j] >= 0x20 && line[j] <= 0x2f {
			j++
		}
		if j < len(line) {
			_, size := utf8.DecodeRuneInString(line[j:])
			j += size
		}
		return j, "", false
	}
}

func applySGR(style ansiStyle, params string, t *ui.Theme) ansiStyle {
	palette := ansiPalette(t)
	fields := strings.Split(params, ";")
	if len(fields) == 1 && fields[0] == "" {
		fields[0] = "0"
	}
	values := make([]int, len(fields))
	for i, field := range fields {
		// Empty parameters mean 0, as a terminal reads them.
		if field == "" {
			values[i] = 0
			continue
		}
		n, err := strconv.Atoi(field)
		if err != nil {
			values[i] = 0
			continue
		}
		values[i] = n
	}

	reset := func() { style = ansiStyle{fgIndex: -1} }

	for i := 0; i < len(values); i++ {
		switch n := values[i]; {
		case n == 0:
			reset()
		case n == 1:
			style.bold = true
		case n == 2:
			style.dim = true
		case n == 3:
			style.italic = true
		case n == 4:
			style.underline = true
		case n == 7:
			style.reverse = true
		case n == 9:
			style.strike = true
		case n == 22:
			style.bold, style.dim = false, false
		case n == 23:
			style.italic = false
		case n == 24:
			style.underline = false
		case n == 27:
			style.reverse = false
		case n == 29:
			style.strike = false
		case n >= 30 && n <= 37:
			style.fg, style.fgSet, style.fgIndex = palette[n-30], true, n-30
		case n == 39:
			style.fgSet, style.fgIndex = false, -1
		case n >= 40 && n <= 47:
			style.bg, style.bgSet = palette[n-40], true
		case n == 49:
			style.bgSet = false
		case n >= 90 && n <= 97:
			style.fg, style.fgSet, style.fgIndex = palette[n-90+8], true, n-90+8
		case n >= 100 && n <= 107:
			style.bg, style.bgSet = palette[n-100+8], true
		case n == 38 || n == 48:
			color, next, ok := extendedColor(values, i, t)
			if !ok {
				continue
			}
			if n == 38 {
				style.fg, style.fgSet, style.fgIndex = color, true, -1
			} else {
				style.bg, style.bgSet = color, true
			}
			i = next
		}
	}
	return style
}

// extendedColor reads the 38/48 form: 5;n for the 256-color palette, or
// 2;r;g;b for direct color. It returns the index of the last parameter it used.
func extendedColor(values []int, at int, t *ui.Theme) (ui.Color, int, bool) {
	if at+1 >= len(values) {
		return ui.Color{}, at, false
	}
	switch values[at+1] {
	case 5:
		if at+2 >= len(values) {
			return ui.Color{}, at, false
		}
		return ansi256(values[at+2], t), at + 2, true
	case 2:
		if at+4 >= len(values) {
			return ui.Color{}, at, false
		}
		return ui.RGBA(clampByte(values[at+2]), clampByte(values[at+3]), clampByte(values[at+4]), 1), at + 4, true
	default:
		return ui.Color{}, at, false
	}
}

func ansiPalette(t *ui.Theme) *[16]ui.Color {
	if t.Dark {
		return &ansiDark
	}
	return &ansiLight
}

func ansi256(n int, t *ui.Theme) ui.Color {
	if n < 0 {
		n = 0
	}
	if n < 16 {
		return ansiPalette(t)[n]
	}
	if n < 232 {
		n -= 16
		levels := [6]uint8{0, 95, 135, 175, 215, 255}
		return ui.RGB(levels[n/36], levels[(n/6)%6], levels[n%6])
	}
	if n > 255 {
		n = 255
	}
	g := uint8(8 + (n-232)*10)
	return ui.RGB(g, g, g)
}

func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func (s ansiStyle) span(text string, t *ui.Theme) ui.Span {
	span := ui.Span{Text: text}

	fg, fgSet := s.fg, s.fgSet
	if s.bold && s.fgIndex >= 0 && s.fgIndex < 8 {
		// Terminals brighten a standard color when it is bold.
		fg, fgSet = ansiPalette(t)[s.fgIndex+8], true
	}
	if s.reverse {
		// Swap what a terminal would draw: the text color becomes the
		// background and the console's background becomes the text.
		foreground := t.Text
		if fgSet {
			foreground = fg
		}
		background := t.Background
		if s.bgSet {
			background = s.bg
		}
		span.Color, span.Background = background, foreground
	} else {
		if fgSet {
			span.Color = fg
		}
		if s.bgSet {
			span.Background = s.bg
		}
	}
	if s.dim {
		if span.Color.A == 0 {
			span.Color = t.TextMuted
		}
		span.Color = span.Color.Alpha(0.7)
	}
	if s.bold {
		span.Weight = 600
	}
	if s.italic {
		span.Italic = true
	}
	if s.underline {
		span.Underline = true
	}
	if s.strike {
		span.Strikethrough = true
	}
	return span
}
