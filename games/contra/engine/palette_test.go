package contra

// Before implementation: partial-color loads and request replacement, fade
// exceptions/underflow, write events without value changes, deferred palette
// mirroring and the Japanese 0x47 queue gate, cycle/flash ordering, Boss queue
// append budget, one-step snow thresholds, and atomic failure across owners.
import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type paletteSnapshot struct {
	State struct {
		Desired, Applied                                          string
		RequestedColors, FadeTimer, Cycle, PauseCycle, FlashTimer uint8
	}
	Header                    string
	Snow                      struct{ Visibility, Cursor, Pace uint8 }
	Context                   struct{ GameRoutine, Counter, Level, SceneType, Alternate, Screen, Scroll uint8 }
	QueueUsed, GenerationBias uint8
	Queue                     string
}

func paletteHex(t *testing.T, s string, n int) []uint8 {
	t.Helper()
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != n {
		t.Fatal("bad palette oracle bytes", e, len(b), n)
	}
	return b
}
func (s paletteSnapshot) output(t *testing.T) PaletteOutput {
	p := PaletteOutput{RequestedColors: s.State.RequestedColors, FadeTimer: s.State.FadeTimer, Cycle: s.State.Cycle, PauseCycle: s.State.PauseCycle, FlashTimer: s.State.FlashTimer}
	copy(p.Desired[:], paletteHex(t, s.State.Desired, 32))
	copy(p.Applied[:], paletteHex(t, s.State.Applied, 32))
	return p
}
func (s paletteSnapshot) header(t *testing.T) PaletteState {
	b := paletteHex(t, s.Header, 15)
	return PaletteState{CollisionLimits: [3]uint8(b[:3]), CycleIndices: [4]uint8(b[3:7]), PaletteIndices: [8]uint8(b[7:])}
}

type paletteRecord struct {
	Routine          string
	Frame            int
	InputA           uint8
	Before, After    paletteSnapshot
	GenerationWrites []struct {
		PCAfter uint16
		Value   uint8
	}
}
type paletteTrace struct {
	Name    string
	Records []paletteRecord
}

func paletteFixture(t *testing.T) ([]paletteTrace, []paletteTrace, *LevelResources) {
	t.Helper()
	raw, e := os.ReadFile("testdata/original-palette.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	meta, e := os.ReadFile("testdata/original-palette-source.json")
	if e != nil {
		t.Fatal(e)
	}
	var source struct{ FixtureSHA256 string }
	if e = json.Unmarshal(meta, &source); e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(raw)
	if hex.EncodeToString(h[:]) != source.FixtureSHA256 {
		t.Fatal("palette oracle identity")
	}
	z, e := gzip.NewReader(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var data struct {
		Natural, Controlled struct {
			Records int
			Traces  []paletteTrace
		}
	}
	if e = json.NewDecoder(z).Decode(&data); e != nil {
		t.Fatal(e)
	}
	if data.Natural.Records != 6400 || data.Controlled.Records != 137 {
		t.Fatal("palette coverage changed")
	}
	raw, e = os.ReadFile("leveldata/levels.json")
	if e != nil {
		t.Fatal(e)
	}
	resources, e := DecodeLevelResources(raw)
	if e != nil {
		t.Fatal(e)
	}
	return data.Natural.Traces, data.Controlled.Traces, resources
}

func TestOriginalPaletteAndSnowPhases(t *testing.T) {
	natural, controlled, resources := paletteFixture(t)
	counts := map[string]int{}
	domainRejections := 0
	for _, group := range [][]paletteTrace{natural, controlled} {
		for _, trace := range group {
			t.Run(trace.Name, func(t *testing.T) {
				for _, r := range trace.Records {
					b := r.Before
					p := b.output(t)
					header := b.header(t)
					q := GraphicsQueue{Used: b.QueueUsed}
					snow := SnowControlState{Visibility: b.Snow.Visibility, Cursor: b.Snow.Cursor}
					chr := CHRAnimation{Pace: b.Snow.Pace, Timer: 73, Phase: 2}
					var result PaletteResult
					var err error
					switch r.Routine {
					case "load":
						result, err = p.Load(r.InputA, b.Context.SceneType, header, resources)
					case "cycle":
						result, err = p.Step(PaletteCycleContext{GameRoutine: b.Context.GameRoutine, Counter: b.Context.Counter, Level: b.Context.Level, SceneType: b.Context.SceneType, Alternate: b.Context.Alternate, Header: &header, Resources: resources, Queue: &q})
					case "commit":
						var committed bool
						committed, err = p.Commit(q.Used)
						if committed != (b.State.RequestedColors != 0 && q.Used < 0x47) {
							t.Fatal("wrong palette commit admission")
						}
					case "snow":
						_, err = snow.Advance(b.Context.Level, b.Context.Screen, b.Context.Scroll, &chr)
					default:
						t.Fatal("unknown palette oracle", r.Routine)
					}
					bossAppend := b.Context.Level == 1 && b.Context.SceneType&128 != 0 && b.State.RequestedColors < b.Context.GameRoutine && b.Context.Counter&7 == 5 && b.State.PauseCycle == 0
					if r.Routine == "cycle" && (b.QueueUsed > 192 || bossAppend && b.QueueUsed > 183) {
						// These explicit original-RAM probes overlap palette/CHR
						// storage. A semantic queue must reject that invalid domain.
						if err == nil || p != b.output(t) || header != b.header(t) || q.Used != b.QueueUsed || len(q.Packets) != 0 {
							t.Fatal("overlapping queue was not rejected atomically")
						}
						domainRejections++
						continue
					}
					if err != nil {
						t.Fatalf("frame%d %s: %v", r.Frame, r.Routine, err)
					}
					if p != r.After.output(t) {
						t.Fatalf("frame%d %s output got%+v want%+v", r.Frame, r.Routine, p, r.After.output(t))
					}
					if header != r.After.header(t) {
						t.Fatalf("frame%d %s header got%+v want%+v", r.Frame, r.Routine, header, r.After.header(t))
					}
					if snow.Visibility != r.After.Snow.Visibility || snow.Cursor != r.After.Snow.Cursor || chr.Pace != r.After.Snow.Pace || chr.Timer != 73 || chr.Phase != 2 {
						t.Fatalf("snow/CHR ownership differs: %+v %+v", snow, chr)
					}
					oldQueue := paletteHex(t, b.Queue, int(b.QueueUsed))
					newQueue := paletteHex(t, r.After.Queue, int(r.After.QueueUsed))
					if !bytes.HasPrefix(newQueue, oldQueue) || q.Used != r.After.QueueUsed {
						t.Fatal("queue prefix/budget changed")
					}
					var appended []byte
					for _, packet := range q.Packets {
						encoded, e := packet.Encode()
						if e != nil {
							t.Fatal(e)
						}
						appended = append(appended, encoded...)
					}
					if !bytes.Equal(appended, newQueue[len(oldQueue):]) {
						t.Fatalf("wrong palette attribute packet: %x want %x", appended, newQueue[len(oldQueue):])
					}
					changed := len(r.GenerationWrites) != 0
					value := b.GenerationBias
					if result.GenerationBiasChanged != changed {
						t.Fatal("palette generation-write event differs", r.Routine, result, changed)
					}
					if changed {
						value = result.GenerationBiasValue
						if value != r.GenerationWrites[len(r.GenerationWrites)-1].Value {
							t.Fatal("wrong palette pointer low byte")
						}
					}
					if value != r.After.GenerationBias {
						t.Fatal("palette compatibility value changed incorrectly")
					}
					counts[r.Routine]++
				}
			})
		}
	}
	want := map[string]int{"load": 309, "cycle": 2107, "commit": 2060, "snow": 2058}
	if !reflect.DeepEqual(counts, want) {
		t.Fatal("palette coverage", counts, want)
	}
	if domainRejections != 3 {
		t.Fatal("overlap rejection coverage", domainRejections)
	}
	t.Logf("verified complete original phase counts: %v", counts)
}

func TestPaletteFailureIsAtomic(t *testing.T) {
	_, _, resources := paletteFixture(t)
	info, e := resources.Stage(0)
	if e != nil {
		t.Fatal(e)
	}
	header := info.Initial
	p := PaletteOutput{FadeTimer: 5, Cycle: 1, FlashTimer: 9, RequestedColors: 0}
	p.Desired[4] = 17
	p.Applied[4] = 33
	before := p
	if _, e = p.Load(12, 0, header, resources); e == nil || p != before {
		t.Fatal("unsupported load count partially committed")
	}
	header.PaletteIndices[0] = 255
	if _, e = p.Load(32, 0, header, resources); e == nil || p != before {
		t.Fatal("missing color partially committed")
	}
	q := GraphicsQueue{Used: 247}
	header = info.Initial
	oldHeader := header
	if _, e = p.Step(PaletteCycleContext{GameRoutine: 5, Counter: 5, Level: 1, SceneType: 128, Header: &header, Resources: resources, Queue: &q}); e == nil || p != before || header != oldHeader || q.Used != 247 || len(q.Packets) != 0 {
		t.Fatal("Boss queue overflow partially committed")
	}
	p.RequestedColors = 12
	before = p
	if _, e = p.Commit(0); e == nil || p != before {
		t.Fatal("invalid commit partially changed palette")
	}
	s := SnowControlState{Visibility: 1, Cursor: 13}
	old := s
	c := CHRAnimation{Pace: 7, Timer: 4, Phase: 3}
	oldC := c
	if _, e = s.Advance(4, 20, 0, &c); e == nil || s != old || c != oldC {
		t.Fatal("invalid snow cursor partially committed")
	}
}
