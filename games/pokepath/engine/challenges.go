package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
)

type challengeRequest struct {
	LevelCap     int  `json:"levelCap"`
	SlotLimit    int  `json:"slotLimit"`
	ToughEnemies int  `json:"toughEnemies"`
	Draft        bool `json:"draft"`
	NoItems      bool `json:"noItems"`
	Permadeath   bool `json:"permadeath"`
	Mirror       bool `json:"mirror"`
}

func progressionChallengeAvailable(state *State) (bool, string) {
	if state.Progression.CustomRoute != nil {
		return false, "自定义练习不能开启路线挑战"
	}
	if state.Stars < 540 {
		return false, "需要先取得 540 颗星"
	}
	if state.Records[state.Route] < 100 {
		return false, "需要先完成当前路线的百波挑战"
	}
	if len(state.Team)+len(state.Box) < 30 {
		return false, "需要至少拥有 30 只伙伴"
	}
	if state.Paused || state.Health <= 0 {
		return false, "请先恢复游戏再开启挑战"
	}
	return true, ""
}

func applyChallengeAction(state *State, action Action) error {
	switch action.Type {
	case "challenge":
		if state.Progression.Challenge.Active {
			return errors.New("请先结束当前挑战")
		}
		if ok, reason := progressionChallengeAvailable(state); !ok {
			return errors.New(reason)
		}
		var request challengeRequest
		if len(action.Settings) == 0 || len(action.Settings) > 4096 || bytes.Equal(bytes.TrimSpace(action.Settings), []byte("null")) {
			return errors.New("请选择至少一项有效挑战")
		}
		decoder := json.NewDecoder(bytes.NewReader(action.Settings))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || !slices.Contains([]int{0, 50, 75}, request.LevelCap) || !slices.Contains([]int{0, 6, 8}, request.SlotLimit) || !slices.Contains([]int{0, 25, 50}, request.ToughEnemies) {
			return errors.New("挑战参数无效")
		}
		if request == (challengeRequest{}) {
			return errors.New("请选择至少一项挑战")
		}
		route, _ := routeFor(state.Route)
		state.Weather = route.WeatherID
		state.WeatherUntil = 0
		progressionClearField(state)
		state.Progression.ChallengeTeam = nil
		for _, pokemon := range state.Team {
			state.Progression.ChallengeTeam = append(state.Progression.ChallengeTeam, pokemon.ID)
		}
		state.Progression.Challenge = ChallengeRules{Active: true, LevelCap: request.LevelCap, SlotLimit: request.SlotLimit, ToughEnemies: request.ToughEnemies, Draft: request.Draft, NoItems: request.NoItems, Permadeath: request.Permadeath, Mirror: request.Mirror}
		state.Progression.ChallengeStartMS = nil
		state.Progression.WaveRepeat = false
		state.Progression.WaveMirror = false
		state.Progression.ActiveMirror = false
		state.Wave = 1
		state.RouteWaves[state.Route] = 1
		state.Health = 14
		state.MaxHealth = 14
		state.AutoWave = false
		state.Phase = "prepare"
		if err := progressionResetChallengeRoster(state); err != nil {
			return err
		}
		if request.Draft {
			progressionStartDraft(state)
		}
		state.Epoch++
		return nil
	case "surrender":
		if !state.Progression.Challenge.Active {
			return errors.New("当前没有进行中的挑战")
		}
		if state.Paused || state.Health <= 0 {
			return errors.New("暂停或战败时不能放弃挑战")
		}
		if err := progressionEndChallenge(state); err != nil {
			return err
		}
		state.Epoch++
		return nil
	case "draftPick":
		draft := &state.Progression.Draft
		if !draft.Active || action.TargetID != draft.Token || action.Pick < 0 || action.Pick >= len(draft.Offers) {
			return errors.New("轮抽选择已经变化，请重新选择")
		}
		id := draft.Offers[action.Pick]
		pokemon, inTeam, index := progressionOwned(state, id)
		if pokemon == nil || inTeam || slices.Contains(draft.Picked, id) || len(state.Team) >= ProgressionTeamCapacity(state) {
			return errors.New("这只伙伴不再是可选候选")
		}
		state.Team = append(state.Team, *pokemon)
		state.Box = append(state.Box[:index], state.Box[index+1:]...)
		draft.Picked = append(draft.Picked, id)
		draft.Pool = slices.DeleteFunc(draft.Pool, func(value int) bool { return value == id })
		draft.Round++
		draft.CanReroll = true
		if draft.Round >= draft.Rounds {
			draft.Active = false
			draft.Offers = nil
			draft.Token++
			state.Phase = "prepare"
			return nil
		}
		progressionDraftOffers(state)
		return nil
	case "draftReroll":
		draft := &state.Progression.Draft
		if !draft.Active || action.TargetID != draft.Token || !draft.CanReroll {
			return errors.New("本轮已重抽或候选已经变化")
		}
		draft.CanReroll = false
		progressionDraftOffers(state)
		return nil
	case "draftRestart":
		if !state.Progression.Draft.Active || action.TargetID != state.Progression.Draft.Token || state.WaveActive || state.Wave != 1 {
			return errors.New("当前不能重新轮抽")
		}
		progressionClearField(state)
		state.Box = append(state.Box, state.Team...)
		state.Team = []Pokemon{}
		state.Progression.Draft.RestartCount++
		progressionStartDraft(state)
		return nil
	case "claimReward":
		return progressionClaimReward(state, action.Slot)
	}
	return errors.New("未知的挑战操作")
}

func progressionChallengeElapsed(state *State) int64 {
	if start := state.Progression.ChallengeStartMS; state.Progression.Challenge.Active && start != nil {
		return state.Clock - *start
	}
	return 0
}

func progressionResetChallengeRoster(state *State) error {
	for index := range state.Inventory {
		state.Inventory[index].EquippedBy = 0
	}
	state.Box = append(state.Box, state.Team...)
	state.Team = []Pokemon{}
	for index := range state.Box {
		pokemon := &state.Box[index]
		pokemon.TileID = -1
		if base := state.Progression.Forms[pokemon.ID]; base != "" {
			pokemon.Species = base
		}
		species := ProgressionSpecies(state, pokemon)
		switch {
		case pokemon.Item == "ringTarget" || pokemon.Item != "" && species.ID == 53:
			pokemon.Target = "random"
		case pokemon.Item == "silphScope":
			pokemon.Target = "first"
			if slices.Contains([]string{"frisk", "vigilantFrisk", "moongeistBeam", "grimNeigh"}, species.AbilityID) {
				pokemon.Target = "invisible"
			}
		}
		pokemon.Item = ""
		if err := refreshPokemon(state, pokemon); err != nil {
			return err
		}
	}
	state.Progression.Forms = map[int]string{}
	return nil
}

func progressionEndChallenge(state *State) error {
	state.Progression.Challenge = ChallengeRules{}
	state.Progression.ChallengeStartMS = nil
	state.Progression.WaveRepeat = false
	state.Progression.WaveMirror = false
	state.Progression.ActiveMirror = false
	state.Progression.Draft = DraftState{Token: state.Progression.Draft.Token + 1}
	state.Towers = []Tower{}
	state.Projectiles = []Projectile{}
	state.World.Towers = nil
	state.World.PassengerHosts = nil
	state.World.Links = nil
	state.World.StrippedItems = nil
	state.World.Zones = slices.DeleteFunc(state.World.Zones, func(zone WorldZone) bool { return !zone.PersistentArea })
	state.World.ChessRecomputeAt = state.Tick + 50
	OnAttackModeRouteChanged(state)
	if err := progressionResetChallengeRoster(state); err != nil {
		return err
	}
	return worldUpdateWeather(state)
}

func progressionClearField(state *State) {
	resetWaveDamage(state)
	for index := range state.Team {
		state.Team[index].TileID = -1
	}
	for index := range state.Box {
		state.Box[index].TileID = -1
		resetPokemonDamage(state, &state.Box[index])
	}
	state.Towers = []Tower{}
	state.Enemies = []Enemy{}
	state.Projectiles = []Projectile{}
	state.WaveActive = false
	state.WaveKills = 0
	state.WaveGold = 0
	state.WaveHeartsLost = 0
	state.WaveHeartsHealed = 0
	WorldOnRouteChange(state)
	OnAttackModeRouteChanged(state)
}

func progressionStartDraft(state *State) {
	rounds := 10
	if state.Progression.Challenge.SlotLimit > 0 {
		rounds = state.Progression.Challenge.SlotLimit
	}
	previous := state.Progression.Draft
	draft := DraftState{Active: true, Rounds: rounds, Token: previous.Token, CanReroll: true, RestartCount: previous.RestartCount}
	for _, pokemon := range state.Box {
		if !state.Progression.Dead[pokemon.ID] {
			draft.Pool = append(draft.Pool, pokemon.ID)
		}
	}
	state.Progression.Draft = draft
	state.Phase = "draft"
	progressionDraftOffers(state)
}

func progressionDraftOffers(state *State) {
	draft := &state.Progression.Draft
	pool := draft.Pool
	for index := len(pool) - 1; index > 0; index-- {
		other := NextInt(state, index+1)
		pool[index], pool[other] = pool[other], pool[index]
	}
	draft.Offers = []int{}
	add := func(id int) {
		if id > 0 && !slices.Contains(draft.Offers, id) && len(draft.Offers) < 3 {
			draft.Offers = append(draft.Offers, id)
		}
	}
	guaranteed := 0
	find := func(predicate func(Species) bool) int {
		for _, id := range pool {
			if id == guaranteed {
				continue
			}
			pokemon, _, _ := progressionOwned(state, id)
			if pokemon != nil && predicate(ProgressionSpecies(state, pokemon)) {
				return id
			}
		}
		return 0
	}
	if draft.Round == 0 {
		guaranteed = find(func(species Species) bool { return slices.Contains([]int{36, 4, 3, 92, 163}, species.ID) })
		add(guaranteed)
	}
	if slices.Contains([]int{21, 26}, state.Route) && slices.Contains([]int{1, 2, 3, 5}, draft.Round) {
		add(find(func(species Species) bool {
			return slices.Contains([]string{"curse", "curseDoubleShot"}, species.AbilityID)
		}))
	}
	if slices.Contains([]int{22, 23, 24}, state.Route) && slices.Contains([]int{1, 3, 5}, draft.Round) {
		add(find(func(species Species) bool {
			return slices.Contains([]string{"solarBeam", "chlorophyll", "drySkin", "cloudNine"}, species.AbilityID)
		}))
	}
	if slices.Contains([]int{10, 11, 12, 13, 14, 15}, state.Route) {
		add(find(func(species Species) bool { return slices.Contains(species.Terrains, 3) }))
	} else if slices.Contains([]int{21, 26}, state.Route) {
		add(find(func(species Species) bool { return slices.Contains(species.Terrains, 4) }))
	} else if state.Route == 17 && draft.Round < 3 {
		add(find(func(species Species) bool {
			return slices.Contains([]string{"heal", "bitterBlade", "takeHeart"}, species.AbilityID)
		}))
	} else if state.Route == 16 && draft.Round < 3 {
		add(find(func(species Species) bool {
			return slices.Contains([]string{"stunArea", "stunMono", "stunMonoNerf", "static", "zapCannon"}, species.AbilityID)
		}))
	} else if state.Route == 8 && draft.Round < 3 {
		add(find(func(species Species) bool {
			return slices.Contains([]string{"poison", "poisonDoubleShot"}, species.AbilityID)
		}))
	}
	for _, id := range pool {
		add(id)
	}
	if !draft.CanReroll {
		for _, id := range draft.Offers {
			pokemon, _, _ := progressionOwned(state, id)
			if pokemon != nil && slices.Contains([]int{18, 84, 105}, CatalogData().Species[pokemon.Species].ID) {
				draft.CanReroll = true
			}
		}
	}
	draft.Token++
}

func progressionRibbonCount(state *State, route int) int {
	count := 0
	for _, value := range state.Progression.Ribbons[route] {
		count += value
	}
	return count
}

func awardChallengeRibbons(state *State) {
	if !state.Progression.Challenge.Active {
		return
	}
	if state.Progression.Ribbons[state.Route] == nil {
		state.Progression.Ribbons[state.Route] = map[string]int{}
	}
	rules := state.Progression.Challenge
	levels := map[string]int{}
	if rules.LevelCap > 0 {
		levels["levelCap"] = 1
		if rules.LevelCap == 50 {
			levels["levelCap"] = 2
		}
	}
	if rules.SlotLimit > 0 {
		levels["slotLimit"] = 1
		if rules.SlotLimit == 6 {
			levels["slotLimit"] = 2
		}
	}
	if rules.ToughEnemies > 0 {
		levels["toughEnemies"] = 1
		if rules.ToughEnemies == 50 {
			levels["toughEnemies"] = 2
		}
	}
	for key, enabled := range map[string]bool{"draft": rules.Draft, "noItems": rules.NoItems, "permadeath": rules.Permadeath, "mirror": rules.Mirror} {
		if enabled {
			levels[key] = 1
		}
	}
	for key, level := range levels {
		previous := state.Progression.Ribbons[state.Route][key]
		if level > previous {
			state.Ribbons += level - previous
			state.Progression.Ribbons[state.Route][key] = level
		}
	}
}

func progressionClaimReward(state *State, slot int) error {
	route, ok := routeFor(state.Route)
	thresholds := []int{2, 3, 5, 1}
	if !ok || state.Progression.CustomRoute != nil || slot < 0 || slot >= len(route.ChallengeRewards) || slot >= len(thresholds) || progressionRibbonCount(state, state.Route) < thresholds[slot] {
		return errors.New("当前路线奖带不足，或奖励不存在")
	}
	key := route.ChallengeRewards[slot]
	claims := state.Progression.Claims[state.Route]
	for len(claims) < 4 {
		claims = append(claims, false)
	}
	if slot == 1 {
		if progressionHasSpecies(state, key) {
			return errors.New("已经拥有这个奖励伙伴")
		}
		if err := progressionGrantPokemon(state, key, false); err != nil {
			return err
		}
	} else {
		item, exists := CatalogData().Items[key]
		if !exists || item.Equipable && progressionOwnsItem(state, key) || claims[slot] && !item.Equipable {
			return errors.New("奖励已领取或已经拥有")
		}
		if item.Equipable {
			state.Inventory = append(state.Inventory, OwnedItem{ID: key})
		} else {
			state.Progression.Unlocks[key] = true
		}
		var amount int64
		_ = json.Unmarshal(item.Facts["amount"], &amount)
		if amount > 0 {
			progressionGold(state, amount)
		}
	}
	claims[slot] = true
	state.Progression.Claims[state.Route] = claims
	return nil
}
