package util

import "testing"

func TestStripANSI(t *testing.T) {
	in := "\x1b[31mError:\x1b[0m package not found\n\x1b[1;33mretry\x1b[0m"
	want := "Error: package not found\nretry"
	if got := StripANSI(in); got != want {
		t.Fatalf("StripANSI = %q, want %q", got, want)
	}
}

func TestStripANSIRemovesCSIAndOSC(t *testing.T) {
	in := "a\x1b[2Kb\x1b]8;;https://example.com\x1b\\c\x1b(Bd"
	if got := StripANSI(in); got != "abcd" {
		t.Fatalf("StripANSI = %q, want abcd", got)
	}
}

func TestStripANSILeavesPlainTextAlone(t *testing.T) {
	const text = "plain output"
	if got := StripANSI(text); got != text {
		t.Fatalf("StripANSI = %q, want %q", got, text)
	}
}
