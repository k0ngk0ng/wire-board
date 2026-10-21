package racing

// Before implementation: scenery must stay outside the road, cannot trap the
// starting grid, and a visible solid object must not let a car pass through.
import "testing"

func TestRoadsideScenerySharesCollisionFootprint(t *testing.T) {
	for _, track := range Tracks() {
		if len(track.Props) == 0 || len(track.Props) > 100 {
			t.Fatalf("unbounded/missing scenery: %s", track.ID)
		}
		for _, prop := range track.Props {
			_, _, d, _ := nearest(track, Point{prop.X, prop.Y})
			if d < track.RoadWidth/2+prop.Radius+25 {
				t.Fatalf("scenery blocks road: %s %+v", track.ID, prop)
			}
		}
	}
	track := Track{Width: 1000, Height: 1000, RoadWidth: 160, Points: []Point{{50, 50}, {950, 50}, {950, 950}, {50, 950}}, Props: []Scenery{{X: 250, Y: 200, Radius: 25, Height: 50, Kind: "rock"}}}
	car := Car{X: 210, Y: 200, Angle: 0, VX: 200, Speed: 200, LastInputAt: 1000, Throttle: 1}
	for at := int64(1020); at <= 1200; at += 20 {
		stepCar(&car, track, 20, at)
	}
	if car.X > 202.01 {
		t.Fatalf("car passed through visible obstacle: %+v", car)
	}
}
