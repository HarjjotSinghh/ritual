<div align="center">

# ritual

**Your coding agents have been writing down how you work. Read it back.**

Cross-agent process mining for developers: find the work you repeat, and turn
it into skills, rules, commands, and hooks.

[![CI](https://github.com/HarjjotSinghh/ritual/actions/workflows/ci.yml/badge.svg)](https://github.com/HarjjotSinghh/ritual/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/HarjjotSinghh/ritual.svg)](https://pkg.go.dev/github.com/HarjjotSinghh/ritual)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

[ritual-harjjot.vercel.app](https://ritual-harjjot.vercel.app)

</div>

---

Every coding agent you use writes a transcript to disk. Months of them are
sitting in `~/.claude`, `~/.codex`, `~/.cursor` and half a dozen other
directories right now — every task you gave, every command that ran, every time
you had to say *"no, also check mobile"* for the fourth time.

That is a record of how you actually work, and nothing reads it.

`ritual` does. It reads every agent's history on your machine, finds the
procedures you repeat, and writes them out as artifacts your agents can use.

Example output — yours will name your own work:

```
$ ritual scan

Scanned
  claude       211 sessions   21200 turns    9123 tool calls  2026-07-02 → 2026-09-19
  codex         39 sessions    2841 turns    1204 tool calls  2026-07-14 → 2026-09-18
  cursor        25 sessions    5016 turns    3676 tool calls  2026-08-02 → 2026-09-15
  opencode      10 sessions    1636 turns     791 tool calls  2026-09-01 → 2026-09-17
  831 task arcs, 215 of them part of a repeating workflow, in 9.8s

Found 6 recurring workflows
  id         score  kind      runs  days  workflow
  360119359c   71   skill       14     9  Verify the storefront after a theme push
             Ran 14 times across 12 sessions in storefront, about daily.
  0248c624a4   64   skill        9     9  Draft the end-of-day update
             Ran 9 times across 9 sessions in storefront, about daily.
  0452ba80b6   58   hook         7     6  Run the regression pass after a push
             Ran 7 times across 7 sessions in 2 repositories, every few days.
  479d0ebd2b   44   command      6     5  Summarise the open pull requests
  0c5ff3cbbe   31   reference    5     4  Where the billing webhooks are wired
  c54cf75eea   12   ignore       9     2  Read a file, then read another file

Found 3 standing preferences you keep restating
  5154ac55f7  x4 Always check the mobile layout before reporting done.
  80800a871b  x3 Never edit the generated schema files directly.
  a91b2c3d4e  x2 Use the existing analytics event names; do not invent new ones.
```

Then:

```bash
ritual show 360119359c    # every run it came from, with the transcript paths
ritual build 360119359c   # write the SKILL.md
ritual install 360119359c # into Claude Code, Codex, Cursor, OpenCode, …
```

## Why this is not another skill generator

Several tools turn Claude Code history into skills. `ritual` differs in three
ways that matter.

**It reads every agent, not one.** Claude Code, Codex CLI, Cursor CLI,
OpenCode, Gemini CLI, Grok CLI, Qwen Code, Kimi Code, Copilot CLI, Cline, and
Pi. A workflow you do in Cursor on Monday and in Codex on Thursday is one
workflow, and only a tool that reads both can see that.

**It decides where a pattern belongs, not just that one exists.** A repeated
behaviour can be six different things, and putting it in the wrong place is
worse than leaving it alone:

| Verdict | When | Why not a skill |
|---|---|---|
| `skill` | a multi-step procedure with judgement in it | — |
| `command` | two or three steps, run on demand | a skill is ceremony around two actions |
| `rule` | a standing preference: *always*, *never* | a preference you have to invoke is one you will forget |
| `hook` | work that always follows an event | you should not have to remember it at all |
| `reference` | the agent keeps rediscovering the same facts | write the answer down once |
| `update` | you already have a skill for this | adding a second one is how skill directories rot — `ritual build` writes a drift report instead |
| `ignore` | real pattern, not worth an artifact | `git status` runs a hundred times a week |

**Every suggestion carries its receipts.** No finding is a vibe. Each one
reports how many separate days it happened on, which sessions it came from,
which steps appeared in what share of runs, and what the score is made of:

```
Canonical sequence
   1. Run `shopify theme push`                      ██████████ 14/14 runs
   2. Verify in a browser                           ██████████ 14/14 runs
   3. Run `npm run test:e2e`                        ████████·· 11/14 runs
   4. Read the relevant files                       █████····· 7/14 runs

Score breakdown
  recurrence   ██████████████████·· 0.91
  regularity   ███████████████····· 0.74
  complexity   ████████████████████ 1.00
  repetition   ██████████████·····  0.71
  friction     ████████············ 0.40
  ...
```

## Nothing leaves your machine

There is no account, no upload, and no telemetry — not as a setting you can turn
off, but because the input is your entire working history. It contains client
code, customer data, internal URLs, and every credential anyone ever pasted into
a prompt. A tool that shipped that anywhere would be indefensible.

So `ritual` is a single binary that reads local files and writes local files.
Secrets are stripped at the read boundary, home paths are rewritten to
`${HOME}`, and the dashboard (`ritual ui`) binds to loopback behind a token and
is read-only unless you say otherwise.

The optional prose-polishing step shells out to an agent CLI you have already
installed and trusted — `claude`, `codex`, `opencode`, `gemini`, `ollama` — and
sends it only the finding, never the transcripts. Skip it with `--no-author` and
you get the deterministic template, which is complete, if flatter.

## Install

```bash
# Go — builds from source, needs Go 1.26+
go install github.com/HarjjotSinghh/ritual/cmd/ritual@latest

# Homebrew (macOS)
brew install --cask HarjjotSinghh/tap/ritual

# Scoop (Windows)
scoop bucket add harjjotsinghh https://github.com/HarjjotSinghh/scoop-bucket
scoop install ritual
```

Or grab a signed archive from
[releases](https://github.com/HarjjotSinghh/ritual/releases) — macOS, Linux, and
Windows on amd64 and arm64.

> The Homebrew and Scoop entries are published by the release workflow only when
> a `TAP_TOKEN` secret with write access to the tap repositories is configured.
> Until then, use `go install` or the release archives.

## Use

```bash
ritual scan                     # read your history and rank what it finds
ritual scan --days 30           # only the last month
ritual scan --agents claude,codex
ritual list                     # the last scan, without rescanning
ritual show <id>                # the evidence behind one finding
ritual build <id>               # write the artifact to ~/.ritual/out
ritual install <id> --to claude,codex
ritual rules                    # the preferences you keep restating
ritual rules install <id>       # append one to your rules files
ritual ui                       # the same report, in a browser, locally
ritual agents                   # where ritual looked, and what it found
ritual doctor                   # what it can and cannot see
ritual eval                     # would it have found the skills you wrote?
```

### `ritual eval` — the honest test

The hardest question about a tool like this is whether it finds workflows a
person would actually have written down. `ritual eval` answers it using your own
skills as an answer key: for each skill you wrote by hand, it re-runs the miner
over only the sessions that predate it and checks whether the workflow shows up
in the ranking.

The numbers below are an example run. Yours depend entirely on how much history
predates each skill — `ritual eval` prints that alongside every result, because
a miss with five sessions of history says nothing about the tool.

```
Benchmark
  12 skills evaluated, 3 skipped for want of history, in 14s

  recall@1   ██████·················· 25%
  recall@3   ███████████············· 42%
  recall@5   ██████████████·········· 58%
  recall@10  ███████████████████····· 75%
  MRR        ████████················ 0.34

Per skill
  ✓ storefront-verify          rank 1, 74% match — Verify the storefront after a theme push
  ✓ daily-digest            rank 3, 61% match — Draft the end-of-day update
  ✗ incident-patch            not surfaced (58 sessions of history)
```

It is a benchmark that can fail in public, which is the only kind worth
shipping.

## Where your agents keep their history

| Agent | Location | Format |
|---|---|---|
| Claude Code | `~/.claude/projects/<slug>/*.jsonl` | JSONL |
| Codex CLI | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` | JSONL |
| Cursor CLI | `~/.cursor/projects/<slug>/agent-transcripts/**/*.jsonl` | JSONL |
| OpenCode | `~/.local/share/opencode/opencode.db` | SQLite |
| Gemini CLI | `~/.gemini/tmp/<hash>/chats/*.json` | JSON |
| Grok CLI | `~/.grok/sessions/<cwd>/<id>/chat_history.jsonl` | JSONL |
| Qwen Code | `~/.qwen/projects/<slug>/chats/*.jsonl` | JSONL |
| Kimi Code | `~/.kimi-code/sessions/**/state.json` | JSON |
| Copilot CLI | `~/.copilot/session-state/**/events.jsonl` | JSONL |
| Cline | `~/.cline/tasks/<id>/api_conversation_history.json` | JSON |
| Pi | `~/.pi/agent/sessions/<slug>/*.jsonl` | JSONL |

`ritual agents` prints what it found on your machine. Storage layouts move;
[`internal/agentspec`](internal/agentspec/spec.go) is the one file that has to
change when they do, and [a pull request adding an agent](CONTRIBUTING.md) is
about forty lines.

## How it works

```
 session stores        normalize          mine                judge            emit
 ─────────────         ─────────          ────                ─────            ────
 ~/.claude      ┐                  ┌ segment into task arcs
 ~/.codex       │   redact ─────►  │ cluster repeated work   ┌ score          ┌ SKILL.md
 ~/.cursor      ├─► one Session ──►│ canonicalize the steps ─┤ classify ─────►┤ AGENTS.md
 ~/.local/share │   shape          │ measure the cadence     └ check inventory└ command / hook
 …              ┘                  └ collect corrections
```

Every stage is deterministic: the same history always produces the same report,
because a suggestion you cannot reproduce is a suggestion you cannot argue with.
No model is involved in deciding what a workflow is.

Details in [docs/architecture.md](docs/architecture.md), the scoring model —
every weight and every penalty — in [docs/scoring.md](docs/scoring.md), and the
full flag reference in [docs/commands.md](docs/commands.md).

## Configuration

`~/.ritual/config.toml`, created by `ritual config --init`:

```toml
days = 90               # default lookback
min_score = 28          # below this, a finding is marked "ignore"
min_occurrences = 3     # runs needed before a workflow is proposed
min_sessions = 2        # distinct sessions needed
threshold = 0.42        # clustering similarity cutoff
author = "auto"         # agent CLI for prose, or "none"
install = ["claude"]    # default install targets
```

## Limitations

Worth knowing before you trust it:

- **Redaction is a filter, not a guarantee.** A credential with a recognizable
  shape is caught. A bare password or an internal hostname is not.
- **Cursor and Gemini record no per-turn timestamps.** Sessions from those
  agents are dated by file, which is good enough for cadence and not good enough
  for ordering within a session.
- **Clustering is lexical.** Two prompts describing the same work in entirely
  different words may not group. Cohesion is reported on every finding so you
  can see when a cluster is loose.
- **It cannot see why.** `ritual` knows a step happened in eleven of fourteen
  runs; it does not know what you were thinking on the other three. The
  generated artifact says what, and leaves why to you.

## Contributing

Adding an agent is the most useful contribution, and the smallest: one entry in
`internal/agentspec`, one reader in `internal/ingest`, one fixture, one test.
See [CONTRIBUTING.md](CONTRIBUTING.md).

## Prior art

`ritual` exists because several people had this idea at once and stopped at
Claude Code. [skill-workshop](https://github.com/longlo8061/skill-workshop) and
[skill-miner](https://github.com/SohamBanerjee853/skill-miner) mine Claude
history into skills; [stratless](https://github.com/stratless-ai/stratless)
covers Claude and Codex; [session-aggregator](https://github.com/jayshah5696/session-aggregator)
and [Agent Sessions](https://github.com/jazzyalex/agent-sessions) built
excellent multi-agent ingestion but aim at search rather than process mining.
The storage research in [reinstate](https://github.com/HarjjotSinghh/reinstate)
made the adapter layer here possible.

## License

Apache 2.0. See [LICENSE](LICENSE).
