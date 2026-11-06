package score

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

// InstrumentWork is immutable resource identity, not another mutable selected
// tone or a native index. RecordsSkipped is bounded semantic seek work; source
// banks, addresses and original selectors remain outside runtime resources.
type InstrumentWork struct {
	ID             string `json:"id"`
	PatchID        string `json:"patch_id"`
	RecordsSkipped uint8  `json:"records_skipped"`
}

type instrumentKey struct {
	ScoreID uint8  `json:"score_id"`
	VoiceID int    `json:"voice_id"`
	Section string `json:"section"`
	Index   int    `json:"event_index"`
}

func (c *Catalog) Instrument(reference string) (InstrumentWork, bool) {
	if c == nil {
		return InstrumentWork{}, false
	}
	work, ok := c.instruments[reference]
	return work, ok
}

func (c *Catalog) patch(reference string) (param.FMPatch, bool) {
	if work, ok := c.instruments[reference]; ok {
		reference = work.PatchID
	}
	patch, ok := c.patches[reference]
	return patch, ok
}
func (c *Catalog) patchIdentity(reference string) string {
	if work, ok := c.instruments[reference]; ok {
		return work.PatchID
	}
	return reference
}

func instrumentIdentity(work InstrumentWork) string {
	raw, _ := json.Marshal(struct {
		PatchID        string `json:"patch_id"`
		RecordsSkipped uint8  `json:"records_skipped"`
	}{work.PatchID, work.RecordsSkipped})
	digest := sha256.Sum256(raw)
	return "instrument-" + hex.EncodeToString(digest[:])[:24]
}

// BindInstrumentWork returns an immutable catalog variant. It binds logical
// initial/event choices to resource identities and reuses the original sole
// patch payload map. VoiceState.Tone remains the only dynamic selected owner;
// native views derive the audible patch ID rather than storing it twice.
func BindInstrumentWork(c *Catalog, reader io.Reader) (*Catalog, error) {
	if c == nil || c.instruments != nil || reader == nil {
		return nil, fmt.Errorf("invalid instrument binding owner")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, (2<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 2<<20 {
		return nil, fmt.Errorf("instrument binding exceeds resource limit")
	}
	var document struct {
		Schema     int              `json:"schema"`
		Format     string           `json:"format"`
		CatalogSHA string           `json:"catalog_json_sha256"`
		Fallback   string           `json:"fallback_reference"`
		Profiles   []InstrumentWork `json:"profiles"`
		Selectors  []struct {
			Key       instrumentKey `json:"key"`
			Reference string        `json:"reference"`
		} `json:"selectors"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&document); err != nil {
		return nil, err
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("trailing instrument resource data")
	}
	if document.Schema != 1 || document.Format != "typed-instrument-work-v1" || document.CatalogSHA != c.jsonSHA || len(document.Profiles) == 0 || len(document.Profiles) > 1024 || len(document.Selectors) > 10000 {
		return nil, fmt.Errorf("instrument resource profile differs")
	}
	profiles := make(map[string]InstrumentWork, len(document.Profiles))
	for _, work := range document.Profiles {
		if work.ID != instrumentIdentity(work) {
			return nil, fmt.Errorf("instrument content identity differs")
		}
		if _, ok := c.patches[work.PatchID]; !ok {
			return nil, fmt.Errorf("unknown instrument sound reference")
		}
		if _, ok := profiles[work.ID]; ok {
			return nil, fmt.Errorf("duplicate instrument identity")
		}
		profiles[work.ID] = work
	}
	fallback, ok := profiles[document.Fallback]
	if !ok || fallback.PatchID != c.fallbackPatchID || fallback.RecordsSkipped != 0 {
		return nil, fmt.Errorf("instrument fallback differs")
	}
	selectors := make(map[instrumentKey]string, len(document.Selectors))
	for _, item := range document.Selectors {
		if _, ok := selectors[item.Key]; ok {
			return nil, fmt.Errorf("duplicate logical instrument choice")
		}
		if _, ok := profiles[item.Reference]; !ok {
			return nil, fmt.Errorf("unknown logical instrument reference")
		}
		selectors[item.Key] = item.Reference
	}
	used := map[string]bool{document.Fallback: true}
	selectReference := func(key instrumentKey, patch string) (string, error) {
		ref, ok := selectors[key]
		if !ok || profiles[ref].PatchID != patch {
			return "", fmt.Errorf("missing or mismatched logical instrument choice %+v", key)
		}
		delete(selectors, key)
		used[ref] = true
		return ref, nil
	}
	next := *c
	next.instruments = profiles
	next.fallbackPatchID = document.Fallback
	next.scores = make(map[uint8]definition, len(c.scores))
	for sid, definition := range c.scores {
		bound := definition
		bound.Voices = append([]voice(nil), definition.Voices...)
		for vi := range bound.Voices {
			v := &bound.Voices[vi]
			if v.Initial.PatchReference != "" {
				v.Initial.PatchReference, err = selectReference(instrumentKey{ScoreID: sid, VoiceID: v.ID, Section: "initial"}, v.Initial.PatchReference)
				if err != nil {
					return nil, err
				}
			}
			v.Timeline.Prefix = append([]event(nil), v.Timeline.Prefix...)
			v.Timeline.Cycle = append([]event(nil), v.Timeline.Cycle...)
			for _, part := range []struct {
				name   string
				events []event
			}{{"prefix", v.Timeline.Prefix}, {"cycle", v.Timeline.Cycle}} {
				for ei := range part.events {
					e := &part.events[ei]
					if e.PatchID == "" {
						continue
					}
					e.PatchID, err = selectReference(instrumentKey{ScoreID: sid, VoiceID: v.ID, Section: part.name, Index: ei}, e.PatchID)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		next.scores[sid] = bound
	}
	if len(selectors) != 0 || len(used) != len(profiles) {
		return nil, fmt.Errorf("unused instrument selectors or profiles")
	}
	return &next, nil
}
