package main

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	basketball "github.com/k0ngk0ng/wire-board/games/basketball/server"
	bubble "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"sort"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	report := map[string]any{"scope": "native Module.OnTimer and projected JSON only; no shared room envelope, compression or browser timing", "basketballSeed": 1}
	for _, mod := range []gameapi.Module{basketball.Module(), bubble.Module()} {
		ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.UnixMilli(1000000), Automated: []bool{false, false}}
		raw := json.RawMessage(`{}`)
		if mod.ID == "basketball" {
			raw = json.RawMessage(`{"mode":"versus"}`)
		}
		m, e := mod.Start(ctx, raw)
		must(e)
		if mod.ID == "basketball" {
			var s basketball.State
			must(json.Unmarshal(m.Data, &s))
			c := s.Runtime.Config
			c.Match.Seed = 1
			s.Runtime, e = engine.NewRuntime(c)
			must(e)
			m.Data, e = json.Marshal(s)
			must(e)
			for seat := 0; seat < 2; seat++ {
				ctx.Actor = seat
				ack, _ := json.Marshal(map[string]any{"type": "loaded", "epoch": s.Epoch})
				m, e = mod.Apply(ctx, m, ack)
				must(e)
			}
		}
		var sizes []int
		var micros []int64
		var last []byte
		for n := 0; n < 400; n++ {
			ctx.Actor = -1
			ctx.Now = ctx.Now.Add(100 * time.Millisecond)
			started := time.Now()
			m, e = mod.OnTimer(ctx, m)
			must(e)
			v, e := mod.View(m, 0)
			must(e)
			last, e = json.Marshal(v)
			must(e)
			if n >= 250 {
				micros = append(micros, time.Since(started).Microseconds())
				sizes = append(sizes, len(last))
			}
		}
		sort.Ints(sizes)
		sort.Slice(micros, func(i, j int) bool { return micros[i] < micros[j] })
		total := 0
		for _, n := range sizes {
			total += n
		}
		mean := total / len(sizes)
		report[mod.ID] = map[string]any{"samples": len(sizes), "viewMeanBytes": mean, "viewP95Bytes": sizes[len(sizes)*95/100], "projectedBytesPerSecondAt10Hz": mean * 10, "timerProjectEncodeP50Microseconds": micros[len(micros)/2], "timerProjectEncodeP95Microseconds": micros[len(micros)*95/100]}
		if mod.ID == "basketball" && os.Getenv("BASKETBALL_PROFILE_SAMPLE") != "" {
			must(os.WriteFile(os.Getenv("BASKETBALL_PROFILE_SAMPLE"), last, 0600))
		}
	}
	data, e := json.MarshalIndent(report, "", "  ")
	must(e)
	fmt.Println(string(data))
}
