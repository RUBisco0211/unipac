package util

import (
	"context"
	"runtime"
	"strconv"
	"strings"
)

// SystemProxy reports the proxy the desktop is configured with, such as
// "http://127.0.0.1:7890", and "" when the system has none or this build cannot
// read them. macOS keeps it in System Settings rather than in the environment an
// app is started with, which is why a command can find no proxy even though the
// user set one; Windows and Linux pass theirs in the environment, which the
// commands already inherit, so this answers nothing there.
func SystemProxy(ctx context.Context) string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	out, err := Exec(ctx, "/usr/sbin/scutil", []string{"--proxy"}, nil)
	if err != nil {
		return ""
	}
	return parseScutilProxy(out)
}

// parseScutilProxy reads the dictionary `scutil --proxy` prints: an HTTPS proxy
// when it is enabled, the HTTP one otherwise, and "" when neither is.
func parseScutilProxy(out string) string {
	values := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), " : ")
		if !ok {
			continue
		}
		values[name] = strings.TrimSpace(value)
	}
	for _, kind := range []string{"HTTPS", "HTTP", "SOCKS"} {
		if values[kind+"Enable"] != "1" {
			continue
		}
		host := values[kind+"Proxy"]
		port, ok := proxyPort(values[kind+"Port"])
		if host == "" || !ok {
			continue
		}
		return proxyScheme(kind) + "://" + host + ":" + strconv.Itoa(port)
	}
	return ""
}

// proxyScheme names the scheme to reach a proxy scutil calls kind. Its HTTP and
// HTTPS entries both name a plain HTTP proxy: the word says which traffic goes
// through it, not how it is spoken to — a system proxy for HTTPS is reached over
// HTTP and tunnels with CONNECT. Only its SOCKS entry is a SOCKS proxy, and a
// SOCKS5 one, which is what http.Client dials.
func proxyScheme(kind string) string {
	if kind == "SOCKS" {
		return "socks5"
	}
	return "http"
}

// proxyPort is a port that scutil prints as a number: it is kept in its own
// function so the parse reads as the shape of the dictionary.
func proxyPort(value string) (int, bool) {
	n, err := strconv.Atoi(value)
	return n, err == nil && n > 0
}
