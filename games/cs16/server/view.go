package cs16

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type PlayerView struct {
	Seat          int                  `json:"seat"`
	Team          string               `json:"team"`
	Character     string               `json:"character"`
	Alive         bool                 `json:"alive"`
	Life          int                  `json:"life"`
	Visible       bool                 `json:"visible"`
	X             *int                 `json:"x,omitempty"`
	Y             *int                 `json:"y,omitempty"`
	Z             *int                 `json:"z,omitempty"`
	VX            *int                 `json:"vx,omitempty"`
	VY            *int                 `json:"vy,omitempty"`
	VZ            *int                 `json:"vz,omitempty"`
	RemX          *int                 `json:"remX,omitempty"`
	RemY          *int                 `json:"remY,omitempty"`
	RemZ          *int                 `json:"remZ,omitempty"`
	Input         *engine.Input        `json:"input,omitempty"`
	Previous      *engine.Input        `json:"previous,omitempty"`
	Yaw           *int                 `json:"yaw,omitempty"`
	Pitch         *int                 `json:"pitch,omitempty"`
	Health        *int                 `json:"health,omitempty"`
	Armor         *int                 `json:"armor,omitempty"`
	Money         *int                 `json:"money,omitempty"`
	Helmet        *bool                `json:"helmet,omitempty"`
	Defuser       *bool                `json:"defuser,omitempty"`
	Weapon        string               `json:"weapon,omitempty"`
	Ammo          *int                 `json:"ammo,omitempty"`
	Reserve       *int                 `json:"reserve,omitempty"`
	Inventory     []engine.WeaponState `json:"inventory,omitempty"`
	Kills         int                  `json:"kills"`
	Deaths        int                  `json:"deaths"`
	Crouch        bool                 `json:"crouch"`
	Grounded      bool                 `json:"grounded"`
	WaterLevel    *int                 `json:"waterLevel,omitempty"`
	ReceivedSeq   int64                `json:"receivedSeq"`
	AppliedSeq    int64                `json:"appliedSeq"`
	NextAttack    int64                `json:"nextAttack,omitempty"`
	RecoilYaw     int                  `json:"recoilYaw,omitempty"`
	RecoilPitch   int                  `json:"recoilPitch,omitempty"`
	FlashUntil    int64                `json:"flashUntil,omitempty"`
	FlashStrength int                  `json:"flashStrength,omitempty"`
	Action        string               `json:"action,omitempty"`
	ActionSince   int64                `json:"actionSince,omitempty"`
}

type EventView struct {
	ID     int64  `json:"id"`
	Frame  int64  `json:"frame"`
	Kind   string `json:"type"`
	Seat   int    `json:"seat"`
	Target int    `json:"target"`
	Weapon string `json:"weapon,omitempty"`
	X      *int   `json:"x,omitempty"`
	Y      *int   `json:"y,omitempty"`
	Z      *int   `json:"z,omitempty"`
	Value  int    `json:"value,omitempty"`
	Life   int    `json:"life,omitempty"`
	Global bool   `json:"global,omitempty"`
}

type View struct {
	Schema      int                  `json:"schema"`
	Epoch       string               `json:"epoch"`
	ServerAt    int64                `json:"serverAt"`
	StartAt     int64                `json:"startAt"`
	Frame       int64                `json:"frame"`
	MapID       string               `json:"mapId"`
	RulesSHA256 string               `json:"rulesSHA256"`
	Options     engine.Options       `json:"options"`
	Players     []PlayerView         `json:"players"`
	Events      []EventView          `json:"events"`
	EventID     int64                `json:"eventId"`
	Round       int                  `json:"round"`
	Phase       string               `json:"phase"`
	PhaseEndsAt int64                `json:"phaseEndsAt"`
	BuyEndsAt   int64                `json:"buyEndsAt"`
	BuyAllowed  bool                 `json:"buyAllowed"`
	TeamScores  [2]int               `json:"teamScores"`
	Bomb        engine.Bomb          `json:"bomb"`
	Hostages    []engine.Hostage     `json:"hostages"`
	Grenades    []engine.Grenade     `json:"grenades"`
	Drops       []engine.Drop        `json:"drops"`
	Brushes     *[]engine.BrushState `json:"brushes,omitempty"`
	Finished    bool                 `json:"finished"`
	You         int                  `json:"you"`
	CanAct      bool                 `json:"canAct"`
	CatchingUp  bool                 `json:"catchingUp"`
	Recover     bool                 `json:"recover,omitempty"`
}

func hasViewSeat(state *matchState, seat int) bool {
	return seat >= 0 && seat < len(state.World.Players)
}

func pointVisible(state *matchState, seat int, position engine.Vec3) bool {
	if !hasViewSeat(state, seat) {
		return true
	}
	if state.World.Players[seat].Alive {
		return pointVisibleFrom(state, seat, position)
	}
	for index, player := range state.World.Players {
		if player.Alive && player.Team == state.World.Players[seat].Team && pointVisibleFrom(state, index, position) {
			return true
		}
	}
	return false
}

func pointVisibleFrom(state *matchState, seat int, position engine.Vec3) bool {
	trace := engine.TraceWorld(&state.World, engine.Eye(state.World.Players[seat]), position, -1)
	return trace.Fraction >= engine.NormalUnit && !trace.StartSolid
}

func opposingVisible(state *matchState, seat, target int) bool {
	if !hasViewSeat(state, seat) {
		return true
	}
	if state.World.Players[seat].Alive {
		return engine.Visible(&state.World, seat, target)
	}
	for index, player := range state.World.Players {
		if player.Alive && player.Team == state.World.Players[seat].Team && engine.Visible(&state.World, index, target) {
			return true
		}
	}
	return false
}

func eventAudible(state *matchState, seat int, position engine.Vec3) bool {
	if !hasViewSeat(state, seat) {
		return true
	}
	viewer := state.World.Players[seat]
	if viewer.Alive {
		return engine.Distance(engine.Position(viewer), position) <= 2048*engine.Unit
	}
	for _, player := range state.World.Players {
		if player.Alive && player.Team == viewer.Team && engine.Distance(engine.Position(player), position) <= 2048*engine.Unit {
			return true
		}
	}
	return false
}

func eventVisible(state *matchState, seat int, event engine.Event) bool {
	if event.Kind == "buy" {
		return hasViewSeat(state, seat) && event.Seat == seat
	}
	if !hasViewSeat(state, seat) || event.Seat == seat || event.Target == seat {
		return true
	}
	switch event.Kind {
	case "round_start", "round_end", "kill", "death", "bomb_planted", "bomb_defused", "bomb_exploded", "hostage_rescued", "finished":
		return true
	case "buy":
		return false
	}
	if event.Seat >= 0 && event.Seat < len(state.World.Players) {
		if state.World.Players[event.Seat].Team == state.World.Players[seat].Team || opposingVisible(state, seat, event.Seat) {
			return true
		}
	}
	if event.Kind == "shot" || event.Kind == "fire" || event.Kind == "step" || event.Kind == "grenade_explode" {
		return eventAudible(state, seat, engine.Vec3{X: event.X, Y: event.Y, Z: event.Z})
	}
	return false
}

func projectedEvent(event engine.Event) EventView {
	value := EventView{ID: event.ID, Frame: event.Frame, Kind: event.Kind, Seat: event.Seat, Target: event.Target, Weapon: event.Weapon, X: &event.X, Y: &event.Y, Z: &event.Z, Value: event.Value, Life: event.Life}
	switch event.Kind {
	case "round_start", "round_end", "finished", "kill", "death", "bomb_planted", "bomb_defused", "bomb_exploded", "hostage_rescued":
		value.Global = true
		value.X, value.Y, value.Z = nil, nil, nil
		if event.Kind != "kill" && event.Kind != "death" {
			value.Seat, value.Target = -1, -1
		}
	}
	return value
}

func projection(state *matchState, seat int) View {
	world := &state.World
	brushes := world.Brushes
	if brushes == nil {
		brushes = []engine.BrushState{}
	}
	value := View{Schema: 1, Epoch: state.Epoch, ServerAt: state.BaseAt + world.Frame*1000/engine.FPS, StartAt: state.BaseAt, Frame: world.Frame, MapID: world.MapID, RulesSHA256: state.RulesSHA256, Options: world.Options, Players: make([]PlayerView, len(world.Players)), Events: []EventView{}, EventID: world.EventID, Round: world.Round, Phase: world.Phase, PhaseEndsAt: world.PhaseEndsAt, BuyEndsAt: world.BuyEndsAt, TeamScores: world.TeamScores, Bomb: world.Bomb, Hostages: []engine.Hostage{}, Grenades: []engine.Grenade{}, Drops: []engine.Drop{}, Brushes: &brushes, Finished: world.Finished, You: seat, CanAct: hasViewSeat(state, seat) && world.Players[seat].Alive && !world.Finished, BuyAllowed: engine.CanBuy(world, seat)}
	value.CatchingUp = state.ObservedAt-value.ServerAt > 200
	value.CanAct = value.CanAct && !value.CatchingUp
	value.BuyAllowed = value.BuyAllowed && !value.CatchingUp
	for index, player := range world.Players {
		visible := !hasViewSeat(state, seat) || index == seat || world.Players[seat].Team == player.Team || opposingVisible(state, seat, index)
		view := PlayerView{Seat: player.Seat, Team: player.Team, Character: player.Character, Alive: player.Alive, Life: player.Life, Visible: visible, Kills: player.Kills, Deaths: player.Deaths}
		if visible {
			view.X, view.Y, view.Z = &player.X, &player.Y, &player.Z
			view.Yaw, view.Pitch = &player.Yaw, &player.Pitch
			view.Weapon, view.Crouch = player.Weapon, player.Crouch
		}
		if index == seat {
			view.VX, view.VY, view.VZ = &player.VX, &player.VY, &player.VZ
			view.RemX, view.RemY, view.RemZ = &player.RemX, &player.RemY, &player.RemZ
			logicalInput, logicalPrevious := player.Input, player.Previous
			logicalInput.QueuedFrame, logicalPrevious.QueuedFrame = 0, 0
			view.Input, view.Previous = &logicalInput, &logicalPrevious
			view.Health, view.Armor, view.Money = &player.Health, &player.Armor, &player.Money
			view.Helmet, view.Defuser = &player.Helmet, &player.Defuser
			view.Ammo, view.Reserve = &player.Ammo, &player.Reserve
			view.Inventory = append([]engine.WeaponState{}, player.Inventory...)
			view.Grounded = player.Grounded
			view.WaterLevel = &player.WaterLevel
			view.ReceivedSeq, view.AppliedSeq = player.ReceivedSeq, player.AppliedSeq
			view.NextAttack, view.RecoilYaw, view.RecoilPitch = player.NextAttack, player.RecoilYaw, player.RecoilPitch
			view.FlashUntil, view.FlashStrength = player.FlashUntil, player.FlashStrength
			view.Action, view.ActionSince = player.Action, player.ActionSince
		}
		value.Players[index] = view
	}
	for _, hostage := range world.Hostages {
		if hostage.Rescued || !hasViewSeat(state, seat) || world.Players[seat].Team == "ct" || pointVisible(state, seat, engine.Vec3{X: hostage.X, Y: hostage.Y, Z: hostage.Z}) {
			value.Hostages = append(value.Hostages, hostage)
		}
	}
	for _, grenade := range world.Grenades {
		if grenade.Active || grenade.Owner == seat || pointVisible(state, seat, engine.Vec3{X: grenade.X, Y: grenade.Y, Z: grenade.Z}) {
			value.Grenades = append(value.Grenades, grenade)
		}
	}
	for _, drop := range world.Drops {
		if pointVisible(state, seat, engine.Vec3{X: drop.X, Y: drop.Y, Z: drop.Z}) {
			value.Drops = append(value.Drops, drop)
		}
	}
	if hasViewSeat(state, seat) && world.Players[seat].Team == "ct" {
		value.Bomb.Carrier = -1
		if !pointVisible(state, seat, engine.Vec3{X: world.Bomb.X, Y: world.Bomb.Y, Z: world.Bomb.Z}) {
			value.Bomb.X, value.Bomb.Y, value.Bomb.Z = 0, 0, 0
			value.Bomb.Dropped = false
		}
	}
	if hasViewSeat(state, seat) && world.Players[seat].Team == "t" {
		value.Bomb.Defuser = -1
		value.Bomb.DefuseAt = 0
	}
	for _, event := range world.Events {
		if eventVisible(state, seat, event) {
			value.Events = append(value.Events, projectedEvent(event))
		}
	}
	return value
}

func project(match gameapi.Match, seat int) (any, error) {
	state, err := decode(match)
	if err != nil {
		return nil, err
	}
	value := projection(state, seat)
	if len(value.Events) > 24 {
		value.Events = value.Events[len(value.Events)-24:]
	}
	return value, nil
}

func syncSnapshot(match gameapi.Match, seat int) (any, error) {
	state, err := decode(match)
	if err != nil {
		return nil, err
	}
	value := projection(state, seat)
	value.Events = []EventView{}
	value.Recover = true
	return value, nil
}

func streamView(match gameapi.Match, seat int, cursor string) (gameapi.StreamViewResult, error) {
	baseline, valid := readStreamCursor(cursor, seat)
	if valid {
		if unchanged, err := unchangedProjection(match, seat, baseline.Header); err != nil {
			return gameapi.StreamViewResult{}, err
		} else if unchanged {
			return gameapi.StreamViewResult{Cursor: cursor, Unchanged: true}, nil
		}
	}
	state, err := decode(match)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	brushBytes, err := json.Marshal(state.World.Brushes)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	brushDigest := fmt.Sprintf("%x", sha256.Sum256(brushBytes))
	recover := cursor != "" && !valid
	if valid {
		recover = baseline.Epoch != state.Epoch || baseline.Frame > state.World.Frame || baseline.EventID > state.World.EventID || baseline.Round > state.World.Round
		if len(state.World.Events) > 0 && baseline.EventID < state.World.Events[0].ID-1 {
			recover = true
		}
	}
	value := projection(state, seat)
	value.Recover = recover
	lastEvent := int64(0)
	if recover {
		lastEvent = value.EventID
	} else if valid {
		lastEvent = baseline.EventID
	}
	events := make([]EventView, 0, len(value.Events))
	for _, event := range value.Events {
		if event.ID > lastEvent {
			events = append(events, event)
		}
	}
	value.Events = events
	object, err := baselineObject(value)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	nextCursor, err := makeStreamCursor(value, object, brushDigest)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	if valid && !recover {
		delta, deltaErr := fieldDelta(baseline, value, object)
		if deltaErr == nil {
			return gameapi.StreamViewResult{Data: delta, Cursor: nextCursor}, nil
		}
		value.Recover, value.Events = true, []EventView{}
	}
	return gameapi.StreamViewResult{Data: value, Cursor: nextCursor}, nil
}
