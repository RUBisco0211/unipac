package util

import (
	"slices"
	"strings"
	"testing"
)

// A configured proxy reaches every name the managers' tools read, and the
// caller's own variables come along.
func TestProxyEnvs(t *testing.T) {
	for _, name := range []string{
		"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY",
		"ALL_PROXY", "all_proxy", "npm_config_proxy", "npm_config_https_proxy",
		"CARGO_HTTP_PROXY",
	} {
		if !slices.Contains(proxyEnvs("http://127.0.0.1:7890"), name+"=http://127.0.0.1:7890") {
			t.Errorf("%s is not set", name)
		}
	}
	if got := proxyEnvs(""); got != nil {
		t.Errorf("no proxy set %v, want none", got)
	}
}

// An entry the command replaces is not left behind: a duplicated variable
// resolves to the first of the two, so the configured proxy would go unread.
func TestEnvironReplacesRatherThanDuplicates(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://inherited:1")
	t.Setenv("KEEP_ME", "inherited")
	SetProxy("http://configured:2")
	t.Cleanup(func() { SetProxy("") })

	env := environ([]string{"FORCE_COLOR=1"})
	count := func(name string) int {
		n := 0
		for _, kv := range env {
			if strings.HasPrefix(kv, name+"=") {
				n++
			}
		}
		return n
	}
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "CARGO_HTTP_PROXY"} {
		if got := count(name); got != 1 {
			t.Errorf("%s appears %d times, want once", name, got)
		}
	}
	if !slices.Contains(env, "HTTP_PROXY=http://configured:2") {
		t.Error("the configured proxy is not what the command gets")
	}
	if count("KEEP_ME") != 1 {
		t.Error("a variable the proxy does not name was dropped")
	}
	if !slices.Contains(env, "FORCE_COLOR=1") {
		t.Error("the call's own variable was dropped")
	}
}

// With no proxy configured the environment is the app's own, untouched.
func TestEnvironWithoutAProxyKeepsTheEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://inherited:1")
	SetProxy("")
	env := environ(nil)
	if !slices.Contains(env, "HTTP_PROXY=http://inherited:1") {
		t.Error("the inherited proxy was dropped")
	}
}
