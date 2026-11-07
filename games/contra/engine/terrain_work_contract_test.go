package contra

// Contract written before the TerrainWork owner. Native writes are evidence,
// never projection inputs. Failures: swapped query/packing byte roles, lost
// scans/restores, odd-column partial updates, false writes on player overflow,
// erased perspective offsets, rejected graphics replacing work, reordered
// same-frame producers, or a new frame silently resetting retained work.
import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

type terrainWorkNativeInput struct {
	A, X, Y, ScrollX, ScrollY, Control, PageToggle, Axis, PlayerX, SourceX, GraphicX uint8
	Work, TileAddress                                                                uint16
	Collision                                                                        string
}
type terrainWorkNativeOp struct {
	ID, Parent, First, Last int
	Name, Phase             string
	Input                   terrainWorkNativeInput
	After                   uint16
}
type terrainWorkNativeCheckpoint struct {
	Name, RAM string
	Sequence  int
	Work      uint16
	QueueUsed uint8
}
type terrainWorkNativeFrame struct {
	Frame         int
	Before, After struct {
		Work           uint16
		Collision, RAM string
	}
	Operations   []terrainWorkNativeOp
	Checkpoints  []terrainWorkNativeCheckpoint
	Writes       []struct{ Sequence int }
	PriorGap     []json.RawMessage
	PrefixWrites []struct {
		Offset, KeepMask, Value uint8
		Work                    uint16
	}
}

func readTerrainWorkNative(t *testing.T, name string) []terrainWorkNativeFrame {
	t.Helper()
	f, err := os.Open("../../../docs/research/nes-contra/evidence/terrain-work-" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d struct {
		Original bool `json:"source_is_original_execution"`
		Frames   []terrainWorkNativeFrame
	}
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if !d.Original {
		t.Fatal("not original execution")
	}
	return d.Frames
}
func terrainWorkSemantic(name string) bool {
	switch name {
	case "support-query", "query-coordinate", "query-prepared-x", "query-page-toggle", "support-index", "read-index", "stream-column", "stream-row", "graphic-location", "player-perspective", "weapon-perspective", "entity-probe", "collision-clear", "guardian-pair-clear", "guardian-row-shift":
		return true
	}
	return false
}
func terrainWorkOperations(f terrainWorkNativeFrame) []terrainWorkNativeOp {
	byID := map[int]terrainWorkNativeOp{}
	for _, o := range f.Operations {
		byID[o.ID] = o
	}
	var out []terrainWorkNativeOp
	for _, o := range f.Operations {
		if !terrainWorkSemantic(o.Name) {
			continue
		}
		nested := false
		for p := o.Parent; p != 0; p = byID[p].Parent {
			if terrainWorkSemantic(byID[p].Name) {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].First < out[j].First })
	return out
}
func applyTerrainWorkNative(t *testing.T, w *TerrainWork, o terrainWorkNativeOp) {
	t.Helper()
	i := o.Input
	scroll := CollisionScroll{X: i.ScrollX, Y: i.ScrollY, NameTable: i.Control}
	switch o.Name {
	case "support-query", "query-coordinate", "query-prepared-x", "query-page-toggle":
		b, err := hex.DecodeString(i.Collision)
		if err != nil || len(b) != 128 {
			t.Fatal("invalid native terrain")
		}
		terrain := CollisionWindow(b)
		x := i.A
		if o.Name == "query-prepared-x" {
			x = uint8(w.Value >> 8)
		}
		if o.Name == "query-page-toggle" {
			scroll.NameTable ^= i.PageToggle
		}
		w.RecordQuery(terrain.At(x, i.Y, scroll))
	case "support-index", "read-index":
		w.RecordIndexRead(i.Y)
	case "stream-column":
		w.RecordStreamColumn(i.TileAddress)
	case "stream-row":
		w.RecordStreamRow(i.Axis)
	case "graphic-location":
		w.RecordGraphicLocation(locateGraphics(i.GraphicX, i.Y, scroll).address)
	case "player-perspective":
		w.RecordPerspectiveX(i.PlayerX)
	case "weapon-perspective":
		w.RecordPerspectiveX(i.SourceX)
	default:
		t.Fatalf("unexpected upstream operation %s", o.Name)
	}
}
func terrainWorkEnemyCheckpoint(t *testing.T, f terrainWorkNativeFrame) terrainWorkNativeCheckpoint {
	t.Helper()
	for _, c := range f.Checkpoints {
		if c.Name == "enemy-entry" {
			return c
		}
	}
	t.Fatal("missing native E54F entry")
	return terrainWorkNativeCheckpoint{}
}
func projectTerrainWorkUpstream(t *testing.T, f terrainWorkNativeFrame) (TerrainWork, int) {
	t.Helper()
	w := TerrainWork{Value: f.Before.Work}
	end := terrainWorkEnemyCheckpoint(t, f)
	count := 0
	covered := map[int]bool{}
	for _, o := range terrainWorkOperations(f) {
		if o.Last >= end.Sequence {
			continue
		}
		if w.Value != o.Input.Work {
			t.Fatalf("frame%d before%s %04x/%04x", f.Frame, o.Name, w.Value, o.Input.Work)
		}
		applyTerrainWorkNative(t, &w, o)
		if w.Value != o.After {
			t.Fatalf("frame%d after%s %04x/%04x", f.Frame, o.Name, w.Value, o.After)
		}
		for _, write := range f.Writes {
			if write.Sequence > o.First && write.Sequence < o.Last {
				covered[write.Sequence] = true
			}
		}
		count++
	}
	for _, write := range f.Writes {
		if write.Sequence < end.Sequence && !covered[write.Sequence] {
			t.Fatalf("frame%d unmodeled upstream write at order%d", f.Frame, write.Sequence)
		}
	}
	if w.Value != end.Work {
		t.Fatalf("frame%d E54F work%04x/%04x", f.Frame, w.Value, end.Work)
	}
	return w, count
}

func TestTerrainWorkC001UpstreamContract(t *testing.T) {
	frames, operations := 0, 0
	for _, name := range []string{"stage8-v2", "stage6-v2", "guardian-cases-v5", "vertical-cases-v1", "indoor-cases-v2"} {
		for _, f := range readTerrainWorkNative(t, name) {
			if len(f.PriorGap) != 0 {
				t.Fatalf("%s frame%d has unmodeled between-frame writes", name, f.Frame)
			}
			_, n := projectTerrainWorkUpstream(t, f)
			frames++
			operations += n
		}
	}
	if frames != 2677 || operations != 24310 {
		t.Fatalf("native prefix coverage frames%d operations%d", frames, operations)
	}
}

func TestTerrainWorkGraphicAndOverflowContracts(t *testing.T) {
	graphics, overflow := 0, 0
	for _, name := range []string{"stage8-v2", "stage6-v2"} {
		for _, f := range readTerrainWorkNative(t, name) {
			for _, o := range terrainWorkOperations(f) {
				if o.Name != "graphic-location" {
					continue
				}
				w := TerrainWork{Value: o.Input.Work}
				applyTerrainWorkNative(t, &w, o)
				if w.Value != o.After {
					t.Fatalf("frame%d native graphic location %04x/%04x", f.Frame, w.Value, o.After)
				}
				graphics++
			}
		}
	}
	d, trace := alienLocatorFixture(t)
	for index, r := range trace.Records {
		if !strings.Contains(trace.Cases[index].Setup.Category, "overflow") {
			continue
		}
		var before, after uint16
		for _, slot := range r.Slots {
			if slot.Slot == 14 {
				before = slot.Locator
			}
			if slot.Slot == 13 {
				after = slot.Locator
			}
		}
		w := TerrainWork{Value: before}
		w.RecordEntityProbeOverflow(d.Records[index].Before.Enemies[14][2])
		if w.Value != after {
			t.Fatalf("native overflow case%d %04x/%04x", index, w.Value, after)
		}
		overflow++
	}
	if graphics != 73 || overflow != 18 {
		t.Fatalf("source graphic/overflow coverage %d/%d", graphics, overflow)
	}
}

func TestTerrainWorkGuardianConsumptionContract(t *testing.T) {
	meta := loadFrameOracle(t)
	resources := levelResources(t, "stage-8.json")
	tiles, err := resources.Graphics(7, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	rows := readTerrainWorkNative(t, "guardian-cases-v5")
	if len(rows) != 50 {
		t.Fatal("Guardian prefix coverage")
	}
	for _, f := range rows {
		work, _ := projectTerrainWorkUpstream(t, f)
		entry := terrainWorkEnemyCheckpoint(t, f)
		if entry.QueueUsed < 64 {
			t.Fatal("native queue was not occupied")
		}
		// Consumer context is the observed E54F input. Its Work value is supplied
		// exclusively by the preceding semantic projection, never the oracle.
		in := activeWorldFromOriginal(t, meta, entry.RAM)
		b, _ := hex.DecodeString(entry.RAM)
		terrain := in.Combat.Terrain
		mapped := in.Terrain.Supertiles
		q := terrainWorkOriginalQueue(t, b[0x700:0x700+int(entry.QueueUsed)])
		prefixAt := 0
		c := EnemyStepContext{Level: 7, FrameScroll: b[0x68], Terrain: &terrain, Scroll: in.Combat.World.Scroll}
		c.ClearQueuedGraphicsPrefix = func(offset, andMask uint8) error {
			if prefixAt >= len(f.PrefixWrites) {
				t.Fatalf("frame%d extra graphics-prefix clear %d/%02x", f.Frame, offset, andMask)
			}
			want := f.PrefixWrites[prefixAt]
			prefixAt++
			if offset != want.Offset || andMask != want.KeepMask || work.Value != want.Work {
				t.Fatalf("frame%d prefix clear %d/%02x at%04x want%+v", f.Frame, offset, andMask, work.Value, want)
			}
			if err := q.ClearPrefix(offset, andMask); err != nil {
				return err
			}
			encoded, err := q.Encode()
			if err != nil {
				return err
			}
			if encoded[offset] != want.Value {
				t.Fatalf("frame%d prefix result%02x/%02x", f.Frame, encoded[offset], want.Value)
			}
			return nil
		}
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
			v, e := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tiles})
			if v.Accepted {
				work.RecordGraphicLocation(v.Address)
				t.Fatal("native refusal unexpectedly accepted")
			}
			return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address}, e
		}
		auto := in.Combat.World.AutoScroll[1]
		boss := AlienBossContext{AutoScroll: &auto, CollisionAddress: &work.Value}
		if err = in.Enemies.StepAlienGuardian(13, c, boss); err != nil {
			t.Fatalf("frame%d: %v", f.Frame, err)
		}
		wantTerrain, _ := hex.DecodeString(f.After.Collision)
		if work.Value != f.After.Work || !bytes.Equal(terrain[:], wantTerrain) {
			t.Fatalf("frame%d Guardian input/output mismatch work%04x/%04x", f.Frame, work.Value, f.After.Work)
		}
		after, _ := hex.DecodeString(f.After.RAM)
		encoded, err := q.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if q.Used != entry.QueueUsed || prefixAt != len(f.PrefixWrites) || !bytes.Equal(encoded, after[0x700:0x700+int(entry.QueueUsed)]) {
			t.Fatalf("frame%d original queued bytes/budget differ", f.Frame)
		}
		if encoded[0] == 0 && len(q.Writes()) != 0 {
			t.Fatalf("frame%d cleared leading kind did not stop drawing", f.Frame)
		}
	}
}

func terrainWorkOriginalQueue(t *testing.T, raw []uint8) GraphicsQueue {
	t.Helper()
	q := GraphicsQueue{}
	for pos := 0; pos < len(raw); {
		if pos+3 > len(raw) {
			t.Fatal("truncated original graphics header")
		}
		p := GraphicsPacket{Kind: raw[pos], Length: raw[pos+1]}
		count := int(raw[pos+2])
		pos += 3
		for range count {
			if pos+2+int(p.Length) > len(raw) {
				t.Fatal("truncated original graphics segment")
			}
			p.Segments = append(p.Segments, GraphicsSegment{Address: uint16(raw[pos])<<8 | uint16(raw[pos+1]), Values: append([]uint8(nil), raw[pos+2:pos+2+int(p.Length)]...)})
			pos += 2 + int(p.Length)
		}
		if err := q.Append(p); err != nil {
			t.Fatal(err)
		}
	}
	return q
}
