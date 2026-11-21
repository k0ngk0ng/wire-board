package engine

import (
	"errors"
	"slices"
)

func NormalizeOptions(options Options, players int) (Options, error) {
	if players < 2 || players > 10 {
		return options, errors.New("反恐精英支持2至10人")
	}
	if options.Map == "" {
		options.Map = "de_dust2"
	}
	if options.RoundSeconds == 0 {
		options.RoundSeconds = 120
	}
	if options.MaxRounds == 0 {
		options.MaxRounds = 10
	}
	world := MapByID(options.Map)
	if world.ID == "" || options.RoundSeconds < 60 || options.RoundSeconds > 300 || options.MaxRounds < 1 || options.MaxRounds > 30 {
		return options, errors.New("地图、回合时长或局数无效")
	}
	if len(options.Teams) == 0 {
		options.Teams = make([]string, players)
		for seat := range players {
			options.Teams[seat] = "t"
			if seat%2 == 1 {
				options.Teams[seat] = "ct"
			}
		}
	} else {
		options.Teams = append([]string(nil), options.Teams...)
	}
	if len(options.Teams) != players {
		return options, errors.New("队伍数量与席位不一致")
	}
	counts := map[string]int{}
	for _, team := range options.Teams {
		if team != "t" && team != "ct" {
			return options, errors.New("请选择T或CT队伍")
		}
		counts[team]++
	}
	if counts["t"] < 1 || counts["ct"] < 1 || counts["t"] > 5 || counts["ct"] > 5 {
		return options, errors.New("每队须有1至5人")
	}
	if len(options.Characters) == 0 {
		options.Characters = make([]string, players)
		for seat := range players {
			characters := Characters(options.Teams[seat])
			options.Characters[seat] = characters[(seat/2)%len(characters)]
		}
	} else {
		options.Characters = append([]string(nil), options.Characters...)
	}
	if len(options.Characters) != players {
		return options, errors.New("角色数量与席位不一致")
	}
	for seat, character := range options.Characters {
		if !slices.Contains(Characters(options.Teams[seat]), character) {
			return options, errors.New("角色与队伍不一致")
		}
	}
	return options, nil
}

func New(options Options, players int, seed uint64) (*State, error) {
	options, err := NormalizeOptions(options, players)
	if err != nil {
		return nil, err
	}
	state := &State{Options: options, World: MapByID(options.Map), MapID: options.Map, Seed: seed, Players: make([]Player, players), Events: []Event{}, Grenades: []Grenade{}, Drops: []Drop{}, LoserBonusBase: 1400}
	if seed == 0 {
		state.Seed = 1
	}
	for seat := range players {
		state.Players[seat] = Player{Seat: seat, Team: options.Teams[seat], Character: options.Characters[seat], Money: 800, BotTarget: -1}
	}
	InitRound(state)
	for seat := range state.Players {
		groundCheck(state, &state.Players[seat])
	}
	return state, nil
}

func QueueInput(state *State, seat int, input Input) error {
	if seat < 0 || seat >= len(state.Players) || state.Finished {
		return errors.New("当前席位不能操作")
	}
	p := &state.Players[seat]
	if input.Life != p.Life || input.Seq <= p.ReceivedSeq || input.Seq > MaxInputSeq || input.Seq-p.ReceivedSeq > 4096 || input.ViewFrame < 0 || input.ViewFrame > state.Frame {
		return errors.New("操作序号、生命或参考帧无效")
	}
	if input.MoveX < -1000 || input.MoveX > 1000 || input.MoveY < -1000 || input.MoveY > 1000 || input.Yaw < 0 || input.Yaw >= 65536 || input.Pitch < -16000 || input.Pitch > 16000 {
		return errors.New("移动或视角超出范围")
	}
	if len(p.Pending) >= MaxPending {
		return errors.New("待执行操作过多")
	}
	if !p.Alive && !InputReleased(input) {
		return errors.New("死亡后不能操作")
	}
	if input.Select != "" {
		if _, ok := WeaponByID(input.Select); !ok {
			return errors.New("武器无效")
		}
	}
	input.QueuedFrame = state.Frame
	p.Pending = append(p.Pending, input)
	p.ReceivedSeq = input.Seq
	return nil
}

func InputReleased(input Input) bool {
	return input.MoveX == 0 && input.MoveY == 0 && !input.Fire && !input.Secondary && !input.Jump && !input.Crouch && !input.Walk && !input.Reload && !input.Use && !input.Drop && input.Select == ""
}

func ClearInput(state *State, seat int) {
	if seat < 0 || seat >= len(state.Players) {
		return
	}
	p := &state.Players[seat]
	p.Input = Input{Life: p.Life, Yaw: p.Yaw, Pitch: p.Pitch}
	p.Previous = p.Input
	p.Pending = nil
	p.Action, p.ActionSince = "", 0
	CancelGrenade(p)
	p.VX, p.VY = 0, 0
}

func Advance(state *State, targetFrame int64) {
	if state.Finished || targetFrame <= state.Frame {
		return
	}
	limit := min(targetFrame, state.Frame+1000)
	for state.Frame < limit && !state.Finished {
		state.Frame++
		stepBrushes(state)
		for seat := range state.Players {
			p := &state.Players[seat]
			p.Previous = p.Input
			for len(p.Pending) > 0 {
				input := p.Pending[0]
				p.Pending = p.Pending[1:]
				if input.Life != p.Life || state.Frame-input.QueuedFrame > 20 {
					p.AppliedSeq = input.Seq
					continue
				}
				p.Input = input
				p.AppliedSeq = input.Seq
				p.LastInputFrame = state.Frame
				break
			}
			if state.Frame-p.LastInputFrame > 30 {
				if p.Input.Fire {
					CancelGrenade(p)
				}
				p.Input = Input{Life: p.Life, Yaw: p.Yaw, Pitch: p.Pitch}
			}
			movePlayer(state, p)
			if len(state.World.WorldWeapons) > 0 {
				TouchMapWeapons(state, seat)
			}
		}
		StepCombat(state)
		StepObjectives(state)
		StepBotNavigation(state)
		recordHistory(state)
	}
}

func recordHistory(state *State) {
	history := History{Frame: state.Frame, Players: make([]HistoryPlayer, len(state.Players)), Brushes: append([]BrushState(nil), state.Brushes...)}
	for seat, p := range state.Players {
		history.Players[seat] = HistoryPlayer{Seat: seat, X: p.X, Y: p.Y, Z: p.Z, Alive: p.Alive, Life: p.Life, Crouch: p.Crouch, Yaw: p.Yaw, Weapon: p.Weapon}
	}
	state.History = append(state.History, history)
	if len(state.History) > MaxHistory {
		copy(state.History, state.History[len(state.History)-MaxHistory:])
		state.History = state.History[:MaxHistory]
	}
}
