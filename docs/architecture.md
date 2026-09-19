# Architecture

`ritual` is a pipeline with five stages. Each one has a single job, a single
package, and no knowledge of the stages on either side of it.

```
 session stores        normalize          mine                judge            emit
 ─────────────         ─────────          ────                ─────            ────
 ~/.claude      ┐                  ┌ segment into task arcs
 ~/.codex       │   redact ─────►  │ cluster repeated work   ┌ score          ┌ SKILL.md
 ~/.cursor      ├─► one Session ──►│ canonicalize the steps ─┤ classify ─────►┤ AGENTS.md
 ~/.local/share │   shape          │ measure the cadence     └ check inventory└ command / hook
 …              ┘                  └ collect corrections
```

## 1. Discovery — `internal/agentspec`

The catalog of agents: where each one keeps its sessions, what its files are
called, what must never be walked. One `Spec` per agent, one `Layout` per
on-disk shape.

Storage layouts move. This is the only file that has to change when they do,
which is why adding an agent is about forty lines rather than a refactor.

An environment override (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, …) wins over the
default root, and a root only counts when its marker directory exists — an empty
`~/.gemini` does not get to claim sessions it does not have.

## 2. Ingestion — `internal/ingest`

Eleven readers, one output shape: `session.Session`, a list of `Turn` values
with an actor, a kind, a timestamp, and either prose or a tool call.

Three things happen at this boundary and nowhere else:

**Redaction.** Every string passes through `internal/redact` before it exists
anywhere else in the program. Provider-shaped keys, assignment-shaped secrets,
JWTs, private key blocks, and basic-auth URLs are replaced; absolute home paths
become `${HOME}`. See [privacy.md](privacy.md) for what that does and does not
cover.

**Canonicalization.** `Read`, `read_file`, `view`, and `str_replace_editor_view`
are the same verb. Without this, the same workflow mined from two agents looks
like two workflows. Shell commands are reduced to their shape — `git commit -m
"fix the cart"` becomes `git commit` — so a workflow is not split into one
cluster per commit message.

**Noise removal.** Harness scaffolding injected into the user role (system
reminders, environment dumps, slash-command expansion, hook output) is stripped,
and records the harness submitted on the operator's behalf are dropped
entirely. A session with no human turn at all is an automation run, and
counting it would invent workflows nobody performed.

Readers are lenient by construction. A malformed line is skipped and counted; a
crashed agent leaves one in almost every store.

## 3. Mining — `internal/mine`

**Segmentation** cuts sessions into *task arcs*: one prompt, the work that
followed, and any corrections before the operator moved on. A new substantive
prompt ends the previous arc; an acknowledgement ("yes, go ahead") extends it;
a gap longer than ninety minutes splits it, because a session resumed the next
morning is two pieces of work.

**Clustering** groups arcs that are the same work done again. Similarity blends
two views that fail in opposite directions: TF-IDF over the prompt text (which
alone would group "fix the build" with "fix the tests") and Jaccard over the
step sequence and its adjacent pairs (which alone would group everything that
starts with `git status`). A greedy leader pass is followed by a merge pass over
centroids, which removes most of the leader pass's order sensitivity without
reintroducing platform-dependent tie-breaking.

**Canonicalization** reduces a cluster to the sequence its runs share: each
action scored by the share of runs that performed it, ordered by its median
normalized position. Actions below the support floor are dropped — a one-off
`git stash` is noise, not a step.

**Cadence** measures recurrence in *distinct days*, not runs. Twelve runs in one
afternoon while fighting a flaky deploy is one day of evidence, and a tool that
counted it as twelve would be reacting to a bad day.

**Corrections** are detected separately, because they are the highest-value
signal in a transcript. When someone says "no, always run the mobile check",
they are stating a rule the agent did not know and will not know next time
either. Standing preferences ("always", "never", "from now on") are grouped by
shared vocabulary so three restatements become one rule with three citations.

## 4. Judgement — `internal/score`, `internal/classify`, `internal/inventory`

**Scoring** blends nine components — recurrence, regularity, complexity, context
repetition, friction, breadth, time spent, cohesion, corrections — and applies
named penalties for single-day bursts, loose clusters, single-step "workflows",
and sequences made entirely of look-around actions. Every component and every
penalty is reported. See [scoring.md](scoring.md).

**Inventory** reads what is already installed: skills, commands, and rules files
across every harness, plus the repositories the sessions ran in. Its job is to
stop `ritual` from proposing a skill the operator wrote months ago.

**Classification** decides what the finding should become. The order matters: an
invoked skill takes precedence over any heuristic, because proposing to rewrite
a skill the operator just ran is the worst suggestion this tool could make.

## 5. Emission — `internal/artifact`, `internal/authoring`, `internal/install`

Artifacts follow the open Agent Skills layout — a directory with `SKILL.md` and
YAML frontmatter — because every major harness reads it. The frontmatter
description is phrased as a trigger condition, not a summary, since it is the
only thing an agent reads when deciding whether a skill applies.

Every artifact carries its evidence and a provenance comment. A skill whose
origin is invisible is one nobody can audit when it starts firing at the wrong
time.

**Authoring** is optional and strictly bounded: a local agent CLI is given the
finding as JSON and the template as a starting point, and asked to improve only
the wording. The response is validated — frontmatter intact, name unchanged,
provenance preserved, length plausible — and discarded when it drifts. The model
never sees a transcript and never decides what a workflow is.

**Installation** is planned before it is performed. Every target resolves to an
explicit list of writes, `--dry-run` prints them, and an existing file is never
replaced without `--force`. Those directories hold work the operator wrote by
hand.

## Determinism

The same history always produces the same report. Map iteration never reaches
output, ties break lexically, and floating-point comparisons are never used for
ordering without a deterministic fallback. This is a correctness property, not a
cosmetic one: the entire claim of the tool is that its suggestions can be
checked, and a report that shuffles between runs cannot be.
