package engine

import "errors"

const legacySharedRevision = "401d71da4e3677ea2f18c69822760e066026d1e96ed303234a43b5e8a7d5a616"

func LegacySharedSave(state *State) bool {
	return state != nil && len(state.Trainers) == 0 && state.RulesVersion == "pokepath-native-1/"+legacySharedRevision+"/"+CatalogData().Source.SourceSHA256
}

func remapPartnerMap[T any](values map[int]T, ids map[int]int) map[int]T {
	result := map[int]T{}
	for id, value := range values {
		if next := ids[id]; next > 0 {
			result[next] = value
		}
	}
	return result
}

func remapPartnerList(values []int, ids map[int]int) []int {
	if values == nil {
		return nil
	}
	result := make([]int, len(values))
	for index, id := range values {
		result[index] = ids[id]
	}
	return result
}

func remapTrainer(state *State, actor int) {
	ids := map[int]int{}
	for _, group := range [][]Pokemon{state.Team, state.Box} {
		for index := range group {
			pokemon := &group[index]
			ids[pokemon.ID] = allocateID(state)
			pokemon.ID, pokemon.Owner, pokemon.TileID = ids[pokemon.ID], actor, -1
		}
	}
	for index := range state.Inventory {
		item := &state.Inventory[index]
		item.Owner, item.EquippedBy = actor, ids[item.EquippedBy]
	}
	p := &state.Progression
	p.Forms = remapPartnerMap(p.Forms, ids)
	p.Reset = remapPartnerMap(p.Reset, ids)
	p.DNA = remapPartnerMap(p.DNA, ids)
	p.Appearance = remapPartnerMap(p.Appearance, ids)
	p.HiddenShiny = remapPartnerMap(p.HiddenShiny, ids)
	p.CustomSkins = remapPartnerMap(p.CustomSkins, ids)
	p.UnownForms = remapPartnerMap(p.UnownForms, ids)
	p.Dead = remapPartnerMap(p.Dead, ids)
	p.ChallengeTeam = remapPartnerList(p.ChallengeTeam, ids)
	p.Draft.Pool = remapPartnerList(p.Draft.Pool, ids)
	p.Draft.Offers = remapPartnerList(p.Draft.Offers, ids)
	p.Draft.Picked = remapPartnerList(p.Draft.Picked, ids)
	for key, formations := range p.Formations {
		for index := range formations {
			for member := range formations[index].Members {
				value := &formations[index].Members[member]
				value.PokemonID = ids[value.PokemonID]
			}
		}
		p.Formations[key] = formations
	}
}

func grantStartingPartners(state *State) error {
	for _, key := range []string{"charmander", "treecko", "froaki"} {
		if !progressionHasSpecies(state, key) {
			if err := progressionGrantPokemon(state, key, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func MigrateSharedSave(state *State) error {
	if !LegacySharedSave(state) {
		return errors.New("没有可迁移的已知共享存档，原存档已保留")
	}
	state.RulesVersion = RulesVersion()
	if err := Validate(state); err != nil {
		return err
	}
	if state.Players == 1 {
		if err := grantStartingPartners(state); err != nil {
			return err
		}
		return Validate(state)
	}
	original, err := Clone(state)
	if err != nil {
		return err
	}
	state.Trainers = make([]TrainerState, state.Players)
	for actor := range state.Trainers {
		state.Trainers[actor] = TrainerState{Gold: original.Gold, Progression: original.Progression}
	}
	for actor := 0; actor < state.Players; actor++ {
		value, err := Clone(original)
		if err != nil {
			return err
		}
		value.NextID = state.NextID
		if actor > 0 {
			remapTrainer(value, actor)
		}
		if err := grantStartingPartners(value); err != nil {
			return err
		}
		commitTrainer(state, value, actor)
	}
	return Validate(state)
}
