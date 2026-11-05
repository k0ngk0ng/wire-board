package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

type fighter4DragonRaw struct {
	fighter3AirRaw
	History    string
	BankCursor uint16 `json:"bank_cursor"`
}

type fighter4DragonCase struct {
	fighter3AirCase
	Before, After  fighter4DragonRaw
	Sequence       int
	SourceRegister string    `json:"source_register"`
	DrawRequest    WorldDraw `json:"draw_request"`
	CurveReads     []struct {
		PC           string
		Address      int
		Index, Value int
	} `json:"curve_reads"`
	RAMChanges []struct{ Offset, Before, After int } `json:"ram_changes"`
}

type fighter4DragonModel struct {
	Base       fighter3AirModel
	History    Fighter4DragonHistory
	Raw        fighter4DragonRaw
	BankCursor uint16
}

func decodeFighter4Dragon(t *testing.T, raw fighter4DragonRaw) fighter4DragonModel {
	t.Helper()
	m := fighter4DragonModel{Base: fighter3AirModel{State: ChuWorldState{Objects: make(WorldObjects)}, Raw: raw.fighter3AirRaw}, Raw: raw, BankCursor: raw.BankCursor}
	for _, region := range []struct {
		Raw               string
		Base, Count, Size int
	}{{raw.Family, int(raw.ObjectID), 4, 256}, {raw.Pool.Bytes, 0x9400, 16, 128}} {
		b := attackContactBytes(t, region.Raw, region.Count*region.Size)
		for i := 0; i < region.Count; i++ {
			id := uint16(region.Base + i*region.Size)
			padded := append(append([]byte(nil), b[i*region.Size:(i+1)*region.Size]...), make([]byte, 256-region.Size)...)
			m.Base.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(padded))
		}
	}
	if m.Base.State.Objects[raw.ObjectID].Tag != 4 {
		t.Fatal("not an original source4 family")
	}
	b := attackContactBytes(t, raw.Buffers, 1024)
	for i := range m.Base.State.Buffers {
		m.Base.State.Buffers[i] = attackContactBuffer(t, raw.Offsets[i], hex.EncodeToString(b[i*256:(i+1)*256]))
	}
	m.Base.State.PendingSound, m.Base.State.SoundQueue, m.Base.State.RandomSeed, m.Base.State.EffectCursor = raw.PendingSound, raw.SoundQueue, raw.RandomSeed, raw.Pool.Cursor
	e := raw.Environment
	m.Base.State.Clock.PauseFlags, m.Base.State.Clock.SlowFlag, m.Base.State.Clock.ScreenShake = e.PauseFlags, e.SlowFlag, uint16(e.Shake)
	m.Base.State.Camera.Left, m.Base.State.Camera.Right, m.Base.State.Camera.X, m.Base.State.Camera.Y = e.ArenaLeft, e.ArenaRight, e.CameraX, e.CameraY
	m.Base.State.Match.AttributeMask = e.PaletteAttributes
	h := attackContactBytes(t, raw.History, 2048)
	for bank := range m.History {
		for i := range m.History[bank] {
			p := h[bank*512+i*2:]
			w := func(n int) uint16 { return binary.BigEndian.Uint16(p[n:]) }
			m.History[bank][i] = Fighter4DragonSample{X: int16(w(0)), Y: int16(w(0x40)), Angle: w(0x80), Depth: w(0xc0), Layer: p[0x100], SkipGraphics: p[0x101], Status: p[0x140]}
		}
	}
	return m
}

func (m *fighter4DragonModel) encode(t *testing.T, cleared map[uint16]uint16) fighter4DragonRaw {
	t.Helper()
	raw := m.Raw
	raw.fighter3AirRaw = m.Base.encode(t, cleared)
	raw.BankCursor = m.BankCursor
	p := attackContactBytes(t, m.Raw.Pool.Bytes, 2048)
	for i := 0; i < 16; i++ {
		id := uint16(0x9400 + i*128)
		previous := append(append([]byte(nil), p[i*128:(i+1)*128]...), make([]byte, 128)...)
		encoded := encodeWorldObjectTest(t, m.Base.State.Objects[id], hex.EncodeToString(previous), cleared[id])
		copy(p[i*128:], attackContactBytes(t, encoded, 256)[:128])
	}
	raw.Pool.Bytes = hex.EncodeToString(p)
	h := attackContactBytes(t, m.Raw.History, 2048)
	for bank := range m.History {
		for i, s := range m.History[bank] {
			p := h[bank*512+i*2:]
			w := func(n int, v uint16) { binary.BigEndian.PutUint16(p[n:], v) }
			w(0, uint16(s.X))
			w(0x40, uint16(s.Y))
			w(0x80, s.Angle)
			w(0xc0, s.Depth)
			p[0x100], p[0x101], p[0x140] = s.Layer, s.SkipGraphics, s.Status
		}
	}
	raw.History = hex.EncodeToString(h)
	m.Raw, m.Base.Raw = raw, raw.fighter3AirRaw
	return raw
}

func compareFighter4Dragon(t *testing.T, got, want fighter4DragonRaw) {
	t.Helper()
	compareFighter3AirRaw(t, got.fighter3AirRaw, want.fighter3AirRaw)
	if got.BankCursor != want.BankCursor {
		t.Fatal("dragon bank cursor differs", got.BankCursor, want.BankCursor)
	}
	a, b := attackContactBytes(t, got.History, 2048), attackContactBytes(t, want.History, 2048)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("dragon history byte%x got%x want%x", i, a[i], b[i])
		}
	}
}

func fighter4Visual(o WorldObject, s ChuWorldState) Fighter8ProjectileState {
	return Fighter8ProjectileState{Header: o.Header, Animation: o.AsAnimation(), GraphicsSet: o.GraphicsSet, TilesBase: o.TilesBase, GraphicCache: o.GraphicCache,
		Health: o.Body.Health, Shake: o.Shake, ScreenX: o.ScreenX, ScreenY: o.ScreenY, ScreenOffsetX: o.ScreenOffsetX, ScreenOffsetY: o.ScreenOffsetY, Buffers: s.Buffers, PendingSound: s.PendingSound, SoundQueue: s.SoundQueue}
}

func mergeFighter4Visual(t *testing.T, o WorldObject, v Fighter8ProjectileState, clear uint16) WorldObject {
	t.Helper()
	var err error
	o, err = ApplyObjectClearPrefix(o, clear)
	if err != nil {
		t.Fatal(err)
	}
	o, err = o.MergeAnimation(v.Animation)
	if err != nil {
		t.Fatal(err)
	}
	o.Header, o.GraphicsSet, o.TilesBase, o.GraphicCache = v.Header, v.GraphicsSet, v.TilesBase, v.GraphicCache
	o.Body.Health, o.Shake = v.Health, v.Shake
	o.ScreenX, o.ScreenY, o.ScreenOffsetX, o.ScreenOffsetY = v.ScreenX, v.ScreenY, v.ScreenOffsetX, v.ScreenOffsetY
	return o
}

func (m *fighter4DragonModel) step(t *testing.T, row fighter4DragonCase, library Fighter4DragonLibrary) ChuProjectileOutcome {
	t.Helper()
	s := &m.Base.State
	id := row.ObjectID
	o := s.Objects[id]
	e := m.Raw.Environment
	env := ChuAttachmentWorld{PauseFlags: s.Clock.PauseFlags, SlowFlag: s.Clock.SlowFlag, AttributeMask: s.Match.AttributeMask, CameraX: s.Camera.X, CameraY: s.Camera.Y, Shake: int16(s.Clock.ScreenShake)}
	var out ChuProjectileOutcome
	var err error
	if row.Kind == "clock" {
		o.Body.Clock = StepAnimationClock(o.Body.Clock)
		s.Objects[id] = o
		return out
	}
	if row.Kind == "record" {
		v := fighter4Visual(o, *s)
		sub, er := SubmitAnimationRecord(AnimationSubmissionState{Actor: v.Animation, Buffers: v.Buffers, PendingSound: v.PendingSound, SoundQueue: v.SoundQueue}, library.Animations)
		if er != nil {
			t.Fatal(er)
		}
		v.Animation, v.Buffers, v.PendingSound, v.SoundQueue = sub.Actor, sub.Buffers, sub.PendingSound, sub.SoundQueue
		o = mergeFighter4Visual(t, o, v, 0)
		s.Objects[id] = o
		s.Buffers, s.PendingSound, s.SoundQueue = v.Buffers, v.PendingSound, v.SoundQueue
		return out
	}
	pool, er := BuildWorldEffectPool(s.Objects, s.EffectCursor)
	if er != nil {
		t.Fatal(er)
	}
	if row.Kind == "creator_head" || row.Kind == "creator_boost" {
		v := Fighter4MotionState{ChuSpecialMotionState: ChuSpecialMotionState{ObjectID: id, Body: o.Body, Controls: o.Controls, Energy: o.Energy, Power: o.Power, AlternateVelocity: o.Aux48, ContactFlags: o.ContactFlags, TargetBit: o.TargetBit, Pool: pool}, MovementSource: o.MovementSource, DragonBankCursor: m.BankCursor}
		for i := range v.Children {
			c := s.Objects[id+uint16(i+1)*256]
			v.Children[i] = c.AsChuChild()
			v.ChildExtras[i] = Fighter4ChildExtra{Direction: c.Aux48, Held: uint8(c.Controls.Held), Pressed: uint8(c.Controls.Pressed), ContactFlags: c.ContactFlags, DragonBank: c.DragonBank}
		}
		for i := range v.EffectExtras {
			v.EffectExtras[i] = s.Objects[0x9400+uint16(i)*128].AsReactionExtra()
		}
		v, err = StepFighter4Motion(v, m.Raw.Parameters)
		if err != nil {
			t.Fatal(err)
		}
		o.Body, o.Controls, o.Energy, o.Power = v.Body, v.Controls, v.Energy, v.Power
		o.Aux48, o.ContactFlags, o.TargetBit, o.MovementSource = v.AlternateVelocity, v.ContactFlags, v.TargetBit, v.MovementSource
		s.Objects[id] = o
		for i, c := range v.Children {
			cid := id + uint16(i+1)*256
			co := s.Objects[cid].MergeChuChild(c)
			x := v.ChildExtras[i]
			co.Aux48, co.ContactFlags, co.DragonBank = x.Direction, x.ContactFlags, x.DragonBank
			co.Controls.Held = co.Controls.Held&0xff00 | Buttons(x.Held)
			co.Controls.Pressed = co.Controls.Pressed&0xff00 | Buttons(x.Pressed)
			s.Objects[cid] = co
		}
		s.Objects, err = MergeWorldEffectPool(s.Objects, v.Pool)
		if err != nil {
			t.Fatal(err)
		}
		for i, x := range v.EffectExtras {
			cid := uint16(0x9400 + i*128)
			s.Objects[cid] = s.Objects[cid].MergeReactionExtra(x)
		}
		s.EffectCursor, m.BankCursor = v.Pool.Cursor, v.DragonBankCursor
		return out
	}
	switch row.Kind {
	case "head":
		v := Fighter4DragonHeadState{Visual: fighter4Visual(o, *s), Direction: o.Aux48, EffectiveAngle: o.Body.VelocityX, WavePhase: uint16(o.Body.Gravity >> 16), Age: uint16(o.Body.VerticalVelocity >> 16), SoundClock: uint16(o.Energy >> 16), DragonBank: o.DragonBank, Ground: int16(o.Body.Ground >> 16), Depth: o.Body.Depth, Layer: o.Body.Layer, SkipGraphics: o.SkipGraphics, Held: uint8(o.Controls.Held), ContactFlags: o.ContactFlags, Pool: pool}
		for i := range v.EffectExtras {
			p := s.Objects[0x9400+uint16(i)*128]
			v.EffectExtras[i] = Fighter4DragonTrailExtra{Owner: uint16(p.Aux48 >> 16), Bank: uint16(p.Body.Gravity >> 16)}
		}
		e.PauseFlags, e.SlowFlag = s.Clock.PauseFlags, s.Clock.SlowFlag
		v, m.History, out, err = StepFighter4DragonHead(v, m.History, e, library)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		s.Objects, err = MergeWorldEffectPool(s.Objects, v.Pool)
		if err != nil {
			t.Fatal(err)
		}
		for i, x := range v.EffectExtras {
			cid := uint16(0x9400 + i*128)
			p := s.Objects[cid]
			p.Aux48 = replaceChuSpecialHighWord(p.Aux48, x.Owner)
			p.Body.Gravity = replaceChuSpecialHighWord(p.Body.Gravity, x.Bank)
			s.Objects[cid] = p
		}
		o = mergeFighter4Visual(t, o, v.Visual, out.ClearedBytes)
		o.Aux48, o.Body.VelocityX, o.DragonBank = v.Direction, v.EffectiveAngle, v.DragonBank
		o.Body.Gravity = replaceChuSpecialHighWord(o.Body.Gravity, v.WavePhase)
		o.Body.VerticalVelocity = replaceChuSpecialHighWord(o.Body.VerticalVelocity, v.Age)
		o.Energy = replaceChuSpecialHighWord(o.Energy, v.SoundClock)
		o.Body.Ground = replaceChuSpecialHighWord(o.Body.Ground, uint16(v.Ground))
		o.Body.Depth, o.Body.Layer, o.SkipGraphics = v.Depth, v.Layer, v.SkipGraphics
		s.Buffers, s.PendingSound, s.SoundQueue, s.EffectCursor = v.Visual.Buffers, v.Visual.PendingSound, v.Visual.SoundQueue, v.Pool.Cursor
	case "trail":
		v := Fighter4DragonTrailState{Header: o.Header, Draw: o.DrawRequest(), WorldX: o.Body.X, WorldY: o.Body.Y, ScreenOffsetX: o.ScreenOffsetX, ScreenOffsetY: o.ScreenOffsetY, Angle: uint16(o.Body.VelocityX >> 16), Age: uint16(o.Body.VerticalVelocity >> 16), Bank: uint16(o.Body.Gravity >> 16)}
		v, out, err = StepFighter4DragonTrail(v, m.History, env)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		o, err = ApplyObjectClearPrefix(o, out.ClearedBytes)
		if err != nil {
			t.Fatal(err)
		}
		o.Header, o.GraphicsSet, o.TilesBase, o.GraphicParameter = v.Header, v.Draw.GraphicsSet, v.Draw.TileBank, v.Draw.Parameter
		o.ScreenX, o.ScreenY = int16(v.Draw.ScreenX), int16(v.Draw.ScreenY)
		o.GraphicOffsetX, o.GraphicOffsetY = v.Draw.OffsetX, v.Draw.OffsetY
		o.Body.Attributes, o.Body.Depth, o.Body.Layer, o.SkipGraphics, o.GraphicCache = v.Draw.Attributes, v.Draw.Depth, v.Draw.Layer, v.Draw.SkipTiles, v.Draw.Cache
		o.Body.X, o.Body.Y, o.ScreenOffsetX, o.ScreenOffsetY = v.WorldX, v.WorldY, v.ScreenOffsetX, v.ScreenOffsetY
		o.Body.VelocityX = replaceChuSpecialHighWord(o.Body.VelocityX, v.Angle)
		o.Body.VerticalVelocity = replaceChuSpecialHighWord(o.Body.VerticalVelocity, v.Age)
		o.Body.Gravity = replaceChuSpecialHighWord(o.Body.Gravity, v.Bank)
	case "boost":
		pid := id - 256
		p := s.Objects[pid]
		v := Fighter4BoostState{Visual: fighter4Visual(o, *s), Depth: o.Body.Depth, Layer: o.Body.Layer}
		parent := Fighter4BoostParent{Body: p.Body, Energy: p.Energy, AttackValue: p.AttackValue, MovementSource: p.MovementSource, StatusDelay: p.StatusDelay, StunTicks: p.StunTicks}
		v, parent, out, err = StepFighter4Boost(v, parent, env, library.Animations)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		p.Body, p.Energy, p.AttackValue, p.MovementSource, p.StatusDelay, p.StunTicks = parent.Body, parent.Energy, parent.AttackValue, parent.MovementSource, parent.StatusDelay, parent.StunTicks
		s.Objects[pid] = p
		o = mergeFighter4Visual(t, o, v.Visual, out.ClearedBytes)
		o.Body.Depth, o.Body.Layer = v.Depth, v.Layer
		s.Buffers, s.PendingSound, s.SoundQueue = v.Visual.Buffers, v.Visual.PendingSound, v.Visual.SoundQueue
	default:
		t.Fatal("unknown dragon callback", row.Kind)
	}
	s.Objects[id] = o
	return out
}

func walkFighter4Dragon(t *testing.T, suffix string, apply func(*json.Decoder, Fighter4DragonLibrary)) int {
	t.Helper()
	name := "testdata/fighter4-projectile-" + suffix
	packed, err := os.ReadFile(name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile(name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass    bool
		Records int
		Match   bool   `json:"frames_match_original_core"`
		SHA     string `json:"fixture_sha256"`
		Gzip    string `json:"gzip_sha256"`
	}
	if json.Unmarshal(mb, &manifest) != nil || !manifest.Pass || !manifest.Match {
		t.Fatal("unverified dragon CPU evidence")
	}
	zh := sha256.Sum256(packed)
	if hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("dragon gzip digest differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	h := sha256.New()
	tee := io.TeeReader(z, h)
	d := json.NewDecoder(tee)
	if tok, e := d.Token(); e != nil || tok != json.Delim('{') {
		t.Fatal(e)
	}
	if tok, _ := d.Token(); tok != "library" {
		t.Fatal("missing dragon library")
	}
	var frames []struct {
		Key  AnimationFrameKey
		Data AnimationRecordData
	}
	if err = d.Decode(&frames); err != nil {
		t.Fatal(err)
	}
	lib := Fighter4DragonLibrary{Animations: make(AnimationLibrary)}
	for _, f := range frames {
		if _, ok := lib.Animations[f.Key]; ok {
			t.Fatal("duplicate dragon AN")
		}
		lib.Animations[f.Key] = f.Data
	}
	if tok, _ := d.Token(); tok != "curve" {
		t.Fatal("missing word curve")
	}
	if err = d.Decode(&lib.Curve); err != nil {
		t.Fatal(err)
	}
	if lib.Curve.FirstIndex != -32 || len(lib.Curve.Values) != 1344 {
		t.Fatal("wrong curve address range")
	}
	if tok, _ := d.Token(); tok != "calls" {
		t.Fatal("missing dragon calls")
	}
	if tok, _ := d.Token(); tok != json.Delim('[') {
		t.Fatal("missing dragon array")
	}
	count := 0
	for d.More() {
		apply(d, lib)
		count++
	}
	d.Token()
	d.Token()
	if _, err = io.Copy(io.Discard, tee); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Records || hex.EncodeToString(h.Sum(nil)) != manifest.SHA {
		t.Fatal("dragon fixture digest/count differs")
	}
	return count
}

func auditFighter4Dragon(t *testing.T, row fighter4DragonCase) {
	t.Helper()
	if row.RNGCalls != 0 || len(row.Unexplained) != 0 {
		t.Fatal("unexplained dragon CPU mutation")
	}
	if row.SourceRegister != fmt.Sprintf("0xffff%04x", row.ObjectID) {
		t.Fatal("noncanonical dragon A5", row.SourceRegister)
	}
	for _, c := range row.ExternalChanges {
		value, found := c.Before, false
		for _, irq := range row.Interrupts {
			if irq.Entry != "0xfff000" || (irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a") {
				t.Fatal("unrecognized IRQ")
			}
			for _, delta := range irq.Changes {
				if delta.Offset == c.Offset {
					if value != delta.Before {
						t.Fatal("IRQ delta chain mismatch")
					}
					value, found = delta.After, true
				}
			}
		}
		if !found || value != c.After {
			t.Fatal("unattributed global write", c)
		}
	}
	for _, r := range row.CurveReads {
		if r.Index < -32 || r.Index > 1311 || r.Address != 0x34000+r.Index*2 {
			t.Fatal("CPU curve address out of frozen range")
		}
	}
}

func TestFighter4DragonOriginalCallbacks(t *testing.T) {
	for _, suffix := range []string{"boundaries", "oracle"} {
		t.Run(suffix, func(t *testing.T) {
			seen := map[string]int{}
			count := walkFighter4Dragon(t, suffix, func(d *json.Decoder, lib Fighter4DragonLibrary) {
				var row fighter4DragonCase
				if err := d.Decode(&row); err != nil {
					t.Fatal(err)
				}
				auditFighter4Dragon(t, row)
				t.Run(row.Name, func(t *testing.T) {
					m := decodeFighter4Dragon(t, row.Before)
					out := m.step(t, row, lib)
					if out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes {
						t.Fatal("dragon output differs", out, row.Draw, row.ClearedBytes)
					}
					compareFighter4Dragon(t, m.encode(t, map[uint16]uint16{row.ObjectID: out.ClearedBytes}), row.After)
					if out.Draw && m.Base.State.Objects[row.ObjectID].DrawRequest() != row.DrawRequest {
						t.Fatal("callback-time WorldDraw differs")
					}
				})
				seen[row.Kind]++
			})
			if suffix == "boundaries" && count != 2607 {
				t.Fatal("missing dragon dirty boundary coverage", count, seen)
			}
			t.Logf("full original callbacks=%d kinds=%v", count, seen)
		})
	}
}

func TestFighter4DragonCreatorClockLifecycles(t *testing.T) {
	steps, draws, heads, trails, boosts := 0, 0, 0, 0, 0
	count := walkFighter4Dragon(t, "sequences", func(d *json.Decoder, lib Fighter4DragonLibrary) {
		var flow struct {
			Name    string
			Initial fighter4DragonRaw
			Calls   []fighter4DragonCase
		}
		if err := d.Decode(&flow); err != nil || len(flow.Calls) < 3 {
			t.Fatal("missing single-initial-state dragon flow", err)
		}
		t.Run(flow.Name, func(t *testing.T) {
			m := decodeFighter4Dragon(t, flow.Initial)
			var preserved []WorldDraw
			for _, row := range flow.Calls {
				auditFighter4Dragon(t, row)
				for key, value := range row.External {
					if key == "pause_flags" {
						m.Base.State.Clock.PauseFlags = value
						m.Raw.Environment.PauseFlags = value
					} else if key == "contact_flags" {
						o := m.Base.State.Objects[row.ObjectID]
						o.ContactFlags = value
						m.Base.State.Objects[row.ObjectID] = o
					} else {
						t.Fatal("undeclared dragon flow input", key)
					}
				}
				compareFighter4Dragon(t, m.encode(t, nil), row.Before)
				out := m.step(t, row, lib)
				if out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes {
					t.Fatal("flow boundary differs", row.Name)
				}
				compareFighter4Dragon(t, m.encode(t, map[uint16]uint16{row.ObjectID: out.ClearedBytes}), row.After)
				if row.Draw {
					got := m.Base.State.Objects[row.ObjectID].DrawRequest()
					if got != row.DrawRequest {
						t.Fatal("flow draw differs", row.Name)
					}
					preserved = append(preserved, got)
					draws++
				}
				switch row.Kind {
				case "head":
					heads++
				case "trail":
					trails++
				case "boost":
					boosts++
				}
				steps++
			}
			for _, draw := range preserved {
				if draw.ObjectID == 0 {
					t.Fatal("prior draw mutated after slot clear")
				}
			}
			if m.Base.State.Objects[0x8500].Header.Size != 0 {
				t.Fatal("child did not clear")
			}
			for id, o := range m.Base.State.Objects {
				if id >= 0x9400 && o.Header.Behavior == 0x13e9a && o.Header.Size != 0 {
					t.Fatal("body segment not independently cleared")
				}
			}
		})
	})
	if count != 16 || heads == 0 || trails == 0 || boosts == 0 || draws == 0 {
		t.Fatal("missing creator/clock/head/body/boost flow", count, steps, heads, trails, boosts)
	}
	t.Logf("flows=%d calls=%d draws=%d head=%d body=%d boost=%d", count, steps, draws, heads, trails, boosts)
}

func TestFighter4DragonSemanticData(t *testing.T) {
	raw, err := os.ReadFile("testdata/fighter4-projectile-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Animations []struct {
			Key  AnimationFrameKey
			Data AnimationRecordData
		}
		Curve Fighter4DragonCurve
	}
	if json.Unmarshal(raw, &data) != nil || len(data.Animations) != 9 || data.Curve.FirstIndex != -32 || len(data.Curve.Values) != 1344 {
		t.Fatal("missing immutable dragon semantic data")
	}
	seen := map[AnimationFrameKey]AnimationRecordData{}
	for _, f := range data.Animations {
		seen[f.Key] = f.Data
	}
	called := false
	walkFighter4Dragon(t, "boundaries", func(d *json.Decoder, lib Fighter4DragonLibrary) {
		var discard json.RawMessage
		if err := d.Decode(&discard); err != nil {
			t.Fatal(err)
		}
		if called {
			return
		}
		called = true
		if !reflect.DeepEqual(lib.Curve, data.Curve) {
			t.Fatal("CPU word curve differs")
		}
		for key, value := range seen {
			if !reflect.DeepEqual(lib.Animations[key], value) {
				t.Fatal("semantic AN differs", key)
			}
		}
	})
}

func TestFighter4DragonAtomicErrors(t *testing.T) {
	visual := Fighter8ProjectileState{Header: MatchObjectHeader{Behavior: 0x13bb6, Size: 256}, Animation: AnimationActorState{ObjectID: 0x8500, WorldX: 100 << 16, WorldY: 200 << 16}, Health: -1, Shake: 5}
	head := Fighter4DragonHeadState{Visual: visual, Direction: 0x100ffff, Ground: 320, DragonBank: 0, Age: 3, Pool: EffectPool{Cursor: 0}}
	history := Fighter4DragonHistory{}
	history[2][31] = Fighter4DragonSample{X: -1, Layer: 255, Status: 2}
	env := ChuProjectileEnvironment{ArenaRight: 600}
	curve := Fighter4DragonCurve{FirstIndex: -32, Values: make([]int16, 1344)}
	for _, mutate := range []func(*Fighter4DragonHeadState, *Fighter4DragonLibrary){func(s *Fighter4DragonHeadState, l *Fighter4DragonLibrary) { l.Curve.Values = nil }, func(s *Fighter4DragonHeadState, l *Fighter4DragonLibrary) { s.DragonBank = 2 }, func(s *Fighter4DragonHeadState, l *Fighter4DragonLibrary) { s.Pool.Cursor = 1 }, func(s *Fighter4DragonHeadState, l *Fighter4DragonLibrary) {}, func(s *Fighter4DragonHeadState, l *Fighter4DragonLibrary) {
		s.Visual.Header.Size = 1
		s.ContactFlags = 16
	}, func(s *Fighter4DragonHeadState, l *Fighter4DragonLibrary) {
		l.Curve = Fighter4DragonCurve{FirstIndex: 100, Values: []int16{1}}
	}, func(s *Fighter4DragonHeadState, l *Fighter4DragonLibrary) {
		l.Animations = AnimationLibrary{AnimationFrameKey{AnimationSet: 0xd92ca, ShapeSet: 0xd9a3a, Pose: 16, Index: 0}: AnimationRecordData{ShapeFlags: 0x40}}
	}} {
		s, lib := head, Fighter4DragonLibrary{Curve: curve}
		mutate(&s, &lib)
		got, h, out, err := StepFighter4DragonHead(s, history, env, lib)
		if err == nil || !reflect.DeepEqual(got, s) || h != history || out != (ChuProjectileOutcome{}) {
			t.Fatal("head error leaked state/history/pool", err)
		}
	}
	parent := Fighter4BoostParent{Body: FighterBodyState{Health: 1, X: 300 << 16, Y: 280 << 16}, Energy: 100<<16 | 0x1234, AttackValue: 0x83, MovementSource: 0x12578}
	boost := Fighter4BoostState{Visual: visual}
	got, p, out, err := StepFighter4Boost(boost, parent, ChuAttachmentWorld{}, nil)
	if err == nil || !reflect.DeepEqual(got, boost) || p != parent || out != (ChuProjectileOutcome{}) {
		t.Fatal("late AN failure leaked boosted parent or child", err)
	}
	broken, parentNegative := boost, parent
	broken.Visual.Header.Size = 1
	parentNegative.Energy = -1
	got, p, out, err = StepFighter4Boost(broken, parentNegative, ChuAttachmentWorld{}, nil)
	if err == nil || !reflect.DeepEqual(got, broken) || p != parentNegative || out != (ChuProjectileOutcome{}) {
		t.Fatal("late clear error leaked full parent constructor", err)
	}
	trail := Fighter4DragonTrailState{Header: MatchObjectHeader{Behavior: 0x13e9a, Size: 128}, Draw: WorldDraw{ObjectID: 0x9480}, Bank: 2}
	g, out, err := StepFighter4DragonTrail(trail, history, ChuAttachmentWorld{})
	if err == nil || g != trail || out != (ChuProjectileOutcome{}) {
		t.Fatal("misaligned trail bank was silently accepted")
	}
	trail.Bank = 0
	trail.Header.Size = 256
	badHistory := history
	badHistory[0][1].Status = 1
	g, out, err = StepFighter4DragonTrail(trail, badHistory, ChuAttachmentWorld{})
	if err == nil || g != trail || out != (ChuProjectileOutcome{}) {
		t.Fatal("trail clear escaped its128byte slot")
	}
	trail.Header.Size = 128
	for _, role := range []string{"head", "boost", "trail"} {
		if role == "trail" {
			trail.Bank = 0
			g, out, err = StepFighter4DragonTrail(trail, history, ChuAttachmentWorld{PauseFlags: 128})
			if err != nil || !out.Draw || g.Header.Stage != 2 {
				t.Fatal("paused trail initialized incorrectly")
			}
		} else if role == "head" {
			s := head
			s.DragonBank = 2
			s.Pool.Cursor = 1
			got, _, out, err := StepFighter4DragonHead(s, history, ChuProjectileEnvironment{PauseFlags: 128}, Fighter4DragonLibrary{})
			if err != nil || !out.Draw || got.Visual.Health != 255 || got.Visual.Header.Stage != 2 {
				t.Fatal("paused head incorrectly evaluates unused data")
			}
		} else {
			got, p, out, err := StepFighter4Boost(boost, parent, ChuAttachmentWorld{SlowFlag: 255}, nil)
			if err != nil || !out.Draw || p != parent || got.Visual.Health != 255 || got.Visual.Header.Stage != 2 {
				t.Fatal("paused boost touched parent")
			}
		}
	}
}
