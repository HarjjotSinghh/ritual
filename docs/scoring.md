# Scoring

Raw frequency is actively misleading. `git status` runs a hundred times a week
and deserves no skill; a nine-step release verification run every Friday
deserves one. What separates them is not how often they happen but how much
undocumented judgement they carry.

Every component below is 0..1. The total is their weighted sum, multiplied by
each penalty's remainder, scaled to 0..100. `ritual show <id>` prints all of it.

## Components

| Component | Weight | What it measures |
|---|---|---|
| `recurrence` | 0.20 | Distinct **days** the workflow ran, on a log curve that saturates near twelve. Not runs — a dozen retries in one afternoon is one day of evidence. |
| `regularity` | 0.10 | How evenly spaced those days are. Evenly spaced runs score near 1; a burst followed by silence scores near 0. |
| `complexity` | 0.18 | Step count, median turns, and how many steps are *not* look-around actions. This is where undocumented judgement lives. |
| `repetition` | 0.12 | How much context is restated each time, measured by the strength of the phrases the prompts share. Literal evidence that instructions are being re-typed. |
| `friction` | 0.10 | Error rate and correction count. Failures mean the agent needed information the prompt did not carry. |
| `breadth` | 0.08 | Distinct repositories and agents. Surviving a change of project or tool is the clearest sign of a practice rather than a project detail. |
| `time_spent` | 0.08 | Median duration times run count, saturating at four hours. The crudest estimate of what automation returns. |
| `cohesion` | 0.09 | Mean similarity of the cluster's runs to its centroid. A low value means the grouping is loose and the finding is suspect. |
| `corrections` | 0.05 | Standing preferences issued during these runs, weighted above one-off fixes. |

## Penalties

Each is a multiplicative reduction with a stated reason.

| Penalty | Reduction | When |
|---|---|---|
| `generic` | 50% | Over 80% of steps are read / list / `git status` — someone looking around, not a procedure. |
| `single-step` | 35% | One action. That is a command or an alias. |
| `loose-cluster` | 30% | Cohesion under 0.35. This may be several workflows wearing one name. |
| `single-day` | 45% | Every run on one day. A task, not a habit. |
| `in-session-loop` | 25% | Five or more runs across fewer than three sessions — usually retrying, not repeating. |

## Tuning

Weights live in `score.DefaultWeights()` and can be overridden in
`~/.ritual/config.toml`. They were set by running `ritual eval` against
histories where the operator had already written skills by hand, and adjusting
until the mined ranking put those workflows near the top.

If you disagree with a ranking, `ritual show <id>` tells you exactly which
component produced it. That is the point: an argument about a weight is a useful
argument, and an argument about an opaque score is not.

## Thresholds

| Setting | Default | Effect |
|---|---|---|
| `min_score` | 28 | Below this a finding is classified `ignore` and hidden without `--all`. |
| `min_occurrences` | 3 | Runs needed before a cluster is a candidate at all. |
| `min_sessions` | 2 | Distinct sessions needed. Four runs inside one session is a loop. |
| `threshold` | 0.42 | Clustering similarity cutoff. Higher means more, tighter clusters. |
