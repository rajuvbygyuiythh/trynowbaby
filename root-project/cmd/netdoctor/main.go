// Command netdoctor checks, for any site and any proxy list, whether the site
// is reachable directly and through each proxy — in one run, with plain-language
// reasons. It is a standalone tool: it never opens a browser, never logs in,
// never saves settings and never touches the bot, licence or chat code.
//
// Typical use on Windows (built by netdoctor.bat):
//
//	netdoctor -proxies proxies.txt -targets https://app.chitchat.gg/,https://newsite.com -exitip
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"chitchatbot/internal/netdiag"
	"chitchatbot/internal/proxy"
)

type config struct {
	proxies     string
	proxyFile   string
	defaultType string
	targets     []string
	checkExitIP bool
	exitIPURL   string
	direct      bool
	jsonOut     string
	timeoutSec  int
}

func parseFlags(args []string, out io.Writer) (config, error) {
	var cfg config
	var targets string
	fs := flag.NewFlagSet("netdoctor", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.StringVar(&cfg.proxies, "proxies", "", "proxy list file (one line per proxy; same formats as the dashboard)")
	fs.StringVar(&cfg.proxyFile, "proxy", "", "single proxy line, repeatable with commas")
	fs.StringVar(&cfg.defaultType, "type", "http", "protocol for plain host:port lines: http or socks5")
	fs.StringVar(&targets, "targets", "https://app.chitchat.gg/", "comma-separated http(s) URLs to test")
	fs.BoolVar(&cfg.checkExitIP, "exitip", false, "also ask an IP echo service, through each proxy, for the exit IP (one extra request per proxy)")
	fs.StringVar(&cfg.exitIPURL, "exitip-url", proxy.DefaultExitIPURL, "URL of the IP echo service used by -exitip")
	fs.BoolVar(&cfg.direct, "direct", true, "also test each target directly, to separate site problems from proxy problems")
	fs.StringVar(&cfg.jsonOut, "json", "", "write the full machine-readable report to this file")
	fs.IntVar(&cfg.timeoutSec, "timeout", 65, "overall seconds allowed for the whole run (1-600)")
	fs.Usage = func() {
		fmt.Fprintln(out, "netdoctor — direct + proxy connectivity report for any site")
		fmt.Fprintln(out, "usage: netdoctor -proxies proxies.txt -targets https://site1,https://site2 [-exitip] [-json report.json]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	for _, raw := range strings.Split(targets, ",") {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			cfg.targets = append(cfg.targets, trimmed)
		}
	}
	if len(cfg.targets) == 0 {
		return cfg, fmt.Errorf("give at least one -targets URL")
	}
	if cfg.defaultType != "http" && cfg.defaultType != "socks5" {
		return cfg, fmt.Errorf("-type must be http or socks5")
	}
	if cfg.timeoutSec < 1 || cfg.timeoutSec > 600 {
		return cfg, fmt.Errorf("-timeout must be between 1 and 600 seconds")
	}
	return cfg, nil
}

type proxyLine struct {
	Raw    string            `json:"raw_label"`
	Result proxy.TestResult  `json:"result"`
	Target string            `json:"target"`
	Extra  *proxy.TestResult `json:"-"`
	_      struct{}          `json:"-"`
}

type report struct {
	Generated string         `json:"generated"`
	Targets   []targetReport `json:"targets"`
	Verdict   []string       `json:"verdict"`
	Version   string         `json:"tool"`
}

type targetReport struct {
	Target  string                `json:"target"`
	Direct  *netdiag.DirectResult `json:"direct,omitempty"`
	Proxies []proxy.TestResult    `json:"proxies,omitempty"`
}

func readProxyLines(cfg config) (string, error) {
	var lines []string
	if strings.TrimSpace(cfg.proxies) != "" {
		data, err := os.ReadFile(cfg.proxies)
		if err != nil {
			return "", fmt.Errorf("cannot read proxy file: %w", err)
		}
		lines = append(lines, strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n")...)
	}
	if strings.TrimSpace(cfg.proxyFile) != "" {
		for _, part := range strings.Split(cfg.proxyFile, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				lines = append(lines, trimmed)
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

// applyDefaultScheme mirrors the dashboard rule: plain lines get the chosen
// protocol, explicit scheme:// prefixes always win.
func applyDefaultScheme(raw, defaultType string) string {
	if defaultType == "http" {
		return raw
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "://") {
			continue
		}
		lines[index] = "socks5://" + trimmed
	}
	return strings.Join(lines, "\n")
}

func run(args []string, out io.Writer) int {
	cfg, err := parseFlags(args, out)
	if err != nil {
		fmt.Fprintln(out, "netdoctor:", err)
		return 2
	}
	raw, err := readProxyLines(cfg)
	if err != nil {
		fmt.Fprintln(out, "netdoctor:", err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.timeoutSec)*time.Second)
	defer cancel()

	full := report{Generated: time.Now().Format(time.RFC3339), Version: "netdoctor/1.0"}
	failures := 0
	directFailures := 0
	haveProxy := strings.TrimSpace(raw) != ""

	for _, target := range cfg.targets {
		entry := targetReport{Target: target}
		fmt.Fprintf(out, "\n== Target %s ==\n", target)
		if cfg.direct {
			direct := netdiag.Direct(ctx, target)
			entry.Direct = &direct
			fmt.Fprintf(out, "  direct  %s\n", direct.Summarize())
			if direct.Hint != "" {
				fmt.Fprintf(out, "          → %s\n", direct.Hint)
			}
			if !direct.OK {
				directFailures++
			}
		}
		if haveProxy {
			results, err := proxy.TestListWithOptions(ctx, applyDefaultScheme(raw, cfg.defaultType), proxy.TestOptions{
				Endpoint:    target,
				CheckExitIP: cfg.checkExitIP,
				ExitIPURL:   cfg.exitIPURL,
			})
			if err != nil {
				fmt.Fprintf(out, "  proxy   FAIL (list not usable: %v)\n", err)
				failures++
				full.Targets = append(full.Targets, entry)
				continue
			}
			entry.Proxies = results
			for _, result := range results {
				status := "PASS"
				if !result.OK {
					status = "FAIL"
					failures++
				}
				line := fmt.Sprintf("  line %-3d %-4s %-40s %5d ms", result.Line, status, result.Label, result.LatencyMS)
				if result.Status != 0 {
					line += fmt.Sprintf("  HTTP %d", result.Status)
				}
				fmt.Fprintln(out, line)
				if result.ExitIP != "" {
					geo := result.ExitIP
					if result.ExitCountry != "" {
						geo += " (" + result.ExitCountry + ")"
					}
					fmt.Fprintf(out, "            exit IP: %s\n", geo)
				} else if result.ExitIPError != "" {
					fmt.Fprintf(out, "            exit IP: unknown — %s\n", result.ExitIPError)
				}
				if !result.OK {
					fmt.Fprintf(out, "            %s\n", result.Detail)
					if result.ProviderMessage != "" {
						fmt.Fprintf(out, "            provider said: %q\n", result.ProviderMessage)
					}
					if result.ProxyAuthenticate != "" {
						fmt.Fprintf(out, "            auth challenge: %s\n", result.ProxyAuthenticate)
					}
					if result.Hint != "" {
						fmt.Fprintf(out, "            → %s\n", result.Hint)
					}
				}
			}
		}
		full.Targets = append(full.Targets, entry)
	}

	full.Verdict = verdict(full, directFailures, failures, haveProxy)
	fmt.Fprintln(out, "\n== Verdict ==")
	for _, line := range full.Verdict {
		fmt.Fprintln(out, "  "+line)
	}
	if cfg.jsonOut != "" {
		payload, err := json.MarshalIndent(full, "", "  ")
		if err != nil {
			fmt.Fprintln(out, "netdoctor: cannot encode report:", err)
			return 2
		}
		if err := os.WriteFile(cfg.jsonOut, append(payload, '\n'), 0o600); err != nil {
			fmt.Fprintln(out, "netdoctor: cannot write report:", err)
			return 2
		}
		fmt.Fprintln(out, "  JSON report written to "+cfg.jsonOut)
	}
	if failures > 0 || directFailures > 0 {
		return 1
	}
	return 0
}

// verdict turns the raw results into the shortest possible action list, so a
// small error never costs a debugging session.
func verdict(full report, directFailures, proxyFailures int, haveProxy bool) []string {
	var lines []string
	for _, target := range full.Targets {
		if target.Direct != nil && !target.Direct.OK {
			lines = append(lines, fmt.Sprintf("%s: this machine cannot reach the site directly (%s) — fix that first, every proxy result for it is then explained.", target.Target, target.Direct.Detail))
		}
		if len(target.Proxies) == 0 {
			continue
		}
		passed, noCreds, rejected, blocked := 0, 0, 0, 0
		for _, result := range target.Proxies {
			switch {
			case result.OK:
				passed++
			case result.Status == 402 || result.Status == 407:
				rejected++
				if !result.Authenticated {
					noCreds++
				}
			case result.Status == 403:
				blocked++
			}
		}
		lines = append(lines, fmt.Sprintf("%s: %d/%d proxies passed.", target.Target, passed, len(target.Proxies)))
		if noCreds > 0 {
			lines = append(lines, fmt.Sprintf("%s: %d line(s) have no username:password and were rejected — add user:pass or authorize this machine's IP with the provider.", target.Target, noCreds))
		}
		if rejected-noCreds > 0 {
			lines = append(lines, fmt.Sprintf("%s: %d line(s) were refused with their credentials — re-copy address/port/username/password from one Connection Method view in the provider dashboard.", target.Target, rejected-noCreds))
		}
		if blocked > 0 {
			lines = append(lines, fmt.Sprintf("%s: %d line(s) were blocked for this destination (HTTP 403 class) — try another proxy type or another provider for this site.", target.Target, blocked))
		}
		if passed > 0 && passed < len(target.Proxies) {
			lines = append(lines, fmt.Sprintf("%s: use a passing line for this site; failing lines can be parked.", target.Target))
		}
	}
	if !haveProxy {
		lines = append(lines, "No proxies were given: only the direct result above applies. Add -proxies <file> to test through proxies.")
	}
	if proxyFailures == 0 && directFailures == 0 {
		lines = append(lines, "Everything that was tested passed. A PASS here is still not proof of a browser login.")
	}
	return lines
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}
