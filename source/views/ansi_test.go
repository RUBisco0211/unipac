package views

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func palette() *ui.Theme { return theme.Palette(false) }

func TestANSIPlainLineInherits(t *testing.T) {
	spans := ansiSpans("plain text", palette())
	if len(spans) != 1 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Text != "plain text" {
		t.Fatalf("text = %q", spans[0].Text)
	}
	if spans[0].Color.A != 0 {
		t.Errorf("plain text set a color: %+v", spans[0])
	}
}

func TestANSIColorAndReset(t *testing.T) {
	spans := ansiSpans("\x1b[31mred\x1b[0m plain", palette())
	if len(spans) != 2 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Text != "red" || spans[0].Color != ansiLight[1] {
		t.Errorf("red span = %+v", spans[0])
	}
	if spans[1].Text != " plain" || spans[1].Color.A != 0 {
		t.Errorf("reset span = %+v", spans[1])
	}
}

// Bold brightens a standard foreground, as a terminal does.
func TestANSIBoldBrightensStandardColor(t *testing.T) {
	spans := ansiSpans("\x1b[1;31mbold red\x1b[0m", palette())
	if len(spans) != 1 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Color != ansiLight[9] || spans[0].Weight != 600 {
		t.Errorf("bold span = %+v, want bright red and weight 600", spans[0])
	}
}

func TestANSExtendedColors(t *testing.T) {
	spans := ansiSpans("\x1b[38;5;196mred\x1b[0m", palette())
	if len(spans) != 1 || spans[0].Color != ui.RGB(255, 0, 0) {
		t.Errorf("256-color span = %+v", spans[0])
	}

	spans = ansiSpans("\x1b[38;2;1;2;3mtrue\x1b[0m", palette())
	if len(spans) != 1 || spans[0].Color != ui.RGB(1, 2, 3) {
		t.Errorf("truecolor span = %+v", spans[0])
	}
}

func TestANSIStripsNonColorSequences(t *testing.T) {
	spans := ansiSpans("a\x1b[2Jb\x1b]8;;https://example.com\x07c", palette())
	text := ""
	for _, span := range spans {
		text += span.Text
	}
	if text != "abc" {
		t.Fatalf("text = %q, want the visible characters only", text)
	}
}

// Two-byte escapes may carry an intermediate byte and a final rune, and the
// final rune can be multi-byte UTF-8.
func TestANSIStripsTwoByteEscapes(t *testing.T) {
	spans := ansiSpans("a\x1b(Bb\x1b(中c", palette())
	text := ""
	for _, span := range spans {
		text += span.Text
	}
	if text != "abc" {
		t.Fatalf("text = %q, want the visible characters only", text)
	}
}

func TestANSIBackgroundAndUnderline(t *testing.T) {
	spans := ansiSpans("\x1b[44;4mselected\x1b[0m", palette())
	if len(spans) != 1 {
		t.Fatalf("spans = %+v", spans)
	}
	if spans[0].Background != ansiLight[4] || !spans[0].Underline {
		t.Errorf("span = %+v, want blue background and underline", spans[0])
	}
}
