# Privacy

## What ritual reads

Every session transcript your coding agents have written to disk: prompts, model
replies, tool calls, tool output. That is your entire working history. It
contains source code, client names, internal URLs, customer records, and every
credential anyone has ever pasted into a prompt.

## What ritual sends

Nothing. There is no account, no server, no telemetry, and no opt-out setting
for any of those, because none of them exist. The binary reads local files and
writes local files.

The one outbound path is optional and explicit: `ritual build` and `ritual
install` can shell out to an agent CLI you already have installed — `claude`,
`codex`, `opencode`, `gemini`, `ollama` — to improve the wording of a generated
artifact. That subprocess receives the finding (title, step names, counts,
dates, commands) and the template. It never receives a transcript. Disable it
entirely with `--no-author`, or `author = "none"` in config.

## What ritual redacts

At the read boundary, before anything is stored, printed, or served:

- Provider-shaped keys: Anthropic, OpenAI, GitHub, GitLab, Slack, Stripe,
  Shopify, Google, AWS, npm, Vercel, HuggingFace
- Assignment-shaped secrets: anything named `*_TOKEN`, `*_SECRET`, `*_KEY`,
  `PASSWORD`, `CLIENT_SECRET`, `AUTH*` with a value long enough to be one
- `Bearer` tokens and basic-auth credentials inside URLs
- JWTs and PEM private key blocks
- Email addresses (unless `--keep-emails`)
- Absolute home paths, rewritten to `${HOME}`

Tool arguments are additionally truncated: a 40 KB file body in a `Write` call
teaches the miner nothing and is the largest source of accidental content
capture.

## What ritual does not redact

**This is a filter, not a guarantee.** A secret with no recognizable shape
survives:

- A bare password with no surrounding key name
- An internal hostname, IP address, or S3 bucket name
- Customer names, order numbers, or record identifiers in prose
- Proprietary code quoted inside a prompt

Mined output is far smaller than the input — a finding keeps step names, counts,
dates, and a truncated first line per cited session — but a prompt that opened
with a secret can still reach a report's evidence list.

**Before sharing a report or a generated skill, read it.**

## Where ritual writes

| Path | Contents | Permissions |
|---|---|---|
| `~/.ritual/config.toml` | settings | 0600 |
| `~/.ritual/reports/*.json` | full reports, including prompt excerpts | 0600 |
| `~/.ritual/out/<slug>/` | generated artifacts before install | 0600 |

Reports are pruned to the newest twenty. The directory is 0700.

## The dashboard

`ritual ui` binds to `127.0.0.1` and requires a token generated per run and
printed in your terminal. Every route checks it, including reads, because any
page in your browser can issue a cross-origin GET. The page declares a strict
CSP with `default-src 'none'` and cannot reach the network.

It is read-only by default. `--allow-install` lets it write to your agent
directories; without that flag the install endpoint refuses anything that is not
a dry run.

## Why there is no hosted version

The obvious product is a website: run a command, upload the output, get skills
back. It will not be built.

Asking a developer to upload six months of agent transcripts to a third party is
asking them to upload their employer's source code, their customers' data, and
their production credentials. No privacy policy makes that a reasonable thing to
ask, and no amount of convenience makes it a reasonable thing to accept.

The dashboard exists so the browser experience can happen without any of that.
