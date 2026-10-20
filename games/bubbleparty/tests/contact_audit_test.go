package bubbleparty_test

// Failure inventory precedes these checks in
// docs/research/bubbleparty/contact-audit-20261021.md.
import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestContactAuditDirectionalApproach(t *testing.T) {
	for _, enemy := range []bool{false, true} {
		for _, speed := range []int{150, 70} {
			for _, offset := range []float64{-.45, 0, .45} {
				for _, direction := range []struct {
					name string
					x, y int
				}{{"right", -1, 0}, {"left", 1, 0}, {"down", 0, -1}, {"up", 0, 1}} {
					t.Run(fmt.Sprintf("enemy=%v/speed=%d/offset=%.2f/%s", enemy, speed, offset, direction.name), func(t *testing.T) {
						actor := 1
						if enemy {
							actor = 2
						}
						m := refactorArena(t, func(s *bubbleparty.State) {
							s.Players[1].X, s.Players[1].Y = 2, 2
							p := &s.Players[0]
							p.OffsetX, p.OffsetY = offset, offset
							p.TrappedUntil = s.ServerAt + 5000
							p.TrappedBy = 3
							q := &s.Players[actor]
							q.X, q.Y = 5+direction.x*2, 5+direction.y*2
							q.SpeedMS = speed
						})
						at := refactorRead(t, m, 0).ServerAt
						// Public reclaim must preserve trap lifetime and allow fresh controls.
						var err error
						m, err = bubbleparty.Module().Reclaim(gameapi.Context{Actor: actor, Players: 4, Now: time.UnixMilli(at)}, m)
						if err != nil {
							t.Fatal(err)
						}
						if refactorRead(t, m, 0).Players[0].TrappedUntil != at+5000 {
							t.Fatal("reclaim changed trap deadline")
						}
						m = refactorInput(t, m, actor, direction.name, false)
						for elapsed := int64(20); elapsed <= 500; elapsed += 20 {
							m = refactorTick(t, m, at+elapsed)
							p := refactorRead(t, m, 0).Players[0]
							if !p.Alive || p.TrappedUntil == 0 {
								break
							}
						}
						v := refactorRead(t, m, 0)
						if enemy {
							if v.Players[0].Alive || v.Players[actor].Eliminations != 1 || v.Players[3].Eliminations != 0 {
								t.Fatalf("enemy contact/credit failed: %+v", v.Players)
							}
						} else if !v.Players[0].Alive || v.Players[0].TrappedUntil != 0 || v.Players[0].InvulnerableUntil <= v.ServerAt {
							t.Fatalf("ally contact failed: %+v", v.Players)
						}
					})
				}
			}
		}
	}
}

func TestContactAuditWaterAndFractionalSeparation(t *testing.T) {
	for _, water := range []bool{false, true} {
		for _, enemy := range []bool{false, true} {
			t.Run(fmt.Sprintf("water=%v/enemy=%v", water, enemy), func(t *testing.T) {
				actor := 1
				if enemy {
					actor = 2
				}
				m := refactorArena(t, func(s *bubbleparty.State) {
					s.Players[1].X, s.Players[1].Y = 2, 2
					p := &s.Players[0]
					p.X, p.Y = 5, 5
					p.OffsetX = .45
					p.TrappedUntil = s.ServerAt + 5000
					p.TrappedBy = 3
					q := &s.Players[actor]
					q.X, q.Y = 5, 5
					q.OffsetX = -.45
					if water {
						s.Blasts = []bubbleparty.Blast{{X: 5, Y: 5, Seat: 3, At: s.ServerAt, EndAt: s.ServerAt + 1000}}
					}
				})
				at := refactorRead(t, m, 0).ServerAt
				m = refactorTick(t, m, at+20)
				v := refactorRead(t, m, 0)
				if !v.Players[0].Alive || v.Players[0].TrappedUntil == 0 {
					t.Fatal("same integer cell falsely triggered distant contact")
				}
				if water {
					if v.Players[actor].TrappedUntil == 0 {
						t.Fatal("active water did not trap approaching player")
					}
					m = refactorInput(t, m, actor, "right", true)
					m = refactorTick(t, m, at+400)
					v = refactorRead(t, m, 0)
					if !v.Players[0].Alive || v.Players[0].TrappedUntil == 0 || len(v.Bombs) != 0 {
						t.Fatal("trapped attacker performed contact or bomb action")
					}
				} else {
					m = refactorInput(t, m, actor, "right", false)
					m = refactorTick(t, m, at+200)
					v = refactorRead(t, m, 0)
					if enemy && v.Players[0].Alive || !enemy && v.Players[0].TrappedUntil != 0 {
						t.Fatalf("fresh contact failed: %+v", v.Players)
					}
				}
			})
		}
	}
}

func TestContactAuditBotUsesSameTouchOutcome(t *testing.T) {
	for _, enemy := range []bool{false, true} {
		t.Run(fmt.Sprintf("enemy=%v", enemy), func(t *testing.T) {
			target := 1
			if enemy {
				target = 2
			}
			m := refactorArena(t, func(s *bubbleparty.State) {
				s.Players[1].X, s.Players[1].Y = 2, 2
				p := &s.Players[target]
				p.X, p.Y = 6, 5
				p.TrappedUntil = s.ServerAt + 5000
				p.TrappedBy = 3
			})
			at := refactorRead(t, m, 0).ServerAt
			raw, err := bubbleparty.Module().BotAction(gameapi.Context{Actor: 0, Players: 4, Now: time.UnixMilli(at)}, m)
			if err != nil {
				t.Fatal(err)
			}
			var action bubbleparty.Action
			if err = json.Unmarshal(raw, &action); err != nil {
				t.Fatal(err)
			}
			if action.Move != "right" || action.Bomb {
				t.Fatalf("bot did not choose contact: %+v", action)
			}
			m, err = bubbleparty.Module().Apply(gameapi.Context{Actor: 0, Players: 4, Now: time.UnixMilli(at)}, m, raw)
			if err != nil {
				t.Fatal(err)
			}
			m = refactorTick(t, m, at+200)
			v := refactorRead(t, m, 0)
			if enemy && (v.Players[target].Alive || v.Players[0].Eliminations != 1) || !enemy && v.Players[target].TrappedUntil != 0 {
				t.Fatalf("bot touch outcome failed: %+v", v.Players)
			}
		})
	}
}

func TestContactAuditProtectionAndDoubleTrap(t *testing.T) {
	for _, enemy := range []bool{false, true} {
		for _, protected := range []bool{false, true} {
			for _, actorTrapped := range []bool{false, true} {
				t.Run(fmt.Sprintf("enemy=%v/protected=%v/actorTrapped=%v", enemy, protected, actorTrapped), func(t *testing.T) {
					actor := 1
					if enemy {
						actor = 2
					}
					m := refactorArena(t, func(s *bubbleparty.State) {
						s.Players[1].X, s.Players[1].Y = 2, 2
						p := &s.Players[0]
						p.TrappedUntil = s.ServerAt + 5000
						p.TrappedBy = 3
						q := &s.Players[actor]
						q.X, q.Y = 5, 5
						q.OffsetX = -.1
						if protected {
							q.InvulnerableUntil = s.ServerAt + 1000
						}
						if actorTrapped {
							q.TrappedUntil = s.ServerAt + 5000
							q.TrappedBy = 3
						}
						s.Blasts = []bubbleparty.Blast{{X: 5, Y: 5, Seat: 3, At: s.ServerAt, EndAt: s.ServerAt + 1000}}
					})
					at := refactorRead(t, m, 0).ServerAt
					m = refactorTick(t, m, at+20)
					v := refactorRead(t, m, 0)
					if !protected || actorTrapped {
						if !v.Players[0].Alive || v.Players[0].TrappedUntil == 0 || !v.Players[actor].Alive || v.Players[actor].TrappedUntil == 0 {
							t.Fatalf("two trapped actors performed contact: %+v", v.Players)
						}
					} else if enemy {
						if v.Players[0].Alive || v.Players[actor].Eliminations != 1 || v.Players[actor].TrappedUntil != 0 {
							t.Fatalf("protected enemy contact failed: %+v", v.Players)
						}
					} else if !v.Players[0].Alive || v.Players[0].TrappedUntil != 0 || v.Players[actor].TrappedUntil != 0 {
						t.Fatalf("protected rescue failed: %+v", v.Players)
					}
				})
			}
		}
	}
}

func TestContactAuditWallBlocksApproach(t *testing.T) {
	for _, direction := range []struct {
		name string
		x, y int
	}{{"right", 1, 0}, {"left", -1, 0}, {"down", 0, 1}, {"up", 0, -1}} {
		t.Run(direction.name, func(t *testing.T) {
			m := refactorArena(t, func(s *bubbleparty.State) {
				s.Players[1].X, s.Players[1].Y = 2, 2
				p := &s.Players[0]
				p.TrappedUntil = s.ServerAt + 5000
				p.TrappedBy = 3
				q := &s.Players[2]
				q.X, q.Y = 5-direction.x*2, 5-direction.y*2
				s.Cells[(5-direction.y)*bubbleparty.Cols+5-direction.x] = bubbleparty.CellWall
			})
			at := refactorRead(t, m, 0).ServerAt
			m = refactorInput(t, m, 2, direction.name, false)
			m = refactorTick(t, m, at+500)
			v := refactorRead(t, m, 0)
			if !v.Players[0].Alive || v.Players[0].TrappedUntil == 0 || v.Players[2].Eliminations != 0 {
				t.Fatalf("contact crossed wall: %+v", v.Players)
			}
		})
	}
}
