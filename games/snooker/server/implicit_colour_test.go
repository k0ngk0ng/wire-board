package snooker

// Before removing the hidden default nomination: every colour after a red must
// score its own value; a red first, two colours, white pot, and wrong clearance
// remain fouls; missing everything must not invent a seven-point black target.
import "testing"

func TestImplicitColourAfterRed(t *testing.T) {
	for id := 16; id <= 21; id++ {
		c := rulePosition("colour", 2, 1, 16, 17, 18, 19, 20, 21)
		r := Judge(c, Strike{}, ruleMotion(c, []int{id}, []int{id}))
		if r.Foul != 0 || r.Points != BallValue(id) || !r.KeepTurn || r.Target != "red" {
			t.Fatalf("colour %d: %+v", id, r)
		}
	}
	c := rulePosition("colour", 2, 1, 16, 17, 18, 19, 20, 21)
	cases := []struct {
		first, pots []int
		foul        int
	}{
		{[]int{1}, []int{21}, 7}, {[]int{16}, []int{16, 21}, 7},
		{[]int{16, 20}, nil, 6}, {[]int{20}, []int{0, 20}, 6}, {nil, nil, 4},
	}
	for _, tc := range cases {
		r := Judge(c, Strike{}, ruleMotion(c, tc.first, tc.pots))
		if r.Foul != tc.foul || r.Points != 0 {
			t.Fatalf("%+v: %+v", tc, r)
		}
	}
	c = rulePosition("clear", 2, 16, 17, 18, 19, 20, 21)
	if r := Judge(c, Strike{}, ruleMotion(c, []int{20}, []int{20})); r.Foul != 6 || r.Points != 0 {
		t.Fatal("clearance order lost", r)
	}
}
