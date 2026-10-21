package fortressblue

// Failures specified before reconstruction: all second weapons share one sprite/
// arc; ninth projectile rejected; first impact damages future missiles; restore
// duplicates impacts; same-team heal hurts enemies; homing retargets or excludes
// allies; mine visibility leaks to opponents/spectators; movement skips mines;
// pending mine damage arrives before its contact/explosion animation.
import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

func TestNativeWeaponProfilesAllFourteen(t *testing.T) {
	counts := map[int][2]int{4: {1, 3}, 7: {3, 9}, 8: {1, 3}, 9: {1, 2}, 12: {1, 2}, 13: {3, 9}}
	for id := 1; id <= 14; id++ {
		want := [2]int{1, 1}
		if n, ok := counts[id]; ok {
			want = n
		}
		for i, mode := range []string{"primary", "special"} {
			p := nativeWeapon(id, mode)
			if p.Count != want[i] {
				t.Fatalf("tank %d %s count %d != %d", id, mode, p.Count, want[i])
			}
		}
	}
	if nativeWeapon(14, "special").Kind != "team_heal" || nativeWeapon(6, "special").Motion != "homing" || nativeWeapon(12, "special").Motion != "wave" || nativeWeapon(9, "special").Kind != "mine" {
		t.Fatal("distinct secondary behavior missing")
	}
}
func TestNineProjectilesSurviveSerialization(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	m, s = selectAll(t, s, m, []int{7, 1})
	at := time.UnixMilli(s.StartAt + 1000)
	raw, _ := json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: 35, Power: 65, Facing: 1, Weapon: "special"})
	m, err := apply(ctx(2, 0, at), m, raw)
	if err != nil {
		t.Fatal(err)
	}
	s, err = decode(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.LastShot.Arcs) != 9 {
		t.Fatal("nine missiles truncated", len(s.LastShot.Arcs))
	}
	if s.LastShot.Mode != "special" {
		t.Fatal("mode not retained")
	}
	for _, a := range s.LastShot.Arcs {
		if a.ImpactAt <= s.LastShot.LaunchAt || len(a.PathTimes) != len(a.Path) {
			t.Fatal("invalid independent path")
		}
	}
}
func TestVolleyImpactsAndRestoreAreIndependent(t *testing.T) {
	s := supplyState(t)
	at := s.StartAt + 1000
	target := tankCenter(s.Tanks[1])
	hp := s.Tanks[1].HP
	s.LastShot = &Shot{ID: 1, Seat: 0, Mode: "special", At: at, LaunchAt: at + 334, ImpactAt: at + 1200, SettleAt: at + 1650, Arcs: []Arc{{Path: []Point{{X: target.X - 100, Y: target.Y}, target}, PathTimes: []float64{334, 1000}, Impact: target, Hit: 1, ImpactAt: at + 1000}, {Path: []Point{{X: target.X - 100, Y: target.Y}, target}, PathTimes: []float64{334, 1200}, Impact: target, Hit: 1, ImpactAt: at + 1200}}}
	s.PendingShot = &PendingShot{Weapon: Weapon{Damage: 10, Blast: 0}}
	s.Resolution = &Resolution{Stage: "flying", ShotID: 1, At: at, LaunchAt: at + 334, ImpactAt: at + 1000, SettleAt: at + 1650}
	s.Deadline = 0
	processResolution(s, time.UnixMilli(at+999))
	if s.Tanks[1].HP != hp {
		t.Fatal("pre-impact damage")
	}
	processResolution(s, time.UnixMilli(at+1000))
	firstHP := s.Tanks[1].HP
	if firstHP >= hp || len(s.LastShot.Blasts) != 1 || s.LastShot.Blasts[0].At != at+1000 || s.LastShot.Arcs[1].Resolved {
		t.Fatal("first impact settled future projectile")
	}
	m, _ := s.envelope()
	restored, err := restore(ctx(2, 0, time.UnixMilli(at+1100)), m)
	if err != nil {
		t.Fatal(err)
	}
	s, err = decode(restored)
	if err != nil {
		t.Fatal(err)
	}
	processResolution(s, time.UnixMilli(at+1199))
	if s.Tanks[1].HP != firstHP || len(s.LastShot.Blasts) != 1 {
		t.Fatal("restore/retry duplicated early impact")
	}
	processResolution(s, time.UnixMilli(at+1200))
	if s.Tanks[1].HP >= firstHP || len(s.LastShot.Blasts) != 2 || s.LastShot.Blasts[1].At != at+1200 || s.Resolution.Stage != "settling" {
		t.Fatal("last projectile did not settle independently")
	}
}
func TestNativeTeamHealDoesNotDamageEnemyOrExceedMaximum(t *testing.T) {
	s := supplyState(t)
	s.Tanks[0].HP = 100
	s.Tanks[1].X = s.Tanks[0].X + 20
	s.Tanks[1].Y = s.Tanks[0].Y
	enemyHP := s.Tanks[1].HP
	blast := applyTeamHealingBlast(s, 0, Weapon{Damage: 150, Blast: 22}, tankCenter(s.Tanks[0]), s.StartAt+1000)
	if s.Tanks[0].HP <= 100 || s.Tanks[0].HP > s.Tanks[0].MaxHP || s.Tanks[1].HP != enemyHP || blast.Kind != "heal" {
		t.Fatal("native heal target rules violated")
	}
}
func TestHomingLocksNearestPointIncludingAlly(t *testing.T) {
	tanks := []Tank{{Seat: 0, Selected: true, Alive: true, X: 20, Y: 100, Team: 0}, {Seat: 1, Selected: true, Alive: true, X: 50, Y: 100, Team: 0}, {Seat: 2, Selected: true, Alive: true, X: 250, Y: 100, Team: 1}}
	p, ok := homingTarget(Point{X: 55, Y: 82}, tanks, 100)
	if !ok || math.Abs(p.X-50) > 0.01 {
		t.Fatal("homing incorrectly excludes allies")
	}
}
func TestMinesPrivateProjectionAndDelayedContact(t *testing.T) {
	s := supplyState(t)
	at := s.StartAt + 1000
	point := Point{X: s.Tanks[0].X + 60, Y: s.Tanks[0].Y}
	plantMine(s, 0, point, 320, at)
	if len(s.Mines) != 1 || len(s.project(0).Mines) != 1 || len(s.project(1).Mines) != 0 || len(s.project(-1).Mines) != 0 {
		t.Fatal("mine location leaked")
	}
	s.Tanks[1].X = point.X - 45
	s.Tanks[1].Y = point.Y
	before := s.Tanks[1].HP
	latchMineContact(s, 1, Point{X: point.X - 45, Y: point.Y}, Point{X: point.X + 45, Y: point.Y}, at+100)
	if s.Mines[0].TriggerAt != at+280 {
		t.Fatal("fast crossing failed to latch")
	}
	tickMines(s, at+279)
	if s.Tanks[1].HP != before {
		t.Fatal("damage before contact display")
	}
	s.Tanks[1].X = point.X
	tickMines(s, at+280)
	if len(s.Mines) != 0 || s.Tanks[1].HP >= before || len(s.MineBlasts) != 1 || s.MineBlasts[0].At != at+280 {
		t.Fatal("mine did not resolve at confirmed contact")
	}
}

func TestNativeShamanBotDoesNotFireHealingAtHealthyEnemy(t *testing.T) {
	s, m := startAt(t, 2, "stage-16")
	_, s = selectAll(t, s, m, []int{14, 1})
	s.Tanks[0].Items = nil
	s.Tanks[0].Fuel = 0
	for i := 0; i < 10; i++ {
		s.Epoch = fmt.Sprint("shaman-", i)
		s.Turn = i + 1
		m, _ = s.envelope()
		raw, err := botAction(ctx(2, 0, time.UnixMilli(s.StartAt+1000)), m)
		if err != nil {
			t.Fatal(err)
		}
		var a Action
		_ = json.Unmarshal(raw, &a)
		if a.Type == "fire" && a.Weapon != "primary" {
			t.Fatal("healthy team should use damage weapon", a)
		}
	}
}
func TestMovementLockSuppressesThreeRefillsWithoutChangingOtherTank(t *testing.T) {
	s := supplyState(t)
	addStatus(&s.Tanks[1], Status{Kind: "immobile", Turns: 3, Source: 0})
	s.NextSupplyTurn = 0
	for cycle := 0; cycle < 3; cycle++ {
		s.Current = 0
		advanceTurn(s, time.UnixMilli(s.StartAt+int64(cycle+1)*1000))
		if s.Current != 1 || s.Tanks[1].Fuel != 0 {
			t.Fatal("move refill was not suppressed", cycle)
		}
		s.Current = 1
		advanceTurn(s, time.UnixMilli(s.StartAt+int64(cycle+1)*1000+100))
		if s.Tanks[0].Fuel <= 0 {
			t.Fatal("restriction affected other player")
		}
	}
	s.Current = 0
	advanceTurn(s, time.UnixMilli(s.StartAt+5000))
	if s.Tanks[1].Fuel <= 0 {
		t.Fatal("platform restriction never expires")
	}
}

func TestLandingTriggersMineAndSupportRemovalChains(t *testing.T) {
	s := supplyState(t)
	at := s.StartAt + 1000
	tank := &s.Tanks[1]
	ground := tank.Y
	x := tank.X
	s.Mines = []Mine{{ID: 100, Source: 0, Team: 0, X: x, Y: ground, Damage: 100, At: at}}
	tank.Y = ground - 50
	s.MoveRemainingMS = 10000
	s.Deadline = 0
	s.Resolution = &Resolution{Stage: "settling", At: at, ImpactAt: at, SettleAt: at + 450, Falls: []Fall{{Seat: 1, FromX: x, FromY: ground - 50, ToX: x, ToY: ground, At: at, EndAt: at + 200}}}
	processResolution(s, time.UnixMilli(at+200))
	if len(s.MineBlasts) != 0 {
		t.Fatal("mine damage before resolved contact")
	}
	processResolution(s, time.UnixMilli(at+450))
	if len(s.Mines) != 0 || len(s.MineBlasts) == 0 {
		t.Fatal("landed tank bypassed mine contact")
	}
	s = supplyState(t)
	at = s.StartAt + 1000
	x = s.Tanks[1].X
	ground = s.Tanks[1].Y
	s.Mines = []Mine{{ID: 100, Source: 0, Team: 0, X: x, Y: ground, Damage: 10, At: at, TriggerAt: at + 100}, {ID: 101, Source: 0, Team: 0, X: x + 4, Y: ground, Damage: 10, At: at}}
	tickMines(s, at+100)
	if len(s.Mines) != 0 || len(s.MineBlasts) != 2 {
		t.Fatal("unsupported neighboring mine failed to resolve in same timeline")
	}
}

func TestNativePoisonFriendlyContactDoesNotRewardFriendlyDamage(t *testing.T) {
	s, m := startAt(t, 4, "stage-16")
	_, s = selectAll(t, s, m, []int{2, 1, 1, 1})
	s.Tanks[1].HP = 20
	addStatus(&s.Tanks[1], Status{Kind: "poison", Turns: 1, Damage: 25, Source: 0})
	tickStatuses(s, 1, s.StartAt+1000)
	if s.Tanks[1].HP != 0 {
		t.Fatal("native all-team status should still apply")
	}
	if s.Tanks[0].Damage != 0 || s.Tanks[0].Kills != 0 || s.DamageTotal[0] != 0 {
		t.Fatal("friendly poison awarded enemy damage/kill credit")
	}
}
