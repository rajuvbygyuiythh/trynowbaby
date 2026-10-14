package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MaxTestEntries = 20
	// DefaultExitIPURL is the opt-in service used to learn the public exit IP
	// behind a proxy. It is only contacted when the caller asks for it.
	DefaultExitIPURL = "https://api.ipify.org/?format=json"
	perProxyBudget   = 12 * time.Second
	exitIPBudget     = 8 * time.Second
	maxExitIPBody    = 4 << 10
)

type TestResult struct {
	Line      int    `json:"line"`
	Label     string `json:"label"`
	OK        bool   `json:"ok"`
	TunnelOK  bool   `json:"tunnel_ok"`
	Status    int    `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	Detail    string `json:"detail"`
	// Hint is a short, actionable next step derived from the failure kind.
	Hint string `json:"hint,omitempty"`
	// ProviderMessage is a sanitized excerpt of the proxy provider's own reply.
	ProviderMessage string `json:"provider_message,omitempty"`
	// ProxyAuthenticate is a sanitized Proxy-Authenticate value, when offered.
	ProxyAuthenticate string `json:"proxy_authenticate,omitempty"`
	// Authenticated reports whether the tested line carried a username/password.
	Authenticated bool `json:"authenticated"`
	// ExitIP/ExitCountry are only filled when CheckExitIP was requested; the
	// public exit IP identifies the egress used for this line (useful when a
	// target site needs a specific country).
	ExitIP      string `json:"exit_ip,omitempty"`
	ExitCountry string `json:"exit_country,omitempty"`
	// ExitIPError explains why the optional exit-IP lookup did not produce a
	// value. It never changes the PASS/FAIL verdict of the target test.
	ExitIPError string `json:"exit_ip_error,omitempty"`
}

// TestOptions keeps the per-run knobs in one place so the dashboard and the
// netdoctor CLI test proxies in exactly the same way.
type TestOptions struct {
	// Endpoint is any http(s) URL; https:// is the meaningful case.
	Endpoint string
	// CheckExitIP opts in to one extra request through each proxy.
	CheckExitIP bool
	// ExitIPURL defaults to DefaultExitIPURL.
	ExitIPURL string
	// MaxEntries defaults to MaxTestEntries.
	MaxEntries int
}

// hintFor turns a failure into one plain-language next step. It never guesses
// about a specific provider beyond what its own status code actually means.
func hintFor(entry Entry, status int, snippet string, networkErr bool) string {
	lower := strings.ToLower(snippet)
	switch {
	case status == 407:
		if !entry.HasAuth() {
			return "This line has no username:password. Use host:port:user:pass or allow your IP with the provider."
		}
		return "The proxy rejected the credentials. Re-copy username and password for this exact endpoint (a rotating/backbone username differs from a direct one)."
	case status == 402:
		if !entry.HasAuth() {
			return "The proxy answered 402 for a line without credentials. Add user:pass (host:port:user:pass) or authorize this machine's IP with the provider."
		}
		if strings.Contains(lower, "authenticat") || strings.Contains(lower, "credential") || strings.Contains(lower, "password") || strings.Contains(lower, "username") {
			return "The proxy answered 402 and its own message points at authentication. Re-copy the username/password that belong to this exact endpoint."
		}
		return "402 is a provider account/entitlement answer: expired plan, exhausted bandwidth, or credentials that do not match this endpoint (e.g. a direct-connection login used against the rotating endpoint)."
	case status == 403:
		return "The proxy refused this destination. Some providers block specific sites on some proxy types; test another proxy or another proxy plan."
	case status == 429:
		return "Provider rate/concurrency limit reached. Wait, or use fewer parallel browsers through this proxy."
	case status >= 500:
		return "The provider had trouble reaching the destination. Retry or pick another proxy from the list."
	case status != 0:
		return "The proxy answered HTTP " + strconv.Itoa(status) + ". Test the same line with curl or a browser to confirm the provider side."
	case networkErr:
		return "No HTTP answer at all: wrong host/port, blocked local network, or the proxy is offline."
	default:
		return ""
	}
}

// TestList never saves settings, rotates worker proxies or opens an account.
// Only an HTTPS HEAD request to the supplied fixed public endpoint is made.
func TestList(ctx context.Context, raw, endpoint string) ([]TestResult, error) {
	return TestListWithOptions(ctx, raw, TestOptions{Endpoint: endpoint})
}

// TestListWithOptions is the shared implementation behind the dashboard button
// and the netdoctor CLI: same parser, same check, same sanitized diagnostics.
func TestListWithOptions(ctx context.Context, raw string, opts TestOptions) ([]TestResult, error) {
	if len(raw) > 64<<10 {
		return nil, fmt.Errorf("proxy list is too large (maximum 64 KiB)")
	}
	endpoint := strings.TrimSpace(opts.Endpoint)
	if endpoint == "" {
		endpoint = "https://app.chitchat.gg/"
	}
	if !isHTTPURL(endpoint) {
		return nil, fmt.Errorf("test endpoint must be an http(s) URL")
	}
	if opts.CheckExitIP && strings.TrimSpace(opts.ExitIPURL) != "" && !isHTTPURL(opts.ExitIPURL) {
		return nil, fmt.Errorf("exit-IP endpoint must be an http(s) URL")
	}
	limit := opts.MaxEntries
	if limit <= 0 {
		limit = MaxTestEntries
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")
	results := make([]TestResult, 0)
	entries := make([]Entry, 0)
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(results) >= limit {
			return nil, fmt.Errorf("test at most %d proxy lines at a time", limit)
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
				c, cancel := context.WithTimeout(ctx, perProxyBudget)
				tested := testEndpointFull(c, entries[i], endpoint, nil, opts)
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
func testEndpoint(ctx context.Context, entry Entry, endpoint string, roots *x509.CertPool) TestResult {
	return testEndpointFull(ctx, entry, endpoint, roots, TestOptions{})
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Hostname() != "" && (u.Scheme == "http" || u.Scheme == "https")
}

// probeResult is the raw outcome of one request through a proxy tunnel.
type probeResult struct {
	tunnel      bool // TCP + proxy handshake succeeded
	tlsOK       bool
	https       bool
	status      int
	body        []byte
	detail      string
	err         error
	tlsRejected bool
}

// probe opens a fresh tunnel through this proxy for exactly one request. It
// never falls back to a direct connection and never retries elsewhere.
func (e Entry) probe(ctx context.Context, endpoint, method string, roots *x509.CertPool, maxBody int) probeResult {
	result := probeResult{}
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		result.detail = "invalid test endpoint (needs an http(s) URL)"
		return result
	}
	result.https = u.Scheme == "https"
	port := u.Port()
	if port == "" {
		if result.https {
			port = "443"
		} else {
			port = "80"
		}
	}
	conn, err := e.DialTunnel(ctx, net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		result.detail = err.Error()
		result.err = err
		return result
	}
	defer conn.Close()
	result.tunnel = true
	var stream interface {
		io.ReadWriter
		SetDeadline(time.Time) error
	} = conn
	if result.https {
		secure := tls.Client(conn, &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12, RootCAs: roots})
		if err := secure.HandshakeContext(ctx); err != nil {
			result.detail = "proxy tunnel opened, but destination TLS verification/handshake failed"
			result.tlsRejected = true
			return result
		}
		result.tlsOK = true
		stream = secure
	}
	deadline := time.Now().Add(perProxyBudget)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	stream.SetDeadline(deadline)
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		result.detail = "invalid test request"
		return result
	}
	req.Header.Set("User-Agent", "ChitChat-Proxy-Test/1.0")
	req.Close = true
	if err := req.Write(stream); err != nil {
		result.detail = "tunnel/TLS succeeded; test request failed"
		return result
	}
	response, err := http.ReadResponse(bufio.NewReader(io.LimitReader(stream, 64<<10)), req)
	if err != nil {
		result.detail = "tunnel/TLS succeeded; destination response failed or timed out"
		return result
	}
	defer response.Body.Close()
	result.status = response.StatusCode
	if maxBody > 0 {
		result.body, _ = io.ReadAll(io.LimitReader(response.Body, int64(maxBody)))
	}
	return result
}

func testEndpointFull(ctx context.Context, entry Entry, endpoint string, roots *x509.CertPool, opts TestOptions) (result TestResult) {
	result.Label = entry.Label()
	result.Authenticated = entry.HasAuth()
	start := time.Now()
	defer func() { result.LatencyMS = time.Since(start).Milliseconds() }()

	p := entry.probe(ctx, endpoint, http.MethodHead, roots, 0)
	result.TunnelOK = p.tunnel && (!p.https || p.tlsOK)
	if !p.tunnel {
		result.Detail = p.detail
		status := 0
		networkErr := false
		var tunnelErr *TunnelError
		if errors.As(p.err, &tunnelErr) {
			status = tunnelErr.Status
			result.ProviderMessage = tunnelErr.ProviderMessage
			result.ProxyAuthenticate = tunnelErr.ProxyAuth
			networkErr = tunnelErr.Kind == "network"
		}
		result.Status = status
		result.Hint = hintFor(entry, status, result.ProviderMessage, networkErr)
		return result
	}
	if p.status == 0 {
		result.Detail = p.detail
		if p.tlsRejected {
			result.Hint = "The tunnel opened, so the proxy credentials work. The HTTPS handshake failed: re-test in a moment, or the destination may be intercepted/blocked for this proxy."
		}
		return result
	}
	result.Status = p.status
	result.OK = p.status >= 200 && p.status < 400
	if result.OK {
		result.Detail = "PASS: proxy authentication, HTTPS tunnel and destination response succeeded"
	} else {
		result.Detail = fmt.Sprintf("proxy tunnel/TLS works, but destination returned HTTP %d (site/provider access restriction)", p.status)
		result.Hint = hintFor(Entry{}, p.status, "", false)
	}
	if opts.CheckExitIP {
		exitURL := strings.TrimSpace(opts.ExitIPURL)
		if exitURL == "" {
			exitURL = DefaultExitIPURL
		}
		result.ExitIP, result.ExitCountry, result.ExitIPError = entry.exitIP(ctx, exitURL, roots)
	}
	return result
}

// exitIP asks an external echo service, through this proxy only, which public
// IP the connection egresses from. Used to confirm country/geo before spending
// time on a target site.
func (e Entry) exitIP(ctx context.Context, exitURL string, roots *x509.CertPool) (ip, country, detail string) {
	child, cancel := context.WithTimeout(ctx, exitIPBudget)
	defer cancel()
	p := e.probe(child, exitURL, http.MethodGet, roots, maxExitIPBody)
	if !p.tunnel || p.status == 0 {
		return "", "", p.detail
	}
	if p.status < 200 || p.status >= 400 {
		return "", "", fmt.Sprintf("exit-IP service returned HTTP %d", p.status)
	}
	ip, country = parseExitIPBody(p.body)
	if ip == "" {
		return "", "", "exit-IP service response did not contain an IP address"
	}
	return ip, country, ""
}

// parseExitIPBody accepts the common echo-service shapes: JSON with an ip-ish
// field (optionally a country) or a plain-text body containing the IP.
func parseExitIPBody(body []byte) (ip, country string) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", ""
	}
	if strings.HasPrefix(trimmed, "{") {
		var payload map[string]any
		if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
			for key, value := range payload {
				text, ok := value.(string)
				if !ok {
					continue
				}
				switch strings.ToLower(key) {
				case "ip", "ipv4", "query", "ip_address", "origin":
					if ip == "" {
						if candidate := firstIP(strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' })); candidate != "" {
							ip = candidate
						}
					}
				case "country", "country_code", "countrycode", "country_name":
					if country == "" {
						country = sanitizeProviderText(text, Entry{}, 32)
					}
				}
			}
		}
	}
	if ip == "" {
		ip = firstIP(strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		}))
	}
	if ip == "" {
		return "", ""
	}
	return ip, country
}

func firstIP(fields []string) string {
	for _, field := range fields {
		// Accept both "1.2.3.4" and "ip=1.2.3.4" style plain-text echoes.
		if index := strings.LastIndex(field, "="); index >= 0 {
			field = field[index+1:]
		}
		candidate := strings.Trim(field, "\"'[]{}:,")
		if net.ParseIP(candidate) != nil {
			return candidate
		}
	}
	return ""
}
