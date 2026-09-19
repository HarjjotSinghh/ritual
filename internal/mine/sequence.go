package mine

import (
	"sort"
	"strings"
)

// CanonicalSteps reduces a cluster's arcs to the ordered sequence they share.
//
// The arcs of one workflow are never identical: one run skipped a check,
// another retried a failing command four times, a third did the steps in a
// slightly different order because the human interrupted. What is stable is
// which actions appear in most runs and roughly where. So each action is scored
// by the share of arcs that performed it, and ordered by its median position
// across those arcs.
//
// minSupport drops actions that appear in too few runs to be part of the
// workflow: a one-off `git stash` in a single arc is noise, not a step.
func CanonicalSteps(arcs []Arc, minSupport float64) []Step {
	if len(arcs) == 0 {
		return nil
	}
	type agg struct {
		count     int
		positions []float64
	}
	seen := make(map[string]*agg, 32)
	for _, a := range arcs {
		if len(a.Steps) == 0 {
			continue
		}
		// Position is normalized to 0..1 so a 4-step arc and a 40-step arc
		// contribute comparable evidence about ordering.
		span := float64(len(a.Steps) - 1)
		local := make(map[string]float64, len(a.Steps))
		for i, s := range a.Steps {
			pos := 0.0
			if span > 0 {
				pos = float64(i) / span
			}
			if _, ok := local[s]; !ok {
				local[s] = pos
			}
		}
		for s, pos := range local {
			e, ok := seen[s]
			if !ok {
				e = &agg{}
				seen[s] = e
			}
			e.count++
			e.positions = append(e.positions, pos)
		}
	}

	total := float64(len(arcs))
	out := make([]Step, 0, len(seen))
	for action, e := range seen {
		support := float64(e.count) / total
		if support < minSupport {
			continue
		}
		out = append(out, Step{
			Action:   action,
			Support:  round3(support),
			Count:    e.count,
			Position: round3(median(e.positions)),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Action < out[j].Action
	})
	return out
}

// DescribeStep renders a canonical action as an imperative instruction. The
// generated artifacts are read by both humans and agents, and "shell:npm run
// build" is neither.
func DescribeStep(action string) string {
	if cmd, ok := strings.CutPrefix(action, "shell:"); ok {
		return "Run `" + cmd + "`"
	}
	if server, ok := strings.CutPrefix(action, "mcp:"); ok {
		parts := strings.SplitN(server, ":", 2)
		if len(parts) == 2 {
			return "Call the " + parts[0] + " integration (" + strings.ReplaceAll(parts[1], "-", " ") + ")"
		}
		return "Call the " + parts[0] + " integration"
	}
	if tool, ok := strings.CutPrefix(action, "tool:"); ok {
		return "Use " + strings.ReplaceAll(tool, "-", " ")
	}
	switch action {
	case "read":
		return "Read the relevant files"
	case "write":
		return "Write the new file"
	case "edit":
		return "Edit the affected files"
	case "search":
		return "Search the codebase for the affected code"
	case "list":
		return "List the candidate files"
	case "shell":
		return "Run the project's commands"
	case "web_fetch":
		return "Fetch the referenced page"
	case "web_search":
		return "Search the web for current information"
	case "browser":
		return "Verify in a browser"
	case "subagent":
		return "Delegate the independent parts to subagents"
	case "plan":
		return "Write the task list before starting"
	case "ask":
		return "Ask the operator to resolve the open decision"
	case "skill":
		return "Invoke the matching skill"
	}
	return "Perform " + strings.ReplaceAll(action, "_", " ")
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func round3(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}
