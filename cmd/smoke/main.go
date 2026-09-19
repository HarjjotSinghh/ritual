package main

import (
	"fmt"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/ingest"
	"github.com/HarjjotSinghh/ritual/internal/mine"
)

func main() {
	res, err := ingest.Scan(ingest.Options{Since: time.Now().AddDate(0, 0, -45)})
	if err != nil {
		panic(err)
	}
	out := mine.Run(res.Sessions, mine.DefaultOptions())
	fmt.Printf("arcs=%d clustered=%d candidates=%d rules=%d\n\n", out.Arcs, out.Clustered, len(out.Candidates), len(out.Rules))
	for i, c := range out.Candidates {
		if i >= 12 {
			break
		}
		fmt.Printf("%2d. %-52s x%-3d sess=%-3d coh=%.2f %s\n", i+1, c.Title, c.Occurrences, c.Sessions, c.Cohesion, c.Cadence.Label)
		fmt.Printf("    %s\n", c.Summary)
		fmt.Printf("    repos=%v agents=%v\n", c.Repos, c.Agents)
		fmt.Printf("    tools=%v\n    phrases=%v\n    keywords=%v\n", c.Tools, c.Phrases, c.Keywords)
	}
	fmt.Println("\n--- rules")
	for i, r := range out.Rules {
		if i >= 8 {
			break
		}
		fmt.Printf("%2d. x%-2d %s\n", i+1, r.Occurrences, r.Text)
	}
}
