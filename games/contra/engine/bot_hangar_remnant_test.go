package contra

import "testing"

// Hangar U04 replaces the spike-wall support's visible top tiles but retains
// its collision. Treating the new leading code as a fresh map cell invents
// air. The identical lower artwork is Empty after spike destruction but
// Platform at the armored-door exit, so its material must remain unknown.
func TestBuiltinBotVisionHangarRemnantPreservesVisiblePlatform(t *testing.T) {
	o := botVisionObservePNG(t, "testdata/bot-hangar-remnant-platform.png")
	for _, x := range []int{70, 100, 150} {
		if !o.terrainKnownAt(x, 148) || o.terrainAt(x, 148) != TerrainPlatform {
			t.Errorf("retained bridge became air at %d: known=%v kind=%v", x, o.terrainKnownAt(x, 148), o.terrainAt(x, 148))
		}
	}
	if o.terrainKnownAt(100, 164) {
		t.Fatal("identical lower artwork disclosed an ambiguous material")
	}
}
