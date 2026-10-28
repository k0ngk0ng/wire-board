package contra

// Failure cases before the tunnel setter: treating code4 as four identical
// cells, swapping top/bottom rows, erasing neighboring packed cells, crossing a
// packed byte incorrectly, and applying collision after a rejected draw.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalBossDoorCollisionPatterns(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-shooters-bossdoor.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Cases    []struct{ Routine, Index, Used uint8 }
		Records  []struct {
			Case                                   int
			CollisionBefore, CollisionAfter, Queue string
			Used                                   uint8
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original {
		t.Fatal("not original")
	}
	count := 0
	for _, r := range oracle.Records {
		c := oracle.Cases[r.Case-1]
		if c.Routine != 7 || r.Used == c.Used {
			continue
		}
		queue, err := hex.DecodeString(r.Queue)
		if err != nil {
			t.Fatal(err)
		}
		at := int(c.Used)
		var address uint16
		found := false
		for at < int(r.Used) {
			kind, length, segments := queue[at], int(queue[at+1]), int(queue[at+2])
			if kind == 1 && length == 4 && segments == 4 {
				address = uint16(queue[at+3])<<8 | uint16(queue[at+4])
				found = true
				break
			}
			at += 3 + segments*(2+length)
		}
		if !found {
			t.Fatal("missing actual original pattern packet", r.Case)
		}
		before, _ := hex.DecodeString(r.CollisionBefore)
		want, _ := hex.DecodeString(r.CollisionAfter)
		window := CollisionWindow(before)
		pattern := [...]uint8{0, 0, 0, 4, 0, 4, 0, 4}[c.Index]
		if err := setBossDoorCollision(&window, address, pattern); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(window[:], want) {
			t.Fatalf("original tunnel collision case%d address%x pattern%d differs", r.Case, address, pattern)
		}
		count++
	}
	if count == 0 {
		t.Fatal("no accepted original tunnel writes")
	}
	t.Logf("verified %d original tunnel collision writes", count)
}
