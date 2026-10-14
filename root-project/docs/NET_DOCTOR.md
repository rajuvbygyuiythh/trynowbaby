# NetDoctor — multi-site connectivity & proxy playbook (2026-10-15)

Purpose: stop losing time to small errors when working on **many** sites. One
run answers three questions for any target: *does this machine reach the site
directly?* *does each proxy reach it?* *what country does each proxy egress
from?* — with the provider's own words attached to every failure.

Nothing here opens a browser, logs in, reads cookies, saves settings or touches
the bot/worker/licence/chat code. The dashboard test is unchanged in that
respect: one HEAD request per proxy per target, plus one optional extra request
when the exit-IP box is ticked.

## 1. Dashboard (no command line)

Proxy tab → new controls above the buttons:

| Control | Meaning |
|---|---|
| **Test site URL** | Any site. Default `https://app.chitchat.gg/`. Test `https://newsite.com/` before spending time on that site. |
| **Also check exit IP + country** | Off by default. When ticked, each proxy makes one extra request to the exit-IP service so you can see which country the proxy egresses from. |
| **Exit IP service** | Default `https://api.ipify.org/?format=json`. Any echo service works; JSON (`ip`, `country`, `country_code`, `query`) and plain text are both understood. |
| **Check site directly (no proxy)** | Runs DNS → TCP → TLS → HTTP **without** any proxy and names the failing layer. This is the baseline that separates "the site is blocking us" from "our side/proxy is broken". |
| **Copy report** | Copies the rendered report to the clipboard for pasting into a chat. The proxy list (which contains passwords) is deliberately **never** copied. |

Every failing line still shows `Provider said:` / `Auth challenge:` / `Next step:`,
and now also `Exit IP:` when the lookup succeeded.

## 2. CLI — netdoctor.bat (one command for many sites)

```bat
netdoctor.bat -proxies proxies.txt -targets https://site1.com/,https://site2.com -exitip
netdoctor.bat -targets https://newsite.com -direct
netdoctor.bat -proxies proxies.txt -targets https://site1.com -type socks5 -json report.json
```

`netdoctor.bat` builds `build\bin\netdoctor.exe` once and then runs it with your
arguments. The same binary works from any Windows shell.

| Flag | Default | Meaning |
|---|---|---|
| `-proxies <file>` | none | One proxy per line, exactly the dashboard formats (`host:port`, `host:port:user:pass`, `http://user:pass@host:port`, `socks5://…`). |
| `-proxy <line>` | none | A single line; repeat with commas. |
| `-type http\|socks5` | `http` | Protocol for plain lines (matches the dashboard's default-protocol selector). |
| `-targets u1,u2` | `https://app.chitchat.gg/` | Any http/https URLs. |
| `-exitip` | off | Also ask each proxy for its exit IP + country. |
| `-exitip-url` | `https://api.ipify.org/?format=json` | Override the echo service. |
| `-direct` | on | Also test each target with no proxy. Use `-direct=false` to skip. |
| `-json <file>` | none | Write the full machine-readable report (for later diffing). |
| `-timeout <s>` | 65 | Whole-run budget, 1–600. |

Exit codes: **0** everything passed, **1** something failed (details are printed),
**2** wrong arguments (nothing was tested).

Output shape (real example of a mixed list):

```text
== Target https://app.chitchat.gg/ ==
  direct  PASS HTTPS app.chitchat.gg HTTP 200 — reachable directly: HTTP 200 in 210 ms
  line 1   PASS http://p.example.io:80                       410 ms  HTTP 200
            exit IP: 203.0.113.7 (SG)
  line 2   FAIL http://p.example.io:80                       251 ms  HTTP 402
            proxy tunnel/TLS works, but destination returned HTTP 402 (site/provider access restriction)
            provider said: "402 Payment Required — plan expired"
            → 402 is a provider account/entitlement answer: …

== Verdict ==
  https://app.chitchat.gg/: 1/2 proxies passed.
  https://app.chitchat.gg/: 1 line(s) were refused with their credentials — re-copy address/port/username/password from one Connection Method view in the provider dashboard.
```

## 3. Decision table (use this instead of guessing)

| Direct check | Proxy result | What it means | First action |
|---|---|---|---|
| PASS | PASS | Working path | Use that line for that site. |
| PASS | FAIL 402/407 | Provider-side: credentials, plan, bandwidth, or IP-allowlist | Read `Provider said`/`Next step`; re-copy address+port+username+password from one Connection Method view. |
| PASS | FAIL 403 | Provider blocks **this destination** for that proxy type | Try another proxy type/plan or provider for that site. |
| PASS | FAIL 429 | Too many parallel requests | Fewer browsers per proxy. |
| PASS | FAIL (no HTTP answer) | Proxy offline/port wrong/local firewall | Recheck the endpoint; the site is not the problem. |
| FAIL | any | Site unreachable from this machine even without a proxy | Fix DNS/VPN/firewall or the URL first; every proxy row is then explained. |
| FAIL HTTP 4xx | FAIL same status | Site is refusing both direct and proxied traffic | Site-side: IP/geo/anti-bot. The app is not the cause. |
| PASS | PASS but site behaves badly | Proxy IP is accepted but the site still blocks (captcha, geo mismatch) | Re-check `Exit IP` country; pick a proxy in the country the site expects. |

## 4. Browser-side failures are now explained in logs.txt

Chrome/WebView shows only a generic proxy error page. The relay now writes the
real reason once per distinct problem:

```text
[proxy] upstream connection to app.chitchat.gg:443 failed — HTTP 402: HTTP proxy rejected CONNECT (402): check provider permissions, IP allowlist and target access | provider said: 402 Payment Required | auth challenge: Basic realm="webshare"
```

Repeated identical failures are suppressed, so a navigation loop cannot flood the
file. Credentials are never printed; the same sanitizer as the dashboard test is
used.

## 5. Limits (kept honest)

- A PASS is not a browser-login guarantee: HEAD may differ from real browser
  requests, and sites change their protections.
- Exit-IP lookups are **opt-in** and use a third-party service; if that service
  is blocked by a proxy the main verdict stays unchanged and only
  `Exit IP: not determined — …` is shown.
- Only DNS/TCP/TLS/HTTP layers are checked here; WebRTC/UDP behaviour is
  unchanged and not tested.
- The tool never saves, uploads or logs credentials, cookies or profile data.
- Evidence in this repo is synthetic/local only (see `internal/netdiag`,
  `internal/proxy/*_test.go`, `cmd/netdoctor/main_test.go`); the owner's real
  Windows/proxy run remains the acceptance test.
