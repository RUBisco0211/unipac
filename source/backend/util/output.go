package util

import (
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// EventKind separates the start of a command from a line it prints.
type EventKind uint8

const (
	// EventCommand reports a command that is about to run.
	EventCommand EventKind = iota
	// EventOutput reports one line the command printed.
	EventOutput
)

// Event is something a running command reports to its action.
type Event struct {
	Kind EventKind
	// Command and Path describe the command (EventCommand).
	Command string
	Path    string
	// Text is one line of output (EventOutput).
	Text string
}

// Reporter receives a command's events while it runs.
type Reporter func(Event)

type reporterKey struct{}

// WithReporter returns a context whose commands report their invocation and
// output to fn. The command still collects its whole output in ExecResult, so a
// caller that only needs the final text can ignore the events.
func WithReporter(ctx context.Context, fn Reporter) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, reporterKey{}, fn)
}

func reporterFrom(ctx context.Context) Reporter {
	if ctx == nil {
		return nil
	}
	fn, _ := ctx.Value(reporterKey{}).(Reporter)
	return fn
}

// ansiSGR matches the color and style sequences commands add to stderr when
// their output is not a terminal. Stripping only SGR leaves other terminal
// controls alone, which plain text does not normally carry.
var ansiSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes SGR color sequences from text shown outside the console.
func StripANSI(s string) string {
	if !strings.ContainsRune(s, 0x1b) {
		return s
	}
	return ansiSGR.ReplaceAllString(s, "")
}

// FormatCommand renders a command line the way a console would show it,
// quoting arguments that contain shell-significant characters.
func FormatCommand(bin string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteCommandPart(bin))
	for _, arg := range args {
		parts = append(parts, quoteCommandPart(arg))
	}
	return strings.Join(parts, " ")
}

func quoteCommandPart(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\n\"'\\$`!&|;<>()*?[]{}") {
		return s
	}
	return strconv.Quote(s)
}

// maxOutputLine bounds one reported line, so a command that prints a huge
// document does not grow the console state without limit.
const maxOutputLine = 8 << 10

// lineWriter splits a command's writes into lines and reports them through fn.
// Its lock is shared between the stdout and stderr writers, so their lines
// never interleave inside a line.
type lineWriter struct {
	mu  *sync.Mutex
	fn  Reporter
	buf []byte
}

func newLineWriter(mu *sync.Mutex, fn Reporter) *lineWriter {
	return &lineWriter{mu: mu, fn: fn}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.emit(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
	// A command that never ends its last line would otherwise hold it until it
	// exits; report the part that is there once it grows past the bound.
	if len(w.buf) > maxOutputLine {
		w.emit(string(w.buf))
		w.buf = nil
	}
	return len(p), nil
}

// flush reports what is left of an unterminated line after the command exits.
func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.emit(string(w.buf))
		w.buf = nil
	}
}

func (w *lineWriter) emit(line string) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return
	}
	if len(line) > maxOutputLine {
		line = line[:maxOutputLine] + "…"
	}
	w.fn(Event{Kind: EventOutput, Text: line})
}
