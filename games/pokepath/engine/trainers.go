package engine

import (
	"errors"
	"slices"
)

type TrainerState struct {
	Gold        int64            `json:"gold"`
	Progression ProgressionState `json:"progression"`
}

type TrainerView struct {
	Actor       int             `json:"actor"`
	Gold        int64           `json:"gold"`
	Team        []Pokemon       `json:"team"`
	Box         []Pokemon       `json:"box"`
	Items       []OwnedItem     `json:"items"`
	Progression ProgressionView `json:"progression"`
}

func ownedPartners(values []Pokemon, actor int) []Pokemon {
	result := []Pokemon{}
	for _, value := range values {
		if value.Owner == actor {
			result = append(result, value)
		}
	}
	return result
}

func ownedItems(values []OwnedItem, actor int) []OwnedItem {
	result := []OwnedItem{}
	for _, value := range values {
		if value.Owner == actor {
			result = append(result, value)
		}
	}
	return result
}

func partnerMap[T any](values map[int]T, owned map[int]bool) map[int]T {
	result := map[int]T{}
	for id, value := range values {
		if owned[id] {
			result[id] = value
		}
	}
	return result
}

func partnerIDsValid[T any](values map[int]T, owned map[int]bool) bool {
	if values == nil {
		return false
	}
	for id := range values {
		if !owned[id] {
			return false
		}
	}
	return true
}

func mergePartnerMap[T any](original, changed map[int]T, owned map[int]bool) map[int]T {
	result := map[int]T{}
	for id, value := range original {
		if !owned[id] {
			result[id] = value
		}
	}
	for id, value := range changed {
		result[id] = value
	}
	return result
}

func sharedProgression(personal, shared ProgressionState) ProgressionState {
	personal.Profile.EnemyStats = shared.Profile.EnemyStats
	personal.Profile.DefeatedSpecies = shared.Profile.DefeatedSpecies
	personal.Profile.DefeatedEnemies = shared.Profile.DefeatedEnemies
	personal.Encounters = shared.Encounters
	personal.Challenge = shared.Challenge
	personal.ChallengeStartMS = shared.ChallengeStartMS
	personal.Ribbons = shared.Ribbons
	personal.SecretMaps = shared.SecretMaps
	personal.MirageVisible = shared.MirageVisible
	personal.RouteHealth = shared.RouteHealth
	personal.WaveRepeat = shared.WaveRepeat
	personal.WaveMirror = shared.WaveMirror
	personal.ActiveMirror = shared.ActiveMirror
	personal.BellUses = shared.BellUses
	personal.Bell = shared.Bell
	personal.ExtraGold = shared.ExtraGold
	personal.Completed = shared.Completed
	personal.PreviousRoute = shared.PreviousRoute
	personal.CustomData = shared.CustomData
	personal.CustomRoute = shared.CustomRoute
	return personal
}

func trainerLens(state *State, actor int) *State {
	value := *state
	value.Trainers = nil
	value.Team = ownedPartners(state.Team, actor)
	value.Box = ownedPartners(state.Box, actor)
	value.Inventory = ownedItems(state.Inventory, actor)
	if actor > 0 && actor < len(state.Trainers) {
		value.Gold = state.Trainers[actor].Gold
		value.Progression = sharedProgression(state.Trainers[actor].Progression, state.Progression)
	}
	owned := map[int]bool{}
	for _, pokemon := range append(append([]Pokemon{}, value.Team...), value.Box...) {
		owned[pokemon.ID] = true
	}
	value.Progression.Forms = partnerMap(state.Progression.Forms, owned)
	value.Progression.Reset = partnerMap(state.Progression.Reset, owned)
	value.Progression.DNA = partnerMap(state.Progression.DNA, owned)
	value.Progression.Appearance = partnerMap(state.Progression.Appearance, owned)
	value.Progression.HiddenShiny = partnerMap(state.Progression.HiddenShiny, owned)
	value.Progression.CustomSkins = partnerMap(state.Progression.CustomSkins, owned)
	value.Progression.UnownForms = partnerMap(state.Progression.UnownForms, owned)
	value.Progression.Dead = partnerMap(state.Progression.Dead, owned)
	return &value
}

func initializeTrainers(state *State) error {
	starters := []string{state.Options.Starter}
	for _, key := range []string{"charmander", "treecko", "froaki"} {
		if key != state.Options.Starter {
			starters = append(starters, key)
		}
	}
	trainers := make([]TrainerState, state.Players)
	team := []Pokemon{}
	var first ProgressionState
	for actor := 0; actor < state.Players; actor++ {
		state.Team = []Pokemon{}
		state.Progression = ProgressionState{}
		if err := InitProgression(state); err != nil {
			return err
		}
		for _, key := range starters {
			if err := progressionGrantPokemon(state, key, false); err != nil {
				return err
			}
		}
		for index := range state.Team {
			state.Team[index].Owner = actor
		}
		team = append(team, state.Team...)
		trainers[actor] = TrainerState{Gold: state.Gold, Progression: state.Progression}
		if actor == 0 {
			first = state.Progression
		}
	}
	state.Team, state.Progression = team, first
	if state.Players > 1 {
		state.Trainers = trainers
	}
	return nil
}

func trainerActionAllowed(state *State, actor int, action Action) error {
	for _, id := range []int{action.PokemonID, action.TowerID} {
		if id == 0 {
			continue
		}
		pokemon := ownedPokemonFor(state, id)
		if pokemon == nil && id == action.TowerID {
			for _, tower := range state.Towers {
				if tower.ID == id {
					pokemon = ownedPokemonFor(state, tower.PokemonID)
					break
				}
			}
		}
		if pokemon == nil || pokemon.Owner != actor {
			return errors.New("只能操作自己的宝可梦")
		}
	}
	if action.Type == "dna" {
		pokemon := ownedPokemonFor(state, action.TargetID)
		if pokemon == nil || pokemon.Owner != actor {
			return errors.New("只能选择自己的伙伴复制 DNA")
		}
	}
	return nil
}

func personalAction(kind string) bool {
	switch kind {
	case "deploy", "move", "retire", "upgrade", "target", "aim", "egg", "buyItem", "swapItems", "team", "teamSlot", "teamAll", "equip", "unequip", "reset", "dna", "form", "appearance", "shiny", "customSkin", "profile", "profileSecret", "tutorialSecret", "unown", "portrait", "profileRecordReset", "saveTeam", "loadTeam", "settings", "rename", "favorite", "claimReward", "secret", "mapOpen", "draftPick", "draftReroll", "draftRestart":
		return true
	}
	return false
}

func trainerLifecycleAction(kind string) bool {
	return slices.Contains([]string{"route", "challenge", "surrender", "retry", "continueRoute", "selectWave", "custom_play", "custom_retry", "custom_exit"}, kind)
}

func commitTrainer(state, value *State, actor int) {
	trainers := state.Trainers
	team, box, inventory := state.Team, state.Box, state.Inventory
	progression, gold := state.Progression, state.Gold
	owned := map[int]bool{}
	for _, pokemon := range append(append([]Pokemon{}, team...), box...) {
		if pokemon.Owner == actor {
			owned[pokemon.ID] = true
		}
	}
	for index := range value.Team {
		value.Team[index].Owner = actor
	}
	for index := range value.Box {
		value.Box[index].Owner = actor
	}
	for index := range value.Inventory {
		value.Inventory[index].Owner = actor
	}
	trainers[actor] = TrainerState{Gold: value.Gold, Progression: value.Progression}
	base := sharedProgression(progression, value.Progression)
	if actor == 0 {
		base, gold = value.Progression, value.Gold
	}
	base.Forms = mergePartnerMap(progression.Forms, value.Progression.Forms, owned)
	base.Reset = mergePartnerMap(progression.Reset, value.Progression.Reset, owned)
	base.DNA = mergePartnerMap(progression.DNA, value.Progression.DNA, owned)
	base.Appearance = mergePartnerMap(progression.Appearance, value.Progression.Appearance, owned)
	base.HiddenShiny = mergePartnerMap(progression.HiddenShiny, value.Progression.HiddenShiny, owned)
	base.CustomSkins = mergePartnerMap(progression.CustomSkins, value.Progression.CustomSkins, owned)
	base.UnownForms = mergePartnerMap(progression.UnownForms, value.Progression.UnownForms, owned)
	base.Dead = mergePartnerMap(progression.Dead, value.Progression.Dead, owned)
	*state = *value
	state.Gold, state.Trainers, state.Progression = gold, trainers, base
	state.Team, state.Box, state.Inventory = []Pokemon{}, []Pokemon{}, []OwnedItem{}
	for owner := 0; owner < state.Players; owner++ {
		if owner == actor {
			state.Team = append(state.Team, value.Team...)
			state.Box = append(state.Box, value.Box...)
			state.Inventory = append(state.Inventory, value.Inventory...)
		} else {
			state.Team = append(state.Team, ownedPartners(team, owner)...)
			state.Box = append(state.Box, ownedPartners(box, owner)...)
			state.Inventory = append(state.Inventory, ownedItems(inventory, owner)...)
		}
	}
}

func applyTrainerAction(state *State, actor int, action Action) error {
	if len(state.Trainers) == 0 {
		return executeAction(state, actor, action)
	}
	if err := trainerActionAllowed(state, actor, action); err != nil {
		return err
	}
	if slices.Contains([]string{"deploy", "move", "retire", "aim", "target"}, action.Type) {
		return executeAction(state, actor, action)
	}
	if personalAction(action.Type) {
		value := trainerLens(state, actor)
		if err := executeAction(value, actor, action); err != nil {
			return err
		}
		commitTrainer(state, value, actor)
		if action.Type == "draftPick" {
			for _, trainer := range state.Trainers {
				if trainer.Progression.Draft.Active {
					state.Phase = "draft"
				}
			}
		}
		return nil
	}
	if trainerLifecycleAction(action.Type) {
		original, err := Clone(state)
		if err != nil {
			return err
		}
		for owner := 0; owner < state.Players; owner++ {
			value := trainerLens(original, owner)
			value.Random, value.NextID, value.NextEventID, value.Events = state.Random, state.NextID, state.NextEventID, state.Events
			if err := executeAction(value, owner, action); err != nil {
				return err
			}
			commitTrainer(state, value, owner)
		}
		return nil
	}
	if action.Type == "wave" || action.Type == "auto" {
		for _, trainer := range state.Trainers {
			if trainer.Progression.Draft.Active {
				return errors.New("请等待所有训练家完成轮抽")
			}
		}
	}
	return executeAction(state, actor, action)
}

func validateTrainers(state *State) error {
	if len(state.Trainers) == 0 {
		return ValidateProgression(state)
	}
	if len(state.Trainers) != state.Players {
		return errors.New("训练家存档人数不一致")
	}
	owned := map[int]bool{}
	for _, pokemon := range append(append([]Pokemon{}, state.Team...), state.Box...) {
		owned[pokemon.ID] = true
	}
	p := state.Progression
	if !partnerIDsValid(p.Forms, owned) || !partnerIDsValid(p.Reset, owned) || !partnerIDsValid(p.DNA, owned) || !partnerIDsValid(p.Appearance, owned) || !partnerIDsValid(p.HiddenShiny, owned) || !partnerIDsValid(p.CustomSkins, owned) || !partnerIDsValid(p.UnownForms, owned) || !partnerIDsValid(p.Dead, owned) {
		return errors.New("训练家成长存档包含未知的伙伴标识")
	}
	for _, item := range state.Inventory {
		if item.Owner < 0 || item.Owner >= state.Players {
			return errors.New("背包物品归属无效")
		}
	}
	for actor := range state.Trainers {
		value := trainerLens(state, actor)
		if value.Gold < 0 || value.Gold > 9007199254740991 || len(value.Team) > state.TeamSlots {
			return errors.New("训练家金币或队伍无效")
		}
		if err := ValidateProgression(value); err != nil {
			return err
		}
	}
	return nil
}

func pokemonSettings(state *State, pokemon *Pokemon) ProgressionSettings {
	if pokemon.Owner > 0 && pokemon.Owner < len(state.Trainers) {
		return state.Trainers[pokemon.Owner].Progression.Settings
	}
	return state.Progression.Settings
}

func PlayerGold(state *State, actor int) int64 {
	if actor > 0 && actor < len(state.Trainers) {
		return state.Trainers[actor].Gold
	}
	return state.Gold
}

func projectTrainers(state *State, view View) View {
	view.Balances = make([]int64, state.Players)
	for actor := range view.Balances {
		view.Balances[actor] = PlayerGold(state, actor)
	}
	if len(state.Trainers) == 0 {
		return view
	}
	view.Trainers = make([]TrainerView, state.Players)
	for actor := range state.Trainers {
		personal := trainerLens(state, actor)
		progression := ProjectProgression(personal)
		projectProgressionLists(&progression)
		view.Trainers[actor] = TrainerView{Actor: actor, Gold: personal.Gold, Team: ownedPartners(view.Team, actor), Box: projectList(personal.Box, projectPokemonLists), Items: personal.Inventory, Progression: progression}
	}
	return view
}

func ProjectPlayer(state *State, actor int) View {
	return PersonalView(Project(state), actor)
}

func PersonalView(view View, actor int) View {
	if actor < 0 || actor >= len(view.Trainers) {
		return view
	}
	personal := view.Trainers[actor]
	view.BattleTeam = view.Team
	view.Gold, view.Team, view.Box, view.Items, view.Progression = personal.Gold, personal.Team, personal.Box, personal.Items, personal.Progression
	return view
}

func ProposePlayer(state *State, actor int) *Action {
	if len(state.Trainers) == 0 {
		return Propose(state)
	}
	return Propose(trainerLens(state, actor))
}
