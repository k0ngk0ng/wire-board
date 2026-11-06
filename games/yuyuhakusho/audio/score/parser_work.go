package score

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type ParserWorkKind string

const (
	WorkTokenRead       ParserWorkKind = "token-read"
	WorkPitchResolve    ParserWorkKind = "pitch-resolve-site"
	WorkDurationProbe   ParserWorkKind = "duration-probe"
	WorkDurationScale   ParserWorkKind = "duration-scale"
	WorkNoteCommit      ParserWorkKind = "note-commit-site"
	WorkControlDispatch ParserWorkKind = "control-dispatch"
	WorkParameterRead   ParserWorkKind = "parameter-read"
	WorkLoop            ParserWorkKind = "loop"
	WorkJump            ParserWorkKind = "jump"
	WorkCall            ParserWorkKind = "call"
	WorkReturn          ParserWorkKind = "return"
)

type ParserWorkSite string

const (
	SitePrelude     ParserWorkSite = "prelude"
	SitePitch       ParserWorkSite = "pitch"
	SiteEarlyCommit ParserWorkSite = "early-commit"
	SiteDuration    ParserWorkSite = "duration"
	SiteFinalCommit ParserWorkSite = "final-commit"
)

// ParserWork records bounded grammar work. It is neither executable control
// flow nor a price: the score never branches or changes state from these data.
type ParserWork struct {
	Kind      ParserWorkKind `json:"kind"`
	Form      string         `json:"form,omitempty"`
	Family    string         `json:"family,omitempty"`
	Outcome   string         `json:"outcome,omitempty"`
	Phase     string         `json:"phase,omitempty"`
	Action    string         `json:"action,omitempty"`
	Role      string         `json:"role,omitempty"`
	Additions uint8          `json:"additions,omitempty"`
	First     bool           `json:"first,omitempty"`
	Taken     bool           `json:"taken,omitempty"`
}

func choice(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func (w *ParserWork) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	var next ParserWork
	if err := json.Unmarshal(fields["kind"], &next.Kind); err != nil {
		return err
	}
	keys := []string{"kind"}
	readString := func(key string, target *string) error {
		keys = append(keys, key)
		return json.Unmarshal(fields[key], target)
	}
	switch next.Kind {
	case WorkTokenRead:
		if err := readString("form", &next.Form); err != nil {
			return err
		}
		if !choice(next.Form, "control", "duration", "pitch") {
			return fmt.Errorf("unknown token form")
		}
	case WorkPitchResolve:
		if err := readString("form", &next.Form); err != nil {
			return err
		}
		if err := readString("family", &next.Family); err != nil {
			return err
		}
		if !choice(next.Form, "chromatic", "dac", "rest") || !choice(next.Family, "dac", "fm", "psg") {
			return fmt.Errorf("unknown pitch work")
		}
	case WorkDurationProbe:
		if err := readString("outcome", &next.Outcome); err != nil {
			return err
		}
		if !choice(next.Outcome, "take", "reuse") {
			return fmt.Errorf("unknown duration probe")
		}
	case WorkDurationScale:
		keys = append(keys, "additions")
		if err := json.Unmarshal(fields["additions"], &next.Additions); err != nil {
			return err
		}
		if next.Additions > 1 {
			return fmt.Errorf("unproved duration multiplier")
		}
	case WorkNoteCommit:
		if err := readString("phase", &next.Phase); err != nil {
			return err
		}
		if !choice(next.Phase, "after-pitch", "final") {
			return fmt.Errorf("unknown note commit site")
		}
	case WorkControlDispatch:
		if err := readString("action", &next.Action); err != nil {
			return err
		}
		if !choice(next.Action, "call", "detune", "enqueue-replace-first", "fm-patch", "fm-patch-select", "gate", "jump", "level-add", "loop", "modulation", "modulation-enable", "pan", "pan-automation", "psg-noise", "return", "tempo", "tie", "track-end", "transpose-add", "volume-envelope") {
			return fmt.Errorf("unknown semantic control")
		}
	case WorkParameterRead:
		if err := readString("role", &next.Role); err != nil {
			return err
		}
		if !choice(next.Role, "base-level-delta", "detune", "gate-definition", "gate-remaining-seed", "instrument-selection", "modulation-delay", "modulation-delta", "modulation-rate", "modulation-steps", "noise-latch", "pan-elapsed-seed", "pan-limit", "pan-mode", "pan-pattern", "pan-period", "pan-start-step", "queue-sound", "stereo", "tempo-period", "tempo-remaining-seed", "transpose-delta", "volume-envelope") {
			return fmt.Errorf("unknown parameter role")
		}
	case WorkLoop:
		keys = append(keys, "first", "taken")
		if err := json.Unmarshal(fields["first"], &next.First); err != nil {
			return err
		}
		if err := json.Unmarshal(fields["taken"], &next.Taken); err != nil {
			return err
		}
	case WorkJump, WorkCall, WorkReturn:
	default:
		return fmt.Errorf("unknown parser work kind")
	}
	if len(fields) != len(keys) {
		return fmt.Errorf("unexpected parser work fields")
	}
	for _, key := range keys {
		if value, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("missing parser work field")
		}
	}
	*w = next
	return nil
}

func workSite(work ParserWork) ParserWorkSite {
	switch work.Kind {
	case WorkPitchResolve:
		return SitePitch
	case WorkDurationProbe, WorkDurationScale:
		return SiteDuration
	case WorkNoteCommit:
		if work.Phase == "after-pitch" {
			return SiteEarlyCommit
		}
		return SiteFinalCommit
	}
	return SitePrelude
}
func siteOrder(site ParserWorkSite) int {
	switch site {
	case SitePrelude:
		return 0
	case SitePitch:
		return 1
	case SiteEarlyCommit:
		return 2
	case SiteDuration:
		return 3
	case SiteFinalCommit:
		return 4
	}
	return -1
}

type parserKey struct {
	Owner Owner
	Cycle bool
	Index int
}
type parserPlan struct {
	all   []ParserWork
	sites map[ParserWorkSite][]ParserWork
}

// ParserMetadata is an immutable sidecar, bound to the exact decoded catalog
// bytes. It owns no execution cursor, loop counter, target or score state.
type ParserMetadata struct {
	catalogSHA string
	records    map[parserKey]map[EventEntry]*parserPlan
}

func DecodeParserMetadata(c *Catalog, r io.Reader) (*ParserMetadata, error) {
	if c == nil || c.jsonSHA == "" {
		return nil, fmt.Errorf("catalog identity required")
	}
	raw, err := io.ReadAll(io.LimitReader(r, (32<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 32<<20 {
		return nil, fmt.Errorf("parser metadata resource limit")
	}
	var document struct {
		Schema     int    `json:"schema"`
		Format     string `json:"format"`
		CatalogSHA string `json:"catalog_json_sha256"`
		Coverage   string `json:"coverage"`
		Unpriced   bool   `json:"unpriced"`
		Records    []struct {
			Key struct {
				ScoreID uint8  `json:"score_id"`
				VoiceID int    `json:"voice_id"`
				Section string `json:"section"`
				Index   int    `json:"event_index"`
			} `json:"key"`
			Variants []struct {
				Entries []EventEntry `json:"entries"`
				Work    []ParserWork `json:"work"`
			} `json:"variants"`
		} `json:"records"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&document); err != nil {
		return nil, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing parser metadata")
	}
	if document.Schema != 1 || document.Format != "typed-score-parser-work-v1" || document.CatalogSHA != c.jsonSHA || document.Coverage != "parser-structure-only" || !document.Unpriced || len(document.Records) > 100000 {
		return nil, fmt.Errorf("unsupported or unbound parser metadata")
	}
	m := &ParserMetadata{catalogSHA: document.CatalogSHA, records: make(map[parserKey]map[EventEntry]*parserPlan)}
	for _, row := range document.Records {
		s, ok := c.scores[row.Key.ScoreID]
		if !ok || row.Key.VoiceID < 0 || row.Key.VoiceID >= len(s.Voices) || row.Key.Index < 0 || !choice(row.Key.Section, "prefix", "cycle") {
			return nil, fmt.Errorf("unknown parser event key")
		}
		cycle := row.Key.Section == "cycle"
		events := s.Voices[row.Key.VoiceID].Timeline.Prefix
		if cycle {
			events = s.Voices[row.Key.VoiceID].Timeline.Cycle
		}
		if row.Key.Index >= len(events) {
			return nil, fmt.Errorf("parser event index outside catalog")
		}
		key := parserKey{Owner{row.Key.ScoreID, row.Key.VoiceID}, cycle, row.Key.Index}
		if _, ok := m.records[key]; ok {
			return nil, fmt.Errorf("duplicate parser event key")
		}
		variants := make(map[EventEntry]*parserPlan)
		if len(row.Variants) == 0 || len(row.Variants) > 2 {
			return nil, fmt.Errorf("invalid parser variant count")
		}
		for _, variant := range row.Variants {
			if len(variant.Work) == 0 || len(variant.Work) > 32 || len(variant.Entries) == 0 || len(variant.Entries) > 2 {
				return nil, fmt.Errorf("unbounded parser variant")
			}
			plan := &parserPlan{all: append([]ParserWork(nil), variant.Work...), sites: map[ParserWorkSite][]ParserWork{}}
			previous := 0
			for _, work := range plan.all {
				site := workSite(work)
				if siteOrder(site) < previous {
					return nil, fmt.Errorf("parser sites reordered")
				}
				previous = siteOrder(site)
				if events[row.Key.Index].Kind != "note" && site != SitePrelude {
					return nil, fmt.Errorf("note work on a control event")
				}
				plan.sites[site] = append(plan.sites[site], work)
			}
			if err := validateParserEvent(plan, events[row.Key.Index], s.Voices[row.Key.VoiceID].Channel); err != nil {
				return nil, err
			}
			for _, entry := range variant.Entries {
				if cycle && row.Key.Index == 0 {
					if entry != EntryFromPrefix && entry != EntryCycleWrap {
						return nil, fmt.Errorf("cycle entry history unknown")
					}
				} else if entry != EntryContinue {
					return nil, fmt.Errorf("unexpected parser entry variant")
				}
				if _, ok := variants[entry]; ok {
					return nil, fmt.Errorf("duplicate parser entry variant")
				}
				variants[entry] = plan
			}
		}
		if cycle && row.Key.Index == 0 {
			if variants[EntryFromPrefix] == nil || variants[EntryCycleWrap] == nil {
				return nil, fmt.Errorf("incomplete cycle entry variants")
			}
		} else if variants[EntryContinue] == nil {
			return nil, fmt.Errorf("missing parser continuation")
		}
		m.records[key] = variants
	}
	expected := 0
	for _, s := range c.scores {
		for _, v := range s.Voices {
			expected += len(v.Timeline.Prefix) + len(v.Timeline.Cycle)
		}
	}
	if len(m.records) != expected {
		return nil, fmt.Errorf("incomplete parser key coverage")
	}
	return m, nil
}

func validateParserEvent(plan *parserPlan, e event, channel string) error {
	lastToken := ""
	lastControl := ""
	pitch, early, final := 0, 0, 0
	family := "fm"
	if channel == "dac" {
		family = "dac"
	} else if len(channel) >= 3 && channel[:3] == "psg" {
		family = "psg"
	}
	for _, work := range plan.all {
		switch work.Kind {
		case WorkTokenRead:
			lastToken = work.Form
		case WorkControlDispatch:
			lastControl = work.Action
		case WorkPitchResolve:
			pitch++
			if e.Kind != "note" || e.Pitch == nil || e.Pitch.Kind == "previous" || work.Form != e.Pitch.Kind || work.Family != family {
				return fmt.Errorf("pitch work contradicts bound event")
			}
		case WorkNoteCommit:
			if work.Phase == "after-pitch" {
				early++
			} else {
				final++
			}
		}
	}
	if e.Kind != "note" {
		if lastToken != "control" || lastControl != e.Kind {
			return fmt.Errorf("control work contradicts bound event")
		}
		return nil
	}
	form := "pitch"
	wantPitch := 1
	if e.Pitch.Kind == "previous" {
		form = "duration"
		wantPitch = 0
	}
	wantEarly := 0
	if family == "psg" && e.Pitch.Kind != "previous" || family == "fm" && e.Pitch.Kind == "rest" {
		wantEarly = 1
	}
	if lastToken != form || pitch != wantPitch || early != wantEarly || final != 1 {
		return fmt.Errorf("note work does not match bound grammar")
	}
	return nil
}

func (m *ParserMetadata) plan(position EventPosition) (*parserPlan, error) {
	if m == nil || !position.Valid {
		return nil, fmt.Errorf("parser position missing")
	}
	variants := m.records[parserKey{position.Owner, position.Cycle, position.Index}]
	if variants == nil || variants[position.Entry] == nil {
		return nil, fmt.Errorf("unproved parser entry history")
	}
	return variants[position.Entry], nil
}

func (m *ParserMetadata) Lookup(position EventPosition) ([]ParserWork, error) {
	plan, err := m.plan(position)
	if err != nil {
		return nil, err
	}
	return append([]ParserWork(nil), plan.all...), nil
}

func (t *tick) parserSite(site ParserWorkSite) {
	if t.hooks == nil || t.hooks.Parser == nil || t.parserPlan == nil || t.err != nil {
		return
	}
	slot := -1
	if s, ok := t.driver.catalog.scores[t.position.Owner.ScoreID]; ok {
		slot = mustSlot(s, t.position.Owner.VoiceID)
	}
	for _, work := range t.parserPlan.sites[site] {
		t.operation(Operation{Kind: OpParserWork, Routine: RoutineParser, VoiceSlot: int8(slot), OtherSlot: -1, Position: t.position, Work: work, WorkSite: site})
		if t.err != nil {
			return
		}
	}
}
