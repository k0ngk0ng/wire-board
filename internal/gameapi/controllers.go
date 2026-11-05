package gameapi

// ControllerDefinition is lightweight game-owned input metadata. It has no
// rule evaluation or transport behavior; the game still interprets actions.
type ControllerDefinition struct {
	Layout        string             `json:"layout"`
	LegacyProfile string             `json:"legacyProfile,omitempty"`
	Actions       []ControllerAction `json:"actions,omitempty"`
}
type ControllerAction struct {
	Continuous  bool                        `json:"continuous,omitempty"`
	ID          string                      `json:"id"`
	Label       string                      `json:"label"`
	Keys        []string                    `json:"keys,omitempty"`
	Gamepad     []ControllerPhysicalBinding `json:"gamepad,omitempty"`
	TurboTarget string                      `json:"turboTarget,omitempty"`
}
type ControllerPhysicalBinding struct {
	Type      string `json:"type"`
	Index     int    `json:"index"`
	Direction int    `json:"direction,omitempty"`
}
