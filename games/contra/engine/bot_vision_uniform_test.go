package contra

// Before uniform-cell pruning: compare against an independent full rectangle
// scan; a single different interior/edge pixel must prevent rejection. Alpha
// must not change RGB-only semantics, and clipped cells stay unknown.
import "testing"

func TestBotUniformCellMatchesFullRGBScan(t *testing.T) {
	pixels := make([]uint8, 256*240*4)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = 17, 49, 91, uint8(i)
	}
	for _, p := range [][2]int{{8, 8}, {15, 15}, {128, 127}, {255, 239}, {240, 224}} {
		pixels[(p[1]*256+p[0])*4]++
	}
	runs := botSameColorRuns(pixels)
	check := func(x, y int) {
		want := x >= 0 && y >= 0 && x+16 <= 256 && y+16 <= 240
		if want {
			at := (y*256 + x) * 4
			for yy := y; yy < y+16; yy++ {
				for xx := x; xx < x+16; xx++ {
					p := (yy*256 + xx) * 4
					if pixels[p] != pixels[at] || pixels[p+1] != pixels[at+1] || pixels[p+2] != pixels[at+2] {
						want = false
					}
				}
			}
		}
		if got := botUniformCell(pixels, runs, x, y); got != want {
			t.Fatalf("cell %d,%d uniform=%v want%v", x, y, got, want)
		}
		if want {
			if _, known := botImageCell(pixels, x, y); known {
				t.Fatal("pruning contradicted canonical cell")
			}
		}
	}
	for _, p := range [][2]int{{0, 0}, {32, 32}, {112, 112}, {240, 224}, {241, 224}, {-1, 0}, {0, -1}, {0, 225}} {
		check(p[0], p[1])
	}
	seed := uint32(42)
	for i := 0; i < 1000; i++ {
		seed = seed*1664525 + 1013904223
		x := int(seed % 256)
		seed = seed*1664525 + 1013904223
		check(x, int(seed%240))
	}
}
