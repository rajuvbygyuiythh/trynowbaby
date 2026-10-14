package proxy

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testExitIPService starts a local TLS echo service that mimics the common
// public "what is my IP" APIs. It is only used to prove the lookup path.
func testExitIPService(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("exit-IP lookups must use GET, got %s", r.Method)
		}
		if r.Header.Get("Proxy-Authorization") != "" {
			t.Error("proxy credentials leaked to the exit-IP service")
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}

func poolWith(certificates ...*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, certificate := range certificates {
		pool.AddCert(certificate)
	}
	return pool
}

func TestExitIPLookupThroughProxy(t *testing.T) {
	for name, body := range map[string]string{
		"json-ip-and-country": `{"ip":"203.0.113.7","country":"SG"}`,
		"json-query-field":    `{"query":"203.0.113.8","country_code":"DE"}`,
		"plain-text":          "198.51.100.9\n",
	} {
		t.Run(name, func(t *testing.T) {
			entry := testHTTPProxy(t, true, 0)
			service := testExitIPService(t, 200, body)
			roots := poolWith(service.Certificate())
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ip, country, detail := entry.exitIP(ctx, service.URL, roots)
			if detail != "" {
				t.Fatalf("unexpected lookup failure: %q", detail)
			}
			if !strings.HasPrefix(ip, "203.0.113.") && ip != "198.51.100.9" {
				t.Fatalf("exit IP not parsed: %q", ip)
			}
			if !strings.HasPrefix(name, "plain-text") && country == "" {
				t.Fatalf("country not parsed for %s", name)
			}
		})
	}
}

func TestExitIPFailureAndLatencyAreReported(t *testing.T) {
	entry := testHTTPProxy(t, true, 0)
	service := testExitIPService(t, 500, "boom")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ip, _, detail := entry.exitIP(ctx, service.URL, poolWith(service.Certificate()))
	if ip != "" || !strings.Contains(detail, "500") {
		t.Fatalf("HTTP 500 from the echo service must be reported: %q %q", ip, detail)
	}
	offline := Entry{Scheme: "http", Host: "127.0.0.1", Port: "1"}
	if ip, _, detail := offline.exitIP(ctx, "https://app.example.invalid/", nil); ip != "" || detail == "" {
		t.Fatalf("unreachable proxy must explain the lookup failure: %q", detail)
	}
}

func TestExitIPIsOptInAndKeepsVerdict(t *testing.T) {
	entry := testHTTPProxy(t, true, 0)
	target, targetRoots := testTarget(t, 200)
	service := testExitIPService(t, 200, `{"ip":"203.0.113.9","country":"US"}`)
	roots := poolWith(target.Certificate(), service.Certificate())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	plain := testEndpointFull(ctx, entry, target.URL, targetRoots, TestOptions{})
	if plain.ExitIP != "" || plain.ExitIPError != "" {
		t.Fatalf("no exit-IP request may happen unless asked: %+v", plain)
	}

	checked := testEndpointFull(ctx, entry, target.URL, roots, TestOptions{CheckExitIP: true, ExitIPURL: service.URL})
	if !checked.OK || !checked.TunnelOK || checked.ExitIP != "203.0.113.9" || checked.ExitCountry != "US" {
		t.Fatalf("exit-IP lookup must fill the result and keep the verdict: %+v", checked)
	}

	unreachable := testEndpointFull(ctx, entry, target.URL, targetRoots, TestOptions{CheckExitIP: true, ExitIPURL: "https://does-not-resolve.invalid/"})
	if !unreachable.OK || unreachable.TunnelOK != true || unreachable.ExitIP != "" || unreachable.ExitIPError == "" {
		t.Fatalf("an unreachable echo service must not change the target verdict: %+v", unreachable)
	}
}

func TestParseExitIPBody(t *testing.T) {
	cases := map[string][2]string{
		`{"ip":"1.2.3.4"}`:                        {"1.2.3.4", ""},
		`{"ip":"1.2.3.4","country_name":"Japan"}`: {"1.2.3.4", "Japan"},
		"  8.8.8.8  ":                             {"8.8.8.8", ""},
		`ip=9.9.9.9`:                              {"9.9.9.9", ""},
		`{"error":"no such service"}`:             {"", ""},
		"":                                        {"", ""},
	}
	for body, want := range cases {
		ip, country := parseExitIPBody([]byte(body))
		if ip != want[0] || country != want[1] {
			t.Fatalf("parse %q => %q/%q, want %q/%q", body, ip, country, want[0], want[1])
		}
	}
}

func TestTestListWithOptionsValidatesInputs(t *testing.T) {
	ctx := context.Background()
	if _, err := TestListWithOptions(ctx, "host:80", TestOptions{Endpoint: "ftp://host:21"}); err == nil {
		t.Fatal("non-http endpoint accepted")
	}
	if _, err := TestListWithOptions(ctx, "host:80", TestOptions{Endpoint: "https://host/", CheckExitIP: true, ExitIPURL: "not-a-url"}); err == nil {
		t.Fatal("invalid exit-IP endpoint accepted")
	}
	if _, err := TestListWithOptions(ctx, "", TestOptions{Endpoint: "https://host/"}); err == nil {
		t.Fatal("empty list accepted")
	}
	if _, err := TestListWithOptions(ctx, "host:80", TestOptions{Endpoint: "https://host/"}); err != nil {
		t.Fatalf("default limit/endpoint must apply: %v", err)
	}
	// A caller that passes no endpoint still gets the historical default.
	results, err := TestList(ctx, "invalid", "https://does-not-resolve.invalid/")
	if err != nil || len(results) != 1 || results[0].Detail == "" {
		t.Fatalf("TestList wrapper changed: %v %v", results, err)
	}
}
