package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type AudioSettings struct {
	Master  int `json:"master"`
	Music   int `json:"music"`
	UI      int `json:"ui"`
	Effects int `json:"effects"`
}

type ProgressionSettings struct {
	AutoStop         bool          `json:"autoStop"`
	AutoStopBoss     bool          `json:"autoStopBoss"`
	AutoReset        int           `json:"autoReset"`
	DisplayHealth    int           `json:"displayHealth"`
	MapEffectsMode   int           `json:"mapEffectsMode"`
	IndicatorField   bool          `json:"indicatorField"`
	IndicatorShape   bool          `json:"indicatorShape"`
	FastTarget       bool          `json:"fastTarget"`
	DurationCC       bool          `json:"durationCC"`
	Particles        bool          `json:"particles"`
	DisableTagColors bool          `json:"disableTagColors"`
	EffectOpacity    EffectOpacity `json:"effectOpacity"`
	Mute             [5]bool       `json:"mute"`
	Audio            AudioSettings `json:"audio"`
	Language         int           `json:"language"`
	Theme            string        `json:"theme"`
	Font             string        `json:"font"`
	ShowDamage       bool          `json:"showDamage"`
	ShowHealth       bool          `json:"showHealth"`
	MapEffects       bool          `json:"mapEffects"`
	ColorTags        bool          `json:"colorTags"`
	EffectAlpha      int           `json:"effectAlpha"`
}

type ChallengeRules struct {
	Active       bool `json:"active"`
	LevelCap     int  `json:"levelCap"`
	SlotLimit    int  `json:"slotLimit"`
	ToughEnemies int  `json:"toughEnemies"`
	Draft        bool `json:"draft"`
	NoItems      bool `json:"noItems"`
	Permadeath   bool `json:"permadeath"`
	Mirror       bool `json:"mirror"`
}

type DraftState struct {
	Active       bool  `json:"active"`
	Round        int   `json:"round"`
	Rounds       int   `json:"rounds"`
	Token        int   `json:"token"`
	Pool         []int `json:"pool"`
	Offers       []int `json:"offers"`
	Picked       []int `json:"picked"`
	CanReroll    bool  `json:"canReroll"`
	RestartCount int   `json:"restartCount"`
}

type FormationMember struct {
	PokemonID int    `json:"pokemonID"`
	TileID    int    `json:"tileID"`
	Target    string `json:"target"`
	ItemID    string `json:"itemID"`
	Form      string `json:"form,omitempty"`
}

type Formation struct {
	Members []FormationMember `json:"members"`
}

type BellProgress struct {
	Group     int   `json:"group"`
	Clicks    int   `json:"clicks"`
	LastAt    int64 `json:"lastAt"`
	NeedsRest bool  `json:"needsRest"`
}

type EncounterProgress struct {
	Observed           bool    `json:"observed"`
	Hits               int     `json:"hits"`
	ItemAttacks        int     `json:"itemAttacks"`
	IndirectApplied    bool    `json:"indirectApplied"`
	IndirectDamage     float64 `json:"indirectDamage"`
	GrassAttacks       int     `json:"grassAttacks"`
	WaterDamage        float64 `json:"waterDamage"`
	RingDamage         float64 `json:"ringDamage"`
	MaxDamage          float64 `json:"maxDamage"`
	MaxDirectAttempt   float64 `json:"maxDirectAttempt"`
	FinalTerrain       int     `json:"finalTerrain"`
	Criticals          int     `json:"criticals"`
	Stuns              int     `json:"stuns"`
	Slows              int     `json:"slows"`
	InvisibilityBreaks int     `json:"invisibilityBreaks"`
	Dodges             int     `json:"dodges"`
	RegenReduced       bool    `json:"regenReduced"`
	Poisoned           bool    `json:"poisoned"`
	PowerOver100       bool    `json:"powerOver100"`
	NonWaterAttacker   bool    `json:"nonWaterAttacker"`
	BurnedEarly        bool    `json:"burnedEarly"`
	StunActive         bool    `json:"stunActive"`
	SlowActive         bool    `json:"slowActive"`
}

type ProgressionState struct {
	Profile          TrainerProgress           `json:"profile"`
	Encounters       map[int]EncounterProgress `json:"encounters"`
	Version          int                       `json:"version"`
	EggPrice         int64                     `json:"eggPrice"`
	EggPool          []string                  `json:"eggPool"`
	ShopStock        []string                  `json:"shopStock"`
	ShopPools        map[string][]string       `json:"shopPools"`
	Formations       map[string][]Formation    `json:"formations"`
	Challenge        ChallengeRules            `json:"challenge"`
	ChallengeStartMS *int64                    `json:"challengeStartMS,omitempty"`
	Draft            DraftState                `json:"draft"`
	Ribbons          map[int]map[string]int    `json:"ribbons"`
	Claims           map[int][]bool            `json:"claims"`
	Secrets          map[string]bool           `json:"secrets"`
	SecretMaps       map[int]bool              `json:"secretMaps"`
	MirageVisible    bool                      `json:"mirageVisible"`
	SecretItems      map[string]bool           `json:"secretItems"`
	Unlocks          map[string]bool           `json:"unlocks"`
	Achievements     map[string]bool           `json:"achievements"`
	RouteHealth      map[int]int               `json:"routeHealth"`
	Settings         ProgressionSettings       `json:"settings"`
	Forms            map[int]string            `json:"forms"`
	Reset            map[int]bool              `json:"reset"`
	DNA              map[int]string            `json:"dna"`
	Appearance       map[int]string            `json:"appearance"`
	HiddenShiny      map[int]bool              `json:"hiddenShiny"`
	CustomSkins      map[int]bool              `json:"customSkins"`
	WaveRepeat       bool                      `json:"waveRepeat"`
	WaveMirror       bool                      `json:"waveMirror"`
	ActiveMirror     bool                      `json:"activeMirror"`
	ProfileName      string                    `json:"profileName"`
	UnownStage       int                       `json:"unownStage"`
	UnownLetter      string                    `json:"unownLetter"`
	UnownForms       map[int]string            `json:"unownForms"`
	BellUses         int                       `json:"bellUses"`
	Bell             BellProgress              `json:"bell"`
	ExtraGold        int64                     `json:"extraGold"`
	TotalGold        int64                     `json:"totalGold"`
	Completed        int                       `json:"completed"`
	PreviousRoute    int                       `json:"previousRoute"`
	ChallengeTeam    []int                     `json:"challengeTeam"`
	Dead             map[int]bool              `json:"dead"`
	CustomData       *EditorData               `json:"customData,omitempty"`
	CustomRoute      *RouteSpec                `json:"customRoute,omitempty"`
}

type ShopOffer struct {
	Stock       int    `json:"stock"`
	ItemID      string `json:"itemID"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sprite      string `json:"sprite"`
	Tier        string `json:"tier"`
	Price       int64  `json:"price"`
	Affordable  bool   `json:"affordable"`
	Equipable   bool   `json:"equipable"`
}

type SecretHotspot struct {
	Key        string  `json:"key"`
	Name       string  `json:"name"`
	Route      int     `json:"route"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	Target     string  `json:"target"`
	Kind       string  `json:"kind"`
	NeedsQuiet bool    `json:"needsQuiet"`
}

type ProgressionView struct {
	Portrait           int                      `json:"portrait"`
	ProfileStats       TrainerStats             `json:"profileStats"`
	EnemyStats         map[string]DexEnemyStats `json:"enemyStats"`
	Growth             map[int]GrowthView       `json:"growth"`
	EggPrice           int64                    `json:"eggPrice"`
	RemainingEggs      int                      `json:"remainingEggs"`
	Shop               []ShopOffer              `json:"shop"`
	TeamCapacity       int                      `json:"teamCapacity"`
	CanChallenge       bool                     `json:"canChallenge"`
	ChallengeReason    string                   `json:"challengeReason"`
	Challenge          ChallengeRules           `json:"challenge"`
	ChallengeElapsedMS int64                    `json:"challengeElapsedMS,omitempty"`
	Draft              DraftState               `json:"draft"`
	Formations         []Formation              `json:"formations"`
	Ribbons            map[int]map[string]int   `json:"ribbons"`
	Claims             map[int][]bool           `json:"claims"`
	Secrets            map[string]bool          `json:"secrets"`
	SecretMaps         map[int]bool             `json:"secretMaps"`
	SecretItems        map[string]bool          `json:"secretItems"`
	Unlocks            map[string]bool          `json:"unlocks"`
	Achievements       map[string]bool          `json:"achievements"`
	Settings           ProgressionSettings      `json:"settings"`
	Hotspots           []SecretHotspot          `json:"hotspots"`
	CurrentRibbons     int                      `json:"currentRibbons"`
	ExtraGold          int64                    `json:"extraGold"`
	Reset              map[int]bool             `json:"reset"`
	DNA                map[int]string           `json:"dna"`
	Appearance         map[int]string           `json:"appearance"`
	HiddenShiny        map[int]bool             `json:"hiddenShiny"`
	CustomSkins        map[int]bool             `json:"customSkins"`
	Speeds             []float64                `json:"speeds"`
	CanManageWaves     bool                     `json:"canManageWaves"`
	WaveRepeat         bool                     `json:"waveRepeat"`
	WaveMirror         bool                     `json:"waveMirror"`
	ProfileName        string                   `json:"profileName"`
	UnownStage         int                      `json:"unownStage"`
	UnownLetter        string                   `json:"unownLetter"`
	UnownQuestion      string                   `json:"unownQuestion"`
	CustomData         *EditorData              `json:"customData,omitempty"`
	Custom             bool                     `json:"custom"`
	RewardThresholds   []int                    `json:"rewardThresholds"`
}

type GrowthView struct {
	Forms         []string `json:"forms"`
	Appearances   []string `json:"appearances"`
	CanReset      bool     `json:"canReset"`
	CanDNA        bool     `json:"canDNA"`
	CanShiny      bool     `json:"canShiny"`
	CanCustomSkin bool     `json:"canCustomSkin"`
}

func InitProgression(state *State) error {
	catalog := CatalogData()
	if state == nil || catalog == nil {
		return errors.New("成长目录尚未就绪")
	}
	if state.Progression.Version != 0 {
		return nil
	}
	state.Progression = ProgressionState{
		Encounters: map[int]EncounterProgress{}, Version: 1, EggPrice: 10, ShopStock: make([]string, 10), ShopPools: map[string][]string{},
		Formations: map[string][]Formation{}, Ribbons: map[int]map[string]int{}, Claims: map[int][]bool{},
		Secrets: map[string]bool{}, SecretMaps: map[int]bool{}, SecretItems: map[string]bool{}, Unlocks: map[string]bool{},
		Achievements: map[string]bool{}, Forms: map[int]string{}, Reset: map[int]bool{}, DNA: map[int]string{}, Appearance: map[int]string{}, HiddenShiny: map[int]bool{}, CustomSkins: map[int]bool{}, UnownForms: map[int]string{}, ProfileName: "Player", RouteHealth: map[int]int{}, Dead: map[int]bool{}, TotalGold: state.Gold,
		Settings: ProgressionSettings{Audio: AudioSettings{Master: 12, Music: 12, UI: 8, Effects: 8}, Language: 8, Theme: "original", Font: "PressStart2P", ShowDamage: true, MapEffects: true, MapEffectsMode: 1, Particles: true, ColorTags: true, EffectAlpha: 100, EffectOpacity: EffectOpacity{Projectiles: 100, Areas: 100, Beams: 100, Waves: 100, Auras: 50, Bombardment: 100, Special: 100}},
	}
	owned := map[int]bool{}
	if starter, ok := catalog.Species[state.Options.Starter]; ok {
		owned[starter.ID] = true
	}
	for _, pokemon := range append(append([]Pokemon{}, state.Team...), state.Box...) {
		owned[catalog.Species[pokemon.Species].ID] = true
	}
	for _, key := range catalog.EggPool {
		if species, ok := catalog.Species[key]; ok && !owned[species.ID] {
			state.Progression.EggPool = append(state.Progression.EggPool, key)
		}
	}
	for name, pool := range catalog.ItemPools {
		state.Progression.ShopPools[name] = append([]string{}, pool...)
	}
	for index := range state.Progression.ShopStock {
		progressionRestock(state, index)
	}
	return nil
}

func ApplyProgressionAction(state *State, actor int, action Action) (bool, error) {
	switch action.Type {
	case "egg":
		return true, progressionBuyEgg(state, actor, action.Quote)
	case "buyItem":
		return true, progressionBuyItem(state, actor, action)
	case "swapItems":
		return true, progressionSwapItems(state, action.ItemID, action.Name)
	case "team":
		return true, progressionTransfer(state, action.PokemonID, action.Mode)
	case "teamSlot":
		return true, progressionPlaceInTeamSlot(state, action)
	case "teamAll":
		return true, progressionTransferAll(state, action.Mode)
	case "equip":
		return true, progressionEquip(state, action.PokemonID, action.ItemID)
	case "unequip":
		return true, progressionUnequip(state, action.PokemonID)
	case "reset":
		return true, progressionResetPokemon(state, action.PokemonID)
	case "dna":
		return true, progressionDNA(state, action.PokemonID, action.TargetID)
	case "form":
		return true, progressionChangeForm(state, action.PokemonID, action.Mode)
	case "appearance":
		return true, progressionAppearance(state, action.PokemonID, action.Mode)
	case "shiny":
		return true, progressionShiny(state, action.PokemonID, action.Enabled)
	case "customSkin":
		return true, progressionCustomSkin(state, action.PokemonID, action.Enabled)
	case "speed":
		options := progressionSpeeds(state)
		if action.Value == 0 {
			action.Value = options[(slices.Index(options, state.Speed)+1)%len(options)]
		}
		if !slices.Contains(options, action.Value) && action.Value != 1 {
			return true, errors.New("这个速度尚未解锁")
		}
		return true, applyCoreAction(state, actor, action)
	case "selectWave", "repeatWave", "mirrorWave":
		return true, progressionManageWaves(state, action)
	case "profile", "profileSecret", "tutorialSecret", "unown":
		return true, progressionProfileAction(state, action)
	case "portrait", "profileRecordReset":
		return true, trainerProfileAction(state, action)
	case "saveTeam":
		return true, progressionSaveFormation(state, action.Slot)
	case "loadTeam":
		return true, progressionLoadFormation(state, actor, action.Slot)
	case "route":
		return true, progressionChangeRoute(state, action.Route)
	case "mapOpen":
		return true, progressionMapOpen(state)
	case "settings":
		return true, progressionSettings(state, action)
	case "secret", "bell":
		return true, progressionSecret(state, action)
	case "challenge", "surrender", "draftPick", "draftReroll", "draftRestart", "claimReward":
		return true, applyChallengeAction(state, action)
	case "retry":
		if state.Progression.Challenge.Active && state.Progression.Challenge.Permadeath {
			if state.Health > 0 || state.Phase != "defeat" {
				return true, errors.New("当前对局不能重试")
			}
			progressionClearField(state)
			if err := progressionEndChallenge(state); err != nil {
				return true, err
			}
			state.Phase = "defeat"
			return true, applyCoreAction(state, actor, action)
		}
	case "rename":
		pokemon, _, _ := progressionOwned(state, action.PokemonID)
		name := strings.TrimSpace(action.Name)
		if pokemon == nil || len([]rune(name)) > 24 {
			return true, errors.New("请选择已有伙伴并使用不超过 24 个字的昵称")
		}
		pokemon.Alias = name
		if pokemon.Species == "unown" {
			if progressionUnownLetter(name) {
				state.Progression.UnownForms[pokemon.ID] = strings.ToUpper(name)
			}
			return true, refreshPokemon(state, pokemon)
		}
		return true, nil
	case "favorite":
		pokemon, _, _ := progressionOwned(state, action.PokemonID)
		if pokemon == nil {
			return true, errors.New("找不到这只伙伴")
		}
		pokemon.Favorite = action.Enabled
		return true, nil
	}
	return false, nil
}

func OnWaveCompleted(state *State) {
	route, exists := routeFor(state.Route)
	if !exists || state.Health <= 0 || state.Progression.CustomRoute != nil {
		return
	}
	state.Progression.Completed++
	goldBefore := state.Gold
	state.Progression.BellUses = 0
	bonus := int64(math.Floor((5*float64(state.Route+1)*float64(state.Wave) + math.Pow(float64(state.Wave), 1.4)) / 2))
	if state.Stars > 150 {
		bonus = int64(math.Floor(float64(bonus) * float64(state.Stars) / 150))
	}
	if !route.IsSecret && state.Wave <= 100 && state.Records[state.Route] < state.Wave {
		state.Records[state.Route] = state.Wave
		if state.Stars > 50 {
			progressionGold(state, bonus)
		}
		state.Stars++
		state.TeamSlots = 6
		for _, threshold := range []int{40, 160, 320, 540} {
			if state.Stars >= threshold {
				state.TeamSlots++
			}
		}
		for _, threshold := range []struct {
			divisor int
			amount  int64
		}{{25, 1}, {100, 2}, {300, 3}, {400, 4}, {450, 5}, {800, 10}, {850, 20}, {875, 20}} {
			if state.Stars%threshold.divisor == 0 {
				state.Progression.ExtraGold += threshold.amount
			}
		}
	}
	progressionGold(state, bonus)
	recordTrainerWave(state, true, state.WaveGold+state.Gold-goldBefore)
	if state.Progression.Challenge.Active && state.Wave == 100 {
		awardChallengeRibbons(state)
	}
	state.Progression.RouteHealth[state.Route] = state.Health
	progressionCheckAchievements(state)
}

func ProjectProgression(state *State) ProgressionView {
	catalog := CatalogData()
	copyState := ProgressionState{}
	bytes, _ := json.Marshal(state.Progression)
	_ = json.Unmarshal(bytes, &copyState)
	result := ProgressionView{EggPrice: state.Progression.EggPrice, RemainingEggs: len(state.Progression.EggPool), TeamCapacity: ProgressionTeamCapacity(state), Challenge: copyState.Challenge, Draft: copyState.Draft, Formations: append([]Formation{}, copyState.Formations[progressionFormationKey(state)]...), Ribbons: copyState.Ribbons, Claims: copyState.Claims, Secrets: copyState.Secrets, SecretMaps: copyState.SecretMaps, SecretItems: copyState.SecretItems, Unlocks: copyState.Unlocks, Achievements: copyState.Achievements, Settings: copyState.Settings, ExtraGold: copyState.ExtraGold, Reset: copyState.Reset, DNA: copyState.DNA, Appearance: copyState.Appearance, HiddenShiny: copyState.HiddenShiny, CustomSkins: copyState.CustomSkins, CustomData: copyState.CustomData, Custom: copyState.CustomRoute != nil, RewardThresholds: []int{2, 3, 5, 1}}
	for len(result.Formations) < 5 {
		result.Formations = append(result.Formations, Formation{})
	}
	result.CanChallenge, result.ChallengeReason = progressionChallengeAvailable(state)
	result.ChallengeElapsedMS = progressionChallengeElapsed(state)
	result.Portrait = state.Progression.Profile.Portrait
	result.ProfileStats = projectTrainerStats(state)
	result.EnemyStats = copyState.Profile.EnemyStats
	if result.EnemyStats == nil {
		result.EnemyStats = map[string]DexEnemyStats{}
	}
	result.Growth = progressionGrowthView(state)
	result.Speeds = progressionSpeeds(state)
	result.CanManageWaves = progressionWaveManagementAvailable(state)
	result.WaveRepeat = copyState.WaveRepeat
	result.WaveMirror = copyState.WaveMirror
	result.ProfileName = copyState.ProfileName
	result.UnownStage = copyState.UnownStage
	result.UnownLetter = copyState.UnownLetter
	result.UnownQuestion = []string{"能读懂我说的话吗？", "我是未知图腾，你是谁？", "我有多少种形态？", "你最喜欢哪一种？", "这个形态的含义是什么？", "答对了。"}[copyState.UnownStage]
	result.CurrentRibbons = progressionRibbonCount(state, state.Route)
	if state.Progression.MirageVisible {
		result.SecretMaps[30] = true
	}
	result.Hotspots = progressionHotspots(state)
	result.Shop = make([]ShopOffer, 10)
	for index, key := range state.Progression.ShopStock {
		item, ok := catalog.Items[key]
		result.Shop[index] = ShopOffer{Stock: index, ItemID: key}
		if ok {
			result.Shop[index] = ShopOffer{Stock: index, ItemID: key, Name: item.Name, Description: item.Description, Sprite: item.Sprite, Tier: item.Tier, Price: item.Price, Affordable: state.Gold >= item.Price, Equipable: item.Equipable}
		}
	}
	projectProgressionLists(&result)
	return result
}

func ProgressionTeamCapacity(state *State) int {
	capacity := state.TeamSlots
	if state.Progression.Challenge.Active && state.Progression.Challenge.SlotLimit > 0 {
		capacity = min(capacity, state.Progression.Challenge.SlotLimit)
	}
	return max(1, min(capacity, 10))
}

func progressionSpeeds(state *State) []float64 {
	options := []float64{0.8, 1.2, 1.7, 2, 2.5}
	if progressionOwnsItem(state, "rosesItem") {
		options = append(options, 5, 0.4)
	}
	return options
}

func ProgressionMirrored(state *State) bool {
	if state.WaveActive {
		return state.Progression.ActiveMirror
	}
	return state.Progression.Challenge.Active && state.Progression.Challenge.Mirror || state.Progression.WaveMirror
}

func ProgressionRepeatsWave(state *State) bool {
	return state.Progression.WaveRepeat && !state.Progression.Challenge.Active && state.Progression.CustomRoute == nil
}

func progressionWaveManagementAvailable(state *State) bool {
	return state.Progression.CustomRoute == nil && !state.Progression.Challenge.Active && state.Records[state.Route] == 100 && progressionOwnsItem(state, "bicycle")
}

func progressionManageWaves(state *State, action Action) error {
	if !progressionWaveManagementAvailable(state) || state.Paused || state.Health <= 0 {
		return errors.New("波次管理需要自行车与当前路线的百波纪录")
	}
	switch action.Type {
	case "repeatWave":
		state.Progression.WaveRepeat = action.Enabled
	case "mirrorWave":
		state.Progression.WaveMirror = action.Enabled
	case "selectWave":
		if state.WaveActive || !finiteNumber(action.Value) || math.Trunc(action.Value) != action.Value {
			return errors.New("请在波次结束后选择有效波次")
		}
		next := int(action.Value)
		if action.Mode == "delta" {
			if !slices.Contains([]int{-10, -1, 1, 10}, next) {
				return errors.New("请选择前后 1 波或 10 波")
			}
			next += state.Wave
			if next <= 0 {
				next = 100
			} else if next >= 101 {
				next = 1
			}
		} else if action.Mode != "" {
			return errors.New("无效的波次选择方式")
		}
		if next < 1 || next > 100 {
			return errors.New("请选择 1 至 100 波")
		}
		state.Wave = next
		state.RouteWaves[state.Route] = next
		state.WaveGold = 0
		state.WaveKills = 0
		state.WaveHeartsLost = 0
		state.WaveHeartsHealed = 0
		state.Enemies = []Enemy{}
		state.Projectiles = []Projectile{}
		state.Progression.BellUses = 0
		state.Progression.Encounters = map[int]EncounterProgress{}
		state.Phase = "prepare"
		for index := range state.Towers {
			tower := &state.Towers[index]
			tower.CooldownMS = 0
			tower.AbilityCooldownMS = 0
			tower.SameTargetHits = 0
			tower.IdleMS = 0
			tower.AttackCount = 0
			tower.Stacks = map[string]float64{}
		}
		state.Epoch++
	}
	return nil
}

func progressionNormalizeAnswer(value string) string {
	var normalized strings.Builder
	for _, letter := range norm.NFD.String(strings.ToLower(strings.TrimSpace(value))) {
		if !unicode.Is(unicode.Mn, letter) {
			normalized.WriteRune(letter)
		}
	}
	return normalized.String()
}

func progressionUnownLetter(letter string) bool {
	letter = strings.ToUpper(letter)
	return len(letter) == 1 && (letter[0] >= 'A' && letter[0] <= 'Z' || letter == "!" || letter == "?")
}

func progressionProfileAction(state *State, action Action) error {
	if state.Paused || state.Progression.CustomRoute != nil {
		return errors.New("当前不能进行永久资料或秘密交互")
	}
	switch action.Type {
	case "profile":
		name := strings.TrimSpace(action.Name)
		if name == "" || len([]rune(name)) > 11 {
			return errors.New("训练家名称须为 1～11 个字")
		}
		state.Progression.ProfileName = name
	case "profileSecret":
		if state.Progression.Challenge.Active || action.Value != 7 && action.Value != 19 {
			return errors.New("这张成就卡没有隐藏伙伴")
		}
		GrantProgressionSecret(state, "luvdisc")
	case "tutorialSecret":
		if action.Name != "secrets" {
			return errors.New("这张教程插图没有隐藏伙伴")
		}
		GrantProgressionSecret(state, "tyrunt")
	case "unown":
		stage := state.Progression.UnownStage
		if action.TargetID != stage || stage >= 5 || len(action.Name) > 256 {
			return errors.New("未知图腾的问题已经变化")
		}
		answer := progressionNormalizeAnswer(action.Name)
		correct := false
		switch stage {
		case 0:
			accepted := [][]string{{"yes"}, {"si"}, {"oui"}, {"sim"}, {"si"}, {"ja"}, {"はい", "yes"}, {"네", "예", "yes"}, {"是", "是的", "yes"}, {"tak"}}
			correct = slices.Contains(accepted[state.Progression.Settings.Language], answer)
		case 1:
			correct = answer == progressionNormalizeAnswer(state.Progression.ProfileName)
		case 2:
			correct = strings.TrimSpace(action.Name) == "28"
		case 3:
			letter := strings.ToUpper(strings.TrimSpace(action.Name))
			correct = progressionUnownLetter(letter)
			if correct {
				state.Progression.UnownLetter = letter
			}
		case 4:
			meanings := map[string]string{"A": "anger", "B": "bear", "C": "chase", "D": "direct", "E": "engage", "F": "find", "G": "give", "H": "help", "I": "increase", "J": "join", "K": "keep", "L": "laugh", "M": "make", "N": "nuzzle", "O": "observe", "P": "perform", "Q": "quicken", "R": "reassure", "S": "search", "T": "tell", "U": "undo", "V": "vanish", "W": "want", "X": "xxxxx", "Y": "yield", "Z": "zoom", "!": "!!!!!", "?": "?????"}
			correct = answer == meanings[state.Progression.UnownLetter]
		}
		if !correct {
			return errors.New("未知图腾还在等待正确的回答")
		}
		state.Progression.UnownStage++
		if state.Progression.UnownStage == 5 {
			GrantProgressionSecret(state, "unown")
		}
	}
	return nil
}

func progressionGold(state *State, amount int64) {
	if amount > 0 && state.Progression.CustomRoute != nil {
		return
	}
	state.Gold += amount
	if amount > 0 {
		state.Progression.TotalGold += amount
		for actor := 1; actor < len(state.Trainers); actor++ {
			state.Trainers[actor].Gold += amount
			state.Trainers[actor].Progression.TotalGold += amount
		}
	}
}

func progressionOwned(state *State, id int) (*Pokemon, bool, int) {
	for index := range state.Team {
		if state.Team[index].ID == id {
			return &state.Team[index], true, index
		}
	}
	for index := range state.Box {
		if state.Box[index].ID == id {
			return &state.Box[index], false, index
		}
	}
	return nil, false, -1
}

func ProgressionSpecies(state *State, pokemon *Pokemon) Species {
	base := CatalogData().Species[pokemon.Species]
	if base.ID != 70 || state.Progression.DNA[pokemon.ID] == "" {
		return base
	}
	copied, exists := CatalogData().Species[state.Progression.DNA[pokemon.ID]]
	if !exists {
		return base
	}
	copied.Key = base.Key
	copied.ID = base.ID
	copied.Name = base.Name
	copied.Names = base.Names
	copied.Color = base.Color
	copied.CostScale = base.CostScale
	copied.EvolutionKey = ""
	copied.EvolutionLevel = 0
	return copied
}

func ApplyProgressionAppearance(state *State, pokemon *Pokemon) {
	if key := state.Progression.Appearance[pokemon.ID]; key != "" && state.Progression.Forms[pokemon.ID] == "" {
		pokemon.Sprite = CatalogData().Species[key].Sprite
	}
	if pokemon.Species == "unown" {
		letter := state.Progression.UnownForms[pokemon.ID]
		if !progressionUnownLetter(letter) {
			letter = state.Progression.UnownLetter
		}
		if !progressionUnownLetter(letter) {
			letter = "A"
		}
		form := strings.ToLower(letter)
		if letter == "!" {
			form = "exclamation"
		} else if letter == "?" {
			form = "question"
		}
		pokemon.Sprite.Base = strings.Replace(pokemon.Sprite.Base, "/unown-a", "/unown-"+form, 1)
		pokemon.Sprite.Image = strings.Replace(pokemon.Sprite.Image, "/unown-a", "/unown-"+form, 1)
	}
	if pokemon.Shiny && !state.Progression.HiddenShiny[pokemon.ID] && (pokemon.Species != "ditto" || state.Progression.DNA[pokemon.ID] == "" || state.Progression.DNA[pokemon.ID] == "ditto") {
		pokemon.Sprite.Base = strings.ReplaceAll(pokemon.Sprite.Base, "/normal/", "/shiny/")
		pokemon.Sprite.Image = strings.ReplaceAll(pokemon.Sprite.Image, "/normal/", "/shiny/")
	}
	if state.Progression.CustomSkins[pokemon.ID] {
		if !strings.Contains(pokemon.Sprite.Base, "/skin/") {
			pokemon.Sprite.Base = strings.ReplaceAll(strings.ReplaceAll(pokemon.Sprite.Base, "/normal/", "/skin/normal/"), "/shiny/", "/skin/shiny/")
		}
		if !strings.Contains(pokemon.Sprite.Image, "/skin/") {
			pokemon.Sprite.Image = strings.ReplaceAll(strings.ReplaceAll(pokemon.Sprite.Image, "/normal/", "/skin/normal/"), "/shiny/", "/skin/shiny/")
		}
	}
}

func OnProgressionPokemonChanged(state *State, id int, oldKey string) {
	pokemon, _, _ := progressionOwned(state, id)
	if pokemon == nil {
		return
	}
	state.Progression.Profile.HighestPokemonLevel = max(state.Progression.Profile.HighestPokemonLevel, pokemon.Level)
	if oldKey == "nincada" && pokemon.Species == "ninjask" && !state.Progression.Reset[id] && !progressionHasSpecies(state, "shedinja") && state.Progression.CustomRoute == nil {
		if progressionGrantPokemon(state, "shedinja", false) == nil {
			state.Progression.Secrets["shedinja"] = true
		}
	}
	pokemon, _, _ = progressionOwned(state, id)
	if pokemon.Item != "" && !progressionCanEquip(state, pokemon, CatalogData().Items[pokemon.Item]) {
		progressionClearItem(state, pokemon)
		_ = refreshPokemon(state, pokemon)
	}
	ApplyProgressionAppearance(state, pokemon)
}

func progressionFormOptions(pokemon *Pokemon) []string {
	if pokemon == nil {
		return nil
	}
	switch CatalogData().Species[pokemon.Species].ID {
	case 76:
		if pokemon.Level >= 25 {
			return []string{"lycanrocDay", "lycanrocNight"}
		}
	case 80:
		if pokemon.Species == "aegislash" || pokemon.Species == "aegislashSword" {
			return []string{"aegislash", "aegislashSword"}
		}
	case 109:
		if pokemon.Level >= 40 {
			return []string{"armarouge", "ceruledge"}
		}
	case 163:
		if pokemon.Species == "frillishF" || pokemon.Species == "frillishM" {
			return []string{"frillishF", "frillishM"}
		}
		if pokemon.Species == "jellicentF" || pokemon.Species == "jellicentM" {
			return []string{"jellicentF", "jellicentM"}
		}
	case 187:
		if pokemon.Level >= 60 {
			return []string{"flapple", "appletun"}
		}
	case 55:
		if pokemon.Level >= 30 {
			return []string{"gardevoir", "gallade"}
		}
	case 240:
		if pokemon.Level >= 42 {
			return []string{"glalie", "froslass"}
		}
	case 199:
		if pokemon.Level >= 50 {
			return []string{"scyther", "scizor"}
		}
	}
	return nil
}

func progressionChangeForm(state *State, id int, key string) error {
	pokemon, _, _ := progressionOwned(state, id)
	if pokemon == nil || state.Paused || pokemon.TileID >= 0 || state.Progression.Forms[id] != "" || !slices.Contains(progressionFormOptions(pokemon), key) {
		return errors.New("请收回伙伴，并选择已解锁的战斗形态")
	}
	pokemon.Species = key
	delete(state.Progression.Appearance, id)
	if pokemon.Item != "" && !progressionCanEquip(state, pokemon, CatalogData().Items[pokemon.Item]) {
		progressionClearItem(state, pokemon)
	}
	return refreshPokemon(state, pokemon)
}

func progressionAppearanceAvailable(state *State, pokemon *Pokemon, key string) bool {
	if pokemon == nil || state.Progression.Forms[pokemon.ID] != "" || !progressionOwnsItem(state, "sokudosPortfolio") || pokemon.Level != 100 && !state.Progression.Reset[pokemon.ID] {
		return false
	}
	base := CatalogData().Species[pokemon.Species]
	choice, exists := CatalogData().Species[key]
	if !exists || slices.Contains([]int{70, 75, 76, 80, 163, 187, 199, 55, 240}, base.ID) || base.ID != choice.ID || len(choice.Facts["base"]) > 0 {
		return false
	}
	return true
}

func progressionAppearance(state *State, id int, key string) error {
	pokemon, _, _ := progressionOwned(state, id)
	if state.Paused || !progressionAppearanceAvailable(state, pokemon, key) {
		return errors.New("外观需要作品集功能与满级或已重置的合适伙伴")
	}
	state.Progression.Appearance[id] = key
	return refreshPokemon(state, pokemon)
}

func progressionShiny(state *State, id int, visible bool) error {
	pokemon, _, _ := progressionOwned(state, id)
	if pokemon == nil || !pokemon.Shiny || state.Paused {
		return errors.New("这只伙伴尚未获得异色")
	}
	state.Progression.HiddenShiny[id] = !visible
	return refreshPokemon(state, pokemon)
}

func progressionCustomSkinAvailable(state *State, pokemon *Pokemon) bool {
	if pokemon == nil {
		return false
	}
	switch CatalogData().Species[pokemon.Species].ID {
	case 38:
		return progressionOwnsItem(state, "rosesItem")
	case 103:
		return progressionOwnsItem(state, "mitsuesCocktail")
	case 249:
		return progressionOwnsItem(state, "inverter")
	}
	return false
}

func progressionCustomSkin(state *State, id int, enabled bool) error {
	pokemon, _, _ := progressionOwned(state, id)
	if state.Paused || !progressionCustomSkinAvailable(state, pokemon) {
		return errors.New("尚未解锁这只伙伴的自定义皮肤")
	}
	state.Progression.CustomSkins[id] = enabled
	return refreshPokemon(state, pokemon)
}

func progressionClearItem(state *State, pokemon *Pokemon) {
	for index := range state.Inventory {
		if state.Inventory[index].ID == pokemon.Item && state.Inventory[index].EquippedBy == pokemon.ID {
			state.Inventory[index].EquippedBy = 0
		}
	}
	pokemon.Item = ""
	if base := state.Progression.Forms[pokemon.ID]; base != "" {
		pokemon.Species = base
		delete(state.Progression.Forms, pokemon.ID)
	}
}

func progressionResetPokemon(state *State, id int) error {
	pokemon, _, _ := progressionOwned(state, id)
	if !progressionCanReset(state, pokemon) {
		return errors.New("需要已购买的荆棘鸡尾酒与 100 级伙伴")
	}
	collectionID := CatalogData().Species[pokemon.Species].ID
	key := ""
	for _, candidate := range CatalogData().Collection {
		if CatalogData().Species[candidate].ID == collectionID {
			key = candidate
			break
		}
	}
	if key == "" {
		return errors.New("尚未找到伙伴的初始形态")
	}
	if state.Progression.Forms[id] != "" {
		progressionClearItem(state, pokemon)
	}
	pokemon.Species = key
	pokemon.Level = 1
	state.Progression.Reset[id] = true
	if pokemon.Item != "" && !progressionCanEquip(state, pokemon, CatalogData().Items[pokemon.Item]) {
		progressionClearItem(state, pokemon)
	}
	return refreshPokemon(state, pokemon)
}

func progressionCanReset(state *State, pokemon *Pokemon) bool {
	return pokemon != nil && !state.Paused && pokemon.Level == 100 && pokemon.Species != "inkay" && progressionOwnsItem(state, "mitsuesCocktail")
}

func progressionGrowthView(state *State) map[int]GrowthView {
	result := map[int]GrowthView{}
	appearances := map[int][]string{}
	for key, species := range CatalogData().Species {
		appearances[species.ID] = append(appearances[species.ID], key)
	}
	ranks := map[string]int{}
	for _, base := range CatalogData().Collection {
		for key, depth := base, 0; key != "" && depth < 6; depth++ {
			ranks[key] = depth + 1
			key = CatalogData().Species[key].EvolutionKey
		}
	}
	for id := range appearances {
		slices.SortFunc(appearances[id], func(first, second string) int {
			left, right := ranks[first], ranks[second]
			if left == 0 {
				left = 100
			}
			if right == 0 {
				right = 100
			}
			if left != right {
				return left - right
			}
			return strings.Compare(first, second)
		})
	}
	team := map[int]bool{}
	for _, pokemon := range state.Team {
		team[pokemon.ID] = true
	}
	for _, pokemon := range append(append([]Pokemon{}, state.Team...), state.Box...) {
		value := GrowthView{Forms: []string{}, Appearances: []string{}, CanReset: progressionCanReset(state, &pokemon), CanDNA: !state.Paused && team[pokemon.ID] && pokemon.Species == "ditto" && pokemon.TileID < 0, CanShiny: !state.Paused && pokemon.Shiny, CanCustomSkin: !state.Paused && progressionCustomSkinAvailable(state, &pokemon)}
		if !state.Paused && pokemon.TileID < 0 && state.Progression.Forms[pokemon.ID] == "" {
			value.Forms = append(value.Forms, progressionFormOptions(&pokemon)...)
		}
		if !state.Paused {
			for _, key := range appearances[CatalogData().Species[pokemon.Species].ID] {
				if progressionAppearanceAvailable(state, &pokemon, key) {
					value.Appearances = append(value.Appearances, key)
				}
			}
		}
		result[pokemon.ID] = value
	}
	return result
}

func progressionDNATarget(key string) string {
	var base string
	_ = json.Unmarshal(CatalogData().Species[key].Facts["base"], &base)
	if base != "" {
		return base
	}
	return key
}

func progressionDNA(state *State, id, targetID int) error {
	pokemon, inTeam, _ := progressionOwned(state, id)
	target := pokemonFor(state, targetID)
	if pokemon == nil || !inTeam || pokemon.Species != "ditto" || pokemon.TileID >= 0 || target == nil || state.Paused {
		return errors.New("请收回队伍中的百变怪，并选择同队伙伴的 DNA")
	}
	state.Progression.DNA[id] = progressionDNATarget(target.Species)
	old := pokemon.Target
	if slices.Contains([]string{"aura", "invisible", "area", "curseable", "bombardment"}, old) {
		old = "first"
	}
	species := ProgressionSpecies(state, pokemon)
	pokemon.Target = old
	if slices.Contains([]string{"area", "aura", "bombardment"}, species.AttackType) {
		pokemon.Target = species.AttackType
	} else if slices.Contains([]string{"illuminate", "illuminateBuff", "frisk", "vigilantFrisk", "moongeistBeam", "grimNeigh", "scrappy"}, species.AbilityID) {
		pokemon.Target = "invisible"
	} else if species.AbilityID == "burn" {
		pokemon.Target = "notBurned"
	} else if species.AbilityID == "spinda" {
		pokemon.Target = "random"
	} else if species.AbilityID == "curse" {
		pokemon.Target = "curseable"
	}
	resetPokemonDamage(state, pokemon)
	return refreshPokemon(state, pokemon)
}

func progressionHasSpecies(state *State, key string) bool {
	if len(state.Trainers) > 0 {
		for actor := range state.Trainers {
			if !progressionHasSpecies(trainerLens(state, actor), key) {
				return false
			}
		}
		return true
	}
	catalog := CatalogData()
	species, ok := catalog.Species[key]
	if !ok {
		return false
	}
	for _, pokemon := range append(append([]Pokemon{}, state.Team...), state.Box...) {
		if catalog.Species[pokemon.Species].ID == species.ID {
			return true
		}
	}
	return false
}

func GrantProgressionSecret(state *State, key string) bool {
	if state != nil && len(state.Trainers) > 0 {
		granted := false
		for actor := range state.Trainers {
			value := trainerLens(state, actor)
			if GrantProgressionSecret(value, key) {
				commitTrainer(state, value, actor)
				granted = true
			}
		}
		return granted
	}
	if state == nil || state.Progression.CustomRoute != nil || key == "" || state.Progression.Secrets[key] {
		return false
	}
	if _, exists := CatalogData().Species[key]; !exists {
		return false
	}
	if !progressionHasSpecies(state, key) {
		if err := progressionGrantPokemon(state, key, false); err != nil {
			return false
		}
	}
	state.Progression.Secrets[key] = true
	addEvent(state, Event{Type: "secret", Key: key})
	return true
}

func progressionGrantPokemon(state *State, key string, allowDuplicate bool) error {
	if len(state.Trainers) > 0 {
		granted := false
		for actor := range state.Trainers {
			value := trainerLens(state, actor)
			if !allowDuplicate && progressionHasSpecies(value, key) {
				continue
			}
			if err := progressionGrantPokemon(value, key, allowDuplicate); err != nil {
				return err
			}
			commitTrainer(state, value, actor)
			granted = true
		}
		if !granted {
			return errors.New("已经拥有这个收集编号的伙伴")
		}
		return nil
	}
	if !allowDuplicate && progressionHasSpecies(state, key) {
		return errors.New("已经拥有这个收集编号的伙伴")
	}
	pokemon, err := newPokemon(state, key, 1)
	if err != nil {
		return err
	}
	if key == "ditto" && len(state.Team) > 0 {
		state.Progression.DNA[pokemon.ID] = progressionDNATarget(state.Team[0].Species)
		if err := refreshPokemon(state, &pokemon); err != nil {
			return err
		}
	}
	if key == "unown" {
		letter := state.Progression.UnownLetter
		if !progressionUnownLetter(letter) {
			letter = "A"
		}
		state.Progression.UnownForms[pokemon.ID] = letter
		if err := refreshPokemon(state, &pokemon); err != nil {
			return err
		}
	}
	pool := make([]string, 0, len(state.Progression.EggPool))
	for _, candidate := range state.Progression.EggPool {
		if CatalogData().Species[candidate].ID != CatalogData().Species[key].ID {
			pool = append(pool, candidate)
		}
	}
	state.Progression.EggPool = pool
	if !state.Progression.Draft.Active && len(state.Team) < ProgressionTeamCapacity(state) && !(state.Progression.Challenge.Active && state.Progression.Challenge.SlotLimit > 0) {
		state.Team = append(state.Team, pokemon)
	} else {
		state.Box = append(state.Box, pokemon)
	}
	return nil
}

func progressionBuyEgg(state *State, actor, quote int) error {
	pool := state.Progression.EggPool
	if quote != 0 && quote != len(pool) {
		return errors.New("蛋池已经变化，请确认新报价")
	}
	price := min(int64(50000), state.Progression.EggPrice)
	if state.Paused || state.Progression.Draft.Active || len(pool) == 0 || state.Gold < price {
		return errors.New("金币不足、蛋已售罄或当前不能购买")
	}
	catalog := CatalogData()
	candidates := []int{}
	for _, priority := range [][]string{catalog.ForcedInitialPool, catalog.ForcedSecondaryPool} {
		for index, key := range pool {
			if slices.Contains(priority, key) {
				candidates = append(candidates, index)
			}
		}
		if len(candidates) > 0 {
			break
		}
	}
	if len(candidates) == 0 {
		for index := range pool {
			candidates = append(candidates, index)
		}
	}
	index := candidates[NextInt(state, len(candidates))]
	key := pool[index]
	id := state.NextID
	if err := progressionGrantPokemon(state, key, false); err != nil {
		return err
	}
	progressionGold(state, -price)

	state.Progression.EggPrice = min(int64(50000), int64(math.Ceil(float64(price)*1.12+35)))
	addEvent(state, Event{Type: "shopEgg", Key: key, Source: actor + 1, Target: id, Amount: float64(price)})
	return nil
}

func progressionPoolForSlot(index int) string {
	switch index {
	case 0, 1, 4, 5, 8, 9:
		return "normal"
	case 2, 3:
		return "luxury"
	case 6, 7:
		return "admin"
	default:
		return ""
	}
}

func progressionRestock(state *State, stock int) {
	poolName := progressionPoolForSlot(stock)
	pool := state.Progression.ShopPools[poolName]
	state.Progression.ShopStock[stock] = ""
	for len(pool) > 0 {
		key := pool[0]
		pool = pool[1:]
		if _, ok := CatalogData().Items[key]; !ok || progressionOwnsItem(state, key) || slices.Contains(state.Progression.ShopStock, key) {
			continue
		}
		state.Progression.ShopStock[stock] = key
		break
	}
	state.Progression.ShopPools[poolName] = pool
}

func progressionOwnsItem(state *State, id string) bool {
	return slices.ContainsFunc(state.Inventory, func(item OwnedItem) bool { return item.ID == id }) || state.Progression.Unlocks[id]
}

func progressionBuyItem(state *State, actor int, action Action) error {
	if state.Paused || action.Stock < 0 || action.Stock >= len(state.Progression.ShopStock) {
		return errors.New("请选择有效货架")
	}
	key := state.Progression.ShopStock[action.Stock]
	item, ok := CatalogData().Items[key]
	if !ok || key == "" || action.ItemID != "" && action.ItemID != key || state.Gold < item.Price || progressionOwnsItem(state, key) {
		return errors.New("货架已变化、金币不足或已经拥有此物品")
	}
	progressionGold(state, -item.Price)
	if item.Equipable {
		state.Inventory = append(state.Inventory, OwnedItem{ID: key})
	} else {
		state.Progression.Unlocks[key] = true
	}
	revealed := 0
	if key == "gimmighoul" && !progressionHasSpecies(state, "gholdengo") {
		revealed = state.NextID
		if err := progressionGrantPokemon(state, "gholdengo", false); err != nil {
			return err
		}
		if size := len(state.Team); state.Progression.Challenge.Active && size > 0 && state.Team[size-1].ID == revealed {
			state.Box = append(state.Box, state.Team[size-1])
			state.Team = state.Team[:size-1]
		}
		state.Progression.Secrets["gholdengo"] = true
	}
	progressionRestock(state, action.Stock)
	addEvent(state, Event{Type: "shopItem", Key: key, Source: actor + 1, Target: revealed, Amount: float64(item.Price)})
	return nil
}

func progressionTransfer(state *State, id int, mode string) error {
	pokemon, inTeam, index := progressionOwned(state, id)
	if pokemon == nil || state.Paused || state.World.Pressure || mode != "team" && mode != "box" || state.Progression.Draft.Active || state.Progression.Challenge.Active && state.Progression.Challenge.Draft {
		return errors.New("当前不能调整这只伙伴")
	}
	if mode == "team" {
		if inTeam || len(state.Team) >= ProgressionTeamCapacity(state) {
			return errors.New("队伍已满或伙伴已在队伍中")
		}
		resetPokemonDamage(state, pokemon)
		state.Team = append(state.Team, *pokemon)
		state.Box = append(state.Box[:index], state.Box[index+1:]...)
		return nil
	}
	if !inTeam {
		return errors.New("伙伴已经在仓库中")
	}
	if pokemon.TileID >= 0 {
		if err := retirePokemon(state, id); err != nil {
			return err
		}
		pokemon = &state.Team[index]
	}
	resetPokemonDamage(state, pokemon)
	state.Box = append(state.Box, *pokemon)
	state.Team = append(state.Team[:index], state.Team[index+1:]...)
	return nil
}

func progressionPlaceInTeamSlot(state *State, action Action) error {
	if action.Slot < 0 || action.Slot >= ProgressionTeamCapacity(state) {
		return errors.New("请选择已解锁的队伍位置")
	}
	index := min(action.Slot, len(state.Team))
	occupant := 0
	if index < len(state.Team) {
		occupant = state.Team[index].ID
	}
	if occupant != action.TargetID {
		return errors.New("该队伍位置已经变化，请重新选择")
	}
	if pokemon, inTeam, _ := progressionOwned(state, action.PokemonID); pokemon == nil || inTeam {
		return errors.New("请选择仓库里的伙伴")
	}
	if occupant > 0 {
		if err := progressionTransfer(state, occupant, "box"); err != nil {
			return err
		}
	}
	if err := progressionTransfer(state, action.PokemonID, "team"); err != nil {
		return err
	}
	last := len(state.Team) - 1
	added := state.Team[last]
	copy(state.Team[index+1:], state.Team[index:last])
	state.Team[index] = added
	return nil
}

func progressionTransferAll(state *State, mode string) error {
	if mode != "box" || state.Paused || state.World.Pressure || state.Progression.Draft.Active || state.Progression.Challenge.Active && state.Progression.Challenge.Draft {
		return errors.New("当前不能把整队伙伴移入仓库")
	}
	for _, pokemon := range append([]Pokemon{}, state.Team...) {
		if err := progressionTransfer(state, pokemon.ID, mode); err != nil {
			return err
		}
	}
	return nil
}

func progressionFormationKey(state *State) string {
	mode := "normal"
	if state.Progression.Challenge.Active {
		mode = "challenge"
		if state.Progression.Challenge.Draft {
			mode = "draft"
		}
	}
	return strconv.Itoa(state.Route) + "/" + mode
}

func progressionSaveFormation(state *State, slot int) error {
	if slot < 0 || slot >= 5 || state.Progression.Draft.Active {
		return errors.New("请选择当前地图的五个编队槽之一")
	}
	key := progressionFormationKey(state)
	formations := state.Progression.Formations[key]
	for len(formations) < 5 {
		formations = append(formations, Formation{})
	}
	formation := Formation{Members: []FormationMember{}}
	for _, pokemon := range state.Team {
		form := ""
		if slices.Contains(progressionFormOptions(&pokemon), pokemon.Species) {
			form = pokemon.Species
		}
		formation.Members = append(formation.Members, FormationMember{PokemonID: pokemon.ID, TileID: pokemon.TileID, Target: pokemon.Target, ItemID: pokemon.Item, Form: form})
	}
	formations[slot] = formation
	state.Progression.Formations[key] = formations
	return nil
}

func progressionLoadFormation(state *State, actor, slot int) error {
	formations := state.Progression.Formations[progressionFormationKey(state)]
	if state.Paused || state.World.MovementLocked || state.World.Pressure || state.Progression.Draft.Active || slot < 0 || slot >= 5 || slot >= len(formations) || len(formations[slot].Members) == 0 {
		return errors.New("这个编队槽没有可导入的队伍")
	}
	members := formations[slot].Members
	if len(members) > ProgressionTeamCapacity(state) {
		return errors.New("编队超过当前队伍上限")
	}
	seen := map[int]bool{}
	for _, member := range members {
		pokemon, _, _ := progressionOwned(state, member.PokemonID)
		if pokemon == nil || seen[member.PokemonID] || state.Progression.Dead[member.PokemonID] {
			return errors.New("编队成员已失去或重复")
		}
		seen[member.PokemonID] = true
	}
	for _, pokemon := range append([]Pokemon{}, state.Team...) {
		if pokemon.TileID >= 0 {
			if err := retirePokemon(state, pokemon.ID); err != nil {
				return err
			}
		}
	}
	all := append(append([]Pokemon{}, state.Team...), state.Box...)
	for index := range all {
		if seen[all[index].ID] || pokemonFor(state, all[index].ID) != nil {
			resetPokemonDamage(state, &all[index])
		}
	}
	state.Team = nil
	state.Box = nil
	for _, member := range members {
		for _, pokemon := range all {
			if pokemon.ID == member.PokemonID {
				pokemon.TileID = -1
				state.Team = append(state.Team, pokemon)
			}
		}
	}
	for _, pokemon := range all {
		if !seen[pokemon.ID] {
			pokemon.TileID = -1
			state.Box = append(state.Box, pokemon)
		}
	}
	for _, member := range members {
		pokemon, _, _ := progressionOwned(state, member.PokemonID)
		if member.Form != "" {
			if state.Progression.Forms[pokemon.ID] != "" {
				if err := progressionUnequip(state, pokemon.ID); err != nil {
					return err
				}
			}
			if err := progressionChangeForm(state, pokemon.ID, member.Form); err != nil {
				return err
			}
		}
		pokemon.Target = member.Target
		if member.ItemID != "" {
			if err := progressionEquip(state, member.PokemonID, member.ItemID); err != nil {
				return err
			}
		} else if pokemon.Item != "" {
			if err := progressionUnequip(state, member.PokemonID); err != nil {
				return err
			}
		}
		if member.TileID >= 0 {
			if err := applyCoreAction(state, actor, Action{Type: "deploy", PokemonID: member.PokemonID, TileID: member.TileID, Epoch: state.Epoch}); err != nil {
				return fmt.Errorf("编队塔位已不可用：%w", err)
			}
		}
	}
	return nil
}

func progressionChangeRoute(state *State, id int) error {
	route, ok := routeFor(id)
	if state.Paused || state.WaveActive || state.Progression.Challenge.Active || state.Progression.CustomRoute != nil || !ok || state.Stars < route.UnlockStars || !progressionSecretRouteAvailable(state, route) {
		return errors.New("路线未解锁或当前不能切换")
	}
	state.Progression.RouteHealth[state.Route] = state.Health
	state.Progression.WaveRepeat = false
	state.Progression.WaveMirror = false
	state.Progression.ActiveMirror = false
	state.Progression.CustomRoute = nil
	if err := changeRoute(state, id); err != nil {
		return err
	}
	if id == 30 {
		state.Progression.SecretMaps[30] = true
	}
	WorldOnRouteChange(state)
	OnAttackModeRouteChanged(state)
	health, exists := state.Progression.RouteHealth[id]
	if !exists {
		health = 14
	}
	state.Health = health
	return nil
}

func progressionTraitMatches(state *State, pokemon *Pokemon, key string, raw json.RawMessage) bool {
	species := ProgressionSpecies(state, pokemon)
	var stringsList []string
	var numbersList []int
	var text string
	_ = json.Unmarshal(raw, &stringsList)
	_ = json.Unmarshal(raw, &numbersList)
	if json.Unmarshal(raw, &text) == nil {
		stringsList = []string{text}
	}
	switch key {
	case "key":
		return slices.Contains(stringsList, species.Key)
	case "id":
		return slices.Contains(numbersList, species.ID)
	case "ability":
		return slices.Contains(stringsList, species.AbilityID)
	case "tile":
		for _, terrain := range numbersList {
			if slices.Contains(species.Terrains, terrain) {
				return true
			}
		}
	case "attackType":
		return slices.Contains(stringsList, species.AttackType)
	case "rangeType":
		return slices.Contains(stringsList, species.RangeType)
	case "ricochet", "orbital":
		var count float64
		_ = json.Unmarshal(species.Facts[key], &count)
		return count > 0
	case "level":
		var minimum int
		_ = json.Unmarshal(raw, &minimum)
		if state.Progression.Challenge.Active && state.Progression.Challenge.LevelCap == 50 {
			return false
		}
		return state.Progression.Challenge.Active && state.Progression.Challenge.LevelCap == 75 || pokemon.Level > minimum
	}
	return false
}

func progressionCanEquip(state *State, pokemon *Pokemon, item ItemSpec) bool {
	if pokemon != nil && item.MegaStone {
		if base := state.Progression.Forms[pokemon.ID]; base != "" {
			copy := *pokemon
			copy.Species = base
			pokemon = &copy
		}
	}
	if pokemon == nil || !item.Equipable || state.Progression.Challenge.Active && state.Progression.Challenge.NoItems {
		return false
	}
	for key, value := range item.Restriction {
		if progressionTraitMatches(state, pokemon, key, value) {
			return false
		}
	}
	if len(item.Condition) > 0 {
		found := false
		for key, value := range item.Condition {
			if progressionTraitMatches(state, pokemon, key, value) {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func progressionUnequipError(state *State, pokemon *Pokemon) error {
	if pokemon == nil || pokemon.Item == "" || state.World.Pressure {
		return errors.New("这只伙伴没有携带物品")
	}
	item := CatalogData().Items[pokemon.Item]
	species := ProgressionSpecies(state, pokemon)
	if !item.Retirable || state.Paused && species.AbilityID != "timeWarp" {
		return errors.New("当前不能卸下这件物品")
	}
	if pokemon.TileID >= 0 && slices.Contains([]string{"silphScope", "airBalloon", "heavyDutyBoots", "dampMulch", "assaultVest", "twistedSpoon", "subwoofer", "mitsuesCocktail", "blimpKeys", "pokeSkates"}, item.Key) {
		return errors.New("请先收回伙伴，再调整改变部署方式的物品")
	}
	return nil
}

func progressionUnequip(state *State, id int) error {
	pokemon, _, _ := progressionOwned(state, id)
	if err := progressionUnequipError(state, pokemon); err != nil {
		return err
	}
	for index := range state.Inventory {
		if state.Inventory[index].ID == pokemon.Item && state.Inventory[index].EquippedBy == pokemon.ID {
			state.Inventory[index].EquippedBy = 0
		}
	}
	pokemon.Item = ""
	if base := state.Progression.Forms[pokemon.ID]; base != "" {
		pokemon.Species = base
		delete(state.Progression.Forms, pokemon.ID)
	}
	return refreshPokemon(state, pokemon)
}

func progressionEquipEligible(state *State, pokemon *Pokemon, key string) bool {
	item, ok := CatalogData().Items[key]
	if !ok || pokemon == nil || state.World.Pressure || state.Paused && ProgressionSpecies(state, pokemon).AbilityID != "timeWarp" || !progressionCanEquip(state, pokemon, item) {
		return false
	}
	owned := -1
	for index, entry := range state.Inventory {
		if entry.ID == key {
			owned = index
			break
		}
	}
	if owned < 0 {
		return false
	}
	if pokemon.Item == key {
		return true
	}
	if pokemon.Item != "" && progressionUnequipError(state, pokemon) != nil {
		return false
	}
	owner := state.Inventory[owned].EquippedBy
	if owner != 0 && owner != pokemon.ID && ProgressionSpecies(state, pokemon).AbilityID != "magician" {
		other, _, _ := progressionOwned(state, owner)
		if progressionUnequipError(state, other) != nil {
			return false
		}
	}
	if item.MegaStone {
		for id := range state.Progression.Forms {
			if id != pokemon.ID {
				other, _, _ := progressionOwned(state, id)
				if progressionUnequipError(state, other) != nil {
					return false
				}
			}
		}
		key := pokemon.Species
		if base := state.Progression.Forms[pokemon.ID]; base != "" {
			key = base
		}
		var forms []string
		_ = json.Unmarshal(CatalogData().Species[key].Facts["mega"], &forms)
		if item.MegaPos < 0 || item.MegaPos >= len(forms) || forms[item.MegaPos] == "" {
			return false
		}
	}
	return true
}

func progressionEquip(state *State, id int, key string) error {
	pokemon, _, _ := progressionOwned(state, id)
	item, ok := CatalogData().Items[key]
	if !ok || !progressionEquipEligible(state, pokemon, key) {
		return errors.New("这只伙伴不满足物品的携带条件")
	}
	owned := -1
	for index := range state.Inventory {
		if state.Inventory[index].ID == key {
			owned = index
			break
		}
	}
	if owned < 0 {
		return errors.New("尚未拥有这件物品")
	}
	if pokemon.Item == key {
		return nil
	}
	magician := ProgressionSpecies(state, pokemon).AbilityID == "magician"
	if !magician && state.Inventory[owned].EquippedBy != 0 && state.Inventory[owned].EquippedBy != id {
		if err := progressionUnequip(state, state.Inventory[owned].EquippedBy); err != nil {
			return err
		}
	}
	if pokemon.Item != "" {
		if err := progressionUnequip(state, pokemon.ID); err != nil {
			return err
		}
	}
	pokemon.Item = key
	if !magician {
		state.Inventory[owned].EquippedBy = pokemon.ID
	}
	if item.MegaStone {
		ids := []int{}
		for owner := range state.Progression.Forms {
			if owner != id {
				ids = append(ids, owner)
			}
		}
		slices.Sort(ids)
		for _, owner := range ids {
			if err := progressionUnequip(state, owner); err != nil {
				return err
			}
		}
		var forms []string
		_ = json.Unmarshal(CatalogData().Species[pokemon.Species].Facts["mega"], &forms)
		if item.MegaPos < 0 || item.MegaPos >= len(forms) || forms[item.MegaPos] == "" {
			return errors.New("超级进化需要合适的超级石和对应形态")
		}
		state.Progression.Forms[pokemon.ID] = pokemon.Species
		pokemon.Species = forms[item.MegaPos]
	}
	if err := refreshPokemon(state, pokemon); err != nil {
		return err
	}
	for _, secret := range []struct{ item, species, reward string }{{"strangeIdol", "corsola", "corsolaGalar"}, {"poisonBarb", "weavile", "sneaselHisui"}, {"icyRock", "sandslash", "sandshrewAlola"}, {"ancientSword", "samurott", "samurottHisui"}} {
		if key == secret.item && pokemon.Species == secret.species && !state.Progression.Secrets[secret.reward] && state.Progression.CustomRoute == nil {
			if err := progressionGrantPokemon(state, secret.reward, false); err != nil {
				return err
			}
			state.Progression.Secrets[secret.reward] = true
		}
	}
	return nil
}

func progressionSettings(state *State, action Action) error {
	if !finiteNumber(action.Value) {
		return errors.New("设置数值无效")
	}
	settings := &state.Progression.Settings
	if handled, err := applyOriginalSetting(settings, action); handled {
		return err
	}
	if strings.HasPrefix(action.Name, "audio.") {
		if action.Value < 0 || action.Value > 20 || math.Trunc(action.Value) != action.Value {
			return errors.New("音量须为 0～20 的整数")
		}
		value := int(action.Value)
		switch action.Name {
		case "audio.master":
			settings.Audio.Master = value
		case "audio.music":
			settings.Audio.Music = value
		case "audio.ui":
			settings.Audio.UI = value
		case "audio.effects":
			settings.Audio.Effects = value
		default:
			return errors.New("未知的音频设置")
		}
		if settings.Audio == (AudioSettings{Master: 0, Music: 4, UI: 4, Effects: 1}) && !state.Progression.Challenge.Active && state.Progression.CustomRoute == nil && !state.Progression.Secrets["chatot"] {
			if err := progressionGrantPokemon(state, "chatot", false); err != nil {
				return err
			}
			state.Progression.Secrets["chatot"] = true
		}
		return nil
	}
	switch action.Name {
	case "showDamage":
		settings.ShowDamage = action.Enabled
	case "showHealth":
		settings.ShowHealth = action.Enabled
		settings.DisplayHealth = 0
		if action.Enabled {
			settings.DisplayHealth = 2
		}
	case "mapEffects":
		settings.MapEffects = action.Enabled
		settings.MapEffectsMode = 0
		if action.Enabled {
			settings.MapEffectsMode = 2
		}
	case "colorTags":
		settings.ColorTags = action.Enabled
		settings.DisableTagColors = !action.Enabled
	case "effectAlpha":
		if action.Value < 0 || action.Value > 100 || math.Trunc(action.Value) != action.Value || int(action.Value)%5 != 0 {
			return errors.New("特效透明度须为 0～100，且以 5 为步长")
		}
		settings.EffectAlpha = int(action.Value)
	case "language":
		if action.Value < 0 || action.Value > 9 || math.Trunc(action.Value) != action.Value {
			return errors.New("未知的语言设置")
		}
		settings.Language = int(action.Value)
	case "theme":
		if !slices.Contains([]string{"original", "classic", "halloween", "monochrome"}, action.Mode) {
			return errors.New("未知的界面主题")
		}
		settings.Theme = action.Mode
	case "font":
		if !slices.Contains([]string{"PressStart2P", "Lexend", "Rubik", "DynaPuff", "Unown"}, action.Mode) {
			return errors.New("未知的界面字体")
		}
		settings.Font = action.Mode
	default:
		return errors.New("未知的游戏设置")
	}
	return nil
}

var progressionSecretDefinitions = []SecretHotspot{
	{Key: "cacnea", Name: "沙漠中的伙伴", Route: 4, X: 168, Y: 97, Width: 23, Height: 20, Target: "cacnea", Kind: "pokemon"},
	{Key: "dhelmise", Name: "海湾中的伙伴", Route: 11, X: 385, Y: 205, Width: 47, Height: 34, Target: "dhelmise", Kind: "pokemon"},
	{Key: "greavard", Name: "林间的伙伴", Route: 2, X: 361, Y: 289, Width: 23, Height: 20, Target: "greavard", Kind: "pokemon"},
	{Key: "sandygast", Name: "沙滩上的伙伴", Route: 16, X: 123, Y: 555, Width: 44, Height: 42, Target: "sandygast", Kind: "pokemon"},
	{Key: "ducklett", Name: "水边的伙伴", Route: 13, X: 698, Y: 562, Width: 13, Height: 13, Target: "ducklett", Kind: "pokemon"},
	{Key: "manaphyCave", Name: "隐秘洞窟", Route: 11, X: 592, Y: 91, Width: 44, Height: 42, Target: "20", Kind: "route", NeedsQuiet: true},
	{Key: "victiniCave", Name: "隐秘洞窟", Route: 26, X: 446, Y: 1405, Width: 42, Height: 38, Target: "31", Kind: "route", NeedsQuiet: true},
	{Key: "marshadowCave", Name: "隐秘洞窟", Route: 15, X: 186, Y: 45, Width: 34, Height: 32, Target: "32", Kind: "route", NeedsQuiet: true},
	{Key: "secretCastle", Name: "城堡入口", Route: 29, X: 64, Y: 271, Width: 40, Height: 40, Target: "33", Kind: "route", NeedsQuiet: true},
	{Key: "secretChess", Name: "棋盘入口", Route: 34, X: 328, Y: 485, Width: 40, Height: 40, Target: "35", Kind: "route", NeedsQuiet: true},
	{Key: "bronzor", Name: "钟声", Route: 27, X: 171, Y: 195, Width: 65, Height: 65, Target: "bronzor", Kind: "bell"},
	{Key: "spiritomb", Name: "石塔中的伙伴", Route: 28, X: 624, Y: 528, Width: 24, Height: 24, Target: "spiritomb", Kind: "pokemon"},
	{Key: "beedrillite", Name: "发光的物品", Route: 39, X: 341, Y: 70, Width: 6, Height: 6, Target: "beedrillite", Kind: "item", NeedsQuiet: true},
	{Key: "gengarite", Name: "发光的物品", Route: 6, X: 544, Y: 58, Width: 6, Height: 7, Target: "gengarite", Kind: "item", NeedsQuiet: true},
}

func progressionHotspots(state *State) []SecretHotspot {
	result := []SecretHotspot{}
	for _, secret := range progressionSecretDefinitions {
		if secret.Route != state.Route || secret.Kind != "bell" && (state.Progression.Challenge.Active || state.Progression.CustomRoute != nil) || secret.NeedsQuiet && state.WaveActive || secret.Key == "secretCastle" && !state.Progression.Secrets["klefki"] {
			continue
		}
		if secret.Kind == "pokemon" && state.Progression.Secrets[secret.Key] || secret.Kind == "item" && state.Progression.SecretItems[secret.Key] {
			continue
		}
		result = append(result, secret)
	}
	return result
}

func progressionSecret(state *State, action Action) error {
	key := action.Name
	if action.Type == "bell" {
		key = "bronzor"
	}
	for _, secret := range progressionHotspots(state) {
		if secret.Key != key {
			continue
		}
		if state.Paused || action.Type != "bell" && (action.X < secret.X || action.X > secret.X+secret.Width || action.Y < secret.Y || action.Y > secret.Y+secret.Height) {
			return errors.New("请点选地图上实际的机关位置")
		}
		switch secret.Kind {
		case "pokemon":
			if err := progressionGrantPokemon(state, secret.Target, false); err != nil {
				return err
			}
			state.Progression.Secrets[key] = true
		case "item":
			if progressionOwnsItem(state, secret.Target) {
				return errors.New("已经发现这件物品")
			}
			state.Inventory = append(state.Inventory, OwnedItem{ID: secret.Target})
			state.Progression.SecretItems[key] = true
		case "route":
			id, _ := strconv.Atoi(secret.Target)
			state.Progression.SecretMaps[id] = true
			return progressionChangeRoute(state, id)
		case "bell":
			return progressionBell(state)
		}
		return nil
	}
	return errors.New("当前地图上没有可操作的这个机关")
}

func progressionBell(state *State) error {
	if state.Route != 27 {
		return errors.New("钟声机关不在当前路线")
	}
	if state.Wave == 50 {
		sequence := []int{1, 3, 1, 2, 1, 4, 2, 4, 1}
		bell := &state.Progression.Bell
		if bell.Clicks == 0 && bell.Group == 0 {
			bell.Clicks = 1
		} else if bell.NeedsRest {
			gap := state.Clock - bell.LastAt
			if gap < 500 || gap > 3000 {
				*bell = BellProgress{}
				return nil
			}
			bell.Group++
			bell.Clicks = 1
			bell.NeedsRest = false
		} else {
			if state.Clock-bell.LastAt > 450 {
				*bell = BellProgress{}
				return nil
			}
			bell.Clicks++
		}
		bell.LastAt = state.Clock
		if bell.Clicks == sequence[bell.Group] {
			if bell.Group == len(sequence)-1 {
				if !state.Progression.Challenge.Active && state.Progression.CustomRoute == nil && !state.Progression.Secrets["bronzor"] {
					if err := progressionGrantPokemon(state, "bronzor", false); err != nil {
						return err
					}
					state.Progression.Secrets["bronzor"] = true
				}
				*bell = BellProgress{}
			} else {
				bell.NeedsRest = true
			}
		}
		return nil
	}
	if !state.WaveActive {
		return errors.New("当前没有可影响的敌人")
	}
	if state.Wave == 100 {
		for index := range state.Enemies {
			state.Enemies[index].BaseSpeed += 0.3
		}
		return nil
	}
	maximum := 1
	if rules := state.Progression.Challenge; rules.Active && (rules.LevelCap > 0 || rules.ToughEnemies > 0 || rules.SlotLimit > 0 || rules.NoItems) {
		maximum = 2
	}
	if state.Progression.BellUses >= maximum {
		return errors.New("本波钟声已经使用完毕")
	}
	state.Progression.BellUses++
	duration := 1.0 + float64(state.Wave)*0.04
	if state.Progression.Challenge.Active {
		duration++
	}
	for index := range state.Enemies {
		WorldApplyConditions(state, &state.Enemies[index], AttackProfile{Conditions: []ConditionSpec{{Kind: "stun", Chance: 1, DurationMS: int64(duration * 1000), Magnitude: 1, Stacks: 1}}}, 0)
	}
	return nil
}

func progressionCheckAchievements(state *State) {
	if state.Progression.Achievements == nil {
		state.Progression.Achievements = map[string]bool{}
	}
}

func ValidateProgression(state *State) error {
	if state == nil || state.Progression.Version != 1 {
		return errors.New("成长存档版本无效")
	}
	progression := &state.Progression
	if err := validateTrainerProgress(state); err != nil {
		return err
	}
	if progression.ShopPools == nil || progression.Formations == nil || progression.Ribbons == nil || progression.Claims == nil || progression.Secrets == nil || progression.SecretMaps == nil || progression.SecretItems == nil || progression.Unlocks == nil || progression.Achievements == nil || progression.Forms == nil || progression.Reset == nil || progression.DNA == nil || progression.Appearance == nil || progression.HiddenShiny == nil || progression.CustomSkins == nil || progression.UnownForms == nil || progression.RouteHealth == nil || progression.Dead == nil || progression.Encounters == nil {
		return errors.New("成长存档缺少必要容器")
	}
	if progression.EggPrice < 10 || progression.EggPrice > 50000 || len(progression.EggPool) > len(CatalogData().EggPool) || len(progression.ShopStock) != 10 || progression.ExtraGold < 0 || progression.TotalGold < 0 || progression.Completed < 0 {
		return errors.New("成长经济存档无效")
	}
	if (progression.WaveRepeat || progression.WaveMirror) && !progressionWaveManagementAvailable(state) {
		return errors.New("存档波次管理状态尚未解锁")
	}
	if strings.TrimSpace(progression.ProfileName) == "" || len([]rune(progression.ProfileName)) > 11 || progression.UnownStage < 0 || progression.UnownStage > 5 || progression.UnownStage < 4 && progression.UnownLetter != "" || progression.UnownStage >= 4 && !progressionUnownLetter(progression.UnownLetter) || progression.UnownStage == 5 && !progression.Secrets["unown"] {
		return errors.New("存档训练家资料或未知图腾问答无效")
	}
	settings := progression.Settings
	if err := validateOriginalSettings(settings); err != nil {
		return err
	}
	for _, volume := range []int{settings.Audio.Master, settings.Audio.Music, settings.Audio.UI, settings.Audio.Effects} {
		if volume < 0 || volume > 20 {
			return errors.New("存档音量超出有效范围")
		}
	}
	if settings.Language < 0 || settings.Language > 9 || settings.EffectAlpha < 0 || settings.EffectAlpha > 100 || settings.EffectAlpha%5 != 0 || !slices.Contains([]string{"original", "classic", "halloween", "monochrome"}, settings.Theme) || !slices.Contains([]string{"PressStart2P", "Lexend", "Rubik", "DynaPuff", "Unown"}, settings.Font) {
		return errors.New("存档显示设置无效")
	}
	seenEggs := map[int]bool{}
	for _, key := range progression.EggPool {
		species, ok := CatalogData().Species[key]
		if !ok || !slices.Contains(CatalogData().EggPool, key) || seenEggs[species.ID] {
			return errors.New("存档蛋池包含未知或重复的伙伴")
		}
		seenEggs[species.ID] = true
	}
	seenStock := map[string]bool{}
	for index, key := range progression.ShopStock {
		if key == "" {
			continue
		}
		if _, ok := CatalogData().Items[key]; !ok || seenStock[key] || !slices.Contains(CatalogData().ItemPools[progressionPoolForSlot(index)], key) || progressionOwnsItem(state, key) {
			return errors.New("存档货架包含未知、重复或已拥有的物品")
		}
		seenStock[key] = true
	}
	owned := map[int]bool{}
	for _, pokemon := range append(append([]Pokemon{}, state.Team...), state.Box...) {
		owned[pokemon.ID] = true
	}
	inventory := map[string]bool{}
	for _, item := range state.Inventory {
		specification, ok := CatalogData().Items[item.ID]
		if !ok || !specification.Equipable || inventory[item.ID] || item.EquippedBy < 0 || item.EquippedBy != 0 && !owned[item.EquippedBy] {
			return errors.New("存档背包或携带者无效")
		}
		inventory[item.ID] = true
	}
	for _, pokemon := range append(append([]Pokemon{}, state.Team...), state.Box...) {
		if pokemon.Item != "" && !inventory[pokemon.Item] {
			return errors.New("存档伙伴携带了尚未拥有的物品")
		}
	}
	for id, reset := range progression.Reset {
		if !owned[id] || reset && !progressionOwnsItem(state, "mitsuesCocktail") {
			return errors.New("存档重置标记无效")
		}
	}
	for id, letter := range progression.UnownForms {
		pokemon, _, _ := progressionOwned(state, id)
		if pokemon == nil || pokemon.Species != "unown" || !progressionUnownLetter(letter) {
			return errors.New("存档未知图腾形态无效")
		}
	}
	for id, key := range progression.DNA {
		pokemon, _, _ := progressionOwned(state, id)
		species, exists := CatalogData().Species[key]
		if pokemon == nil || pokemon.Species != "ditto" || !exists || len(species.Facts["base"]) > 0 {
			return errors.New("存档百变怪 DNA 无效")
		}
	}
	for id, key := range progression.Appearance {
		pokemon, _, _ := progressionOwned(state, id)
		choice, exists := CatalogData().Species[key]
		if pokemon == nil || !exists || !progressionOwnsItem(state, "sokudosPortfolio") || pokemon.Level != 100 && !progression.Reset[id] || choice.ID != CatalogData().Species[pokemon.Species].ID || len(choice.Facts["base"]) > 0 || slices.Contains([]int{70, 75, 76, 80, 163, 187, 199, 55, 240}, choice.ID) {
			return errors.New("存档伙伴外观无效")
		}
	}
	for id := range progression.HiddenShiny {
		pokemon, _, _ := progressionOwned(state, id)
		if pokemon == nil || !pokemon.Shiny {
			return errors.New("存档异色展示状态无效")
		}
	}
	for id := range progression.CustomSkins {
		pokemon, _, _ := progressionOwned(state, id)
		if !progressionCustomSkinAvailable(state, pokemon) {
			return errors.New("存档自定义皮肤状态无效")
		}
	}
	for id, key := range progression.Forms {
		pokemon, _, _ := progressionOwned(state, id)
		if pokemon == nil || key == "" || progressionDNATarget(pokemon.Species) != key || !CatalogData().Items[pokemon.Item].MegaStone {
			return errors.New("存档超级形态状态无效")
		}
	}
	challenge := progression.Challenge
	if start := progression.ChallengeStartMS; start != nil && (!challenge.Active || *start < 0 || *start > state.Clock) {
		return errors.New("存档挑战时间起点无效")
	}
	if !slices.Contains([]int{0, 50, 75}, challenge.LevelCap) || !slices.Contains([]int{0, 6, 8}, challenge.SlotLimit) || !slices.Contains([]int{0, 25, 50}, challenge.ToughEnemies) || len(state.Team) > ProgressionTeamCapacity(state) {
		return errors.New("存档挑战限制无效")
	}
	if progression.Draft.Active {
		draft := progression.Draft
		if !challenge.Active || !challenge.Draft || !slices.Contains([]int{6, 8, 10}, draft.Rounds) || draft.Round < 0 || draft.Round >= draft.Rounds || draft.Token < 1 || len(draft.Offers) < 1 || len(draft.Offers) > 3 {
			return errors.New("存档轮抽阶段无效")
		}
		seen := map[int]bool{}
		for _, id := range draft.Pool {
			if !owned[id] || seen[id] {
				return errors.New("存档轮抽池包含未知或重复成员")
			}
			seen[id] = true
		}
		seen = map[int]bool{}
		for _, id := range draft.Offers {
			if !owned[id] || !slices.Contains(draft.Pool, id) || seen[id] {
				return errors.New("存档轮抽候选已失效")
			}
			seen[id] = true
		}
	}
	for routeID, traits := range progression.Ribbons {
		if _, ok := routeFor(routeID); !ok {
			return errors.New("存档奖带路线无效")
		}
		for trait, level := range traits {
			maximum := 1
			if slices.Contains([]string{"levelCap", "slotLimit", "toughEnemies"}, trait) {
				maximum = 2
			} else if !slices.Contains([]string{"draft", "noItems", "permadeath", "mirror"}, trait) {
				return errors.New("存档奖带类型无效")
			}
			if level < 0 || level > maximum {
				return errors.New("存档奖带档位无效")
			}
		}
	}
	return nil
}

func OnProgressionWaveStarted(state *State) {
	state.Progression.ActiveMirror = state.Progression.Challenge.Active && state.Progression.Challenge.Mirror || state.Progression.WaveMirror
	state.Progression.Encounters = map[int]EncounterProgress{}
	state.Progression.BellUses = 0
	for _, enemy := range state.Enemies {
		if enemy.Boss {
			state.Progression.Encounters[enemy.ID] = EncounterProgress{Observed: enemy.PassiveFlags["spawned"]}
		}
	}
}

func progressionObserveConditions(metrics *EncounterProgress, enemy *Enemy) {
	stun := HasCondition(&enemy.EnemyEffectState, "stun")
	slow := HasCondition(&enemy.EnemyEffectState, "slow")
	if stun && !metrics.StunActive {
		metrics.Stuns++
	}
	if slow && !metrics.SlowActive {
		metrics.Slows++
	}
	metrics.StunActive, metrics.SlowActive = stun, slow
	if HasCondition(&enemy.EnemyEffectState, "poison") {
		metrics.Poisoned = true
	}
	for _, kind := range []string{"burn", "poison", "nightmare", "scorch"} {
		if HasCondition(&enemy.EnemyEffectState, kind) {
			metrics.IndirectApplied = true
		}
	}
	if HasCondition(&enemy.EnemyEffectState, "burn") && enemy.AgeMS < 6000 {
		metrics.BurnedEarly = true
	}
	metrics.InvisibilityBreaks = max(metrics.InvisibilityBreaks, enemy.PassiveCounters["invisibilityBreaks"])
	metrics.RegenReduced = metrics.RegenReduced || enemy.PassiveFlags["regenerationReduced"]
}

func OnProgressionWorldStep(state *State) {
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if !enemy.Boss {
			continue
		}
		metrics := state.Progression.Encounters[enemy.ID]
		metrics.Observed = metrics.Observed || enemy.PassiveFlags["spawned"]
		progressionObserveConditions(&metrics, enemy)
		state.Progression.Encounters[enemy.ID] = metrics
	}
}

func RecordProgressionHit(state *State, enemy *Enemy, projectile *Projectile, profile AttackProfile, attempted, lost float64, critical bool, before []Condition) {
	if state == nil || enemy == nil || projectile == nil || !finiteNumber(attempted) || !finiteNumber(lost) {
		return
	}
	state.Progression.Profile.HighestHit = max(state.Progression.Profile.HighestHit, enemy.LastResolvedDamage)
	if profile.ShinyChance > 0 && state.Progression.CustomRoute == nil && NextRandom(state) < profile.ShinyChance {
		candidates := []int{}
		for _, pokemon := range append(append([]Pokemon{}, state.Team...), state.Box...) {
			if !pokemon.Shiny && CatalogData().Species[pokemon.Species].EvolutionKey == "" {
				candidates = append(candidates, pokemon.ID)
			}
		}
		if len(candidates) > 0 {
			pokemon, _, _ := progressionOwned(state, candidates[NextInt(state, len(candidates))])
			pokemon.Shiny = true
			_ = refreshPokemon(state, pokemon)
			addEvent(state, Event{Type: "shiny", Source: pokemon.ID, Key: pokemon.Species})
		}
	}
	if !enemy.Boss || state.Progression.CustomRoute != nil {
		return
	}
	metrics := state.Progression.Encounters[enemy.ID]
	metrics.Observed = metrics.Observed || enemy.PassiveFlags["spawned"]
	metrics.Hits++
	metrics.MaxDamage = max(metrics.MaxDamage, min(attempted, 1000))
	metrics.MaxDirectAttempt = max(metrics.MaxDirectAttempt, attempted)
	if critical {
		metrics.Criticals++
	}
	pokemon, _, _ := progressionOwned(state, projectile.PokemonID)
	if pokemon != nil {
		species := ProgressionSpecies(state, pokemon)
		if pokemon.Item != "" && species.ID != 142 {
			metrics.ItemAttacks++
		}
		if pokemon.Stats.Power > 100 {
			metrics.PowerOver100 = true
		}
		if len(species.Terrains) != 1 || species.Terrains[0] != 3 {
			metrics.NonWaterAttacker = true
		}
	}
	metrics.FinalTerrain = projectile.Context.Terrain
	if projectile.Context.Terrain == 2 {
		metrics.GrassAttacks++
	}
	if projectile.Context.Terrain == 3 {
		metrics.WaterDamage += max(0, lost)
	}
	for _, tower := range state.Towers {
		if tower.ID == projectile.TowerID && vectorLength(tower.X-enemy.X, tower.Y-enemy.Y) < 120 {
			metrics.RingDamage += max(0, lost)
			break
		}
	}
	if enemy.PassiveID == "nebulogenesis" && (enemy.ReceivedHits%11 == 9 || enemy.ReceivedHits%11 == 10 || enemy.ReceivedHits%11 == 0) {
		metrics.Dodges++
	}
	progressionObserveConditions(&metrics, enemy)
	state.Progression.Encounters[enemy.ID] = metrics
}

func RecordProgressionIndirectDamage(state *State, enemy *Enemy, lost float64) {
	if state == nil || enemy == nil || !enemy.Boss || lost <= 0 || !finiteNumber(lost) || state.Progression.CustomRoute != nil {
		return
	}
	metrics := state.Progression.Encounters[enemy.ID]
	metrics.Observed = metrics.Observed || enemy.PassiveFlags["spawned"]
	metrics.IndirectDamage += lost
	progressionObserveConditions(&metrics, enemy)
	state.Progression.Encounters[enemy.ID] = metrics
}

func progressionAchievementFor(state *State, enemy Enemy, metrics EncounterProgress) (string, bool) {
	if !metrics.Observed || !enemy.PassiveFlags["spawned"] {
		return "", false
	}
	switch enemy.Key {
	case "shaymin":
		return "0", metrics.ItemAttacks == 0
	case "celebi":
		return "1", enemy.AgeMS <= 7500
	case "lunala":
		return "2", max(metrics.InvisibilityBreaks, enemy.PassiveCounters["invisibilityBreaks"]) <= 5
	case "zapdos":
		return "3", enemy.PassiveCounters["paralyses"] <= 3
	case "moltres":
		return "4", metrics.RingDamage >= enemy.MaxHP*0.88
	case "regirock":
		return "5", !metrics.IndirectApplied && metrics.IndirectDamage == 0
	case "groudon":
		return "6", metrics.GrassAttacks == 0
	case "hooh":
		return "7", enemy.PassiveFlags["revivalInterrupted"] && !enemy.Revived
	case "registeel":
		return "8", metrics.MaxDamage <= 500
	case "regice":
		return "9", enemy.CriticalBlocks >= 50
	case "regigigas":
		return "10", state.Progression.Challenge.Active && state.Progression.Challenge.ToughEnemies == 50
	case "articuno":
		return "11", state.Weather == "hail"
	case "kyogre":
		return "12", metrics.FinalTerrain == 1
	case "thundurus":
		return "13", HasCondition(&enemy.EnemyEffectState, "stun")
	case "tapuFini":
		return "14", metrics.Dodges <= 10
	case "nihilego":
		return "15", !metrics.NonWaterAttacker
	case "tapuKoko":
		return "16", metrics.Stuns <= 3
	case "tapuBulu":
		return "17", !metrics.RegenReduced && !enemy.PassiveFlags["regenerationReduced"]
	case "tapuLele":
		return "18", !metrics.PowerOver100
	case "keldeo":
		return "19", !enemy.PassiveFlags["justifiedTriggered"]
	case "heatran":
		return "20", metrics.BurnedEarly
	case "volcanion":
		return "21", metrics.WaterDamage >= enemy.MaxHP*0.74
	case "pheromosa":
		return "22", !metrics.Poisoned
	case "cobalion":
		return "23", !enemy.PassiveFlags["justifiedTriggered"]
	case "tornadus":
		return "24", metrics.Stuns == 0 && metrics.Slows == 0
	case "stakataka":
		return "25", enemy.ReceivedHits <= 26
	case "terrakion":
		return "26", !enemy.PassiveFlags["justifiedTriggered"]
	case "magearna":
		return "27", enemy.AgeMS > 59500
	case "diancie":
		return "28", metrics.Criticals == 0
	case "zamazenta":
		return "29", enemy.ReceivedHits < 205
	case "zacian":
		return "30", enemy.ReceivedHits < 5
	case "virizion":
		return "32", !enemy.PassiveFlags["justifiedTriggered"]
	case "landorus":
		return "38", metrics.MaxDirectAttempt <= 1000
	}
	return "", false
}

func RecordProgressionEnemyDefeat(state *State, enemy Enemy) {
	if state == nil || !enemy.Boss || enemy.HP > 0 || state.Progression.CustomRoute != nil {
		return
	}
	metrics := state.Progression.Encounters[enemy.ID]
	id, unlocked := progressionAchievementFor(state, enemy, metrics)
	if !unlocked {
		return
	}
	if !state.Progression.Achievements[id] {
		state.Progression.Achievements[id] = true
		state.Progression.ExtraGold += 2
		addEvent(state, Event{Type: "achievement", Key: enemy.Key})
	}
	if !state.Progression.Secrets[enemy.Key] {
		if !progressionHasSpecies(state, enemy.Key) {
			if err := progressionGrantPokemon(state, enemy.Key, false); err != nil {
				return
			}
		}
		state.Progression.Secrets[enemy.Key] = true
	}
}

func ProgressionEnemyEscapePower(state *State, enemy Enemy) int {
	if state != nil && enemy.Key == "klefki" && state.Route == 29 && !state.Progression.Challenge.Active && state.Progression.CustomRoute == nil && !state.Progression.Secrets["klefki"] {
		state.Progression.Secrets["klefki"] = true
		addEvent(state, Event{Type: "unlockSecret", Key: "klefki"})
		return 0
	}
	return enemy.Power
}
