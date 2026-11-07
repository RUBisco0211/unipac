package util

import "testing"

// The dictionary scutil prints, as this machine answers it.
const scutilSample = `<dictionary> {
  ExceptionsList : <array> {
    0 : *zhihu.com
    1 : localhost
  }
  FTPPassive : 1
  HTTPEnable : 1
  HTTPPort : 7890
  HTTPProxy : 127.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 127.0.0.1
  SOCKSEnable : 0
  SOCKSPort : 0
  SOCKSProxy : 
  ProxyAutoConfigEnable : 0
  ProxyAutoDiscoveryEnable : 0
}`

func TestParseScutilProxy(t *testing.T) {
	// The HTTPS entry names a plain HTTP proxy: the word says which traffic goes
	// through it, not how it is spoken to.
	if got, want := parseScutilProxy(scutilSample), "http://127.0.0.1:7890"; got != want {
		t.Errorf("proxy = %q, want %q", got, want)
	}
}

// Nothing enabled, or nothing to read, is no proxy rather than a wrong one.
func TestParseScutilProxyWithoutOne(t *testing.T) {
	off := `<dictionary> {
  HTTPEnable : 0
  HTTPSEnable : 0
  SOCKSEnable : 0
}`
	for _, out := range []string{"", off, "<dictionary> {\n  HTTPEnable : 1\n  HTTPProxy : 127.0.0.1\n}"} {
		if got := parseScutilProxy(out); got != "" {
			t.Errorf("parse(%q) = %q, want none", out, got)
		}
	}
}

// A SOCKS proxy is the answer when it is the only one on.
func TestParseScutilProxySocks(t *testing.T) {
	socks := `<dictionary> {
  HTTPEnable : 0
  HTTPSEnable : 0
  SOCKSEnable : 1
  SOCKSPort : 1080
  SOCKSProxy : 127.0.0.1
}`
	if got, want := parseScutilProxy(socks), "socks5://127.0.0.1:1080"; got != want {
		t.Errorf("proxy = %q, want %q", got, want)
	}
}

// An HTTP-only proxy is reached the same way, which is what a reader with a
// plain HTTP proxy needs.
func TestParseScutilProxyHTTPOnly(t *testing.T) {
	http := `<dictionary> {
  HTTPEnable : 1
  HTTPPort : 8080
  HTTPProxy : proxy.local
  HTTPSEnable : 0
}`
	if got, want := parseScutilProxy(http), "http://proxy.local:8080"; got != want {
		t.Errorf("proxy = %q, want %q", got, want)
	}
}

// A system proxy for HTTPS is a plain HTTP proxy: the word names the traffic that
// goes through it, not how the proxy is spoken to. Asking Go to reach it over TLS
// fails with "proxyconnect tcp: EOF", which is what every registry request did
// while the value carried https://.
func TestSystemProxyIsHTTPForHTTPSTraffic(t *testing.T) {
	got := parseScutilProxy(scutilSample)
	if len(got) < 7 || got[:7] != "http://" {
		t.Fatalf("proxy = %q, want an http:// proxy", got)
	}
}
