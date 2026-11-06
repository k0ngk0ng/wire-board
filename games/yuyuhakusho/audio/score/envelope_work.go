package score

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

// EnvelopePathWork is immutable bounded work for one already decoded logical
// tick. It contains no source navigation tape, target, raw cursor or program.
type EnvelopePathWork struct {
	Restarts uint8 `json:"restarts"`
}
type envelopeWorkKey struct {
	Family string
	ID     uint16
	Cycle  bool
	Index  uint16
}

func (c *Catalog) envelopePosition(family string, id uint16, cursor param.EnvelopeCursor) (envelopeWorkKey, error) {
	if c == nil {
		return envelopeWorkKey{}, fmt.Errorf("missing envelope catalog")
	}
	prefix, cycle := 0, 0
	switch family {
	case "pitch":
		e, ok := c.pitch[id]
		if !ok {
			return envelopeWorkKey{}, fmt.Errorf("unknown pitch envelope")
		}
		prefix, cycle = len(e.Prefix), len(e.Cycle)
	case "volume":
		e, ok := c.volume[id]
		if !ok {
			return envelopeWorkKey{}, fmt.Errorf("unknown volume envelope")
		}
		prefix, cycle = len(e.Prefix), len(e.Cycle)
	default:
		return envelopeWorkKey{}, fmt.Errorf("unknown envelope family")
	}
	if cursor.Cycle {
		if int(cursor.Index) >= cycle {
			return envelopeWorkKey{}, fmt.Errorf("envelope cycle index invalid")
		}
	} else if int(cursor.Index) == prefix && cycle > 0 {
		cursor = param.EnvelopeCursor{Cycle: true}
	} else if int(cursor.Index) >= prefix {
		return envelopeWorkKey{}, fmt.Errorf("envelope prefix index invalid")
	}
	return envelopeWorkKey{family, id, cursor.Cycle, cursor.Index}, nil
}

func (c *Catalog) EnvelopeWork(family string, id uint16, cursor param.EnvelopeCursor) (EnvelopePathWork, error) {
	if c == nil || c.envelopeWork == nil {
		return EnvelopePathWork{}, fmt.Errorf("envelope work is unbound")
	}
	key, err := c.envelopePosition(family, id, cursor)
	if err != nil {
		return EnvelopePathWork{}, err
	}
	work, ok := c.envelopeWork[key]
	if !ok {
		return EnvelopePathWork{}, fmt.Errorf("missing logical envelope work")
	}
	return work, nil
}

func BindEnvelopeWork(c *Catalog, reader io.Reader) (*Catalog, error) {
	if c == nil || c.envelopeWork != nil || reader == nil {
		return nil, fmt.Errorf("invalid envelope work owner")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, (256<<10)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 256<<10 {
		return nil, fmt.Errorf("envelope resource limit")
	}
	var doc struct {
		Schema     int    `json:"schema"`
		Format     string `json:"format"`
		CatalogSHA string `json:"catalog_json_sha256"`
		Records    []struct {
			Key struct {
				Family  string  `json:"family"`
				ID      uint16  `json:"envelope_id"`
				Section string  `json:"section"`
				Index   *uint16 `json:"tick_index"`
			} `json:"key"`
			Work struct {
				Restarts *uint8 `json:"restarts"`
			} `json:"work"`
		} `json:"records"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&doc); err != nil {
		return nil, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing envelope resource data")
	}
	if doc.Schema != 1 || doc.Format != "typed-envelope-work-v1" || doc.CatalogSHA != c.jsonSHA {
		return nil, fmt.Errorf("unbound envelope resource")
	}
	expected := 0
	for _, e := range c.pitch {
		expected += len(e.Prefix) + len(e.Cycle)
	}
	for _, e := range c.volume {
		expected += len(e.Prefix) + len(e.Cycle)
	}
	if len(doc.Records) != expected {
		return nil, fmt.Errorf("incomplete envelope work closure")
	}
	work := make(map[envelopeWorkKey]EnvelopePathWork, expected)
	for _, r := range doc.Records {
		if r.Key.Index == nil || r.Work.Restarts == nil || *r.Work.Restarts > 1 || r.Key.Section != "prefix" && r.Key.Section != "cycle" || r.Key.Family == "volume" && *r.Work.Restarts != 0 {
			return nil, fmt.Errorf("invalid envelope work shape")
		}
		cursor := param.EnvelopeCursor{Cycle: r.Key.Section == "cycle", Index: *r.Key.Index}
		key, err := c.envelopePosition(r.Key.Family, r.Key.ID, cursor)
		if err != nil || key.Cycle != cursor.Cycle || key.Index != cursor.Index {
			return nil, fmt.Errorf("noncanonical envelope resource key")
		}
		if _, exists := work[key]; exists {
			return nil, fmt.Errorf("duplicate envelope resource key")
		}
		work[key] = EnvelopePathWork{Restarts: *r.Work.Restarts}
	}
	next := *c
	next.envelopeWork = work
	return &next, nil
}
