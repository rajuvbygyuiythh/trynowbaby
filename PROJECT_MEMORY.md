## Latest handoff — multi-site NetDoctor toolset (2026-10-15)

Owner: "we will work on many more sites, so do not let small errors cost time."
Implemented (additive, site-agnostic, no chat/AI/worker/licence/store changes):

- `internal/netdiag` — direct (no-proxy) check with per-layer timing
  (DNS → TCP → TLS → HTTP), HEAD-with-GET-fallback, and a one-line hint that
  names the failing layer.
- `cmd/netdoctor` + `netdoctor.bat` — CLI that tests any proxy list against any
  target list, with `-exitip`, `-json`, `-type http|socks5` and a VERDICT
  section; exit codes 0 pass / 1 failure / 2 usage.
- `internal/proxy` — `TestOptions` + `TestListWithOptions`, opt-in exit-IP and
  country lookup (`exit_ip`/`exit_country`/`exit_ip_error`, one extra request,
  never changes the verdict), shared `probe` used for HTTP and HTTPS targets.
- Dashboard Proxy tab — Test site URL, exit-IP + country checkbox and service
  URL, "Check site directly (no proxy)", "Copy report" (never copies the proxy
  list), and exit-IP/country rendering; new bindings `TestProxyEntriesFor` and
  `DiagnoseTarget` (the old `TestProxyEntries` still works).
- Browser relay — the real upstream reason is printed once per distinct failure
  to stdout → `logs.txt` ("[proxy] upstream connection to … failed — HTTP 402 …
  | provider said: …"), credentials never printed.

Evidence: `go test -race` PASS for internal/proxy, internal/netdiag,
cmd/netdoctor; `go vet` PASS; gofmt clean; Windows-target cross-compile of
netdoctor PASS; 67 Node tests PASS; real CLI runs show PASS/FAIL with layer
timings (this sandbox blocks most HTTP egress and the tool reports that
honestly). Full standard gate still dependency-BLOCKED (badger vanity imports);
owner Windows/real-proxy acceptance NOT_RUN. Docs: root-project/docs/NET_DOCTOR.md
(flags, decision table, limits) and PROXY_TEST.md. Source delivery:
`root-project-netdoctor-UNVERIFIED.zip` — 201 entries, packaged with
`node scripts/package-source.cjs --allow-unverified` and re-validated with
`--check` ("Archive integrity PASS"); SHA256
`db0c258e7f694d469a76159055c0d1ae6eafd236a3645f2cdc595077d8a4e515` (final
package, CRLF `netdoctor.bat`); the bundled
report is BLOCKED with liveBrowser NOT_RUN, so the UNVERIFIED label stays.

---

## Latest handoff — provider rejection diagnostics for Test Proxy (2026-10-15)

Owner reported the dashboard proxy test failing with `HTTP proxy rejected CONNECT
(402)` for `http://p.webshare.io:80` (Webshare) and asked for a fresh, non-looping
fix. Verification of the transport: the CONNECT the app sends is correct
(`CONNECT host:port HTTP/1.1` + `Host` + `Proxy-Authorization: Basic …`), so the
402 is the provider's own answer; the app previously discarded that answer, which
is why the same question kept coming back. Change (scoped to the already-authorized
proxy feature): `internal/proxy/connect.go` now returns a `TunnelError` carrying
the proxy's status, `Proxy-Authenticate` value and a bounded (512 B / 1.2 s),
credential-free, single-line excerpt of the proxy's rejection body;
`internal/proxy/testproxy.go` adds `hint`, `provider_message`, `proxy_authenticate`
and `authenticated` to each result and explains each status in plain language
(including "this line has no username:password"); the dashboard renders
"Provider said" / "Auth challenge" / "Next step". No new network request, no exit
IP collection, no change to chat/AI/Home/worker/license code or dependencies.
Evidence: `go test -race ./internal/proxy` PASS, `go vet ./internal/proxy` PASS,
`go build ./internal/browser` PASS, 65 Node tests PASS (`npm test`), and the changed Go files are gofmt-clean.
Full standard gate still dependency-BLOCKED (badger's vanity import paths are
unreachable here); owner's real Windows/proxy acceptance NOT_RUN.
Docs: root-project/docs/PROXY_TEST.md ("Provider rejection diagnostics" +
Webshare checklist) and root-project/docs/PROJECT_MEMORY.md.

Source delivery for this change (owner asked for a full project ZIP + direct
download): `root-project-proxy-diagnostics-UNVERIFIED.zip` at repository root,
194 entries, built with `node scripts/package-source.cjs --allow-unverified` and
re-validated with `--check` ("Archive integrity PASS", every payload hash and
the Windows case/path safety rules verified). SHA256
`11ade46280b1bebaff178021a552ccf98ed00e7eb3bf4e316d6b5a139cd110fb`. The bundled
`VERIFICATION-REPORT.json` is BLOCKED (dependency download), so the archive keeps
the UNVERIFIED label and `liveBrowser: NOT_RUN`; nothing in it is a claim that a
real Windows browser/login run happened. Extract → `install.bat` builds and runs
on Windows.

---

## Latest feature handoff — 2026-10-15

HTTP/SOCKS5 format support and dashboard Test Proxy implemented after owner's
request. See root-project/docs/PROXY_TEST.md. Source delivery:
root-project-proxy-dashboard-UNVERIFIED.zip (193 entries, 192 hashes PASS; SHA256 5224dfe870dabc7bee8ccb80bdc1a0306d1673387829df2b9c6914974360d41d). Unit/local synthetic checks PASS;
full standard gate BLOCKED, native Windows/actual proxy NOT_RUN. No remote push
for this feature yet; prior ba77d6f GitHub download remains older source.

---

## Latest verification handoff — 2026-10-15

All updates rechecked without production/test/dependency code changes. Fresh
Node 62 PASS (Show Mode subset separately 19 PASS), capture mock tests 12 PASS,
actual logger race/vet and Windows-target compilation PASS; actual workerplan/
runtimeassets and other stdlib-only package checks, apitest unit tests/vet PASS.
Isolated transcript/runner/real pinned Rod race/vet/cross-compilation PASS, not
full desktop verification. Required standard gate: 5 PASS, 8 BLOCKED, 0 FAIL;
native Windows/live display acceptance NOT_RUN. See docs/RECHECK_2026-10-15.md
inside the complete root-project-full-rechecked-UNVERIFIED.zip download.
All old working archives unchanged. No GitHub push implied.

---

## Latest handoff — Home User/Bot transcript (2026-10-15)

Owner explicitly approved the scoped plan before implementation. New read-only
Home display uses actual Show Mode DOM IDs/authors/full text, not technical log
parsing or outgoing intent. Native-send snapshots preserve the final actual own
row before Skip. Technical diagnostics stay in logs.txt; raw SMS transcript is
in memory only. See root-project/docs/HOME_CHAT.md for exact scope/limits/tests.
The full current source archive is root-project-home-chat-transcript-UNVERIFIED.zip;
use it (not the older file-logging archive) if frontend assets need restoration.
62 Node tests pass. Isolated actual transcript/runner/real pinned Rod Go race tests,
vet and Windows-target test compilation pass; full desktop verification remains
dependency-BLOCKED and native Windows/live transcript acceptance NOT_RUN.
Existing workers/AI/license/proxy/stores/shared drivers/diagnostics logger remain
unchanged. No remote push of this new feature has been performed.

---

## Latest handoff — Home file logging (2026-10-15)

Owner confirmed Show Mode v2 works on Windows. That chat/browser implementation
is unchanged. Missing frontend assets were restored from its working source ZIP.
The subsequent Home logging task is now implemented in editable source: no Home
log rows; normal diagnostics append to logs.txt beside data/; desktop stdout/
stderr are captured and Go fatal runtime stack output appends directly to that
file. Read root-project/docs/FILE_LOGGING.md and project decision memory for the
exact scope, tests and raw-crash privacy caveat. New full-source download:
root-project-home-file-logs-UNVERIFIED.zip. Old working archives remain intact.
58 Node tests and 13 actual Go logger/race tests plus logger vet PASS; full
standard gate remains dependency-BLOCKED, native Windows retest NOT_RUN.
No remote publication of this new change has been performed.

---

# Project memory — trynowbaby / ChitChat Bot

## Current update — targeted Show Mode fix (2026-10-14)

The owner has now authorized implementation, superseding the earlier read-only
scope below. Editable source is extracted under `root-project/`; the original ZIP
is unchanged. Production modifications are limited to
`internal/browser/domchat.go`, new `internal/browser/showmode_dom.js`, and
`live_chat.go`. Tests/test wiring and scoped docs are updated. Ordinary browser
login drivers, workers, AI core, license, proxy, frontend and lockfiles remain
byte-identical to the archive. Read `root-project/docs/SHOW_MODE_FIX.md` for scope
and evidence, and `root-project/docs/SHOW_MODE.md` for use/debugging. New detector
uses stable IDs, grouped authors and a DOM observer, not time/text guesses.
Show Mode local handle rendering/no-username gating is fixed; worker counters and
AI compose are still not added. Full verification remains BLOCKED; real Windows
site testing NOT_RUN. A separately named UNVERIFIED source ZIP is supplied.
No merge/deployment or remote publication is implied by this implementation.

---

Last reviewed (original archive): **2026-10-14 (UTC)**. This is a documentation-only handoff requested by the owner: read the project and save memory on GitHub. No application code, configuration, dependencies, or archive contents were changed.

## 1. Read this first: actual repository layout

At reviewed commit `4bcd4047331b97107a015761edf03f23102a3dbe`, Git tracks **one file**:

```text
root-project-showmode-dom-UNVERIFIED.zip
```

The application is inside that archive, not checked out as a `root-project/` directory. This memory file is intentionally outside the ZIP so future sessions can read it directly on GitHub. Do not unpack and commit the application, replace the ZIP, regenerate its documentation, or make product changes merely to update memory.

The ZIP contains **170 files**:

- **165 files** under `root-project/` (935,359 uncompressed bytes).
- `VERIFICATION-REPORT.json` and `SOURCE-MANIFEST.json` beside that directory.
- Three logs under `verification-evidence/`.

Archive SHA-256:

```text
428611a91dc63500fa579acd2291b095ef0bffb30fa497bf24d60e58dbbd2342
```

The bundled map's count of 163 excludes its two generated map/memory files. This is not a missing-source discrepancy. The bundled memory's last historical entry says full source was committed, but **that is not the layout of this repository at the reviewed commit**. Trust the actual Git tree, not that copied handoff claim.

Paths in sections below are relative to the extracted `root-project/` unless explicitly described as repository-root paths. Extract to a separate scratch directory for read-only review; retain the original ZIP unchanged.

### Startup reading order for another agent

1. This repository-root `PROJECT_MEMORY.md` for current checkout facts.
2. Archive `root-project/AGENTS.md` for inherited owner decisions and scope.
3. Archive `docs/PROJECT_MAP.md` and `docs/PROJECT_MEMORY.md` for architecture/history.
4. Archive `docs/SHOW_MODE.md` for the latest DOM-only implementation.
5. `docs/VERIFICATION.md` plus the archive-root verification report before any build/release claim.
6. For AI-lab work specifically: `apitest/HANDOFF.md`, then `apitest/PLAN.md`.

The top additions in `AGENTS.md` supersede the older EVA-only restriction for explicitly authorized Browser direct-run, Show Mode, capture, verification, and documentation work. They are not permission for arbitrary refactors. The inherited notes say the owner prefers Bengali/Banglish communication.

## 2. Product and technology

**ChitChat Bot** is a **Windows desktop application**, not a web service. It uses Go + Wails with a plain HTML/CSS/JavaScript dashboard and a separate CLI worker to automate chats on chitchat.gg.

- Main Go module: `chitchatbot`, language/toolchain requirement in `go.mod`: **Go 1.25.0**.
- Wails: **v2.12.0**; Rod **v0.116.2**; ChromeDP **v0.14.2**.
- Storage: Badger **v4.9.1** for dashboard settings/logs; JSON files for login and bot/registry state.
- Realtime worker: Gorilla WebSocket, Socket.IO/Engine.IO parsing, uTLS.
- Windows hardware licensing: WMI, with a remote license check.
- Node package: private `chitchat-verification`; development-only dependencies **jsdom 26.1.0** and **fflate 0.8.2**. Node is not a production dashboard framework.
- Capture is a standalone Node built-in-only tool, version **1.2.1**.
- `apitest/` is a separate dependency-free Go module named `apitest`.

### Canonical entry points

| Path | Responsibility |
|---|---|
| `main.go` | Official Wails desktop entry; embeds `cmd/chitchatui/frontend/dist`; resolves `data/` from current working directory; uses bundled WebView2 when found. |
| `app.go` | Official bound `App`: settings, snapshots/stats, licensing, browser requests/login persistence, Start/Stop workers, maintenance actions, events/logs. |
| `live_chat.go` | Show Mode DOM runner and per-conversation sequence/EVA responders. |
| `cmd/chitchatbot/main.go` | CLI flags/prompts for backend/profile/headless/data-dir; calls `bot.Run`. |
| `cmd/chitchatui/frontend/dist/` | Actual maintained/embedded dashboard assets: `index.html`, `app.js`, `styles.css`. Keep these despite the directory being named `dist`. |
| `cmd/chitchatui/frontend/wailsjs/` | JS/TypeScript Wails bindings and generated model types. Contracts must agree with public Go methods. |
| `cmd/chitchatui/app.go`, `cmd/chitchatui/main.go` | Legacy duplicate desktop entry/app, not the official Wails build path. Leave alone; do not accidentally edit this copy instead of the root app. |

## 3. Architecture and data flow

### Internal package map

| Package | Responsibility |
|---|---|
| `internal/model` | Login/cookie/fingerprint types, conversation/message/account state, EVA session, UI settings/defaults/migrations/snapshots. |
| `internal/uistore` | Badger-backed settings and logs under `data/ui/badger`; normalization and legacy-message migration. |
| `internal/store` | Directory creation and JSON load/save helpers. |
| `internal/browser` | Shared `Backend` contract, Rod and ChromeDP login capture/page fetch, authentication probe, Rod proxy authentication/recovery, optional Rod `DOMChatBackend`. |
| `internal/api` | Cookie-authenticated REST client with browser-style headers, saved user-agent, 30-second timeout and optional proxy rotation. |
| `internal/bot` | Worker loop, friend requests/DMs/matches, sequences, EVA flow/AI compose, realtime events, traffic ownership, state, counters and activity logs. |
| `internal/workerplan` | Discovers saved login workers, deduplicates account candidates, selects newer login records and assigns proxies. |
| `internal/proxy` | Parses proxy lines/schemes/credentials and provides mutex-protected round-robin selection. |
| `internal/runtimeassets` | Finds bundled Chromium/WebView2; compares profile/browser versions to avoid using an older bundled Chromium with a newer profile. |
| `internal/license` | Generates a hardware-derived identifier and verifies licensing; Windows-specific and out of scope for fixes. |

### Two independent runtime routes

**Dashboard Browser → Run:** validates the clicked row/current inputs, validates proxy settings, saves accepted defaults, then asynchronously dispatches fresh login browsers. Count is **1–5 per request**, not a global cap. A successful request acknowledgement does not prove launch/login success. Fresh capture uses disposable OS temp profiles, saves uniquely named login JSON under `data/profiles/<profile>/`, closes the backend before deleting temp files, and preserves accepted settings even if launch later fails.

**Dashboard Home → Start Bot:** discovers saved accounts, creates worker configs, and starts `internal/bot.Run`. Workers load or manually capture a login, identify/register their own account, start realtime listening, poll relationships/DMs, claim traffic, accept pending friend requests, respond, and match when idle. Expired saved sessions stop workers. This worker route still uses REST/WebSocket; it is not the Show Mode DOM loop.

### Persistent/runtime data — never put in Git or memory

- `data/ui/badger/`: UI settings, including optional AI credentials and proxy settings, plus logs.
- `data/profiles/<profile>/`: login JSON, browser profiles, bot state and derived relationship/conversation/user files.
- `data/eva_chat/`: editable starter TXT pack, seeded on demand from embedded source assets.
- Shared bot/traffic registry JSON under the data directory: avoids bot-to-bot replies and multiple accounts claiming the same traffic user; tracks account send failures/status.
- `capture/capture-sessions/` and `capture/profiles/`: browsing evidence and capture browser state.
- `apitest/apitest.config.txt` and `apitest.log`: private tester configuration and transcripts.

Do not read/copy/upload live cookies, tokens, API keys, passwords, login JSON, storage-state JSON, profiles, Badger files, private logs, `.env` files, or raw capture sessions. Embedded starter TXT files under `internal/bot/eva_assets/` are source assets, not private runtime data. Never reproduce source constants as if they were a user's credential.

## 4. Dashboard contracts

Tabs cover Home, Browser, Message, Proxy and General/timing settings. JS invokes `window.go.main.App`; it is not connected to an HTTP backend. Wails events include `ui:log`, `ui:running`, `ui:logs_cleared` and `ui:license_invalid`.

Browser rows:

1. `rod` — ordinary Rod fresh login capture.
2. `chromedp` — ordinary ChromeDP fresh login capture.
3. `show_mode` — visible Rod preset with live DOM chat after login.

Each row has its own Run button, sharing Profile/Count/Headless inputs. Buttons prevent overlapping pending requests; they are usable again after acknowledgement. Browser defaults load once, so routine snapshot refreshes do not overwrite unsaved Browser edits. Worker status wins over a late acknowledgement; routine temp-cleanup logs do not replace a useful login/error status. Legacy `SaveBrowserSettings` and `RunFreshBrowsers` APIs remain for compatibility.

Message mode keys are `single`, `folder`, `eva_chat_flow`. Saved legacy `keyword` mode migrates to EVA at read time. `keyword_chat.go` and old settings fields remain compatibility code; Keyword Based is not a current selectable dashboard mode. Folder mode retains loaded sequence strings and chooses a sequence rather than flattening all files.

Home displays **ACTIVITY-level logs only**. Browser status displays fresh-browser lifecycle messages. Detailed info/error/probe messages may require looking at the corresponding event/status or persisted logs; do not assume every persisted message appears in Home.

## 5. Show Mode — latest decision is DOM-only

**Do not restore WebSocket/API chat transport for Show Mode.** It was implemented historically, then explicitly retired by the owner on 2026-10-14. Normal worker networking remains unchanged. The protocol documents are evidence for that worker, not a new Show Mode design approval.

`validateBrowserRun` normalizes `show_mode` to `Backend=rod`, `Headless=false`, `LiveChat=true` before persistence/dispatch. The ordinary UI rows omit live-chat activation. Show Mode does not change the visible Headless preference checkbox and is not described as an anti-detection mode.

1. User performs login and CAPTCHA manually in the visible Rod browser.
2. Existing cookie/storage capture and authentication checks save the login normally.
3. After save, `runLiveChat` keeps the browser open and uses optional `DOMChatBackend` methods only for chatting.
4. Runner polls approximately every **1.5 seconds**; reply delay is **900 ms**, with **400 ms** between extra reply lines.
5. Lobby clicks START with a six-second throttle. A new conversation seeds existing rows as history, starts a fresh responder and sends an opener.
6. New messages are deduplicated by **author + text copy counts**; own sent echoes and known system rows are suppressed. Own display name is learned conservatively from repeated echoes.
7. Sequence responders pick one sequence per conversation, opener first; EVA calls exported engine `Start`/`Reply` with fresh in-memory session state. Engine-load failure falls back to detection/logging only.
8. End of flow clicks Skip/End/START; partner-left/input disappearance returns to lobby. Skip resets START throttling so rematching is not unnecessarily delayed.
9. Closing the browser page ends the runner; backend close/temp cleanup are deferred.

### DOM heuristics and debugging

`internal/browser/domchat.go` contains injected page helpers. It identifies rendered rows containing an AM/PM clock time, extracts author/content lines, excludes nested containers and caps extraction to the last **40 rows**. Input discovery prefers placeholder/aria-label containing `message`, then the last visible text input. Buttons are matched by exact label first, substring second. Partner detection searches visible `chatting with ...` text.

Sending focuses the input, uses `execCommand('insertText')` with native-setter/input-event fallback, tries a send button or form submit/Enter, then checks for matching rendered text for up to **three seconds**. This is a DOM-observation check, not server-delivery proof.

If detection fails on Windows, obtain the `Page probe:` diagnostic (URL/input/button/row summary) and relevant layout evidence. Fix only the heuristics/authorized Show Mode path from that evidence. Do not guess new selectors or revive the retired transport.

### Source-review caveats (not fixed in this memory-only task)

- Show Mode does **not** apply the optional worker AI compose layer.
- Its EVA responder calls raw engine `Reply`, not worker `evaChatReply`/`evaChatRender`; sequence responders likewise pass raw lines. Dashboard username substitution, no-username gating and worker counters must not be assumed equivalent in this route. Review this explicitly before promising working handle sharing in Show Mode.
- Fresh browser runs use their own background-derived context; `StopBot` controls workers. Do not promise the Home Stop button cancels Show Mode; the documented reliable stop is closing its browser window.
- DOM matching depends on live site layout, visible rows, labels and time formatting. Real-site success remains pending the owner's Windows retest.

## 6. EVAChat Flow — TXT engine remains authoritative

Key files: `internal/bot/eva_chat.go`, `eva_assets.go`, `eva_ai.go`, their tests, `internal/model/model.go`, worker adapters in `logic.go`, and `docs/CHAT_ENGINE_MAPPING.md`.

- Same-named `input/<category>.txt` and `output/<category>.txt` connect user triggers to bot reply pools. Input ignores comments; output also splits on `|`. New category pairs join scanning automatically after priority categories.
- Embedded starters are copied to `data/eva_chat/` **only when a file is missing**. User edits are not overwritten. Restart workers to reload edited TXT data.
- Stages: **NEW → WAIT_AGE_GENDER → WAIT_COUNTRY → MIDDLE → END**.
- Matching includes exact, whole-word substring, slang normalization, token-subset matching, plus category-specific regex/leet handling; English-only triggers use stricter matching.
- Country detection uses longest keyword first. Worker session retains stage, age/gender/country, pool/line usage, ask/refusal flags, reply counters and a bounded transcript (60 lines).
- A **detected/revealed age under 18 blocks sharing**. This is a text detector, not independently verified age assurance; preserve its rules/tests.
- Direct handle requests may share immediately; bare yes only shares after the bot's ask. Firm refusal must not share. Other refusal gets one nudge, then a closer.
- Suggestive/flirty categories arm the next snap ask. Neutral middle-chat has a proactive ask after three middle replies; the MIDDLE-stage reply cap is eight.
- Most pools fire once per user. Repeatable pools include flirty questions, closer, horny, flirty reply, ask snap and horny answer. Fresh lines are preferred; immediate repeats are avoided when alternatives exist.
- Multi-line incoming text uses the last nonempty line to drive the state machine.
- Worker `%username%`/`{snap}` substitution uses the dashboard Usernames list with per-user assignment and existing share/flag counters; `{country}` uses detected country or fallback. No configured usernames makes worker sharing close politely.

### Existing optional AI compose layer

`eva_ai.go` rewrites **reply wording only**; TXT logic still controls stages/intent/sharing. OpenAI-compatible `/chat/completions`; defaults in this archive are `https://api.openai.com/v1`, `gpt-4o-mini`, 25-second timeout, 60 max tokens, temperature 0.9. Persona age/country are stable per session (age 20–25).

Share/username-bearing lines are not rewritten. Output is constrained/sanitized (first line, short lowercase text; rejects forbidden links/usernames/AI-talk/repeats). API/validation failure retains TXT wording. Settings `AIEnabled`, `AIBaseURL`, `AIAPIKey`, `AIModel` remain local under data storage. Do not copy an API key into documentation.

This already-present compose layer is **not** the separate apitest director integration; do not confuse the two.

## 7. apitest — separate AI design lab, integration not approved yet

`apitest/main.go` contains config/API client, deterministic director, quality guards, interactive loop, scripted scenario/compare runners and logging. Persona/scenarios/config example are source files; actual config/key/log are private. It has no imports from the production bot.

Design recorded in `PLAN.md`/`HANDOFF.md`:

- Interest tiers HOT/WARM/COLD with share targets at bot replies **4/6/8**; ask one reply before and conversational hooks earlier.
- Parses stranger facts (age, gender, place, name/activity), keeps per-chat persona/lore, uses context moods and limited double replies.
- Director handles underage/refusal/end/money/insults/non-English cases; guards retry once for problematic replies and polish obvious bot tells.
- Username is appended by Go, not written by the model.
- CLI commands include `ping`, `scenario <name|all>`, `compare <scenario>`, plus chat commands such as `/new`, `/state`, `/summary`.
- API default documented/configured for Featherless and DeepSeek-V3-0324. Provider availability/model-access notes in the historical handoff are observations, not reverified facts today.
- `AUTO_NOTHINK`/extra JSON options support reasoning-model configuration; they do not alter production AI code.

**Current next step recorded in the archive:** owner runs real scenarios with their own key, supplies output for tuning, then designs integration together. Do not wire the director into `internal/bot` or alter the dashboard for AI-lab work without explicit approval. The handoff references an older repository/branch; use it for design history, not current Git checkout identity.

## 8. Capture and protocol evidence

Capture is manual browsing/recording, not a desktop runtime component or an automatic login/chat bot. It attaches through CDP, records HTTP/WebSocket events, DOM/storage hints and snapshots, correlates candidates with incoming/outgoing markers, and writes HAR, JSONL, catalogs, manifests and summaries. Config defaults to redaction; raw mode is explicit opt-in.

Important files:

- `capture/tool/browser.cjs`: browser discovery/launch, free port, up-to-45-second DevTools wait and browser startup logs.
- `cdp.cjs`, `capture.cjs`, `hooks.js`: CDP connection/session handling and event/hook collection.
- `classifier.cjs`, `catalog.cjs`, `wscatalog.cjs`: message candidates and endpoint/event schema catalogs.
- `redactor.cjs`: redaction; v1.2.1 exempts `author`/`authors` from overbroad auth-key matching.
- `store.cjs`, `har.cjs`, `capabilities.cjs`: output store, HAR and capture coverage summaries.
- `compare.cjs`: baseline/new endpoint/event differences and schema fingerprints; writes diff reports.
- `doctor.cjs` and `selftest/run-selftest.cjs`: **actually launch a browser**; do not run for a read-only/default browser-free task.

Default redaction is not a guarantee of anonymization: messages, authors, HTML and screenshots can still be private. Never commit capture output or profiles, and never share raw-mode sessions.

`docs/CHITCHAT_PROTOCOL.md` and `docs/chitchat-protocol.json` preserve distilled historical capture evidence: cookie-based auth, incoming Socket.IO `chatMessage`, outgoing multipart HTTP messages, and disconnect/match lifecycle. Their historical Show Mode transport design is explicitly superseded by DOM-only Show Mode. The historical example says author is a string, but actual worker parsing/history notes describe an author object `{id, username}`; use current parser/real captured shape rather than that simplified example for future worker changes.

## 9. Build, verification and packaging

Run commands **from extracted `root-project/`**, not this ZIP-only checkout.

### Windows build/run

```bat
install.bat          rem install missing prerequisites, build, optionally run
install.bat build    rem rebuild only
install.bat run      rem launch dashboard
install.bat bundle   rem build portable bundle
run-dashboard.bat
```

Manual official build: `wails build -clean -webview2 error`.
Portable bundle: `scripts/bundle-windows.ps1 -Clean`; targets Windows x64, produces `dist/windows-x64` with desktop/CLI binaries and Chromium/WebView2 runtimes. Runtime downloads are dynamic, not a certified pinned known-good browser set. Launch scripts establish working directories; `data/` is working-directory-relative, not universally executable-relative.

### Browser-free verification gate

```text
npm ci --ignore-scripts --no-audit --no-fund
npm test
npm run verify
```

Windows `verify.bat` wraps dependency installation and the gate. `scripts/verify.cjs` refreshes AUTO map/memory blocks first, clears inherited `GOFLAGS`/`GOWORK` to avoid live browser tags, records commands/log hashes/durations, and writes `verification-output/report.json` atomically after invalidating old reports.

Required named checks cover frontend syntax; UI/contract/package tests; Go toolchain/format/dependencies/module checksums; internal regressions/vet; Windows cross-build; Windows app tests/vet; Wails production build; source/workflow unchanged. Missing tools/network can be BLOCKED; actual code/module/checksum errors are FAIL. **BLOCKED is never PASS.** Windows-only execution/build checks cannot be proved just by Linux source inspection or cross-compilation.

Test inventory includes Go lifecycle/direct-run/live fake-DOM tests; EVA/AI/sequence/registry/realtime/store/worker/runtime tests; Node UI/contracts/package/verification/map tests; separate capture tests; separate apitest signal tests. The proxy-auth integration test is opt-in behind `//go:build browser_live`. `npm test` also includes project-map tests; the current verify runner's Node subset does not. Capture/apitest are separate suites and not part of that runner.

### Source release gate

`npm run package:source` requires complete passing checks, fresh source/workflow fingerprints and matching log evidence. Explicit `--allow-unverified` keeps the UNVERIFIED label, but still requires complete/fresh report evidence. Archives contain source, verification report/evidence and SHA-256 manifest; path checks reject unsafe Windows names/streams/collisions. Integrity checking proves consistency, not authenticity, licensing or live login success.

`node scripts/package-source.cjs --check <zip>` validates archive integrity. Optional bundle `scripts/preflight.cjs` checks x64 executable/runtime layout, checksums, symlinks and writability without running executables. Do not regenerate checksums merely to hide unexpected changes.

The inherited verification docs mention `.github/workflows/browser-free-verification.yml` outside `root-project/`. **No workflow exists in this checkout or this source ZIP.** Do not claim GitHub Actions is configured/running here based on copied docs or the report's historical workflow fingerprint.

## 10. Exact evidence status for this archive

Bundled `VERIFICATION-REPORT.json`:

- Created **2026-10-14T14:54:11.014Z**, Linux, Node **v22.22.3**.
- Overall **BLOCKED**: **3 PASS**, **10 BLOCKED**, **0 FAIL**.
- PASS: frontend syntax; UI/contract/package subset; source unchanged during verification.
- Go toolchain was not on PATH in that recorded run, so dependent Go/build checks did not execute.
- `liveBrowser`: **NOT_RUN**; `liveLogin`: **NOT_RUN**.
- Source fingerprint: `ea375e4f78c0fbe8f8def63e38818438e76061baa5b6b93f7e5ec72278a0c487`.

Historical docs also report isolated helper tests, prior engine tests, fake-DOM/jsdom simulations and a temporary mirrored-dependency Windows cross-build. Those are historical/scope-limited claims, not a replacement for this archive's full BLOCKED report or a new test run. The archive still awaits the owner's real Windows DOM Show Mode retest.

### Checks performed for this memory-only review

- Enumerated the complete archive and source tree; reviewed architecture, owner rules, docs, entry points, contracts, chat/browser flows, tooling and test inventory.
- Read every ZIP entry for ZIP CRC and manifest verification: **all 169 manifest-listed payload SHA-256 hashes matched**; ZIP CRC check passed.
- Recomputed all three included verification-log hashes: matched the bundled report.
- No application tests, installers, live browser/login, remote license checks or AI requests were run. No package install/build was needed. This review **does not upgrade product verification to PASS**.

## 11. Safe continuation rules and pending work

1. Owner requested **read + memory only** in this task. Do not fix issues, refactor code, deploy, rebuild/repackage the archive, or add a CI workflow under that request.
2. Preserve the ZIP and original embedded docs. This external memory augments them without editing AUTO blocks or invalidating their packaged source fingerprint.
3. Keep Show Mode DOM-only and ordinary worker/API routes separate.
4. Preserve underage blocking, refusal/no-username rules, compatibility migration, temp-profile cleanup and private-data boundaries.
5. Never change licensing, driver/proxy behavior or schema just to make tests pass.
6. Before meaningful future changes, get scope approval; use synthetic/temp-data checks by default; append a dated decision with honest evidence. Ask before destructive/remote actions not already authorized.
7. Pending: owner's Windows retest and evidence-led DOM fixes if needed; complete Windows verification before release; real apitest scenario review before any director integration.
8. For this review, GitHub publication is to session branch `arena/e0b2773e-trynowbaby`; no merge to `main` is implied.

## Decision log

- **2026-10-14 — Read-only project memory:** owner asked to read the project and add memory on GitHub. Added this external handoff for the ZIP-only repository, including architecture, owner decisions, source-review caveats and exact verification limitations. Original ZIP/application source unchanged. Archive CRC, all manifest payload hashes and included log hashes checked successfully; application/live tests NOT_RUN.

- **2026-10-14 — Show Mode implementation authorized:** extracted source without replacing the original archive; implemented exact DOM selectors/observer, ID queue, identity filtering, native input/send confirmation and generation-safe rotation within three Show Mode production files. Added isolated synthetic tests and test wiring; see the current-update section and scoped report. No changes to worker/core engines, normal browser drivers, license, proxy or frontend. Full Windows/live verification remains pending.
