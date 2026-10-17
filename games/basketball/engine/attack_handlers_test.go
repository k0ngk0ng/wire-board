package engine

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// Before implementation: reject table aliases, animation reinitialization on
// continuation, repeated sounds, velocity resets, changed landing order, wrong
// mirror/loop progression and unrepresented mutations of another object.
func TestOriginalAttackHandler6(t *testing.T)  { verifyOriginalAttackHandler(t, 6) }
func TestOriginalAttackHandler11(t *testing.T) { verifyOriginalAttackHandler(t, 11) }
func TestOriginalAttackHandler16(t *testing.T) { verifyOriginalAttackHandler(t, 16) }
func TestOriginalAttackHandler29(t *testing.T) { verifyOriginalAttackHandler(t, 29) }
func TestOriginalAttackHandler30(t *testing.T) { verifyOriginalAttackHandler(t, 30) }

func verifyOriginalAttackHandler(t *testing.T, handler uint8) {
	t.Helper()
	f, err := os.Open(fmt.Sprintf("testdata/attack-handler-%d.json.gz", handler))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Name          string
		Actor         uint8
		Before, After observedAttack
		Sounds        []uint8
	}
	if err := json.NewDecoder(z).Decode(&rows); err != nil || len(rows) == 0 {
		t.Fatal("missing original attack records", err)
	}
	for _, row := range rows {
		b, a := row.Before, row.After
		s := AttackState{Player: playerFromObservation(b.observedPlayer), AttackVariant: b.Variant, JumpPhase: int8(b.Boost)}
		ctx := AttackContext{Team: uint8(b.Team), Character: uint8(b.Character), Computer: b.Control&128 != 0, Attachment: uint8(b.Attachment), HasBall: b.Holder == int32(row.Actor), EnhancementEnabled: b.EnhancementEnabled, Enhancement: b.Enhancement}
		profile := KunioProfile()
		profile.RunSpeed = 4*Pixel + Fixed(b.RunGrade)*Pixel/4
		got, err := StepPlayerAttack(s, ctx, profile, NewYorkGroundCourt())
		want := AttackState{Player: playerFromObservation(a.observedPlayer), AttackVariant: a.Variant, JumpPhase: int8(a.Boost)}
		if err != nil || got.State != want || got.Revisit != (a.Attachment&64 == 0) {
			t.Fatalf("%s actor=%d before=%+v\ngot=%+v\nwant=%+v error=%v", row.Name, row.Actor, s, got, want, err)
		}
		if !reflect.DeepEqual(append([]uint8{}, got.SoundIDs...), append([]uint8{}, row.Sounds...)) {
			t.Fatalf("%s sounds got=%v want=%v", row.Name, got.SoundIDs, row.Sounds)
		}
		if b.Owners != a.Owners || got.Ball.Change || got.Ball.ChangeRequest {
			t.Fatalf("%s unrepresented owner/ball change", row.Name)
		}
		for i := range b.Actions {
			if i != int(row.Actor) && (b.Actions[i] != a.Actions[i] || b.Facings[i] != a.Facings[i]) {
				t.Fatalf("%s unrepresented object %d mutation", row.Name, i)
			}
		}
	}
	t.Logf("matched %d isolated original handler %d calls; not natural gameplay", len(rows), handler)
}
