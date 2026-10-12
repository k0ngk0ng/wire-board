// Package game adapts the pre-module room protocol. New games use gameapi.
package game

import (
	"errors"
	carcassonne "github.com/k0ngk0ng/wire-board/games/carcassonne/server"
	catan "github.com/k0ngk0ng/wire-board/games/catan/server"
	dota "github.com/k0ngk0ng/wire-board/games/dota/server"
	rail "github.com/k0ngk0ng/wire-board/games/rail/server"
	sanguosha "github.com/k0ngk0ng/wire-board/games/sanguosha/server"
	splendor "github.com/k0ngk0ng/wire-board/games/splendor/server"
)

type Action struct {
	SpiceLoad   []int      `json:"spiceLoad,omitempty"`
	SpiceUnload []int      `json:"spiceUnload,omitempty"`
	GoldGive    int        `json:"goldGive,omitempty"`
	GoldTake    int        `json:"goldTake,omitempty"`
	Dota        *DotaOrder `json:"dota,omitempty"`
	Cards       []int      `json:"cards"`
	Targets     []int      `json:"targets"`
	Choice      string     `json:"choice"`
	Skill       string     `json:"skill"`
	Direction   string     `json:"direction,omitempty"`
	Prompt      int        `json:"prompt"`
	X           int        `json:"x"`
	Y           int        `json:"y"`
	Rotation    int        `json:"rotation"`
	Feature     int        `json:"feature"`
	Vertex      int        `json:"vertex"`
	Edge        int        `json:"edge"`
	Tile        int        `json:"tile"`
	Target      int        `json:"target"`
	Offer       int        `json:"offer"`
	Give        []int      `json:"give"`
	Take        []int      `json:"take"`
	Type        string     `json:"type"`
	Card        int        `json:"card"`
	Tier        int        `json:"tier"`
	Tokens      []int      `json:"tokens"`
	Noble       int        `json:"noble"`
	Route       int        `json:"route"`
	Color       int        `json:"color"`
	Wild        int        `json:"wild"`
	Slot        int        `json:"slot"`
	Keep        []int      `json:"keep"`
}
type State struct {
	Kind        string       `json:"kind"`
	Turn        int          `json:"turn"`
	Phase       string       `json:"phase"`
	Round       int          `json:"round"`
	Finished    bool         `json:"finished"`
	Winners     []int        `json:"winners"`
	Log         []string     `json:"log"`
	Splendor    *Splendor    `json:"splendor,omitempty"`
	Rail        *Rail        `json:"rail,omitempty"`
	Catan       *Catan       `json:"catan,omitempty"`
	Carcassonne *Carcassonne `json:"carcassonne,omitempty"`
	Sanguosha   *Sanguosha   `json:"sanguosha,omitempty"`
	Dota        *Dota        `json:"dota,omitempty"`
}

func fromSplendor(s *splendor.State) *State {
	if s == nil {
		return nil
	}
	return &State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Splendor: s.Splendor}
}
func (s *State) asSplendor() *splendor.State {
	if s == nil {
		return nil
	}
	return &splendor.State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Splendor: s.Splendor}
}
func (s *State) syncSplendor(v *splendor.State) {
	if s == nil {
		return
	}
	s.Kind = v.Kind
	s.Turn = v.Turn
	s.Phase = v.Phase
	s.Round = v.Round
	s.Finished = v.Finished
	s.Winners = v.Winners
	s.Log = v.Log
	s.Splendor = v.Splendor
}
func toSplendorAction(a Action) splendor.Action {
	return splendor.Action{Cards: a.Cards, Choice: a.Choice, Tile: a.Tile, Target: a.Target, Take: a.Take, Type: a.Type, Card: a.Card, Tier: a.Tier, Tokens: a.Tokens, Noble: a.Noble, Color: a.Color, Slot: a.Slot}
}
func fromSplendorAction(a splendor.Action) Action {
	return Action{Cards: a.Cards, Choice: a.Choice, Tile: a.Tile, Target: a.Target, Take: a.Take, Type: a.Type, Card: a.Card, Tier: a.Tier, Tokens: a.Tokens, Noble: a.Noble, Color: a.Color, Slot: a.Slot}
}
func fromRail(s *rail.State) *State {
	if s == nil {
		return nil
	}
	return &State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Rail: s.Rail}
}
func (s *State) asRail() *rail.State {
	if s == nil {
		return nil
	}
	return &rail.State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Rail: s.Rail}
}
func (s *State) syncRail(v *rail.State) {
	if s == nil {
		return
	}
	s.Kind = v.Kind
	s.Turn = v.Turn
	s.Phase = v.Phase
	s.Round = v.Round
	s.Finished = v.Finished
	s.Winners = v.Winners
	s.Log = v.Log
	s.Rail = v.Rail
}
func toRailAction(a Action) rail.Action {
	return rail.Action{Vertex: a.Vertex, Type: a.Type, Tokens: a.Tokens, Route: a.Route, Color: a.Color, Wild: a.Wild, Slot: a.Slot, Keep: a.Keep}
}
func fromRailAction(a rail.Action) Action {
	return Action{Vertex: a.Vertex, Type: a.Type, Tokens: a.Tokens, Route: a.Route, Color: a.Color, Wild: a.Wild, Slot: a.Slot, Keep: a.Keep}
}
func fromCatan(s *catan.State) *State {
	if s == nil {
		return nil
	}
	return &State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Catan: s.Catan}
}
func (s *State) asCatan() *catan.State {
	if s == nil {
		return nil
	}
	return &catan.State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Catan: s.Catan}
}
func (s *State) syncCatan(v *catan.State) {
	if s == nil {
		return
	}
	s.Kind = v.Kind
	s.Turn = v.Turn
	s.Phase = v.Phase
	s.Round = v.Round
	s.Finished = v.Finished
	s.Winners = v.Winners
	s.Log = v.Log
	s.Catan = v.Catan
}
func toCatanAction(a Action) catan.Action {
	return catan.Action{SpiceLoad: a.SpiceLoad, SpiceUnload: a.SpiceUnload, GoldGive: a.GoldGive, GoldTake: a.GoldTake, Cards: a.Cards, Targets: a.Targets, Choice: a.Choice, Skill: a.Skill, Prompt: a.Prompt, X: a.X, Y: a.Y, Vertex: a.Vertex, Edge: a.Edge, Tile: a.Tile, Target: a.Target, Offer: a.Offer, Give: a.Give, Take: a.Take, Type: a.Type, Card: a.Card, Tokens: a.Tokens, Route: a.Route, Color: a.Color, Slot: a.Slot}
}
func fromCatanAction(a catan.Action) Action {
	return Action{SpiceLoad: a.SpiceLoad, SpiceUnload: a.SpiceUnload, GoldGive: a.GoldGive, GoldTake: a.GoldTake, Cards: a.Cards, Targets: a.Targets, Choice: a.Choice, Skill: a.Skill, Prompt: a.Prompt, X: a.X, Y: a.Y, Vertex: a.Vertex, Edge: a.Edge, Tile: a.Tile, Target: a.Target, Offer: a.Offer, Give: a.Give, Take: a.Take, Type: a.Type, Card: a.Card, Tokens: a.Tokens, Route: a.Route, Color: a.Color, Slot: a.Slot}
}
func fromCarcassonne(s *carcassonne.State) *State {
	if s == nil {
		return nil
	}
	return &State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Carcassonne: s.Carcassonne}
}
func (s *State) asCarcassonne() *carcassonne.State {
	if s == nil {
		return nil
	}
	return &carcassonne.State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Carcassonne: s.Carcassonne}
}
func (s *State) syncCarcassonne(v *carcassonne.State) {
	if s == nil {
		return
	}
	s.Kind = v.Kind
	s.Turn = v.Turn
	s.Phase = v.Phase
	s.Round = v.Round
	s.Finished = v.Finished
	s.Winners = v.Winners
	s.Log = v.Log
	s.Carcassonne = v.Carcassonne
}
func toCarcassonneAction(a Action) carcassonne.Action {
	return carcassonne.Action{X: a.X, Y: a.Y, Rotation: a.Rotation, Feature: a.Feature, Tile: a.Tile, Type: a.Type}
}
func fromCarcassonneAction(a carcassonne.Action) Action {
	return Action{X: a.X, Y: a.Y, Rotation: a.Rotation, Feature: a.Feature, Tile: a.Tile, Type: a.Type}
}
func fromSanguosha(s *sanguosha.State) *State {
	if s == nil {
		return nil
	}
	return &State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Sanguosha: s.Sanguosha}
}
func (s *State) asSanguosha() *sanguosha.State {
	if s == nil {
		return nil
	}
	return &sanguosha.State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Sanguosha: s.Sanguosha}
}
func (s *State) syncSanguosha(v *sanguosha.State) {
	if s == nil {
		return
	}
	s.Kind = v.Kind
	s.Turn = v.Turn
	s.Phase = v.Phase
	s.Round = v.Round
	s.Finished = v.Finished
	s.Winners = v.Winners
	s.Log = v.Log
	s.Sanguosha = v.Sanguosha
}
func toSanguoshaAction(a Action) sanguosha.Action {
	return sanguosha.Action{Cards: a.Cards, Targets: a.Targets, Choice: a.Choice, Skill: a.Skill, Direction: a.Direction, Prompt: a.Prompt, Target: a.Target, Take: a.Take, Type: a.Type, Card: a.Card, Color: a.Color, Slot: a.Slot}
}
func fromSanguoshaAction(a sanguosha.Action) Action {
	return Action{Cards: a.Cards, Targets: a.Targets, Choice: a.Choice, Skill: a.Skill, Direction: a.Direction, Prompt: a.Prompt, Target: a.Target, Take: a.Take, Type: a.Type, Card: a.Card, Color: a.Color, Slot: a.Slot}
}
func fromDota(s *dota.State) *State {
	if s == nil {
		return nil
	}
	return &State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Dota: s.Dota}
}
func (s *State) asDota() *dota.State {
	if s == nil {
		return nil
	}
	return &dota.State{Kind: s.Kind, Turn: s.Turn, Phase: s.Phase, Round: s.Round, Finished: s.Finished, Winners: s.Winners, Log: s.Log, Dota: s.Dota}
}
func (s *State) syncDota(v *dota.State) {
	if s == nil {
		return
	}
	s.Kind = v.Kind
	s.Turn = v.Turn
	s.Phase = v.Phase
	s.Round = v.Round
	s.Finished = v.Finished
	s.Winners = v.Winners
	s.Log = v.Log
	s.Dota = v.Dota
}
func toDotaAction(a Action) dota.Action {
	return dota.Action{Dota: a.Dota, Targets: a.Targets, Choice: a.Choice, Prompt: a.Prompt, Target: a.Target, Type: a.Type, Card: a.Card}
}
func fromDotaAction(a dota.Action) Action {
	return Action{Dota: a.Dota, Targets: a.Targets, Choice: a.Choice, Prompt: a.Prompt, Target: a.Target, Type: a.Type, Card: a.Card}
}
func New(kind string, n int) (*State, error) {
	switch kind {
	case "splendor":
		v, e := splendor.New(kind, n)
		return fromSplendor(v), e
	case "rail":
		v, e := rail.New(kind, n)
		return fromRail(v), e
	case "catan":
		v, e := catan.New(kind, n)
		return fromCatan(v), e
	case "carcassonne":
		v, e := carcassonne.New(kind, n)
		return fromCarcassonne(v), e
	case "sanguosha":
		v, e := sanguosha.New(kind, n)
		return fromSanguosha(v), e
	case "dota":
		v, e := dota.New(kind, n)
		return fromDota(v), e
	}
	return nil, errors.New("未知游戏")
}
func (s *State) Apply(player int, a Action) error {
	if s.Splendor != nil {
		v := s.asSplendor()
		e := v.Apply(player, toSplendorAction(a))
		s.syncSplendor(v)
		return e
	}
	if s.Rail != nil {
		v := s.asRail()
		e := v.Apply(player, toRailAction(a))
		s.syncRail(v)
		return e
	}
	if s.Catan != nil {
		v := s.asCatan()
		e := v.Apply(player, toCatanAction(a))
		s.syncCatan(v)
		return e
	}
	if s.Carcassonne != nil {
		v := s.asCarcassonne()
		e := v.Apply(player, toCarcassonneAction(a))
		s.syncCarcassonne(v)
		return e
	}
	if s.Sanguosha != nil {
		v := s.asSanguosha()
		e := v.Apply(player, toSanguoshaAction(a))
		s.syncSanguosha(v)
		return e
	}
	if s.Dota != nil {
		v := s.asDota()
		e := v.Apply(player, toDotaAction(a))
		s.syncDota(v)
		return e
	}
	return errors.New("未知游戏")
}
func (s *State) BotAction(player int) (Action, error) {
	if s.Splendor != nil {
		v := s.asSplendor()
		a, e := v.BotAction(player)
		return fromSplendorAction(a), e
	}
	if s.Rail != nil {
		v := s.asRail()
		a, e := v.BotAction(player)
		return fromRailAction(a), e
	}
	if s.Catan != nil {
		v := s.asCatan()
		a, e := v.BotAction(player)
		return fromCatanAction(a), e
	}
	if s.Carcassonne != nil {
		v := s.asCarcassonne()
		a, e := v.BotAction(player)
		return fromCarcassonneAction(a), e
	}
	if s.Sanguosha != nil {
		v := s.asSanguosha()
		a, e := v.BotAction(player)
		return fromSanguoshaAction(a), e
	}
	if s.Dota != nil {
		v := s.asDota()
		a, e := v.BotAction(player)
		return fromDotaAction(a), e
	}
	return Action{}, errors.New("未知游戏")
}
func (s *State) View(player int) map[string]any {
	if s.Splendor != nil {
		v := s.asSplendor()
		return v.View(player)
	}
	if s.Rail != nil {
		v := s.asRail()
		return v.View(player)
	}
	if s.Catan != nil {
		v := s.asCatan()
		return v.View(player)
	}
	if s.Carcassonne != nil {
		v := s.asCarcassonne()
		return v.View(player)
	}
	if s.Sanguosha != nil {
		v := s.asSanguosha()
		return v.View(player)
	}
	if s.Dota != nil {
		v := s.asDota()
		return v.View(player)
	}
	return nil
}
