// Package netdiag answers one question fast, for any site: does this machine
// reach the target directly, and if not, at which layer does it break?
// It never opens a browser, never logs in and never uses a proxy.
package netdiag

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	dialBudget    = 10 * time.Second
	requestBudget = 12 * time.Second
	maxBodyProbe  = 32 << 10
)

// DirectResult breaks a direct (proxyless) request into DNS → TCP → TLS → HTTP
// so a failure is attributed to a specific layer instead of "it does not work".
type DirectResult struct {
	URL    string   `json:"url"`
	Scheme string   `json:"scheme"`
	Host   string   `json:"host"`
	DNSIPs []string `json:"dns_ips,omitempty"`
	DNSMS  int64    `json:"dns_ms"`
	TCPMS  int64    `json:"tcp_ms"`
	TLSMS  int64    `json:"tls_ms"`
	Status int      `json:"status"`
	OK     bool     `json:"ok"`
	Detail string   `json:"detail"`
	Hint   string   `json:"hint,omitempty"`
}

// Direct performs a bounded HEAD (falling back to GET when the server refuses
// HEAD) against rawURL without any proxy.
func Direct(ctx context.Context, rawURL string) DirectResult {
	start := time.Now()
	endpoint := strings.TrimSpace(rawURL)
	if endpoint == "" {
		endpoint = "https://app.chitchat.gg/"
	}
	result := DirectResult{URL: endpoint}
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		result.Detail = "invalid URL: needs http://host or https://host"
		result.Hint = "Give a full URL, for example https://app.chitchat.gg/"
		return result
	}
	result.Scheme = u.Scheme
	result.Host = u.Host
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	dnsStart := time.Now()
	addresses, err := net.DefaultResolver.LookupHost(ctx, u.Hostname())
	result.DNSMS = time.Since(dnsStart).Milliseconds()
	if err != nil || len(addresses) == 0 {
		result.Detail = "DNS lookup failed (offline, DNS blocked, or the host has no address)"
		result.Hint = "If the site opens in a normal browser, a VPN/proxy/hosts-file difference or a typo is the cause."
		return result
	}
	result.DNSIPs = addresses

	deadline := start.Add(requestBudget)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	first := attempt(ctx, u, port, http.MethodHead, deadline)
	result.TCPMS = first.tcpMS
	result.TLSMS = first.tlsMS
	switch first.stage {
	case "tcp":
		result.Detail = "TCP connect failed (port closed, firewall, or the address is unreachable from this network)"
		result.Hint = "Check the port, local firewall/VPN, and whether the site is up for other people."
		return result
	case "tls":
		result.Detail = "TLS handshake or certificate verification failed"
		result.Hint = "Expired/mismatched certificate, TLS interception (antivirus/VPN), or a blocked handshake."
		return result
	case "request":
		result.Detail = "no HTTP response (server closed the connection or the request timed out)"
		result.Hint = "The TCP/TLS layers answered, so this is usually a server-side or request-shape problem."
		return result
	}
	status := first.status
	if status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented {
		// Some sites refuse HEAD; retry on a fresh connection with GET so a
		// "the site works in a browser" contradiction cannot hide here.
		if retry := attempt(ctx, u, port, http.MethodGet, deadline); retry.stage == "" && retry.status != 0 {
			status = retry.status
		}
	}
	result.Status = status
	result.OK = status >= 200 && status < 400
	if result.OK {
		result.Detail = fmt.Sprintf("reachable directly: HTTP %d in %d ms", status, time.Since(start).Milliseconds())
	} else {
		result.Detail = fmt.Sprintf("site answered HTTP %d without a proxy (access/country/anti-bot restriction, not a network problem)", status)
		result.Hint = "If the proxy test also fails with this status, the site/proxy IP is the problem, not the app or your network."
	}
	return result
}

type attemptResult struct {
	status int
	tcpMS  int64
	tlsMS  int64
	stage  string // "", "tcp", "tls", "request"
}

// attempt performs exactly one request on a fresh connection. Layers are timed
// separately so a failure can be attributed without guessing.
func attempt(ctx context.Context, u *url.URL, port, method string, deadline time.Time) attemptResult {
	result := attemptResult{}
	dialer := &net.Dialer{Timeout: dialBudget}
	dialStart := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	result.tcpMS = time.Since(dialStart).Milliseconds()
	if err != nil {
		result.stage = "tcp"
		return result
	}
	defer conn.Close()
	var stream interface {
		io.ReadWriter
		SetDeadline(time.Time) error
	} = conn
	if u.Scheme == "https" {
		secure := tls.Client(conn, &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12})
		tlsStart := time.Now()
		if err := secure.HandshakeContext(ctx); err != nil {
			result.tlsMS = time.Since(tlsStart).Milliseconds()
			result.stage = "tls"
			return result
		}
		result.tlsMS = time.Since(tlsStart).Milliseconds()
		stream = secure
	}
	stream.SetDeadline(deadline)
	status, err := sendProbe(stream, u, ctx, method)
	if err != nil {
		result.stage = "request"
		return result
	}
	result.status = status
	return result
}

func sendProbe(stream io.ReadWriter, u *url.URL, ctx context.Context, method string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "ChitChat-NetDoctor/1.0")
	req.Close = true
	if err := req.Write(stream); err != nil {
		return 0, err
	}
	response, err := http.ReadResponse(bufio.NewReader(io.LimitReader(stream, maxBodyProbe)), req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response.StatusCode, nil
}

// Listeners reports whether anything accepts TCP connections on 127.0.0.1 at
// the given port. Used to explain "the port is taken" without guessing.
func Listeners(ctx context.Context, port int) bool {
	if port < 1 || port > 65535 {
		return false
	}
	probe, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(probe, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Summarize renders a one-line verdict for logs and CLI output.
func (r DirectResult) Summarize() string {
	parts := []string{strings.ToUpper(r.Scheme), r.Host}
	if r.Status != 0 {
		parts = append(parts, "HTTP "+strconv.Itoa(r.Status))
	}
	if r.DNSMS > 0 {
		parts = append(parts, "dns="+strconv.FormatInt(r.DNSMS, 10)+"ms")
	}
	if r.TCPMS > 0 {
		parts = append(parts, "tcp="+strconv.FormatInt(r.TCPMS, 10)+"ms")
	}
	if r.TLSMS > 0 {
		parts = append(parts, "tls="+strconv.FormatInt(r.TLSMS, 10)+"ms")
	}
	status := "FAIL"
	if r.OK {
		status = "PASS"
	}
	return status + " " + strings.Join(parts, " ") + " — " + r.Detail
}
