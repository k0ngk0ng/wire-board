package score

// HeaderVoice and HeaderPlan are detached immutable-catalog projections for a
// single initialization. They contain no encoded stream, source address or state.
type HeaderVoice struct {
	Slot    int8
	Channel string
}

type HeaderPlan struct {
	ID     uint8
	Music  bool
	Voices []HeaderVoice
}

func (c *Catalog) HeaderPlan(id uint8) (HeaderPlan, bool) {
	if c == nil {
		return HeaderPlan{}, false
	}
	d, ok := c.scores[id]
	if !ok {
		return HeaderPlan{}, false
	}
	plan := HeaderPlan{ID: id, Music: d.Kind == "music", Voices: make([]HeaderVoice, 0, len(d.Voices))}
	for _, v := range d.Voices {
		slot, err := slotFor(v.Channel, plan.Music)
		if err != nil {
			return HeaderPlan{}, false
		}
		plan.Voices = append(plan.Voices, HeaderVoice{Slot: int8(slot), Channel: v.Channel})
	}
	return plan, true
}
