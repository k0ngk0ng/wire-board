// Development-only replay of observed request/IRQ ordering. No source RAM is
// injected after the first real reset; source-state bytes are comparisons only.
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/k0ngk0ng/wire-board/games/basketball/audiostate"
)

type event struct {
	Order  int    `json:"order"`
	Frame  int    `json:"frame"`
	Cycle  int64  `json:"cycle"`
	Kind   string `json:"kind"`
	Work   byte   `json:"work"`
	ID     byte   `json:"id"`
	Driver string `json:"driver"`
}

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		panic("usage: audio_order_replay TRACE.jsonl [--tick-not-ready|--batch-world-requests]")
	}
	mode := "observed"
	if len(os.Args) == 3 {
		mode = os.Args[2]
	}
	if mode != "observed" && mode != "--tick-not-ready" && mode != "--batch-world-requests" {
		panic("unknown experiment mode")
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var state *audiostate.State
	var pending *event
	checks, ticks, requests, skipped, worldChecks := 0, 0, 0, 0, 0
	var failures []map[string]any
	var firstWorkFailure map[string]any
	var firstTurnFailure map[string]any
	var firstReset *event
	inActions := false
	var queued []byte
	for scanner.Scan() {
		var e event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			panic(err)
		}
		if e.Kind == "actions_start" {
			inActions = true
		}
		if e.Kind == "actions_end" {
			inActions = false
			for _, id := range queued {
				if err := state.Request(id); err != nil {
					panic(err)
				}
			}
			queued = nil
			continue
		}
		if e.Kind == "request_before" {
			copy := e
			pending = &copy
			continue
		}
		if e.Kind == "request_after" {
			if pending == nil || pending.ID != e.ID {
				panic("unpaired source request")
			}
			if state == nil && e.ID == 0 {
				state = audiostate.New(pending.Work)
				firstReset = pending
			}
			pending = nil
			if state == nil {
				continue
			}
			if mode == "--batch-world-requests" && inActions && e.ID > 0 && e.ID <= 76 {
				queued = append(queued, e.ID)
				requests++
				continue
			}
			if err := state.Request(e.ID); err != nil {
				panic(err)
			}
			requests++
		} else if e.Kind == "irq_after" {
			if state == nil {
				continue
			}
			raw, err := hex.DecodeString(e.Driver)
			if err != nil || len(raw) != 231 {
				panic("bad driver observation")
			}
			// The original request clears ready while loading. Such an IRQ
			// executes the source entry's early return, not one full Tick.
			if pending != nil && raw[0xe5]&0x80 == 0 {
				skipped++
				if mode != "--tick-not-ready" {
					continue
				}
			}
			if err := state.Tick(); err != nil {
				panic(err)
			}
			ticks++
		} else if state != nil && (e.Kind == "world_start" || e.Kind == "inputs_start" || e.Kind == "actions_start" || e.Kind == "turn_compare") {
			worldChecks++
			if state.WorkByte != e.Work {
				failure := map[string]any{"order": e.Order, "frame": e.Frame, "kind": e.Kind, "work": state.WorkByte, "expected_work": e.Work}
				failures = append(failures, failure)
				if firstWorkFailure == nil {
					firstWorkFailure = failure
				}
				if e.Kind == "turn_compare" && firstTurnFailure == nil {
					firstTurnFailure = failure
				}
			}
			continue
		} else {
			continue
		}
		if e.Driver != "" && pending == nil {
			expected, err := hex.DecodeString(e.Driver)
			if err != nil {
				panic(err)
			}
			got := sourceState(state)
			checks++
			if state.WorkByte != e.Work || !bytes.Equal(got, expected) {
				differences := map[string][2]byte{}
				for i := range got {
					if got[i] != expected[i] {
						differences[fmt.Sprintf("%04x", 0x700+i)] = [2]byte{got[i], expected[i]}
					}
				}
				failure := map[string]any{"order": e.Order, "frame": e.Frame, "kind": e.Kind, "work": state.WorkByte, "expected_work": e.Work, "driver_differences": differences}
				failures = append(failures, failure)
				if state.WorkByte != e.Work && firstWorkFailure == nil {
					firstWorkFailure = failure
				}
			}
		}
		if len(failures) > 20 {
			failures = failures[:20]
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	result := map[string]any{"status": "passed", "mode": mode, "scope": "observed completion ordering, one carried byte at first source reset, no subsequent state injection; not a world scheduler", "first_reset": firstReset, "requests": requests, "complete_ticks": ticks, "skipped_not_ready_irqs": skipped, "state_checks": checks, "world_work_checks": worldChecks, "failures": failures, "first_work_failure": firstWorkFailure, "first_turn_failure": firstTurnFailure}
	if len(failures) != 0 {
		result["status"] = "failed"
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
	if len(failures) != 0 {
		os.Exit(1)
	}
}
