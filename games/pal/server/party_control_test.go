package pal

// Failure scenarios before changing role allocation: solo loses a party
// member; a reordering transfers an existing role; a departed role's seat is
// not offered to a newcomer; two new roles choose the same idle seat; the
// two-human third role is not shared; fewer roles invent extra party members.
// This isolates allocation only, not original scripted joins/leaves.
import (
	"reflect"
	"testing"
)

func TestPartyControllersPreserveRolesAndFillVacantSeats(t *testing.T) {
	for _, tt := range []struct {
		name    string
		players int
		old     []PartyMember
		roles   []int
		want    []int
	}{
		{"solo whole party", 1, nil, []int{0, 1, 2}, []int{0, 0, 0}},
		{"two humans share third role", 2, nil, []int{0, 1, 2}, []int{0, 1, 0}},
		{"three independent roles", 3, nil, []int{0, 1, 2}, []int{0, 1, 2}},
		{"short party retains empty seats", 3, nil, []int{0}, []int{0}},
		{"reorder preserves owners", 3, []PartyMember{{Role: 0, Controller: 0}, {Role: 1, Controller: 1}, {Role: 2, Controller: 2}}, []int{2, 0, 1}, []int{2, 0, 1}},
		{"departure preserves survivors", 3, []PartyMember{{Role: 0, Controller: 0}, {Role: 1, Controller: 1}, {Role: 2, Controller: 2}}, []int{0, 2}, []int{0, 2}},
		{"new role fills departed middle seat", 3, []PartyMember{{Role: 0, Controller: 0}, {Role: 2, Controller: 2}}, []int{0, 2, 3}, []int{0, 2, 1}},
		{"two new roles fill distinct vacant seats", 3, []PartyMember{{Role: 2, Controller: 2}}, []int{2, 0, 3}, []int{2, 0, 1}},
		{"new role precedes survivor", 3, []PartyMember{{Role: 0, Controller: 0}, {Role: 2, Controller: 2}}, []int{3, 2, 0}, []int{1, 2, 0}},
		{"two humans balance a new role", 2, []PartyMember{{Role: 2, Controller: 1}}, []int{2, 0, 3}, []int{1, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := append([]PartyMember(nil), tt.old...)
			if got := partyControllers(tt.roles, tt.old, tt.players); !reflect.DeepEqual(got, tt.want) {
				t.Fatal("wrong original role allocation", got, "want", tt.want)
			}
			if !reflect.DeepEqual(before, tt.old) {
				t.Fatal("allocation mutated the preceding party")
			}
		})
	}
}
