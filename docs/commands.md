# Commands

Every command reads the local filesystem and writes to it. None of them make a
network call, with the single exception noted under `build`.

## `ritual scan`

Reads every installed agent's session store, mines it, and saves the report.

| Flag | Default | Effect |
|---|---|---|
| `--days` | 90 | How far back to read. `0` reads everything. |
| `--agents` | all installed | Restrict to catalog keys: `claude,codex,cursor,…` |
| `--workspace` | — | Only sessions whose path contains this text. |
| `--min-score` | 28 | Below this a finding is classified `ignore`. |
| `--min-runs` | 3 | Runs needed before a workflow is proposed. |
| `--min-sessions` | 2 | Distinct sessions a workflow must span. |
| `--threshold` | 0.42 | Clustering similarity cutoff, 0..1. |
| `--limit` | 15 | Findings to print. `0` prints all. |
| `--all` | off | Include findings classified as not worth an artifact. |
| `--include-automated` | off | Include sessions with no human turn — scheduled runs, SDK agents, background tasks. |
| `--keep-emails` | off | Keep email addresses instead of redacting them. |
| `--no-inventory` | off | Skip the scan of already-installed skills and rules. |
| `--json` | off | Emit the whole report as JSON. |
| `--no-save` | off | Do not write the report to `~/.ritual/reports`. |

## `ritual list`

The last scan, without rescanning. Same `--limit`, `--all`, and `--json`.

## `ritual show <id>`

Everything behind one finding: score components, canonical steps with per-step
support, corrections issued during those runs, and the cited sessions with their
transcript paths. Accepts an id prefix or a slug.

## `ritual build <id>...`

Writes the artifact files to `~/.ritual/out/<slug>/`.

| Flag | Effect |
|---|---|
| `--out` | Write somewhere else. |
| `--stdout` | Print the files instead of writing them. |
| `--author` | Agent CLI used to improve the prose: `auto`, `none`, `claude`, `codex`, `opencode`, `gemini`, `qwen`, `ollama`. |
| `--no-author` | Never call an agent CLI. |

The authoring step is the only outbound call in the program, it runs a binary
already installed on the machine, and it receives the finding rather than any
transcript.

## `ritual install <id>...`

Plans the writes, prints them, and asks before applying.

| Flag | Effect |
|---|---|
| `--to` | Harnesses: `claude`, `codex`, `cursor`, `opencode`, `gemini`, `grok`, `qwen`, `pi`, `shared`. |
| `--project` | Also install into the current repository. |
| `--dry-run` | Print the plan and stop. |
| `--force` | Replace files that already exist. |
| `--yes` | Skip the confirmation. |

`ritual install targets` lists the harnesses and whether each is set up here.

A finding classified `update` is a drift report about a skill you already have.
Installing it is refused; read it with `ritual build` instead.

## `ritual rules` / `ritual rules install <id>`

The standing preferences you have restated across sessions, and appending one to
your rules files. Appending is duplicate-safe: running it twice does not add the
line twice.

## `ritual ui`

Serves the last report on `127.0.0.1:4783` behind a per-run token.

| Flag | Effect |
|---|---|
| `--addr` | Bind address. Keep it on loopback. |
| `--allow-install` | Let the page write to your agent directories. |
| `--no-open` | Do not launch a browser. |

## `ritual agents`

Every agent in the catalog, every location walked, and how many session files
were found. `--all` includes agents not installed here.

It counts **files**, not sessions. Most stores write one file per session, but a
database holds thousands in one file, and an agent's file count is usually much
higher than its session count because subagent transcripts and runs with no
human turn are excluded. `ritual doctor` reports both numbers.

When an environment override such as `CODEX_HOME` points somewhere new, ritual
reads the default location too and de-duplicates. The variable says where the
agent writes now; the old path often still holds most of the history.

## `ritual inventory`

The skills, commands, and rules files already installed, which is what stops
ritual proposing something you wrote months ago.

## `ritual doctor`

What ritual can and cannot see: its own directory, config, each agent's store,
which agent CLIs are available for authoring, and the warnings from the last
scan.

Per agent it prints `N files → M sessions parsed`. `M` far below `N` is normal
and explained inline. `M` of zero against a non-zero `N` is a bug in the reader
or the layout, is marked in red, and is worth reporting.

## `ritual eval`

The benchmark. Uses your hand-written skills as ground truth by re-mining only
the sessions that predate each one.

| Flag | Effect |
|---|---|
| `--only` | Evaluate only skills whose name contains this text. |
| `--min-sessions` | History a skill needs before a miss counts against the tool. |
| `--match` | Vocabulary overlap at which a finding counts as the same workflow. |
| `--json` | Emit the benchmark as JSON. |

## `ritual config`

Prints the current settings and where they come from. `--init` writes
`~/.ritual/config.toml` with the current values.
