package game

// This persisted stream contains public information only. A private draw carries
// its publicly visible deck, never its identity. Discards and espionage transfers
// omit even the category; only played cards and public VP cards have Card set.
type CatanProgressEvent struct {
	ID     uint64 `json:"id"`
	Kind   string `json:"kind"`
	Player int    `json:"player"`
	Other  int    `json:"other"`
	Track  int    `json:"track"`
	Count  int    `json:"count"`
	Card   *int   `json:"card,omitempty"`
}

func (k *CatanCitiesKnights) recordProgress(kind string, player, other, track, count int, publicCard *int) {
	k.ProgressEventID++
	e := CatanProgressEvent{ID: k.ProgressEventID, Kind: kind, Player: player, Other: other, Track: track, Count: count}
	if publicCard != nil {
		card := *publicCard
		e.Card = &card
	}
	k.ProgressEvents = append(k.ProgressEvents, e)
	if len(k.ProgressEvents) > 18 {
		k.ProgressEvents = k.ProgressEvents[len(k.ProgressEvents)-18:]
	}
}
