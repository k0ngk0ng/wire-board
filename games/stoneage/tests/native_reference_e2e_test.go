package stoneage_test

// Compare production-round impacts to the original C physical-damage,
// element and rider primitives. This is deliberately a public module E2E:
// the oracle is compiled separately and never linked into the game engine.
import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
)

func referenceRandom(state *uint64, n int) int {
	*state += 0x9e3779b97f4a7c15
	z := *state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	return int(z % uint64(n))
}
func TestStoneageNativeReferenceCombatE2E(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "classic-oracle")
	compile := exec.Command("cc", "-std=c99", "reference/classic.c", "-o", binary)
	if output, e := compile.CombinedOutput(); e != nil {
		t.Fatalf("C oracle: %v: %s", e, output)
	}
	cases := []any{}
	for _, mounted := range []bool{false, true} {
		for _, targetMounted := range []bool{false, true} {
			for _, element := range []int{0, 1, 2} {
				m, c, s := combatFixture(t)
				s.Players[0].Riding = mounted
				s.Players[1].Riding = targetMounted
				a, d := &s.Players[0].Body, &s.Players[1].Body
				ar, dr := &s.Players[0].Ride, &s.Players[1].Ride
				a.Attack, a.Defense, a.Speed = 230, 150, 1000
				ar.Attack, ar.Defense, ar.Speed = 110, 100, 1000
				d.Attack, d.Defense, d.Speed = 170, 140, 85
				dr.Attack, dr.Defense, dr.Speed = 140, 120, 70
				s.Players[0].Pet.Speed = 1
				s.Players[1].Pet.Speed = 1
				a.Elements = [4]int{100, 0, 0, 0}
				d.Elements = [4]int{}
				if element == 0 {
					d.Elements[0] = 100
				} else if element == 1 {
					d.Elements[1] = 100
				} else {
					d.Elements[3] = 100
				}
				// Select a noncritical first strike. Eight draws schedule four units;
				// then evasion, critical and the raw damage fraction are consumed.
				seed := uint64(1)
				roll := 0
				for ; seed < 1000; seed++ {
					r := seed
					for i := 0; i < 4; i++ {
						referenceRandom(&r, 100)
						referenceRandom(&r, 1<<30)
					}
					referenceRandom(&r, 100)
					critical := referenceRandom(&r, 100)
					roll = referenceRandom(&r, 10000)
					if critical >= 18 {
						break
					}
				}
				s.Random = seed
				m.Data = raw(s)
				first, second := guards(0), guards(1)
				first.Player = game.Command{Kind: "attack", Target: 2}
				first.Pet = game.Command{Kind: "wait", Target: 1}
				second.Pet = game.Command{Kind: "wait", Target: 3}
				for _, row := range s.Players[1].Inventory {
					if row.Item.Kind == "cure" {
						second.Player = game.Command{Kind: "item", ID: row.Item.ID, Target: 2}
						break
					}
				}
				resolved := combatRound(t, m, c, first, second)
				var impact *game.Event
				for i := range resolved.Events {
					e := &resolved.Events[i]
					if e.Kind == "attack" && e.Actor == 0 {
						impact = e
						break
					}
				}
				if impact == nil || impact.Critical {
					t.Fatal("oracle fixture did not produce the intended first strike")
				}
				am, dm := 0, 0
				if mounted {
					am = 1
				}
				if targetMounted {
					dm = 1
				}
				values := []int{230, 150, 1000, 170, 140, 85, 110, 100, 1000, 140, 120, 70, am, dm, 0, roll, element}
				args := []string{}
				for _, value := range values {
					args = append(args, strconv.Itoa(value))
				}
				output, e := exec.Command(binary, args...).CombinedOutput()
				if e != nil {
					t.Fatalf("oracle execution: %v %s", e, output)
				}
				var expected struct {
					Damage int `json:"damage"`
				}
				if e = json.Unmarshal(output, &expected); e != nil {
					t.Fatal(e, string(output))
				}
				want := expected.Damage
				if targetMounted && want > 0 {
					want++
				}
				got := impact.Amount + impact.RideAmount
				if got != want {
					t.Fatalf("native mismatch mounted=%v targetMounted=%v element=%d roll=%d: got=%d want=%d", mounted, targetMounted, element, roll, got, want)
				}
				cases = append(cases, map[string]any{"case": fmt.Sprintf("ride-%v-vs-%v-element-%d", mounted, targetMounted, element), "seed": seed, "roll": roll, "expected": want, "impact": impact})
			}
		}
	}
	report(t, "native-c-physical-reference", map[string]any{"scope": "original C base physical damage, elements, riding attack/defense and integer damage split; not full-engine parity", "cases": cases})
}
