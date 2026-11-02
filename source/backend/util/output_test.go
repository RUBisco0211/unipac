package util

import "testing"

func TestStripANSI(t *testing.T) {
	in := "\x1b[31mError:\x1b[0m package not found\n\x1b[1;33mretry\x1b[0m"
	want := "Error: package not found\nretry"
	if got := StripANSI(in); got != want {
		t.Fatalf("StripANSI = %q, want %q", got, want)
	}
}

func TestStripANSILeavesPlainTextAlone(t *testing.T) {
	const text = "plain output"
	if got := StripANSI(text); got != text {
		t.Fatalf("StripANSI = %q, want %q", got, text)
	}
}
