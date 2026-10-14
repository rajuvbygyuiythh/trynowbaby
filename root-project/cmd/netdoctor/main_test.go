package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// syntheticProxy is a local, credential-checking HTTP proxy fixture. All
// endpoints in these tests are local; nothing leaves the machine.
func syntheticProxy(t *testing.T, username, password string) string {
	t.Helper()
	expected := ""
	if username != "" || password != "" {
		expected = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if expected != "" && r.Header.Get("Proxy-Authorization") != expected {
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		if r.Method != http.MethodConnect {
			clone := r.Clone(r.Context())
			clone.RequestURI = ""
			clone.Header.Del("Proxy-Authorization")
			transport := &http.Transport{}
			defer transport.CloseIdleConnections()
			response, err := transport.RoundTrip(clone)
			if err != nil {
				http.Error(w, "forward failed", http.StatusBadGateway)
				return
			}
			defer response.Body.Close()
			w.WriteHeader(response.StatusCode)
			io.Copy(w, response.Body)
			return
		}
		remote, err := net.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, "target failed", http.StatusBadGateway)
			return
		}
		client, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			remote.Close()
			return
		}
		defer client.Close()
		defer remote.Close()
		buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		buffer.Flush()
		done := make(chan struct{})
		go func() { io.Copy(remote, buffer); remote.Close(); close(done) }()
		io.Copy(client, remote)
		client.Close()
		remote.Close()
		<-done
	}))
	t.Cleanup(server.Close)
	parsed, _ := url.Parse(server.URL)
	entry := parsed.Host
	if expected != "" {
		entry = username + ":" + password + "@" + parsed.Host
	}
	return entry
}

func TestRunReportsPassAndFailWithReasons(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	good := syntheticProxy(t, "u", "p")
	bad := "127.0.0.1:1"

	file := filepath.Join(t.TempDir(), "proxies.txt")
	// Line 1: a dead endpoint. Line 2: the working, credential-carrying proxy.
	content := bad + "\n" + good + "\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	var out bytes.Buffer
	code := run([]string{
		"-proxies", file,
		"-targets", target.URL,
		"-json", report,
		"-timeout", "30",
	}, &out)
	text := out.String()
	if code != 1 {
		t.Fatalf("one failing line must exit 1, got %d\n%s", code, text)
	}
	if !strings.Contains(text, "PASS") || !strings.Contains(text, "FAIL") {
		t.Fatalf("report must mark both outcomes:\n%s", text)
	}
	if !strings.Contains(text, "Verdict") || !strings.Contains(text, "1/2 proxies passed") {
		t.Fatalf("verdict missing:\n%s", text)
	}
	if !strings.Contains(text, "direct  PASS") {
		t.Fatalf("direct baseline missing:\n%s", text)
	}
	payload, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), target.URL) || !strings.Contains(string(payload), "\"proxies\"") {
		t.Fatalf("json report incomplete:\n%s", payload)
	}
}

func TestRunWithoutCredentialsExplainsTheFix(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	withCredentials := syntheticProxy(t, "u", "p")
	proxyHostPort := withCredentials[strings.Index(withCredentials, "@")+1:]
	var out bytes.Buffer
	code := run([]string{"-proxy", proxyHostPort, "-targets", target.URL, "-direct=false", "-timeout", "30"}, &out)
	text := out.String()
	if code != 1 {
		t.Fatalf("rejected credentials must exit 1, got %d\n%s", code, text)
	}
	if !strings.Contains(text, "HTTP 407") {
		t.Fatalf("provider status missing:\n%s", text)
	}
	if !strings.Contains(text, "no username:password") {
		t.Fatalf("actionable hint missing:\n%s", text)
	}
}

func TestRunDirectOnlyAndFlagValidation(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer target.Close()
	var out bytes.Buffer
	if code := run([]string{"-targets", target.URL, "-timeout", "20"}, &out); code != 0 {
		t.Fatalf("direct-only success must exit 0, got %d\n%s", code, out.String())
	}
	out.Reset()
	if code := run([]string{"-targets", target.URL, "-timeout", "20"}, &out); code != 0 {
		t.Fatalf("unexpected failure: %s", out.String())
	}
	out.Reset()
	if code := run([]string{"-targets", target.URL, "-timeout", "20", "-direct=false"}, &out); code != 0 {
		t.Fatalf("direct-only run without proxies must stay clean: %s", out.String())
	}
	out.Reset()
	if code := run([]string{"-targets", ""}, &out); code != 2 {
		t.Fatalf("empty target list must be a usage error, got %d", code)
	}
	out.Reset()
	if code := run([]string{"-targets", target.URL, "-timeout", "0"}, &out); code != 2 {
		t.Fatalf("bad timeout must be a usage error, got %d", code)
	}
	out.Reset()
	if code := run([]string{"-targets", target.URL, "-type", "ftp"}, &out); code != 2 {
		t.Fatalf("bad protocol must be a usage error, got %d", code)
	}
	out.Reset()
	if code := run([]string{"-proxies", filepath.Join(t.TempDir(), "missing.txt"), "-targets", target.URL}, &out); code != 2 {
		t.Fatalf("missing proxy file must be a usage error, got %d", code)
	}
}

func TestApplyDefaultSchemeMirrorsDashboard(t *testing.T) {
	raw := "host:80\nsocks5://host2:1080\nhost3:80:u:p\n\n"
	withSOCKS := applyDefaultScheme(raw, "socks5")
	if !strings.Contains(withSOCKS, "socks5://host:80") || !strings.Contains(withSOCKS, "socks5://host3:80:u:p") {
		t.Fatalf("plain lines must inherit the default protocol:\n%s", withSOCKS)
	}
	if strings.Count(withSOCKS, "socks5://") != 3 {
		t.Fatalf("explicit prefix must not be doubled:\n%s", withSOCKS)
	}
	if got := applyDefaultScheme(raw, "http"); got != raw {
		t.Fatalf("http default must leave lines untouched:\n%s", got)
	}
}

func TestReadProxyLinesMergesFileAndFlags(t *testing.T) {
	file := filepath.Join(t.TempDir(), "p.txt")
	os.WriteFile(file, []byte("a:1\r\nb:2\n"), 0o600)
	raw, err := readProxyLines(config{proxies: file, proxyFile: "c:3, d:4"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"a:1", "b:2", "c:3", "d:4"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("missing %s in %q", want, raw)
		}
	}
	if _, err := readProxyLines(config{proxies: filepath.Join(t.TempDir(), "nope.txt")}); err == nil {
		t.Fatal("missing file must error")
	}
	_ = bufio.NewReader
}
