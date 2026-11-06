package pvz_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
)

type balanceSample struct {
	Round      int    `json:"round"`
	Seconds    int    `json:"seconds"`
	Money      [2]int `json:"money"`
	Earned     [2]int `json:"earned"`
	Spent      [2]int `json:"spent"`
	Covered    int    `json:"covered"`
	Producers  int    `json:"producers"`
	MowersUsed int    `json:"mowersUsed"`
	Zombies    int    `json:"zombies"`
}
type balanceRun struct {
	Scene       string               `json:"scene"`
	Players     int                  `json:"players"`
	Seed        int                  `json:"seed"`
	Strategy    string               `json:"strategy"`
	Defender    string               `json:"defender"`
	Profile     int                  `json:"profile"`
	Initial     engine.State         `json:"initial"`
	Samples     []balanceSample      `json:"samples"`
	Rounds      []engine.RoundResult `json:"rounds"`
	Actions     []map[string]any     `json:"actions"`
	FinalDigest string               `json:"finalDigest"`
	FinalTick   int64                `json:"finalTick"`
}

func balanceShooter(kind string) bool {
	switch kind {
	case "peashooter", "snowpea", "repeater", "cactus", "cattail", "cabbagepult", "kernelpult", "melonpult", "wintermelon", "puffshroom", "fumeshroom", "seashroom", "scaredyshroom", "gloomshroom", "gatlingpea", "threepeater", "splitpea", "starfruit", "cobcannon":
		return true
	}
	return false
}
func balanceMoney(s engine.State) (money [2]int) {
	for _, p := range s.Players {
		money[p.Team] += p.Money
	}
	return
}
func balancePlay(t *testing.T, scene string, n, seed int, strategy, defender string) balanceRun {
	t.Helper()
	seconds := 180
	if value := os.Getenv("PVZ_BALANCE_SECONDS"); value != "" {
		var err error
		seconds, err = strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	s, e := engine.New(engine.Options{Scene: scene, RoundSeconds: seconds, PrepSeconds: 10}, n, fmt.Sprintf("balance-%s-%d-%d", scene, n, seed))
	if e != nil {
		t.Fatal(e)
	}
	engine.EnableProgression(&s)
	r := balanceRun{Scene: scene, Players: n, Seed: seed, Strategy: strategy, Defender: defender, Profile: s.RulesVersion, Initial: engine.Clone(s), Samples: []balanceSample{}, Actions: []map[string]any{}}
	initial := balanceMoney(s)
	spent := [2]int{}
	sampled := map[string]bool{}
	lastRound := 1
	maxTicks := int64((2*(s.Options.PrepSeconds+s.Options.RoundSeconds) + 7) * 1000 / engine.TickMS)
	for s.Phase != "finished" && s.Tick < maxTicks {
		if s.Round != lastRound {
			initial = balanceMoney(s)
			spent = [2]int{}
			lastRound = s.Round
		}
		// 200ms decisions; every action uses that seat's ordinary visible world.
		if s.Tick%4 == 0 {
			for offset := range n {
				actor := (offset + int(s.Tick/4)) % n
				visible := engine.ProjectState(s, actor)
				a := engine.BotAction(&visible, actor)
				if defender == "formation" && s.Players[actor].Side == "plants" && (a == nil || a.Type != "collect") {
					a = balanceFormation(visible, actor)
				}
				if strategy == "basic-rush" && s.Players[actor].Side == "zombies" {
					a = balanceBasicRush(visible, actor)
				}
				if strategy == "elite-pressure" && s.Players[actor].Side == "zombies" {
					a = balanceElitePressure(visible, actor)
				}
				if strategy == "siege" && s.Players[actor].Side == "zombies" {
					a = balanceSiege(visible, actor)
				}
				if a == nil {
					continue
				}
				before := balanceMoney(s)
				if e = engine.Apply(&s, actor, *a); e != nil {
					t.Fatalf("illegal ordinary strategy action %+v: %v", a, e)
				}
				after := balanceMoney(s)
				if a.Type == "plant" || a.Type == "spawn" {
					spent[s.Players[actor].Team] += before[s.Players[actor].Team] - after[s.Players[actor].Team]
				}
				r.Actions = append(r.Actions, map[string]any{"tick": s.Tick, "round": s.Round, "actor": actor, "action": *a})
			}
		}
		seconds := -s.Options.PrepSeconds + int(s.Tick*engine.TickMS/1000)
		if s.Phase == "battle" {
			seconds = int((s.Tick - s.BattleStartTick) * engine.TickMS / 1000)
		}
		key := fmt.Sprintf("%d:%s:%d", s.Round, s.Phase, seconds)
		if (s.Phase == "prepare" && s.Tick == 0 || s.Phase == "battle" && (seconds == 0 || seconds == 15 || seconds == 30 || seconds == 60 || seconds == 90 || seconds == 120 || seconds == 150)) && !sampled[key] {
			sampled[key] = true
			money := balanceMoney(s)
			q := balanceSample{Round: s.Round, Seconds: seconds, Money: money, Spent: spent, Zombies: len(s.Zombies)}
			for i := range money {
				q.Earned[i] = money[i] + spent[i] - initial[i]
			}
			covered := map[int]bool{}
			for _, p := range s.Plants {
				if p.HP <= 0 {
					continue
				}
				if balanceShooter(p.Kind) && !p.Asleep {
					covered[p.Row] = true
				}
				if p.Kind == "sunflower" || p.Kind == "sunshroom" || p.Kind == "twinsunflower" {
					q.Producers++
				}
			}
			q.Covered = len(covered)
			for _, m := range s.Mowers {
				if m.State != "ready" {
					q.MowersUsed++
				}
			}
			r.Samples = append(r.Samples, q)
		}
		if e = engine.Step(&s, 1); e != nil {
			t.Fatal(e)
		}
	}
	if s.Phase != "finished" || len(s.Rounds) != 2 {
		t.Fatal("natural two-round match did not finish", s.Phase, s.Tick)
	}
	if e = engine.Validate(s); e != nil {
		t.Fatal(e)
	}
	r.Rounds = s.Rounds
	r.FinalDigest = engine.Digest(s)
	r.FinalTick = s.Tick
	return r
}

func TestPVZEconomyRecordedReplayE2E(t *testing.T) {
	dir := os.Getenv("PVZ_BALANCE_REPLAY")
	if dir == "" {
		t.Skip("opt-in recorded balance replay")
	}
	files, e := filepath.Glob(filepath.Join(dir, "*.json"))
	if e != nil || len(files) == 0 {
		t.Fatal("no recorded matches", e)
	}
	for _, name := range files {
		t.Run(filepath.Base(name), func(t *testing.T) {
			raw, e := os.ReadFile(name)
			if e != nil {
				t.Fatal(e)
			}
			var record struct {
				Initial     engine.State `json:"initial"`
				FinalTick   int64        `json:"finalTick"`
				FinalDigest string       `json:"finalDigest"`
				Actions     []struct {
					Tick   int64         `json:"tick"`
					Actor  int           `json:"actor"`
					Action engine.Action `json:"action"`
				} `json:"actions"`
			}
			if e = json.Unmarshal(raw, &record); e != nil || record.Initial.Epoch == "" || record.FinalTick <= 0 {
				t.Fatal("invalid replay", e)
			}
			s := record.Initial
			cursor := 0
			for s.Tick < record.FinalTick {
				for cursor < len(record.Actions) && record.Actions[cursor].Tick == s.Tick {
					a := record.Actions[cursor]
					if e = engine.Apply(&s, a.Actor, a.Action); e != nil {
						t.Fatal("recorded action changed legality", s.Tick, a, e)
					}
					cursor++
				}
				if e = engine.Step(&s, 1); e != nil {
					t.Fatal(e)
				}
			}
			if cursor != len(record.Actions) || engine.Digest(s) != record.FinalDigest {
				t.Fatal("recorded match changed under this engine", name)
			}
		})
	}
}

// A deliberate income-first opening and weakest-lane response, using only the
// same deck, wallet, cooldowns and visible world as a human player.
func balanceFormation(s engine.State, actor int) *engine.Action {
	p := s.Players[actor]
	income := 0
	covered := map[int]bool{}
	nearest := make([]int, s.RowCount())
	for i := range nearest {
		nearest[i] = 12000
	}
	for _, v := range s.Plants {
		if v.HP <= 0 {
			continue
		}
		if v.Owner == actor && (v.Kind == "sunflower" || v.Kind == "sunshroom") {
			income++
		}
		if balanceShooter(v.Kind) && !v.Asleep {
			covered[v.Row] = true
		}
	}
	for _, z := range s.Zombies {
		if z.HP > 0 && z.Allegiance != "plants" {
			nearest[z.Row] = min(nearest[z.Row], z.X)
		}
	}
	attempt := func(kind string, row, col int) *engine.Action {
		a := engine.Action{Type: "plant", Kind: kind, Row: row, Col: col, Epoch: s.Epoch, Seq: p.Seq + 1}
		copy := engine.Clone(s)
		if engine.Apply(&copy, actor, a) == nil {
			return &a
		}
		return nil
	}
	build := func(kind string, row, col int) *engine.Action {
		c, ok := engine.PlantSpec(kind)
		if !ok || p.Money < c.Cost || p.Cooldowns[kind] > s.Tick {
			return nil
		}
		has := false
		for _, k := range p.Deck {
			if k == kind {
				has = true
			}
		}
		if !has {
			return nil
		}
		if a := attempt(kind, row, col); a != nil {
			return a
		}
		support := ""
		if s.IsWater(row) {
			support = "lilypad"
		}
		if s.Map.Roof {
			support = "flowerpot"
		}
		base, _ := engine.PlantSpec(support)
		if support != "" && p.Money >= c.Cost+base.Cost {
			return attempt(support, row, col)
		}
		return nil
	}
	for _, z := range s.Zombies {
		if z.Kind == "balloon" && z.Altitude > 0 {
			for row := 0; row < s.RowCount(); row++ {
				for col := 0; col < 3; col++ {
					if a := build("blover", row, col); a != nil {
						return a
					}
				}
			}
		}
	}
	target := 2
	if len(s.Players) == 4 {
		target = 1
	}
	if income < target {
		producer := "sunflower"
		if s.Map.Night && !s.Map.Roof {
			producer = "sunshroom"
		}
		for row := 0; row < s.RowCount(); row++ {
			if nearest[row] < 4500 {
				continue
			}
			for col := 0; col < 2; col++ {
				if a := build(producer, row, col); a != nil {
					return a
				}
			}
		}
	}
	for range s.RowCount() {
		best := -1
		for row := 0; row < s.RowCount(); row++ {
			if !covered[row] && (best < 0 || nearest[row] < nearest[best]) {
				best = row
			}
		}
		if best < 0 {
			break
		}
		covered[best] = true
		kinds := []string{"peashooter"}
		col := 2
		if s.Map.Roof {
			kinds = []string{"cabbagepult", "kernelpult"}
		} else if s.Map.Night {
			kinds = []string{"puffshroom", "fumeshroom", "seashroom"}
			col = 5
			if s.IsWater(best) {
				kinds = []string{"seashroom", "fumeshroom"}
			}
		}
		for _, kind := range kinds {
			for _, c := range []int{col, col + 1, col - 1} {
				if a := build(kind, best, c); a != nil {
					return a
				}
			}
		}
	}
	// Preserve cash for the next missing shooter instead of consuming it on
	// optional defenses. Once every route has fire, use the production strategy.
	rows := map[int]bool{}
	for _, v := range s.Plants {
		if v.HP > 0 && balanceShooter(v.Kind) && !v.Asleep {
			rows[v.Row] = true
		}
	}
	if len(rows) < s.RowCount() {
		return nil
	}
	return engine.BotAction(&s, actor)
}

func balanceElitePressure(s engine.State, actor int) *engine.Action {
	if s.Phase != "battle" {
		return nil
	}
	choices := []string{"buckethead", "conehead", "duckytube"}
	if s.Tick-s.BattleStartTick >= 900 {
		choices = []string{"football", "dolphin", "buckethead", "conehead"}
	}
	if s.Tick-s.BattleStartTick >= 1800 {
		choices = []string{"gatlinghead", "football", "dolphin", "buckethead", "conehead"}
		if s.Map.Night && !s.Map.Roof {
			choices = []string{"zomboni", "gatlinghead", "football", "dolphin", "buckethead", "conehead"}
		}
	}
	for _, kind := range choices {
		for _, k := range s.Players[actor].Deck {
			if k == kind {
				s.Players[actor].Deck = []string{kind}
				return engine.BotAction(&s, actor)
			}
		}
	}
	return nil
}

func balanceSiege(s engine.State, actor int) *engine.Action {
	if s.Phase != "battle" {
		return nil
	}
	p := s.Players[actor]
	elapsed := int((s.Tick - s.BattleStartTick) * engine.TickMS / 1000)
	if elapsed >= 120 && p.Cooldowns["gargantuar"] <= s.Tick {
		for _, k := range p.Deck {
			if k == "gargantuar" {
				s.Players[actor].Deck = []string{"gargantuar"}
				return engine.BotAction(&s, actor)
			}
		}
	}
	if elapsed < 120 {
		future := 0
		for second := elapsed + 1; second <= 120; second++ {
			if second%2 == 0 {
				amount := 25
				if second >= 60 {
					amount = 30
				}
				future += amount
			}
		}
		reserve := max(0, 600-future/(len(s.Players)/2))
		s.Players[actor].Money = max(0, p.Money-reserve)
	}
	return engine.BotAction(&s, actor)
}

func balanceBasicRush(s engine.State, actor int) *engine.Action {
	if s.Phase != "battle" {
		return nil
	}
	p := s.Players[actor]
	// Prefer low-cost regular attackers, a reproducible pressure policy rather
	// than buying special units. Select the weakest visible eligible lane.
	for _, kind := range []string{"zombie", "conehead", "duckytube"} {
		c, ok := engine.ZombieSpec(kind)
		if !ok || p.Money < c.Cost || p.Cooldowns[kind] > s.Tick {
			continue
		}
		inDeck := false
		for _, k := range p.Deck {
			if k == kind {
				inDeck = true
			}
		}
		if !inDeck {
			continue
		}
		best, bestScore := -1, 100000
		for row := 0; row < s.RowCount(); row++ {
			if s.LaneCooldowns[row] > s.Tick || c.WaterOnly != s.IsWater(row) {
				continue
			}
			score := 0
			for _, v := range s.Plants {
				if v.Row == row && v.HP > 0 {
					score += v.HP / 100
					if balanceShooter(v.Kind) {
						score += 15
					}
				}
			}
			if s.Mowers[row].State == "spent" {
				score -= 20
			}
			if score < bestScore {
				best, bestScore = row, score
			}
		}
		if best >= 0 {
			return &engine.Action{Type: "spawn", Kind: kind, Row: best, Epoch: s.Epoch, Seq: p.Seq + 1}
		}
	}
	return nil
}

func TestPVZEconomyBalanceAssessmentE2E(t *testing.T) {
	if os.Getenv("PVZ_BALANCE_ASSESS") != "1" {
		t.Skip("opt-in repeatable balance assessment")
	}
	label := os.Getenv("PVZ_BALANCE_LABEL")
	if label == "" {
		label = "assessment"
	}
	seedFrom, seeds := 0, 3
	for _, q := range []struct {
		name  string
		value *int
	}{{"PVZ_BALANCE_SEED_FROM", &seedFrom}, {"PVZ_BALANCE_SEEDS", &seeds}} {
		if raw := os.Getenv(q.name); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil || v < 0 || v > 32 {
				t.Fatal("invalid seed range", raw, e)
			}
			*q.value = v
		}
	}
	if seeds < 1 {
		t.Fatal("seed count must be positive")
	}
	out := filepath.Join("..", "..", "..", ".local/pvz/balance", label)
	if e := os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	for _, scene := range []string{"day", "night", "pool", "fog", "roof", "roofnight"} {
		for _, n := range []int{2, 4} {
			for seed := seedFrom; seed < seedFrom+seeds; seed++ {
				for _, defender := range []string{"default", "formation"} {
					for _, strategy := range []string{"default", "basic-rush", "elite-pressure", "siege"} {
						t.Run(fmt.Sprintf("%s-%d-%d-%s-%s", scene, n, seed, defender, strategy), func(t *testing.T) {
							r := balancePlay(t, scene, n, seed, strategy, defender)
							raw, _ := json.MarshalIndent(r, "", "  ")
							if e := os.WriteFile(filepath.Join(out, fmt.Sprintf("%s-%d-%d-%s-%s.json", scene, n, seed, defender, strategy)), raw, 0600); e != nil {
								t.Fatal(e)
							}
						})
					}
				}
			}
		}
	}
}
