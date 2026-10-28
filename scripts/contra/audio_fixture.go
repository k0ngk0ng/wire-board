//go:build ignore

// Export production-controller views for native-media acceptance. This is an
// input schedule, never a runtime ROM or manually authored AudioView fixture.
package main

import (
	"encoding/json"
	contra "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"os"
)

func main() {
	state := contra.NewAudioState("native-audio-fixture")
	views := map[string]contra.AudioView{"empty": state.View()}
	sound := func(id uint8) { state.Queue(contra.AudioCommand{Kind: "sound", ID: id}) }
	sound(0x46)
	views["music"] = state.Advance(false).View
	for state.Tick < 4 {
		state.Advance(false)
	}
	sound(0x1a)
	views["shot"] = state.Advance(false).View
	sound(0x74)
	views["pause"] = state.Advance(true).View
	for state.Tick < 46 {
		state.Advance(true)
	}
	views["pauseEnd"] = state.View()
	sound(0x1a)
	views["resume"] = state.Advance(false).View
	state.Queue(contra.AudioCommand{Kind: "reset"})
	sound(0x5a)
	views["reset"] = state.Advance(false).View
	for state.Tick < 1000 {
		state.Advance(false)
	}
	views["gap"] = state.View()
	terminal := contra.NewAudioState("native-terminal")
	views["terminalEmpty"] = terminal.View()
	terminal.Queue(contra.AudioCommand{Kind: "sound", ID: 0x6e})
	views["terminalStart"] = terminal.Advance(false).View
	for range 4 {
		terminal.Advance(false)
	}
	views["terminal"] = terminal.View()
	if err := json.NewEncoder(os.Stdout).Encode(views); err != nil {
		panic(err)
	}
}
