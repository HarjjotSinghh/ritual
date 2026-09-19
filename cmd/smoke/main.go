package main

import (
	"fmt"
	"time"

	"github.com/HarjjotSinghh/ritual/internal/ingest"
)

func main() {
	res, err := ingest.Scan(ingest.Options{Since: time.Now().AddDate(0, 0, -45)})
	if err != nil {
		panic(err)
	}
	for _, s := range res.Stats {
		fmt.Printf("%-10s sessions=%-5d turns=%-7d tools=%-7d %s..%s\n", s.Agent, s.Sessions, s.Turns, s.ToolCalls, s.Earliest.Format("01-02"), s.Latest.Format("01-02"))
	}
	fmt.Println("total sessions:", len(res.Sessions))
	for i, w := range res.Warnings {
		if i > 4 {
			fmt.Println("  ...", len(res.Warnings)-5, "more warnings")
			break
		}
		fmt.Println("  warn:", w)
	}
	n := 0
	for _, s := range res.Sessions {
		if n >= 3 {
			break
		}
		if len(s.UserTurns()) == 0 {
			continue
		}
		n++
		fmt.Printf("\n[%s] %s | %s | turns=%d\n  prompt: %.140s\n", s.Agent, s.ID, s.Workspace, len(s.Turns), s.UserTurns()[0].Text)
	}
}
