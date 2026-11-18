package engine

import (
	"errors"
	"math"
	"slices"
)

type AttackModeState struct {
	Towers  map[int]*AttackModeTower `json:"towers,omitempty"`
	Effects []AttackModeEffect       `json:"effects,omitempty"`
}

type AttackModeTower struct {
	TowerID        int          `json:"towerID"`
	Aim            Point        `json:"aim"`
	HasAim         bool         `json:"hasAim"`
	AreaRadius     float64      `json:"areaRadius"`
	AreaWidth      float64      `json:"areaWidth"`
	AreaHeight     float64      `json:"areaHeight"`
	Chain          *AttackChain `json:"chain,omitempty"`
	SweepCount     int          `json:"sweepCount"`
	SwordDirection int          `json:"swordDirection,omitempty"`
}

type AttackChain struct {
	Phase       string `json:"phase"`
	AgeMS       int64  `json:"ageMS"`
	DurationMS  int64  `json:"durationMS"`
	Origin      Point  `json:"origin"`
	Destination Point  `json:"destination"`
	Home        Point  `json:"home"`
	HitIDs      []int  `json:"hitIDs"`
	Bonus       int    `json:"bonus"`
	CreatedTick int64  `json:"createdTick"`
}

type AttackModeEffect struct {
	ID              int            `json:"id"`
	Kind            string         `json:"kind"`
	PokemonID       int            `json:"pokemonID"`
	TowerID         int            `json:"towerID"`
	Origin          Point          `json:"origin"`
	Destination     Point          `json:"destination"`
	Position        Point          `json:"position"`
	AgeMS           int64          `json:"ageMS"`
	DelayMS         int64          `json:"delayMS"`
	DurationMS      int64          `json:"durationMS"`
	ImpactMS        int64          `json:"impactMS"`
	Radius          float64        `json:"radius"`
	MaxRadius       float64        `json:"maxRadius"`
	Width           float64        `json:"width"`
	Angle           float64        `json:"angle"`
	Speed           float64        `json:"speed"`
	Travelled       float64        `json:"travelled"`
	MaxDistance     float64        `json:"maxDistance"`
	Impacted        bool           `json:"impacted"`
	HitIDs          []int          `json:"hitIDs"`
	Profile         AttackProfile  `json:"profile"`
	SourceProfile   AttackProfile  `json:"sourceProfile"`
	Context         AbilityContext `json:"context"`
	Sprite          SpriteSpec     `json:"sprite"`
	BoltPoints      []Point        `json:"boltPoints,omitempty"`
	ModeBonusPct    float64        `json:"modeBonusPct,omitempty"`
	Shape           string         `json:"shape,omitempty"`
	HalfAngle       float64        `json:"halfAngle"`
	FollowSource    bool           `json:"followSource"`
	Visibility      string         `json:"visibility,omitempty"`
	CreatedTick     int64          `json:"createdTick"`
	CreatedClock    int64          `json:"createdClock"`
	WallClock       bool           `json:"wallClock"`
	HitDurationMS   int64          `json:"hitDurationMS"`
	FinalMultiplier float64        `json:"finalMultiplier"`
	SharedCritical  *bool          `json:"sharedCritical,omitempty"`
	Direction       int            `json:"direction,omitempty"`
}

func attackModeTower(state *State, tower *Tower) *AttackModeTower {
	if state.AttackModes.Towers == nil {
		state.AttackModes.Towers = map[int]*AttackModeTower{}
	}
	value := state.AttackModes.Towers[tower.PokemonID]
	if value == nil || value.TowerID != tower.ID {
		value = &AttackModeTower{TowerID: tower.ID}
		state.AttackModes.Towers[tower.PokemonID] = value
	}
	return value
}

func attackModeBombardment(profile AttackProfile) bool {
	return profile.AttackMode == "bombardment" || profile.AttackMode == "lightningArea"
}

func attackModeArea(state *State, tower *Tower, pokemon *Pokemon) *AttackModeTower {
	value := attackModeTower(state, tower)
	species := WorldSpecies(state, pokemon)
	area := factNumber(species.Facts, "bombardmentArea")
	value.AreaRadius = area / 2
	value.AreaWidth, value.AreaHeight = 0, 0
	if species.AbilityID == "rockThrow" {
		value.AreaWidth, value.AreaHeight = area*5, area
	}
	return value
}

func OnAttackModeDeploy(state *State, tower *Tower, pokemon *Pokemon) error {
	OnAttackModeRetire(state, pokemon.ID)
	species := WorldSpecies(state, pokemon)
	if species.AttackType != "bombardment" {
		return nil
	}
	value := attackModeArea(state, tower, pokemon)
	if value.HasAim {
		return nil
	}
	profile, err := profileFor(state, tower, pokemon, nil, 0.5)
	if err != nil {
		return err
	}
	if species.RangeType == "cross" {
		distance := (float64(NextRandom(state)*2) - 1) * profile.Range
		value.Aim = Point{X: tower.X, Y: tower.Y + distance}
		if NextRandom(state) < 0.5 {
			value.Aim = Point{X: tower.X + distance, Y: tower.Y}
		}
	} else {
		angle := NextRandom(state) * math.Pi * 2
		distance := NextRandom(state) * profile.Range
		value.Aim = Point{X: tower.X + float64(math.Cos(angle)*distance), Y: tower.Y + float64(math.Sin(angle)*distance)}
	}
	value.HasAim = true
	return nil
}

func OnAttackModeRetire(state *State, pokemonID int) {
	delete(state.AttackModes.Towers, pokemonID)
	state.AttackModes.Effects = slices.DeleteFunc(state.AttackModes.Effects, func(effect AttackModeEffect) bool { return effect.PokemonID == pokemonID })
}

func OnAttackModeWaveStarted(state *State) {
	state.AttackModes.Effects = nil
}

func OnAttackModeWaveEnded(state *State) {
	state.AttackModes.Effects = nil
}

func OnAttackModeRouteChanged(state *State) {
	state.AttackModes = AttackModeState{}
}

func ApplyAttackModeAim(state *State, action Action) error {
	pokemon := pokemonFor(state, action.PokemonID)
	tower := worldTowerFor(state, action.PokemonID)
	if pokemon != nil && tower != nil && pokemon.Item == "blimpKeys" && WorldSpecies(state, pokemon).AbilityID == "unburden" {
		return applyFlyingAim(state, tower, action)
	}
	if pokemon == nil || tower == nil || state.Paused || state.Health <= 0 || !finiteNumber(action.X) || !finiteNumber(action.Y) || math.Abs(action.X) > 1000000 || math.Abs(action.Y) > 1000000 || WorldSpecies(state, pokemon).AttackType != "bombardment" {
		return errors.New("请选择已部署的炮击伙伴与有效目标区")
	}
	profile, err := profileFor(state, tower, pokemon, nil, 0.5)
	if err != nil {
		return err
	}
	value := attackModeArea(state, tower, pokemon)
	dx, dy := action.X-tower.X, action.Y-tower.Y
	point := Point{X: action.X, Y: action.Y}
	species := WorldSpecies(state, pokemon)
	if species.RangeType == "cross" {
		axes := []Point{{X: 1}, {Y: 1}}
		if pokemon.Item == "starPiece" {
			axes = append(axes, Point{X: math.Sqrt(0.5), Y: math.Sqrt(0.5)}, Point{X: math.Sqrt(0.5), Y: -math.Sqrt(0.5)})
		}
		axis, projection := axes[0], -1.0
		for _, candidate := range axes {
			current := math.Abs(float64(dx*candidate.X) + float64(dy*candidate.Y))
			if current > projection {
				axis, projection = candidate, current
			}
		}
		along := float64(dx*axis.X) + float64(dy*axis.Y)
		perpendicular := Point{X: dx - float64(axis.X*along), Y: dy - float64(axis.Y*along)}
		limit := value.AreaRadius
		if pokemon.Item == "wideLens" {
			limit *= 2
		}
		if worldHasAbility(state, "victoryStar") {
			limit *= 1.5
		}
		scale := 1.0
		if length := vectorLength(perpendicular.X, perpendicular.Y); length > limit {
			scale = limit / length
		}
		along = math.Max(-profile.Range, math.Min(profile.Range, along))
		point = Point{X: tower.X + float64(axis.X*along) + float64(perpendicular.X*scale), Y: tower.Y + float64(axis.Y*along) + float64(perpendicular.Y*scale)}
	} else if distance := vectorLength(dx, dy); distance > profile.Range {
		point = Point{X: tower.X + float64(dx*profile.Range/distance), Y: tower.Y + float64(dy*profile.Range/distance)}
	} else if species.RangeType == "donut" && distance > 0 && distance < species.InnerRange {
		point = Point{X: tower.X + float64(dx*species.InnerRange/distance), Y: tower.Y + float64(dy*species.InnerRange/distance)}
	}
	value.Aim, value.HasAim = point, true
	state.AttackModes.Effects = slices.DeleteFunc(state.AttackModes.Effects, func(effect AttackModeEffect) bool {
		return effect.PokemonID == pokemon.ID && (effect.Kind == "bombardment" || effect.Kind == "lightningArea")
	})
	addEvent(state, Event{Type: "aim", Source: pokemon.ID, X: point.X, Y: point.Y})
	return nil
}

func HandleAttackMode(state *State, tower *Tower, pokemon *Pokemon, profile AttackProfile, context AbilityContext, targets []*Enemy) (bool, error) {
	if profile.AttackMode == "chainCharge" {
		return handleAttackModeChain(state, tower, pokemon, profile, context, targets)
	}
	if profile.AttackMode == "wave" || profile.AttackMode == "sweep" || profile.AttackMode == "dune" {
		return handleAttackModeGeometry(state, tower, pokemon, profile, context, targets)
	}
	if !attackModeBombardment(profile) {
		return false, nil
	}
	if value := state.AttackModes.Towers[pokemon.ID]; value == nil || !value.HasAim || value.TowerID != tower.ID {
		if err := OnAttackModeDeploy(state, tower, pokemon); err != nil {
			return true, err
		}
	}
	if !state.WaveActive || tower.CooldownMS > 0 {
		return true, nil
	}
	tower.CooldownMS = profile.AttackMS
	if profile.Disabled || profile.AttackMS <= 0 {
		return true, nil
	}
	value := attackModeArea(state, tower, pokemon)
	species := WorldSpecies(state, pokemon)
	count := 1
	if species.AbilityID == "dragonDarts" {
		count = 2
	} else if species.AbilityID == "volcanicKaboom" {
		count = 3
	}
	if species.AbilityID == "wildboltStorm" {
		if state.Weather != "thunderStorm" {
			return true, nil
		}
		count = 3 + NextInt(state, 5)
	}
	if species.AbilityID == "rockThrow" {
		count = context.FossilCount
	}
	if pokemon.Item == "zoomLens" {
		count++
	}
	if pokemon.Item == "choiceScarf" {
		count = 1
	}
	source, err := sourceProfileFor(state, tower, pokemon, nil, context.Random)
	if err != nil {
		return true, err
	}
	for shot := 0; shot < count; shot++ {
		point := value.Aim
		if species.AbilityID == "rockThrow" {
			angle := math.Atan2(value.Aim.Y-tower.Y, value.Aim.X-tower.X) + math.Pi/2
			x, y := (NextRandom(state)-0.5)*value.AreaWidth, (NextRandom(state)-0.5)*value.AreaHeight
			point.X += float64(x*math.Cos(angle)) - float64(y*math.Sin(angle))
			point.Y += float64(x*math.Sin(angle)) + float64(y*math.Cos(angle))
		} else {
			angle := NextRandom(state) * math.Pi * 2
			radius := value.AreaRadius * math.Sqrt(NextRandom(state))
			point.X += float64(math.Cos(angle) * radius)
			point.Y += float64(math.Sin(angle) * radius)
		}
		effect := AttackModeEffect{ID: allocateID(state), Kind: "bombardment", PokemonID: pokemon.ID, TowerID: tower.ID, Origin: Point{X: tower.X, Y: tower.Y}, Destination: point, Position: Point{X: tower.X, Y: tower.Y}, DurationMS: 500, Radius: factNumber(species.Facts, "bombardmentSplash") / 2, Profile: profile, SourceProfile: source, Context: context, Sprite: CatalogData().Projectiles[species.ProjectileID].Sprite, HitIDs: []int{}}
		if species.AbilityID == "diamondStorm" {
			if slices.ContainsFunc(state.Towers, func(candidate Tower) bool {
				owner := pokemonFor(state, candidate.PokemonID)
				return owner != nil && owner.Species == "carbink"
			}) {
				effect.Radius *= 3
			} else {
				effect.Radius *= 1 + float64(float64(context.HealedHearts)*0.4)
			}
		}
		if species.AbilityID == "wildboltStorm" {
			effect.Kind, effect.DurationMS = "lightningArea", 150
			effect.Origin, effect.Position = Point{X: point.X, Y: point.Y - 260}, Point{X: point.X, Y: point.Y - 260}
			for segment := 0; segment <= 6; segment++ {
				x := point.X
				if segment > 0 && segment < 6 {
					x += float64((NextRandom(state) - 0.5) * 18)
				}
				effect.BoltPoints = append(effect.BoltPoints, Point{X: x, Y: point.Y - 260 + float64(float64(segment)*260/6)})
			}
		}
		state.AttackModes.Effects = append(state.AttackModes.Effects, effect)
		addEvent(state, Event{Type: "fire", Source: tower.ID, X: tower.X, Y: tower.Y, Key: species.ProjectileID})
	}
	tower.AttackCount++
	tower.IdleMS = 0
	return true, WorldAfterAttack(state, tower.ID, profile, nil)
}

func attackModeHit(state *State, effect *AttackModeEffect, enemy *Enemy, multiplier float64) error {
	final := effect.FinalMultiplier
	if effect.Context.AbilityID == "shadowClaw" && ConditionStacks(&enemy.EnemyEffectState, "nightmare") > 0 {
		final *= 2
	}
	if final <= 1 {
		final = 0
	}
	projectile := Projectile{ID: effect.ID, TowerID: effect.TowerID, PokemonID: effect.PokemonID, TargetID: enemy.ID, Kind: "attackMode", X: effect.Position.X, Y: effect.Position.Y, Profile: effect.Profile, SourceProfile: effect.SourceProfile, Context: effect.Context, DamageScale: 1, ModeBonusPct: effect.ModeBonusPct, ModeCritical: effect.SharedCritical, ModeFinalMultiplier: final, LifeMS: effect.AgeMS}
	if err := applyProjectileHit(state, &projectile, enemy, multiplier); err != nil {
		return err
	}
	if effect.Kind == "wave" && effect.Context.AbilityID == "supersonic" {
		WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{{Kind: "stun", Chance: 0.1, DurationMS: 1500, Magnitude: 1, Stacks: 1, SourcePokemonID: effect.PokemonID}}}, NextRandom(state))
	}
	return nil
}

func handleAttackModeGeometry(state *State, tower *Tower, pokemon *Pokemon, profile AttackProfile, context AbilityContext, targets []*Enemy) (bool, error) {
	if len(targets) == 0 {
		if profile.AttackMode == "dune" && profile.Disabled {
			if len(selectTargets(state, tower, pokemon, profile)) > 0 {
				tower.CooldownMS = profile.AttackMS
			}
			return true, nil
		}
		return false, nil
	}
	if profile.Disabled {
		return true, nil
	}
	if profile.AttackMode == "wave" {
		count := 1
		switch context.AbilityID {
		case "boomburst", "originPulse":
			count = 3
		case "liquidVoice", "charmedCharm", "emptyLight":
			count = 2
		}
		if pokemon.Item == "sootheBell" {
			count += 2
		}
		source, err := sourceProfileFor(state, tower, pokemon, nil, context.Random)
		if err != nil {
			return true, err
		}
		appendAttackModeWaves(state, tower, pokemon, profile, source, context, Point{X: tower.X, Y: tower.Y}, count, true)
	} else {
		count := min(max(1, profile.TargetCount), len(targets))
		for _, target := range targets[:count] {
			if !EnemyAttackAllowed(state, pokemon, target, false) {
				tower.CooldownMS = profile.AttackMS
				return true, nil
			}
		}
		for _, target := range targets[:count] {
			current := contextFor(state, tower, pokemon, target, context.Random)
			effect, err := newAttackModeGeometry(state, tower, pokemon, profile, current, target)
			if err != nil {
				return true, err
			}
			if effect.Kind == "sweep" {
				if err := attackModeHitGeometry(state, &effect); err != nil {
					return true, err
				}
			}
			state.AttackModes.Effects = append(state.AttackModes.Effects, effect)
		}
	}
	tower.AttackCount++
	tower.IdleMS = 0
	tower.CooldownMS = max(0, profile.AttackMS)
	tower.TargetID = targets[0].ID
	addEvent(state, Event{Type: "fire", Source: tower.ID, Target: targets[0].ID, X: tower.X, Y: tower.Y, Key: WorldSpecies(state, pokemon).ProjectileID})
	ids := []int{}
	for _, target := range targets {
		ids = append(ids, target.ID)
	}
	return true, WorldAfterAttack(state, tower.ID, profile, ids)
}

func appendAttackModeWaves(state *State, tower *Tower, pokemon *Pokemon, profile, source AttackProfile, context AbilityContext, origin Point, count int, follow bool) {
	for index := 0; index < count; index++ {
		effect := AttackModeEffect{ID: allocateID(state), Kind: "wave", PokemonID: pokemon.ID, TowerID: tower.ID, Origin: origin, Position: origin, DelayMS: int64(index) * 450, DurationMS: int64(math.Ceil(1000.0 / 1.2)), Width: 6, MaxRadius: profile.Range, Speed: profile.Range * 1.2, Profile: profile, SourceProfile: source, Context: context, FollowSource: follow, CreatedTick: state.Tick, CreatedClock: state.Clock, HitIDs: []int{}}
		state.AttackModes.Effects = append(state.AttackModes.Effects, effect)
	}
}

func OnAttackModeProjectileHit(state *State, projectile *Projectile, enemy *Enemy) error {
	if projectile == nil || enemy == nil || projectile.Context.AbilityID != "supersonic" || projectile.Kind == "attackMode" {
		return nil
	}
	tower := worldTowerFor(state, projectile.PokemonID)
	owner := pokemonFor(state, projectile.PokemonID)
	if tower == nil || owner == nil {
		return nil
	}
	source := projectile.SourceProfile
	if source.AttackMS == 0 {
		source = projectile.Profile
	}
	appendAttackModeWaves(state, tower, owner, projectile.Profile, source, projectile.Context, Point{X: enemy.X, Y: enemy.Y}, 3, false)
	return nil
}

func newAttackModeGeometry(state *State, tower *Tower, pokemon *Pokemon, profile AttackProfile, context AbilityContext, target *Enemy) (AttackModeEffect, error) {
	source, err := sourceProfileFor(state, tower, pokemon, target, context.Random)
	if err != nil {
		return AttackModeEffect{}, err
	}
	origin := Point{X: tower.X, Y: tower.Y}
	effect := AttackModeEffect{ID: allocateID(state), Kind: "sweep", PokemonID: pokemon.ID, TowerID: tower.ID, Origin: origin, Position: origin, Destination: Point{X: target.X, Y: target.Y}, Angle: math.Atan2(target.Y-tower.Y, target.X-tower.X), Radius: profile.Range, DurationMS: 400, Profile: profile, SourceProfile: source, Context: context, CreatedTick: state.Tick, CreatedClock: state.Clock, WallClock: true, FollowSource: true, HitIDs: []int{}}
	switch context.AbilityID {
	case "fireLash":
		effect.Shape, effect.HalfAngle, effect.Visibility = "cone", math.Pi/2, "all"
		effect.WallClock = false
	case "secretSword", "ceaselessEdge", "intrepidSword":
		effect.Shape, effect.Width, effect.DurationMS, effect.Visibility = "blade", 50, 300, "neverInvisible"
		value := attackModeTower(state, tower)
		if value.SwordDirection == 0 {
			value.SwordDirection = 1
		}
		effect.Direction = value.SwordDirection
		value.SwordDirection *= -1
		critical := profile.ForcedCritical || NextRandom(state)*100 < profile.CriticalPct
		effect.SharedCritical = &critical
	case "shadowClaw":
		value := attackModeTower(state, tower)
		value.SweepCount++
		effect.Shape, effect.DurationMS, effect.FollowSource = "targetCircle", 340, false
		effect.Origin, effect.Position = effect.Destination, effect.Destination
		effect.Radius, effect.FinalMultiplier = 52, 1
		if value.SweepCount%5 == 0 || ConditionStacks(&target.EnemyEffectState, "nightmare") >= 10 {
			effect.Radius, effect.FinalMultiplier = 78, 2
		}
	case "icyWind", "dazzlingGleam":
		effect.Kind, effect.Shape = "directional", "cone"
		effect.HalfAngle, effect.DurationMS, effect.HitDurationMS = math.Pi/4, 520, 500
	case "psychoCut", "psychoCutMega":
		effect.Kind, effect.Shape, effect.Visibility = "arc", "arc", "all"
		effect.Radius, effect.Width, effect.HalfAngle, effect.Speed = 45, 12, math.Pi*0.32, 300
		effect.MaxDistance = max(0, profile.Range-45)
		effect.DurationMS = max(battleFrameUpperMS, int64(math.Ceil(effect.MaxDistance/effect.Speed*1000)))
		effect.WallClock, effect.FollowSource = false, false
	case "sandsearStorm":
		route, _ := routeForState(state)
		effect.Kind, effect.Shape, effect.Width = "dune", "band", 160
		effect.Speed, effect.MaxDistance = 132, vectorLength(float64(route.Width), float64(route.Height))
		effect.DurationMS = int64(math.Ceil(effect.MaxDistance / effect.Speed * 1000))
		effect.Radius, effect.WallClock, effect.FollowSource = 0, false, false
	default:
		return AttackModeEffect{}, errors.New("尚未实现该方向攻击的原版几何")
	}
	return effect, nil
}

func attackModeVisible(effect *AttackModeEffect, enemy *Enemy) bool {
	if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || enemy.InvulnerableMS > 0 {
		return false
	}
	if !enemy.Invisible || effect.Visibility == "all" {
		return true
	}
	return effect.Visibility != "neverInvisible" && effect.Profile.CanSeeInvisible
}

func attackModeContains(effect *AttackModeEffect, enemy *Enemy) bool {
	dx, dy := enemy.X-effect.Origin.X, enemy.Y-effect.Origin.Y
	distance := vectorLength(dx, dy)
	angle := math.Abs(math.Atan2(math.Sin(math.Atan2(dy, dx)-effect.Angle), math.Cos(math.Atan2(dy, dx)-effect.Angle)))
	switch effect.Kind {
	case "wave":
		return distance <= effect.Radius && distance >= effect.Radius-effect.Width
	case "arc":
		dx, dy = enemy.X-effect.Position.X, enemy.Y-effect.Position.Y
		angle = math.Abs(math.Atan2(math.Sin(math.Atan2(dy, dx)-effect.Angle), math.Cos(math.Atan2(dy, dx)-effect.Angle)))
		return math.Abs(vectorLength(dx, dy)-effect.Radius) <= effect.Width && angle <= effect.HalfAngle
	case "dune":
		along := float64(dx*math.Cos(effect.Angle)) + float64(dy*math.Sin(effect.Angle))
		perpendicular := math.Abs(float64(-dx*math.Sin(effect.Angle)) + float64(dy*math.Cos(effect.Angle)))
		return perpendicular <= effect.Width/2 && along <= effect.Travelled && along >= -effect.Width/2
	case "directional":
		return distance <= effect.Radius+10 && angle <= effect.HalfAngle
	}
	switch effect.Shape {
	case "blade":
		along := float64(dx*math.Cos(effect.Angle)) + float64(dy*math.Sin(effect.Angle))
		perpendicular := math.Abs(float64(-dx*math.Sin(effect.Angle)) + float64(dy*math.Cos(effect.Angle)))
		return along >= 0 && along <= effect.Radius && perpendicular <= effect.Width+math.Max(enemy.Sprite.FrameWidth, enemy.Sprite.FrameHeight)/2
	case "targetCircle":
		return distance <= effect.Radius+math.Max(enemy.Sprite.FrameWidth, enemy.Sprite.FrameHeight)/2
	default:
		return distance <= effect.Radius && angle <= effect.HalfAngle
	}
}

func attackModeHitGeometry(state *State, effect *AttackModeEffect) error {
	ids := []int{}
	for _, enemy := range state.Enemies {
		if !slices.Contains(effect.HitIDs, enemy.ID) && attackModeVisible(effect, &enemy) && attackModeContains(effect, &enemy) {
			ids = append(ids, enemy.ID)
		}
	}
	for _, id := range ids {
		if enemy := enemyFor(state, id); enemy != nil {
			effect.HitIDs = append(effect.HitIDs, id)
			if err := attackModeHit(state, effect, enemy, 1); err != nil {
				return err
			}
			if effect.Kind == "arc" && effect.Context.AbilityID == "psychoCutMega" {
				effect.MaxDistance += 60
			}
		}
	}
	return nil
}

func advanceAttackModeGeometry(state *State, effect *AttackModeEffect) (bool, error) {
	if effect.CreatedTick == state.Tick && (effect.Kind == "arc" || effect.Kind == "dune" || effect.Kind == "directional") {
		return true, nil
	}
	if effect.WallClock {
		effect.AgeMS = state.Clock - effect.CreatedClock
	} else {
		effect.AgeMS += battleStepWholeMS(state)
	}
	if effect.AgeMS < effect.DelayMS {
		return true, nil
	}
	if effect.FollowSource {
		if tower := worldTowerFor(state, effect.PokemonID); tower != nil {
			effect.Origin = Point{X: tower.X, Y: tower.Y}
			effect.Position = effect.Origin
		}
	}
	switch effect.Kind {
	case "wave":
		effect.Radius += float64(effect.Speed * battleStepSeconds(state))
		if err := attackModeHitGeometry(state, effect); err != nil {
			return false, err
		}
		return effect.Radius < effect.MaxRadius, nil
	case "arc", "dune":
		effect.Travelled += float64(effect.Speed * battleStepSeconds(state))
		effect.Position = Point{X: effect.Origin.X + float64(math.Cos(effect.Angle)*effect.Travelled), Y: effect.Origin.Y + float64(math.Sin(effect.Angle)*effect.Travelled)}
		if err := attackModeHitGeometry(state, effect); err != nil {
			return false, err
		}
		return effect.Travelled < effect.MaxDistance, nil
	case "directional":
		if effect.AgeMS <= effect.HitDurationMS {
			if err := attackModeHitGeometry(state, effect); err != nil {
				return false, err
			}
		}
	}
	return effect.AgeMS < effect.DurationMS, nil
}

func attackModeChainDestination(state *State, pokemon *Pokemon, point Point) Point {
	route, _ := routeForState(state)
	width := pokemon.Sprite.FrameWidth
	if width <= 0 {
		width = 24
	}
	margin := width / 2
	return Point{X: math.Max(margin, math.Min(float64(route.Width)-margin, point.X)), Y: math.Max(margin, math.Min(float64(route.Height)-margin, point.Y))}
}

func attackModeChainHit(state *State, tower *Tower, pokemon *Pokemon, enemy *Enemy, bonus int, roll float64) error {
	context := contextFor(state, tower, pokemon, enemy, roll)
	profile, err := profileFor(state, tower, pokemon, enemy, roll)
	if err != nil {
		return err
	}
	source, err := sourceProfileFor(state, tower, pokemon, enemy, roll)
	if err != nil {
		return err
	}
	effect := AttackModeEffect{ID: allocateID(state), Kind: "chainCharge", PokemonID: pokemon.ID, TowerID: tower.ID, Position: Point{X: tower.X, Y: tower.Y}, Profile: profile, SourceProfile: source, Context: context, ModeBonusPct: float64(bonus) * 0.05}
	if err := attackModeHit(state, &effect, enemy, 1); err != nil {
		return err
	}
	tower.TargetID = enemy.ID
	tower.AttackCount++
	tower.IdleMS = 0
	addEvent(state, Event{Type: "chainHit", Source: pokemon.ID, Target: enemy.ID, X: enemy.X, Y: enemy.Y, Amount: float64(bonus)})
	return WorldAfterAttack(state, tower.ID, profile, []int{enemy.ID})
}

func handleAttackModeChain(state *State, tower *Tower, pokemon *Pokemon, profile AttackProfile, context AbilityContext, targets []*Enemy) (bool, error) {
	value := attackModeTower(state, tower)
	if value.Chain != nil {
		return true, nil
	}
	if len(targets) == 0 {
		return false, nil
	}
	if profile.Disabled || !EnemyAttackAllowed(state, pokemon, targets[0], false) {
		return true, nil
	}
	target := targets[0]
	home := Point{X: tower.X, Y: tower.Y}
	destination := attackModeChainDestination(state, pokemon, Point{X: target.X, Y: target.Y})
	value.Chain = &AttackChain{Phase: "moving", Origin: home, Destination: destination, Home: home, DurationMS: 250, HitIDs: []int{target.ID}, CreatedTick: state.Tick}
	return true, attackModeChainHit(state, tower, pokemon, target, 0, context.Random)
}

func advanceAttackModeChain(state *State, tower *Tower, value *AttackModeTower) error {
	chain := value.Chain
	owner := pokemonFor(state, tower.PokemonID)
	if chain == nil || owner == nil {
		return nil
	}
	if chain.Phase == "moving" || chain.Phase == "returning" {
		if chain.CreatedTick == state.Tick {
			return nil
		}
		chain.AgeMS += battleStepWholeMS(state)
		fraction := math.Min(1, float64(chain.AgeMS)/float64(chain.DurationMS))
		eased := float64(fraction*fraction) * (3 - float64(2*fraction))
		tower.X = chain.Origin.X + float64((chain.Destination.X-chain.Origin.X)*eased)
		tower.Y = chain.Origin.Y + float64((chain.Destination.Y-chain.Origin.Y)*eased)
		if fraction < 1 {
			return nil
		}
		if chain.Phase == "returning" {
			tower.X, tower.Y = chain.Home.X, chain.Home.Y
			value.Chain = nil
			profile, err := profileFor(state, tower, owner, nil, 0.5)
			if err != nil {
				return err
			}
			tower.CooldownMS = max(0, profile.AttackMS)
			return nil
		}
		chain.Phase, chain.AgeMS, chain.DurationMS = "waiting", 0, 300
	}
	if chain.Phase != "waiting" {
		return nil
	}
	chain.AgeMS += battleStepWholeMS(state)
	if chain.AgeMS < 300 {
		return nil
	}
	profile, err := profileFor(state, tower, owner, nil, 0.5)
	if err != nil {
		return err
	}
	nearestID := 0
	distance := math.Inf(1)
	for _, enemy := range state.Enemies {
		if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || enemy.InvulnerableMS > 0 || enemy.Invisible && !profile.CanSeeInvisible || slices.Contains(chain.HitIDs, enemy.ID) {
			continue
		}
		current := vectorLength(enemy.X-tower.X, enemy.Y-tower.Y)
		if current <= profile.Range && current < distance {
			nearestID, distance = enemy.ID, current
		}
	}
	chain.Origin = Point{X: tower.X, Y: tower.Y}
	chain.AgeMS, chain.DurationMS, chain.CreatedTick = 0, 250, state.Tick
	if target := enemyFor(state, nearestID); target != nil {
		chain.Bonus++
		chain.HitIDs = append(chain.HitIDs, target.ID)
		chain.Destination = attackModeChainDestination(state, owner, Point{X: target.X, Y: target.Y})
		chain.Phase = "moving"
		return attackModeChainHit(state, tower, owner, target, chain.Bonus, NextRandom(state))
	}
	chain.Phase, chain.Destination = "returning", chain.Home
	return nil
}

func AdvanceAttackModes(state *State) error {
	return advanceAttackModesWhere(state, func(int) bool { return true })
}

func advanceAttackModesWhere(state *State, include func(int) bool) error {
	for pokemonID, value := range state.AttackModes.Towers {
		tower := worldTowerFor(state, pokemonID)
		if tower == nil || value == nil || value.TowerID != tower.ID {
			OnAttackModeRetire(state, pokemonID)
		}
	}
	for index := range state.Towers {
		tower := &state.Towers[index]
		if !include(tower.ID) {
			continue
		}
		if value := state.AttackModes.Towers[tower.PokemonID]; value != nil && value.Chain != nil {
			if err := advanceAttackModeChain(state, tower, value); err != nil {
				return err
			}
		}
	}
	kept := make([]AttackModeEffect, 0, len(state.AttackModes.Effects))
	for _, effect := range state.AttackModes.Effects {
		if tower := worldTowerFor(state, effect.PokemonID); tower == nil || tower.ID != effect.TowerID {
			continue
		}
		if !include(effect.TowerID) {
			kept = append(kept, effect)
			continue
		}
		if effect.Kind != "bombardment" && effect.Kind != "lightningArea" {
			keep, err := advanceAttackModeGeometry(state, &effect)
			if err != nil {
				return err
			}
			if keep {
				kept = append(kept, effect)
			}
			continue
		}
		effect.AgeMS += battleStepWholeMS(state)
		fraction := math.Min(1, float64(effect.AgeMS)/float64(effect.DurationMS))
		effect.Position = Point{X: effect.Origin.X + float64((effect.Destination.X-effect.Origin.X)*fraction), Y: effect.Origin.Y + float64((effect.Destination.Y-effect.Origin.Y)*fraction)}
		if !effect.Impacted && effect.AgeMS >= effect.DurationMS {
			effect.Impacted = true
			ids := []int{}
			for _, enemy := range state.Enemies {
				if enemy.HP > 0 && !enemy.Dead && !enemy.Escaped && enemy.InvulnerableMS <= 0 && (!enemy.Invisible || effect.Profile.CanSeeInvisible) && vectorLength(enemy.X-effect.Destination.X, enemy.Y-effect.Destination.Y) <= effect.Radius+math.Max(enemy.Sprite.FrameWidth, enemy.Sprite.FrameHeight)/2 {
					ids = append(ids, enemy.ID)
				}
			}
			for _, id := range ids {
				if enemy := enemyFor(state, id); enemy != nil {
					effect.HitIDs = append(effect.HitIDs, id)
					if err := attackModeHit(state, &effect, enemy, 1); err != nil {
						return err
					}
				}
			}
		}
		if effect.Impacted {
			effect.ImpactMS += battleStepWholeMS(state)
		}
		if !effect.Impacted || effect.ImpactMS < 400 {
			kept = append(kept, effect)
		}
	}
	state.AttackModes.Effects = kept
	return nil
}

func ValidateAttackModes(state *State) error {
	if len(state.AttackModes.Towers) > len(state.Team) || len(state.AttackModes.Effects) > 8192 {
		return errors.New("攻击模式状态超出上限")
	}
	for pokemonID, value := range state.AttackModes.Towers {
		owner := ownedPokemonFor(state, pokemonID)
		if owner == nil || value == nil || value.TowerID < 1 || value.TowerID >= state.NextID || !attackModePointValid(value.Aim) || !finiteNumber(value.AreaRadius) || value.AreaRadius < 0 || !finiteNumber(value.AreaWidth) || value.AreaWidth < 0 || !finiteNumber(value.AreaHeight) || value.AreaHeight < 0 || value.SweepCount < 0 || !slices.Contains([]int{-1, 0, 1}, value.SwordDirection) {
			return errors.New("攻击模式塔身或瞄准状态无效")
		}
		if chain := value.Chain; chain != nil {
			duration := int64(250)
			if chain.Phase == "waiting" {
				duration = 300
			}
			if !slices.Contains([]string{"moving", "waiting", "returning"}, chain.Phase) || chain.AgeMS < 0 || chain.DurationMS != duration || chain.CreatedTick < 0 || chain.CreatedTick > state.Tick || chain.Bonus < 0 || chain.Bonus != len(chain.HitIDs)-1 || !attackModeIDsValid(state, chain.HitIDs) || !attackModePointValid(chain.Origin) || !attackModePointValid(chain.Destination) || !attackModePointValid(chain.Home) {
				return errors.New("攻击模式移动链状态无效")
			}
		}
	}
	seen := map[int]bool{}
	for _, effect := range state.AttackModes.Effects {
		if effect.ID < 1 || effect.ID >= state.NextID || seen[effect.ID] || ownedPokemonFor(state, effect.PokemonID) == nil || effect.TowerID < 1 || effect.TowerID >= state.NextID || !slices.Contains([]string{"bombardment", "lightningArea", "wave", "sweep", "directional", "arc", "dune"}, effect.Kind) || !attackModePointValid(effect.Position) || !attackModePointValid(effect.Origin) || !attackModePointValid(effect.Destination) || effect.DurationMS <= 0 || effect.AgeMS < 0 || effect.DelayMS < 0 || effect.ImpactMS < 0 || effect.CreatedClock < 0 || effect.CreatedClock > state.Clock || effect.CreatedTick < 0 || effect.CreatedTick > state.Tick || !attackModeIDsValid(state, effect.HitIDs) {
			return errors.New("攻击模式几何或时钟状态无效")
		}
		seen[effect.ID] = true
		for _, value := range []float64{effect.Radius, effect.MaxRadius, effect.Width, effect.Speed, effect.Travelled, effect.MaxDistance, effect.HalfAngle, effect.ModeBonusPct, effect.FinalMultiplier} {
			if !finiteNumber(value) || value < 0 {
				return errors.New("攻击模式几何或倍率无效")
			}
		}
		if !finiteNumber(effect.Angle) || !finiteNumber(effect.Profile.Power) || !finiteNumber(effect.SourceProfile.Power) {
			return errors.New("攻击模式发射属性无效")
		}
		for _, point := range effect.BoltPoints {
			if !attackModePointValid(point) {
				return errors.New("攻击模式折线坐标无效")
			}
		}
	}
	return nil
}

func attackModePointValid(point Point) bool {
	return finiteNumber(point.X) && finiteNumber(point.Y)
}

func attackModeIDsValid(state *State, ids []int) bool {
	seen := map[int]bool{}
	for _, id := range ids {
		if id < 1 || id >= state.NextID || seen[id] {
			return false
		}
		seen[id] = true
	}
	return len(ids) <= 4096
}
