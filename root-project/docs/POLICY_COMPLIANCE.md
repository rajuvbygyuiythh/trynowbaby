# Policy compliance review — why the account was warned (2026-10-16 UTC)

Owner reported: one ChitChat account chatted with 4–6 strangers, then received an
**Account standing: Warning** — *Off-platform redirection* — with
*"Chatting is paused until you link an account"*, and the site now shows a
*"Link an account to continue chatting"* (Apple / Google) dialog.

This document contains only what is verifiable: the platform's own published
rules, the account-standing screen from the owner's screenshots, and this
repository's own source. It does **not** contain, and must never contain,
instructions for evading detection.

## 1. What the platform published (chitchat.gg/guidelines, updated 2026-09-09)

Warning-level rules ("will result in a warning and may lead to having your
account suspended"):

| Published rule | Our product does it? | Where in this repo |
|---|---|---|
| "Redirecting users to other social platforms without prior meaningful conversations." | **Yes, by design** | `internal/bot/eva_chat.go` pools `ask_snap.txt` → `share_snap.txt` → `snapchat.txt`; assets `internal/bot/eva_assets/output/{ask_snap,share_snap,snapchat}.txt` ("u on snp?", "add me on snp?", "here's my 👉: %username%", "let's switch to sc") |
| "Do not spam the same messages repeatedly." / "Do not send messages in bulk or the same message to a large number of users." | **Yes** | worker `internal/workerplan` + `internal/bot/run.go` run the same sequence pool for many users; identical greetings go out from the same pools |
| "Chitchat.gg is not a dating platform. Sending unsolicited sexual advances may lead to a suspension." | **Yes** | persona in `internal/bot/eva_assets/persona.txt` and the `horny*` / flirty pools, plus the owner's Home transcript (screenshots) |
| "Impersonating minors or pretending to be underage … immediate account termination." | **Risk** | worker/profile names and persona ages in the owner's screenshots ("teen-age sublet", "f21"); the flow asks for age and the persona ages are near the boundary |
| Video-chat rule 02: "Do not use third-party software (extensions, plugins, bots, programs…) to change the functionality of our service." | **Yes** (automation) | this whole repository |

Immediate-termination list (account termination **and** possible indefinite
blacklisting) that the current funnel could wander into: soliciting money,
facilitating the sale of illegal content, harassment, and using the service in a
way that is not reasonable under normal usage.

Enforcement mechanism as the platform describes it:

> "Along with the report, our moderators receive a screenshot of the user's video
> stream and their message (if any) … Our system automatically bans any user who
> gets numerous reports within a short amount of time."

Two conclusions follow directly from that sentence:

1. **User reports are a first-class input.** Several strangers being pushed to
   Snap/Telegram can report, and a cluster of reports is enough for automated
   action regardless of how "human" the traffic looks.
2. **The message content is inspected.** The warning text itself is a summary of
   what was said: *"You asked someone to move this chat to another app."*
   That is our `ask_snap` / `share_snap` / `snapchat` pools and the AI funnel
   prompts (`internal/bot/eva_ai.go`: *"ask if they have snapchat / want to move
   to snap"*, *"tell them to hit you up on snap"*).

## 2. Why proxy/fingerprint/timing work cannot fix this

The account standing screen is an **account-level moderation record**, not a
network-layer detection:

- It does not care which IP or country the browser used. Proxies change nothing
  about it — the warning exists because of what was typed to real users.
- The required action the platform demands is **linking an account**
  (`Account standing → Link an account`, Apple/Google). That is an identity
  requirement, the opposite of an IP requirement.
- Therefore: better proxies, "more human" timing, or a bigger pool of accounts
  does not remove the reason the account was flagged. It only changes how many
  accounts carry that same reason.

Because of that, this repository will **not** build or document anything aimed at
evading platform enforcement — no detection-bypass research, no anti-fingerprint
tricks, no ban-evasion account rotation. It would not work against a
content-and-report based system, it escalates the platform's response (Warning →
Limited → At risk → Suspended, then possible blacklisting), and it turns a
fixable product decision into a permanent one.

## 3. What actually exists in the product today (honest inventory)

- The EVA flow is a **redirection funnel**: `ask_snap` → `share_snap` →
  `snapchat`, with the handle injected from the Message → Usernames list.
- The AI compose layer is instructed to steer the conversation toward Snap, and
  its "forbidden" regex (`evaAIForbidden`) only blocks literal
  `telegram|t.me|whatsapp|@|http`, so spellings like "telgm", "sc", "snp", "tg"
  pass through and reach users — as seen in the owner's transcript
  (`Bot: so telgm`, `Bot: add me wewqq78`).
- Bulk behaviour is native to the bot: one persona, one sequence pool, many
  targets, unattended.

So the warning is not a bug in the bot, and not a proxy problem: **the bot is
doing exactly what the platform publishes as a violation.**

## 4. The two sustainable paths (choose one; they do not mix)

### A. Compliant chat bot on this platform

Remove the redirection entirely and reduce the spam surface:

1. `internal/bot/eva_chat.go` — delete the `ask_snap` / `share_snap` /
   `snapchat` stages from the state machine; MIDDLE falls back to
   `middle_chat/*` and `flirty_questions` only.
2. `internal/bot/eva_assets/` — remove/replace the three snap pools and the
   snap-related input matchers; keep the rest of the TXT engine.
3. `internal/bot/eva_ai.go` — drop the "ask for snap / hit me up on snap"
   instructions; keep the persona but forbid off-platform references for real
   (all spellings), and require: no sexual solicitation, no age below 18
   anywhere in persona/profile names.
4. Unknown AIs/config UI — the Message → Usernames handle list becomes unused
   for chat (or is limited to in-app content the platform supports).
5. Worker — add per-account pacing and a daily cap, and refuse to start if the
   compliance guard is off. This is not to "look human"; it is to stop
   same-message bulk sending, which is a published violation.
6. Dashboard — a "Compliance: no off-platform links/asks" state that is visible
   and cannot be silently disabled.

Every one of those is implementable inside the already-authorized scope (EVA
pools + AI layer + dashboard), with the existing test style, and it is the only
version of this bot that can plausibly survive on ChitChat.

### B. Keep the off-platform funnel, but not on a platform that forbids it

If the funnel to Snap/Telegram is the product, this platform is the wrong place
for it — permanently, not temporarily. The honest options are a channel that
allows that traffic (your own site/app, or a network whose terms permit it,
with its own consent and disclosure rules), or dropping the funnel. No amount of
technical work changes a platform's rules, and repeatedly re-registering to
continue is what gets people blacklisted.

## 5. Account recovery for the warned account (no evasion)

- The account is not dead: *"You still have full access"* and the warning is
  limited in time (*"affects your standing until Oct 16, 2026, 3:15 AM"*), but
  chatting stays paused until an account is linked.
- Linking Apple/Google is the platform's requested step; it is one-time and
  expected of normal users too.
- Do **not** create another account to continue the same behaviour. That is ban
  evasion: the second account inherits the same cause, and the published
  guidelines allow indefinite blacklisting.
- If any action was taken in error, their stated route is
  `support@chitchat.gg` with an appeal — useful only for genuine mistakes, not
  for a confirmed funnel.

## 6. Evidence trail in this repo

- Rule text: fetched from `https://www.chitchat.gg/guidelines/` (last updated
  2026-09-09) on 2026-10-16 UTC.
- Our behaviour: `internal/bot/eva_chat.go` (pool names and stage order),
  `internal/bot/eva_assets/output/{ask_snap,share_snap,snapchat}.txt`,
  `internal/bot/eva_ai.go` (AI funnel prompts, `evaAIForbidden`), plus the
  owner's own Home transcript screenshots (`Bot: so telgm`,
  `Bot: add me wewqq78`).
- Account state: owner screenshots of Account standing → Warning →
  Off-platform redirection, and the "Link an account to continue chatting"
  dialog.

Nothing in this change modified product code: this file exists so the finding is
recorded and the next session cannot repeat the "why did it get flagged"
investigation from zero.
