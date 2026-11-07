package contra

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// AudioCommand is emitted by authoritative game rules in their original order.
// It is queued after the current audio update and consumed at the next boundary.
type AudioCommand struct {
	Kind string `json:"kind"` // "reset" or "sound"; reset is not a sound ID.
	ID   uint8  `json:"id,omitempty"`
}

type audioPoint struct {
	Tick uint64 `json:"tick"`
	ID   uint8  `json:"id"`
}
type audioEndCallback struct {
	ID                      uint8 `json:"id"`
	RequiresEndPlayersAlive bool  `json:"requiresEndPlayersAlive"`
}
type audioSlotDefinition struct {
	Slot      uint8              `json:"slot"`
	Release   int32              `json:"release"` // -1 is a verified persistent sequence.
	Prefix    []audioPoint       `json:"prefix"`
	Loop      []audioPoint       `json:"loop"`
	LoopStart uint64             `json:"loopStart"`
	LoopTicks uint64             `json:"loopTicks"`
	End       []audioEndCallback `json:"end"`
}
type audioDefinition struct {
	ID    uint8                 `json:"id"`
	Kind  string                `json:"kind"`
	Slots []audioSlotDefinition `json:"slots"`
}

//go:embed audio_control_data.json
var audioControlData []byte

var audioRegistry = func() struct {
	definitions map[uint8]audioDefinition
	targets     map[uint8][]uint8
} {
	var data struct {
		Version     int               `json:"version"`
		Definitions []audioDefinition `json:"definitions"`
		Targets     []struct {
			ID    uint8   `json:"id"`
			Slots []uint8 `json:"slots"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(audioControlData, &data); err != nil || data.Version != 2 {
		panic("invalid embedded Contra audio control data")
	}
	result := make(map[uint8]audioDefinition, len(data.Definitions))
	for _, definition := range data.Definitions {
		if _, exists := result[definition.ID]; exists {
			panic("duplicate Contra sound definition")
		}
		for _, slot := range definition.Slots {
			if slot.Slot >= 6 || slot.Release < -1 {
				panic("invalid Contra sound slot definition")
			}
		}
		result[definition.ID] = definition
	}
	targets := map[uint8][]uint8{}
	for _, target := range data.Targets {
		for _, slot := range target.Slots {
			if slot >= 6 {
				panic("invalid Contra request target")
			}
		}
		targets[target.ID] = append([]uint8(nil), target.Slots...)
	}
	if len(targets) != 126 {
		panic("incomplete Contra request-target table")
	}
	return struct {
		definitions map[uint8]audioDefinition
		targets     map[uint8][]uint8
	}{result, targets}
}()

var audioDefinitions = audioRegistry.definitions

type AudioSlotState struct {
	ID         uint8  `json:"id"`
	Generation uint64 `json:"generation"`
	Age        uint64 `json:"age"`
	Release    int32  `json:"release"`
	Started    bool   `json:"started"`
	LastUpdate uint64 `json:"lastUpdate"`
}

// AudioVoice describes a resolved lane, not a player or an AudioContext. Position
// is measured in original updates. Only the media adapter resolves resources.
type AudioVoice struct {
	Key           string `json:"key"`
	ID            uint8  `json:"id"`
	Channel       string `json:"channel"`
	PlaybackID    string `json:"playbackID"`
	PositionTicks uint64 `json:"positionTicks"`
	Paused        bool   `json:"paused"`
	Muted         bool   `json:"muted"`
	Restore       bool   `json:"restore"`    // persistent music only, never old effects.
	Continuous    bool   `json:"continuous"` // requires a separately verified media loop.
}

type AudioEvent struct {
	Sequence uint64      `json:"sequence"`
	Tick     uint64      `json:"tick"`
	Action   string      `json:"action"`
	Key      string      `json:"key,omitempty"`
	Value    bool        `json:"value"`
	Voice    *AudioVoice `json:"voice,omitempty"`
}

type AudioView struct {
	Epoch    string       `json:"epoch"`
	Sequence uint64       `json:"sequence"`
	Tick     uint64       `json:"tick"`
	Paused   bool         `json:"paused"`
	SlotIDs  [6]uint8     `json:"slotIDs"`
	Voices   []AudioVoice `json:"voices"`
	Events   []AudioEvent `json:"events"`
}

type AudioUpdate struct {
	Events         []AudioEvent `json:"events"`
	View           AudioView    `json:"view"`
	UnsupportedIDs []uint8      `json:"unsupportedIDs,omitempty"`
}

// AudioState is a serializable control timeline. It contains no ROM, synthesized
// audio, media handles or URLs. Pending commands and published identities survive
// saves, preventing a restored game from assigning fresh IDs to old effects.
// AudioContext carries $0194 from Campaign.AliveAtExit, computed by the
// original CDF8 transition. It is not the current live-player count.
type AudioContext struct {
	EndPlayersAlive uint8 `json:"endPlayersAlive"`
}

type AudioState struct {
	Context        AudioContext          `json:"context"`
	Version        uint8                 `json:"version"`
	Epoch          string                `json:"epoch"`
	Tick           uint64                `json:"tick"`
	Sequence       uint64                `json:"sequence"`
	NextGeneration uint64                `json:"nextGeneration"`
	Paused         bool                  `json:"paused"`
	Slots          [6]AudioSlotState     `json:"slots"`
	Pending        []AudioCommand        `json:"pending,omitempty"`
	Published      map[string]AudioVoice `json:"published,omitempty"`
	History        []AudioEvent          `json:"history,omitempty"`
	ResetMedia     bool                  `json:"resetMedia,omitempty"`
	DPCMLastID     uint8                 `json:"dpcmLastID,omitempty"`
	DPCMGeneration uint64                `json:"dpcmGeneration,omitempty"`
	DPCMPublished  uint64                `json:"dpcmPublished,omitempty"`
	Unresolved     bool                  `json:"unresolved,omitempty"`
}

func NewAudioState(epoch string) AudioState {
	return AudioState{Version: 1, Epoch: epoch, Published: map[string]AudioVoice{}}
}

func (s *AudioState) Queue(commands ...AudioCommand) {
	s.Pending = append(s.Pending, commands...)
}

func (s *AudioState) resetControl() {
	s.Slots = [6]AudioSlotState{}
	s.DPCMLastID = 0
	s.ResetMedia = true
	s.Unresolved = false
}

type audioRequestDecision struct {
	carry, ignored, dpcm, reset bool
	accepted                    []uint8
}

// $87A2 return carry is consumed by some game arithmetic. It depends on the
// actual request guards and the last target slot, not on whether a WAV exists.
func (s *AudioState) decideRequest(id uint8) (audioRequestDecision, error) {
	// The original rejects >=$82 and dispatches DPCM before ordinary guards.
	if id >= 0x82 {
		return audioRequestDecision{ignored: true, carry: true}, nil
	}
	if id >= 0x7f {
		return audioRequestDecision{dpcm: true}, nil
	}
	// Reserved zero never establishes a nonzero sound-code owner. It is not
	// the explicit reset routine and cannot replace an active positive code.
	if id == 0 {
		return audioRequestDecision{ignored: true}, nil
	}
	if s.Slots[1].ID == 0x62 && id != 0x0b {
		return audioRequestDecision{ignored: true, carry: id >= 0x0b}, nil
	}
	if id == 0x62 && (s.Slots[1].ID == 0x77 || s.Slots[1].ID == 0x62) {
		return audioRequestDecision{ignored: true, carry: true}, nil
	}
	if id == 0x18 && s.Slots[4].ID == 0x16 {
		return audioRequestDecision{ignored: true, carry: true}, nil
	}
	targets, ok := audioRegistry.targets[id]
	if !ok {
		return audioRequestDecision{}, errors.New("unverified Contra sound request target")
	}
	decision := audioRequestDecision{reset: id == 0x77}
	for _, slot := range targets {
		current := s.Slots[slot].ID
		if decision.reset {
			current = 0
		}
		decision.carry = false
		if id < current {
			continue // CMP current followed by BCC preserves C=0.
		}
		decision.accepted = append(decision.accepted, slot)
		// Accepted targets finish at CPX #8 with physical register offsets
		// 0,4,8,12,0,12 for slots 0..5 respectively.
		decision.carry = slot == 2 || slot == 3 || slot == 5
	}
	return decision, nil
}

func (s *AudioState) accept(id uint8, unsupported *[]uint8) {
	decision, err := s.decideRequest(id)
	if err != nil || (!decision.ignored && !decision.dpcm && audioDefinitions[id].Kind == "") {
		for _, existing := range *unsupported {
			if existing == id {
				return
			}
		}
		*unsupported = append(*unsupported, id)
		s.Unresolved = true
		return
	}
	if decision.ignored {
		return
	}
	if decision.dpcm {
		s.NextGeneration++
		s.DPCMLastID, s.DPCMGeneration = id, s.NextGeneration
		return
	}
	if decision.reset {
		s.resetControl()
	}
	for _, target := range decision.accepted {
		var release int32
		for _, row := range audioDefinitions[id].Slots {
			if row.Slot == target {
				release = row.Release
				break
			}
		}
		s.NextGeneration++
		s.Slots[target] = AudioSlotState{ID: id, Generation: s.NextGeneration, Release: release}
	}
}

// Clone is safe for phase-local flag projection: no mutable slice, map or
// history voice is shared with the authoritative instance.
func (s *AudioState) Clone() AudioState {
	clone := *s
	clone.Pending = append([]AudioCommand(nil), s.Pending...)
	clone.History = cloneAudioEvents(s.History)
	if s.Published != nil {
		clone.Published = make(map[string]AudioVoice, len(s.Published))
		for key, voice := range s.Published {
			clone.Published[key] = voice
		}
	}
	return clone
}

func (s *AudioState) projectRequest(id uint8) (bool, error) {
	if s.Unresolved && id > 0 && id < 0x7f {
		return false, errors.New("cannot project flags from unresolved audio control state")
	}
	decision, err := s.decideRequest(id)
	if err != nil {
		return false, err
	}
	if decision.reset {
		s.resetControl()
	}
	for _, slot := range decision.accepted {
		s.Slots[slot].ID = id
	}
	return decision.carry, nil
}

// RequestCarry is pure and includes all commands queued since the last audio
// boundary. World uses it on a phase-entry clone plus the effective phase prefix.
func (s *AudioState) RequestCarry(id uint8) (bool, error) {
	projected := s.Clone()
	start := 0
	for i, command := range projected.Pending {
		if command.Kind == "reset" {
			projected.resetControl()
			start = i + 1 // An unconditional reset discards earlier uncertainty.
		}
	}
	for _, command := range projected.Pending[start:] {
		if command.Kind == "sound" {
			if _, err := projected.projectRequest(command.ID); err != nil {
				return false, err
			}
		}
	}
	return projected.projectRequest(id)
}

// QueueSound returns the original request carry and appends exactly one command.
// A World phase projection calls this only on its clone, never its real queue.
func (s *AudioState) QueueSound(id uint8) (bool, error) {
	carry, err := s.RequestCarry(id)
	if err != nil {
		return false, err
	}
	s.Queue(AudioCommand{Kind: "sound", ID: id})
	return carry, nil
}

func (d audioSlotDefinition) requestsAt(age uint64) []audioPoint {
	points := d.Prefix
	if d.LoopTicks != 0 && age >= d.LoopStart {
		age = (age - d.LoopStart) % d.LoopTicks
		points = d.Loop
	}
	i := sort.Search(len(points), func(i int) bool { return points[i].Tick >= age })
	j := i
	for j < len(points) && points[j].Tick == age {
		j++
	}
	return points[i:j]
}

// Advance runs every authoritative video/audio update, including pause. Only
// cue $74 advances while paused; existing music and effects retain their clocks.
func (s *AudioState) Advance(paused bool) AudioUpdate {
	s.Version = 1
	s.Tick++
	var unsupported []uint8
	pending := s.Pending
	s.Pending = nil
	for _, command := range pending {
		switch command.Kind {
		case "reset":
			s.resetControl()
		case "sound":
			s.accept(command.ID, &unsupported)
		}
	}
	if paused && !s.Paused {
		// $809F -> $865F stops the hardware DPCM sample on the pause edge.
		s.DPCMLastID = 0
	}
	s.Paused = paused
	for index := range s.Slots {
		before := s.Slots[index]
		if before.ID == 0 || paused && before.ID != 0x74 {
			continue
		}
		var definition audioSlotDefinition
		for _, candidate := range audioDefinitions[before.ID].Slots {
			if int(candidate.Slot) == index {
				definition = candidate
				break
			}
		}
		for _, request := range definition.requestsAt(before.Age) {
			s.accept(request.ID, &unsupported)
		}
		if before.Release >= 0 && before.Age >= uint64(before.Release) {
			// $865F clears triangle/DPCM when the triangle sequence ends.
			if index == 2 {
				s.DPCMLastID = 0
			}
			s.Slots[index].ID, s.Slots[index].Started = 0, false
			// Callback request sees the source slot already cleared. Any earlier or
			// current-slot replacement is left untouched until the next boundary.
			for _, callback := range definition.End {
				if !callback.RequiresEndPlayersAlive || s.Context.EndPlayersAlive > 0 {
					s.accept(callback.ID, &unsupported)
				}
			}
			continue
		}
		if s.Slots[index].Generation != before.Generation {
			continue
		}
		s.Slots[index].Started = true
		s.Slots[index].LastUpdate = s.Tick
		s.Slots[index].Age++
	}
	events := s.publish()
	s.History = append(s.History, cloneAudioEvents(events)...)
	if len(s.History) > 64 {
		s.History = append([]AudioEvent(nil), s.History[len(s.History)-64:]...)
	}
	return AudioUpdate{Events: cloneAudioEvents(events), View: s.View(), UnsupportedIDs: unsupported}
}

var audioMediaSlots = [...]int{0, 1, 2, 4, 5}
var audioChannels = [...]string{"pulse1", "pulse2", "triangle", "", "pulse1", "noise"}

func (s *AudioState) voices() []AudioVoice {
	voices := make([]AudioVoice, 0, len(audioMediaSlots))
	for _, index := range audioMediaSlots {
		slot := s.Slots[index]
		if slot.ID == 0 || !slot.Started {
			continue
		}
		position := slot.Age
		if position > 0 && slot.LastUpdate == s.Tick {
			position--
		}
		definition := audioDefinitions[slot.ID]
		voices = append(voices, AudioVoice{Key: fmt.Sprintf("slot-%d", index), ID: slot.ID,
			Channel: audioChannels[index], PlaybackID: fmt.Sprintf("%s/%d/%d", s.Epoch, index, slot.Generation),
			PositionTicks: position, Paused: s.Paused && slot.ID != 0x74,
			Muted: index == 0 && s.Slots[4].ID != 0, Restore: definition.Kind == "music",
			Continuous: slot.Release < 0})
	}
	return voices
}

func (s *AudioState) publish() []AudioEvent {
	var events []AudioEvent
	emit := func(event AudioEvent) {
		s.Sequence++
		event.Sequence, event.Tick = s.Sequence, s.Tick
		events = append(events, event)
	}
	if s.Published == nil {
		s.Published = map[string]AudioVoice{}
	}
	if s.ResetMedia {
		emit(AudioEvent{Action: "reset"})
		s.Published = map[string]AudioVoice{}
		s.DPCMPublished = 0
		s.ResetMedia = false
	}
	desired := map[string]AudioVoice{}
	for _, voice := range s.voices() {
		desired[voice.Key] = voice
	}
	// Stable ordering is part of replay, and makes mute precede any resume.
	for _, slot := range audioMediaSlots {
		key := fmt.Sprintf("slot-%d", slot)
		before, exists := s.Published[key]
		after, remains := desired[key]
		if exists && (!remains || before.PlaybackID != after.PlaybackID) {
			emit(AudioEvent{Action: "stop", Key: key})
			delete(s.Published, key)
		}
	}
	for _, slot := range audioMediaSlots {
		key := fmt.Sprintf("slot-%d", slot)
		after, exists := desired[key]
		if !exists {
			continue
		}
		before, published := s.Published[key]
		if published && !before.Muted && after.Muted {
			emit(AudioEvent{Action: "mute", Key: key, Value: true})
		}
		if published && !before.Paused && after.Paused {
			emit(AudioEvent{Action: "pause", Key: key})
		}
	}
	for _, slot := range audioMediaSlots {
		key := fmt.Sprintf("slot-%d", slot)
		after, exists := desired[key]
		if !exists {
			continue
		}
		before, published := s.Published[key]
		if !published {
			voice := after
			emit(AudioEvent{Action: "start", Key: key, Voice: &voice})
		} else {
			if before.Paused && !after.Paused {
				emit(AudioEvent{Action: "resume", Key: key})
			}
			if before.Muted && !after.Muted {
				emit(AudioEvent{Action: "mute", Key: key})
			}
		}
		s.Published[key] = after
	}
	if s.DPCMLastID == 0 {
		if s.DPCMPublished != 0 {
			emit(AudioEvent{Action: "stop", Key: "dpcm"})
			s.DPCMPublished = 0
		}
	} else if s.DPCMPublished != s.DPCMGeneration {
		if s.DPCMPublished != 0 {
			emit(AudioEvent{Action: "stop", Key: "dpcm"})
		}
		voice := AudioVoice{Key: "dpcm", ID: s.DPCMLastID, Channel: "dpcm", PlaybackID: fmt.Sprintf("%s/dpcm/%d", s.Epoch, s.DPCMGeneration)}
		emit(AudioEvent{Action: "start", Key: "dpcm", Voice: &voice})
		s.DPCMPublished = s.DPCMGeneration
	}
	return events
}

func cloneAudioEvents(source []AudioEvent) []AudioEvent {
	copyOf := append([]AudioEvent(nil), source...)
	for i := range copyOf {
		if copyOf[i].Voice != nil {
			voice := *copyOf[i].Voice
			copyOf[i].Voice = &voice
		}
	}
	return copyOf
}

func (s *AudioState) View() AudioView {
	view := AudioView{Epoch: s.Epoch, Sequence: s.Sequence, Tick: s.Tick, Paused: s.Paused,
		Voices: s.voices(), Events: cloneAudioEvents(s.History)}
	for i, slot := range s.Slots {
		view.SlotIDs[i] = slot.ID
	}
	return view
}
