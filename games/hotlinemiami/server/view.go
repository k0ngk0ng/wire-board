package hotlinemiami

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type PlayerView struct {
	Seat            int    `json:"seat"`
	Character       string `json:"character"`
	X               int    `json:"x"`
	Y               int    `json:"y"`
	Aim             int    `json:"aim"`
	Alive           bool   `json:"alive"`
	Life            int    `json:"life"`
	Weapon          string `json:"weapon"`
	Ammo            int    `json:"ammo"`
	Score           int    `json:"score"`
	Kills           int    `json:"kills"`
	Deaths          int    `json:"deaths"`
	RespawnFrame    int64  `json:"respawnFrame"`
	ProtectedUntil  int64  `json:"protectedUntil"`
	DownUntil       int64  `json:"downUntil"`
	BusyUntil       int64  `json:"busyUntil"`
	NextAttackFrame int64  `json:"nextAttackFrame"`
	ReceivedSeq     int64  `json:"receivedSeq"`
	AppliedSeq      int64  `json:"appliedSeq"`
}

type EventView struct {
	ID     int64  `json:"id"`
	Frame  int64  `json:"frame"`
	Type   string `json:"type"`
	Seat   int    `json:"seat"`
	Target int    `json:"target"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Weapon string `json:"weapon,omitempty"`
	Life   int    `json:"life,omitempty"`
}

type View struct {
	Schema      int             `json:"schema"`
	Epoch       string          `json:"epoch"`
	ServerAt    int64           `json:"serverAt"`
	StartAt     int64           `json:"startAt"`
	Frame       int64           `json:"frame"`
	EndFrame    int64           `json:"endFrame"`
	MapID       string          `json:"mapId"`
	Options     engine.Options  `json:"options"`
	Players     []PlayerView    `json:"players"`
	Bullets     []engine.Bullet `json:"bullets"`
	Drops       []engine.Drop   `json:"drops"`
	Doors       []engine.Door   `json:"doors"`
	BrokenGlass []int           `json:"brokenGlass"`
	Corpses     []engine.Corpse `json:"corpses"`
	Events      []EventView     `json:"events"`
	EventID     int64           `json:"eventId"`
	Finished    bool            `json:"finished"`
	You         int             `json:"you"`
	CanAct      bool            `json:"canAct"`
	Map         *engine.Map     `json:"map,omitempty"`
	Recover     bool            `json:"recover,omitempty"`
}

func projection(state *matchState, seat int) View {
	world := &state.World
	value := View{Schema: 1, Epoch: state.Epoch, ServerAt: state.BaseAt + world.Frame*1000/engine.FPS, StartAt: state.BaseAt, Frame: world.Frame, EndFrame: world.EndFrame, MapID: world.MapID, Options: world.Options, Players: make([]PlayerView, len(world.Players)), Bullets: world.Bullets, Drops: world.Drops, Doors: world.Doors, BrokenGlass: world.BrokenGlass, Corpses: world.Corpses, Events: make([]EventView, 0, len(world.Events)), EventID: world.EventID, Finished: world.Finished, You: seat, CanAct: seat >= 0 && seat < len(world.Players) && !world.Finished}
	for index, player := range world.Players {
		value.Players[index] = PlayerView{Seat: player.Seat, Character: player.Character, X: player.X, Y: player.Y, Aim: player.Aim, Alive: player.Alive, Life: player.Life, Weapon: player.Weapon, Ammo: player.Ammo, Score: player.Score, Kills: player.Kills, Deaths: player.Deaths, RespawnFrame: player.RespawnFrame, ProtectedUntil: player.ProtectedUntil, DownUntil: player.DownUntil, BusyUntil: player.BusyUntil, NextAttackFrame: player.NextAttackFrame, ReceivedSeq: player.ReceivedSeq, AppliedSeq: player.AppliedSeq}
	}
	for _, event := range world.Events {
		value.Events = append(value.Events, EventView{ID: event.ID, Frame: event.Frame, Type: event.Kind, Seat: event.Seat, Target: event.Target, X: event.X, Y: event.Y, Weapon: event.Weapon, Life: event.Life})
	}
	if value.Bullets == nil {
		value.Bullets = []engine.Bullet{}
	}
	if value.Drops == nil {
		value.Drops = []engine.Drop{}
	}
	if value.Doors == nil {
		value.Doors = []engine.Door{}
	}
	if value.BrokenGlass == nil {
		value.BrokenGlass = []int{}
	}
	if value.Corpses == nil {
		value.Corpses = []engine.Corpse{}
	}
	if value.Events == nil {
		value.Events = []EventView{}
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
	value.Map = &state.World.World
	value.Recover = true
	return value, nil
}

func streamView(match gameapi.Match, seat int, cursor string) (gameapi.StreamViewResult, error) {
	state, err := decode(match)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	value := projection(state, seat)
	parts := strings.Split(cursor, ":")
	lastEvent := int64(0)
	if len(parts) == 3 && parts[0] == state.Epoch {
		lastEvent, err = strconv.ParseInt(parts[2], 10, 64)
		if err != nil || lastEvent > value.EventID {
			value.Recover = true
			lastEvent = value.EventID
		}
		if len(value.Events) > 0 && lastEvent < value.Events[0].ID-1 {
			value.Recover = true
		}
	} else if cursor != "" {
		value.Recover = true
	}
	events := make([]EventView, 0, len(value.Events))
	for _, event := range value.Events {
		if event.ID > lastEvent {
			events = append(events, event)
		}
	}
	value.Events = events
	if value.Recover {
		value.Map = &state.World.World
	}
	return gameapi.StreamViewResult{Data: value, Cursor: fmt.Sprintf("%s:%d:%d", state.Epoch, value.Frame, value.EventID)}, nil
}
