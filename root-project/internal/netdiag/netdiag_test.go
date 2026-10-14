package netdiag

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDirectReachableHTTPServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := Direct(ctx, server.URL)
	if !result.OK || result.Status != 200 {
		t.Fatalf("plain HTTP server must be reachable: %+v", result)
	}
	if len(result.DNSIPs) == 0 || result.Host == "" {
		t.Fatalf("dns resolution not reported: %+v", result)
	}
	if !strings.Contains(result.Summarize(), "PASS") {
		t.Fatalf("summary must show PASS: %s", result.Summarize())
	}
}

func TestDirectHeadFallbackToGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	result := Direct(context.Background(), server.URL)
	if !result.OK || result.Status != 200 {
		t.Fatalf("HEAD refusal must fall back to GET: %+v", result)
	}
}

func TestDirectFailuresAreAttributed(t *testing.T) {
	t.Run("invalid-url", func(t *testing.T) {
		result := Direct(context.Background(), "not a url")
		if result.OK || !strings.Contains(result.Detail, "invalid URL") || result.Hint == "" {
			t.Fatalf("invalid URL must be explained: %+v", result)
		}
	})
	t.Run("dns", func(t *testing.T) {
		result := Direct(context.Background(), "https://does-not-resolve.invalid/")
		if result.OK || !strings.Contains(result.Detail, "DNS") {
			t.Fatalf("DNS failure must be attributed: %+v", result)
		}
	})
	t.Run("tcp", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		result := Direct(context.Background(), "http://127.0.0.1:"+itoa(port)+"/")
		if result.OK || !strings.Contains(result.Detail, "TCP connect failed") {
			t.Fatalf("closed port must be attributed: %+v", result)
		}
	})
	t.Run("tls", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		result := Direct(context.Background(), server.URL)
		if result.OK || !strings.Contains(result.Detail, "TLS") || !strings.Contains(result.Hint, "certificate") {
			t.Fatalf("self-signed TLS must fail with an explanation: %+v", result)
		}
	})
	t.Run("empty-uses-default", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result := Direct(ctx, "")
		if result.URL != "https://app.chitchat.gg/" {
			t.Fatalf("default target missing: %+v", result)
		}
	})
}

func TestListenersAndSummarize(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if !Listeners(context.Background(), port) {
		t.Fatal("open listener not detected")
	}
	if Listeners(context.Background(), 0) || Listeners(context.Background(), 70000) {
		t.Fatal("invalid ports must report false")
	}
	summary := DirectResult{Host: "example.com:443", Scheme: "https", Status: 200, OK: true, Detail: "ok"}.Summarize()
	if !strings.Contains(summary, "PASS") || !strings.Contains(summary, "HTTP 200") {
		t.Fatalf("summary wrong: %s", summary)
	}
	if !strings.Contains(DirectResult{Detail: "boom"}.Summarize(), "FAIL") {
		t.Fatal("failure summary missing")
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
