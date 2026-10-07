package game

// CatanRevealedEvent keeps only the public face of the latest event. It is
// independent of the private response queue and the hidden deck.
type CatanRevealedEvent struct {
	Kind              string `json:"kind"`
	Production        int    `json:"production"`
	Red               int    `json:"red"`
	Face              int    `json:"face"`
	RollID            int    `json:"rollId"`
	ProductionStarted bool   `json:"productionStarted"`
}

func (g *Catan) rememberCardEvent() {
	if q := g.CardEvent; q != nil {
		g.RevealedEvent = &CatanRevealedEvent{Kind: q.Kind, Production: q.Production, Red: q.Red, Face: q.Face, RollID: g.RollID}
	}
}

// Legacy pending saves can display the public face before their next action,
// without a read operation modifying the persisted state or exposing gifts.
func (g *Catan) revealedEventView() *CatanRevealedEvent {
	if q := g.CardEvent; q != nil {
		return &CatanRevealedEvent{Kind: q.Kind, Production: q.Production, Red: q.Red, Face: q.Face, RollID: g.RollID}
	}
	if q := g.RevealedEvent; q != nil && q.RollID == g.RollID {
		copy := *q
		return &copy
	}
	return nil
}
