package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const MaxTestEntries = 20

type TestResult struct {
	Line      int    `json:"line"`
	Label     string `json:"label"`
	OK        bool   `json:"ok"`
	TunnelOK  bool   `json:"tunnel_ok"`
	Status    int    `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	Detail    string `json:"detail"`
}

// TestList never saves settings, rotates worker proxies or opens an account.
// Only an HTTPS HEAD request to the supplied fixed public endpoint is made.
func TestList(ctx context.Context, raw, endpoint string) ([]TestResult, error) {
	if len(raw) > 64<<10 {
		return nil, fmt.Errorf("proxy list is too large (maximum 64 KiB)")
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")
	results := make([]TestResult, 0)
	entries := make([]Entry, 0)
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(results) >= MaxTestEntries {
			return nil, fmt.Errorf("test at most %d proxy lines at a time", MaxTestEntries)
		}
		entry, err := ParseLine(line)
		result := TestResult{Line: index + 1}
		if err != nil {
			result.Detail = err.Error()
		} else {
			result.Label = entry.Label()
		}
		results = append(results, result)
		entries = append(entries, entry)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("enter at least one proxy to test")
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if results[i].Detail != "" {
					continue
				}
				c, cancel := context.WithTimeout(ctx, 12*time.Second)
				tested := TestEndpoint(c, entries[i], endpoint)
				cancel()
				tested.Line = results[i].Line
				results[i] = tested
			}
		}()
	}
	for i := range results {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results, nil
}
func TestEndpoint(ctx context.Context, entry Entry, endpoint string) TestResult {
	return testEndpoint(ctx, entry, endpoint, nil)
}

// Only local unit tests provide additional roots; dashboard uses system trust.
func testEndpoint(ctx context.Context, entry Entry, endpoint string, roots *x509.CertPool) (result TestResult) {
	result.Label = entry.Label()
	start := time.Now()
	defer func() { result.LatencyMS = time.Since(start).Milliseconds() }()
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		result.Detail = "invalid HTTPS test endpoint"
		return
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	conn, err := entry.DialTunnel(ctx, net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		result.Detail = err.Error()
		return
	}
	defer conn.Close()
	secure := tls.Client(conn, &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12, RootCAs: roots})
	if err := secure.HandshakeContext(ctx); err != nil {
		result.Detail = "proxy tunnel opened, but destination TLS verification/handshake failed"
		return
	}
	result.TunnelOK = true
	deadline := time.Now().Add(12 * time.Second)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	secure.SetDeadline(deadline)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		result.Detail = "invalid test request"
		return
	}
	req.Header.Set("User-Agent", "ChitChat-Proxy-Test/1.0")
	req.Close = true
	if err := req.Write(secure); err != nil {
		result.Detail = "tunnel/TLS succeeded; test request failed"
		return
	}
	response, err := http.ReadResponse(bufio.NewReader(io.LimitReader(secure, 64<<10)), req)
	if err != nil {
		result.Detail = "tunnel/TLS succeeded; destination response failed or timed out"
		return
	}
	response.Body.Close()
	result.Status = response.StatusCode
	result.OK = response.StatusCode >= 200 && response.StatusCode < 400
	if result.OK {
		result.Detail = "PASS: proxy authentication, HTTPS tunnel and destination response succeeded"
	} else {
		result.Detail = fmt.Sprintf("proxy tunnel/TLS works, but destination returned HTTP %d (site/provider access restriction)", response.StatusCode)
	}
	return
}
