package engine

import (
	"errors"
	"testing"
)

func TestSoundQueueObservedUsesCanonicalOwnersAndLegacyOriginal(t *testing.T) {
	var rows []soundQueueOriginalCall
	table, count := readSoundQueueFixture(t, "sound-queue-boundaries", &rows)
	if count != len(rows) {
		t.Fatal("complete original queue scope missing")
	}
	for _, row := range rows {
		before := decodeSoundPriorityState(t, row.Before)
		actual := before
		err := ConsumeSoundQueueObserved(SoundQueueOwners{Priority: &actual.CurrentPriority, Selected: &actual.SelectedSound, Queue: &actual.Queue}, table, func(op SoundQueueOperation, apply func() error) error { return apply() })
		if err != nil || actual != row.After.SoundPriorityState || actual != ConsumeSoundQueue(before, table) {
			t.Fatal("observed consumer changed original queue semantics", row.Name, err)
		}
	}
}

func TestSoundQueueObservedLiveFieldReadAndEffectLease(t *testing.T) {
	var table SoundPriorityTable
	table[5], table[6] = 80, 81
	actual := SoundPriorityState{SelectedSound: 255, Queue: [4]uint8{0, 0, 0, 6}}
	reads := []int8{}
	err := ConsumeSoundQueueObserved(SoundQueueOwners{Priority: &actual.CurrentPriority, Selected: &actual.SelectedSound, Queue: &actual.Queue}, table, func(op SoundQueueOperation, apply func() error) error {
		if op.Kind == QueueReadSlot {
			reads = append(reads, op.Slot)
			if op.Slot == 2 {
				actual.Queue[0] = 7
			}
		}
		return apply()
	})
	if err != nil || actual.SelectedSound != 7 || actual.CurrentPriority != 81 || actual.Queue != ([4]uint8{}) || len(reads) != 4 || reads[0] != 3 || reads[3] != 0 {
		t.Fatal("consumer read a queue snapshot instead of actual owner", actual, reads, err)
	}
	for _, mode := range []string{"missing", "twice", "expired", "error"} {
		s := SoundPriorityState{SelectedSound: 255, Queue: [4]uint8{6}}
		var saved func() error
		err := ConsumeSoundQueueObserved(SoundQueueOwners{Priority: &s.CurrentPriority, Selected: &s.SelectedSound, Queue: &s.Queue}, table, func(op SoundQueueOperation, apply func() error) error {
			if op.Kind != QueueReadPriority {
				return apply()
			}
			if mode == "missing" {
				return nil
			}
			if mode == "error" {
				return errors.New("observer failed")
			}
			if err := apply(); err != nil {
				return err
			}
			if mode == "twice" {
				_ = apply()
				return nil
			}
			saved = apply
			return nil
		})
		if mode == "expired" {
			if err != nil || saved == nil || saved() == nil {
				t.Fatal("expired canonical queue effect usable", err)
			}
		} else if err == nil {
			t.Fatal("queue effect misuse accepted", mode)
		}
	}
}
