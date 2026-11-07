package engine

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func queueMergeTailState(pending uint8) ChuTailState {
	return ChuTailState{ObjectID: 0x8400, PendingSound: pending,
		SoundQueue: [4]uint8{0x81, 0x82, 0x83, 0x84}, RandomSeed: 17}
}

func queueMergeFrames() AnimationLibrary {
	return AnimationLibrary{AnimationFrameKey{}: AnimationRecordData{Duration: 1}}
}

func queueMergeActorState(pending uint8) ChuActorState {
	return ChuActorState{Fighter: ChuActorFighter{ObjectID: 0x8400, Size: 256, HitStopTicks: 1},
		World: ChuActorWorld{PendingSound: pending, SoundQueue: queueMergeTailState(0).SoundQueue,
			Registration: FighterRegistrationState{Owners: make([]uint16, 64)}, HUDCells: make([]uint16, 512), RandomSeed: 17}}
}

func queueMergeLayer(t *testing.T, layer string, pending uint8, port *worldSoundPort) ([4]uint8, uint8, error) {
	t.Helper()
	switch layer {
	case "tail":
		next, _, _, err := stepFighterActionTail(queueMergeTailState(pending), 9, 0, 0, 0, ChuTailParameters{}, queueMergeFrames(), ChuTailHitStop, port)
		return next.SoundQueue, next.PendingSound, err
	case "action":
		next, err := stepFighterActionWithSound(ChuActionState{ChuTailState: queueMergeTailState(pending), HitStopTicks: 1}, 9, ChuActionParameters{}, actionCommandLibrary(t), queueMergeFrames(), port)
		return next.SoundQueue, next.PendingSound, err
	case "actor":
		next, _, err := stepFighterActorWithSound(queueMergeActorState(pending), 9, ChuActorView{Disabled: 1}, actionCommandLibrary(t), queueMergeFrames(), port)
		return next.World.SoundQueue, next.World.PendingSound, err
	default:
		t.Fatal("unknown queue merge layer", layer)
		return [4]uint8{}, 0, nil
	}
}

func TestWorldQueueMergeNoEnqueueAndNilPort(t *testing.T) {
	stale := [4]uint8{0x81, 0x82, 0x83, 0x84}
	canonical := [4]uint8{0x91, 0, 0x93, 0}
	pause := uint8(255)
	port := &worldSoundPort{owners: WorldSoundOwners{Queue: &canonical, AudioPause: &pause}}
	if got := worldSoundQueueView(stale, nil); got != stale {
		t.Fatal("nil-port queue changed", got)
	}
	if got := worldSoundQueueView(stale, port); got != canonical {
		t.Fatal("copied queue replaced canonical owner", got)
	}
	for _, layer := range []string{"tail", "action", "actor"} {
		t.Run(layer, func(t *testing.T) {
			queue, pending, err := queueMergeLayer(t, layer, 0, port)
			if err != nil || queue != canonical || pending != 0 || pause != 255 {
				t.Fatal("no-enqueue component restored stale queue", layer, queue, canonical, pending, err)
			}
			pure, pending, err := queueMergeLayer(t, layer, 0, nil)
			if err != nil || pure != stale || pending != 0 {
				t.Fatal("pure no-enqueue path changed", layer, pure, pending, err)
			}
		})
	}
}

func TestWorldQueueMergeRealProducerAndConsumer(t *testing.T) {
	for _, layer := range []string{"tail", "action", "actor"} {
		t.Run(layer, func(t *testing.T) {
			queue, pause := [4]uint8{}, uint8(0)
			stores, consumed := 0, false
			port := &worldSoundPort{owners: WorldSoundOwners{Queue: &queue, AudioPause: &pause}, objectID: 0x8400}
			port.hook = func(op WorldSoundOperation, owners WorldSoundOwners, apply func() error) error {
				if owners.Queue != &queue || op.ObjectID != 0x8400 || op.Producer != "animation" {
					t.Fatal("producer replaced owner identity", op)
				}
				if err := apply(); err != nil {
					return err
				}
				if op.Request.Kind == RequestStore {
					stores++
					if queue != [4]uint8{0x35, 0, 0, 0} {
						t.Fatal("real producer was dropped or used copied full queue", queue)
					}
				}
				if op.Request.Kind == RequestReturn {
					consumer := ConsumeSoundQueue(SoundPriorityState{Queue: queue}, SoundPriorityTable{})
					if consumer.SelectedSound != 0x35 {
						t.Fatal("consumer missed canonical producer", consumer)
					}
					queue, pause, consumed = consumer.Queue, 255, true
				}
				return nil
			}
			got, pending, err := queueMergeLayer(t, layer, 0x35, port)
			if err != nil || port.err != nil || stores != 1 || !consumed || queue != [4]uint8{} || got != queue || pending != 0 || pause != 255 {
				t.Fatal("nested return restored consumed request", layer, got, queue, stores, consumed, pending, err, port.err)
			}
			// Historical pure-value enqueue semantics remain available. A full
			// negative queue rejects this different code without losing pending.
			pure, pending, err := queueMergeLayer(t, layer, 0x35, nil)
			if err != nil || pure != queueMergeTailState(0).SoundQueue || pending != 0 {
				t.Fatal("pure producer path changed", layer, pure, pending, err)
			}
		})
	}
}

func TestWorldQueueMergeAfterLastEnqueueBeforeOuterReturn(t *testing.T) {
	queue, pause := [4]uint8{}, uint8(0)
	port := &worldSoundPort{owners: WorldSoundOwners{Queue: &queue, AudioPause: &pause}, objectID: 0x8400}
	action, err := stepFighterActionWithSound(ChuActionState{ChuTailState: queueMergeTailState(0x35), HitStopTicks: 1}, 9, ChuActionParameters{}, actionCommandLibrary(t), queueMergeFrames(), port)
	if err != nil || queue != [4]uint8{0x35, 0, 0, 0} || action.SoundQueue != queue {
		t.Fatal("missing nested submission producer", queue, action.SoundQueue, err)
	}
	// A later completed consumer occurs after the entire leaf/action return,
	// before the outer actor merges it. No enqueue follows this consumption.
	consumer := ConsumeSoundQueue(SoundPriorityState{Queue: queue}, SoundPriorityTable{})
	queue = consumer.Queue
	actor := queueMergeActorState(0)
	actor.acceptActorAction(action, port)
	if actor.World.SoundQueue != queue || queue != [4]uint8{} || consumer.SelectedSound != 0x35 {
		t.Fatal("outer return resurrected the last consumed enqueue", actor.World.SoundQueue, queue, consumer)
	}
	pure := queueMergeActorState(0)
	pure.acceptActorAction(action, nil)
	if pure.World.SoundQueue != action.SoundQueue {
		t.Fatal("nil-port actor merge discarded value queue")
	}
}

func TestWorldQueueMergePausedChildWorlds(t *testing.T) {
	for _, kind := range []string{"dragon", "boost", "fighter5", "first", "second"} {
		t.Run(kind, func(t *testing.T) {
			queue, pause := [4]uint8{0, 0x92, 0, 0x94}, uint8(255)
			world := ChuWorldState{Objects: WorldObjects{}, Clock: WorldClockState{PauseFlags: 128}, SoundQueue: queueMergeTailState(0).SoundQueue}
			port := &worldSoundPort{owners: WorldSoundOwners{Queue: &queue, AudioPause: &pause}}
			object := WorldObject{ObjectID: 0x8500, Header: MatchObjectHeader{Stage: 2, Size: 256}}
			var err error
			switch kind {
			case "dragon":
				object.Header.Behavior = 0x13bb6
				_, err = stepFighter4WorldDragonHead(&world, object, Fighter4DragonLibrary{}, port)
			case "boost":
				object.Header.Behavior = 0x13fc8
				_, err = stepFighter4WorldBoost(&world, object, nil, port)
			case "fighter5":
				object.Header.Behavior, object.DragonBank = 0x1414e, 0x8400
				_, err = stepFighter5WorldProjectile(&world, object, nil, port)
			case "first":
				object.Header.Behavior = 0x136e6
				_, err = stepFighter7WorldFirstChild(&world, object, nil, port)
			case "second":
				object.ObjectID, object.Header.Behavior = 0x8600, 0x1391e
				_, err = stepFighter7WorldSecondChild(&world, object, nil, port)
			}
			if err != nil || world.SoundQueue != queue || pause != 255 {
				t.Fatal("paused/no-enqueue world child overwrote canonical queue", kind, world.SoundQueue, queue, err)
			}
		})
	}
}

func TestWorldQueueMergeFailuresRemainTransactional(t *testing.T) {
	for _, failure := range []string{"hook", "late-object", "late-palette"} {
		t.Run(failure, func(t *testing.T) {
			world := ChuWorldState{Objects: WorldObjects{0x8400: {ObjectID: 0x8400, Tag: 9, HitStopTicks: 1,
				Header: MatchObjectHeader{Behavior: 0x12668, Stage: 2, Size: 256}}}, PendingSound: 0x35,
				Registration: FighterRegistrationState{Owners: make([]uint16, 64)}}
			world.Match.Disabled = 1
			library := ChuWorldLibrary{Fighters: FighterSpecLibrary{9: {CharacterID: 9}}, Commands: actionCommandLibrary(t), Animations: queueMergeFrames()}
			if failure == "late-object" {
				world.Objects[0x9fc0] = WorldObject{ObjectID: 0x9fc0, Header: MatchObjectHeader{Behavior: 0xdeadbeef, Size: 64}}
			}
			if failure == "late-palette" {
				world.Palette.Flags, world.Palette.Status, world.Palette.Mode = 1, 128, 0xffff
			}
			before, _ := json.Marshal(world)
			sentinel := errors.New("queue merge hook failure")
			calls := 0
			var lease func() error
			got, events, _, err := StepChuSessionFrameObserved(world, [8]Buttons{}, library, func(op WorldSoundOperation, owners WorldSoundOwners, apply func() error) error {
				calls++
				lease = apply
				if err := apply(); err != nil {
					return err
				}
				if failure == "hook" {
					return sentinel
				}
				if op.Request.Kind == RequestReturn {
					consumer := ConsumeSoundQueue(SoundPriorityState{Queue: *owners.Queue}, SoundPriorityTable{})
					*owners.Queue, *owners.AudioPause = consumer.Queue, 255
				}
				return nil
			})
			after, _ := json.Marshal(world)
			if calls == 0 || err == nil || events != nil || !reflect.DeepEqual(got, world) || string(before) != string(after) {
				t.Fatal("failed nested world leaked owners", failure, calls, err)
			}
			if failure == "hook" && (!errors.Is(err, sentinel) || calls != 1) {
				t.Fatal("failed hook continued", calls, err)
			}
			if lease == nil || !errors.Is(lease(), ErrSoundScheduleSequence) {
				t.Fatal("expired producer apply survived failed frame")
			}
		})
	}
}
