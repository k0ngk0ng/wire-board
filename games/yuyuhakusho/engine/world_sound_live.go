package engine

// WorldSoundOperation identifies an actual producer and one canonical enqueue
// operation. It is transient: it contains no inferred CPU/VDP timestamp.
type WorldSoundOperation struct {
	ObjectID uint16
	Producer string
	Request  SoundRequestOperation
}

// WorldSoundOwners borrows the current world transaction's sole queue and
// pause owners. References and apply are valid only during the synchronous hook.
type WorldSoundOwners struct {
	Queue      *[4]uint8
	AudioPause *uint8
}

// WorldSoundHook must apply each operation exactly once. The caller must stage
// external effects in its own transaction; engine rollback only covers World.
// audio/session.WorldFrame also stages the real sound runtime in that boundary.
type WorldSoundHook func(WorldSoundOperation, WorldSoundOwners, func() error) error

// worldSoundPort is a call-stack context, never a World or component field.
// The first hook error suppresses further external calls and is checked before
// the current object callback can commit. Components still use their pure value
// views, but request reads/writes borrow this canonical world owner.
type worldSoundPort struct {
	owners   WorldSoundOwners
	objectID uint16
	hook     WorldSoundHook
	err      error
}

// worldSoundQueueView returns a temporary component view of the sole queue.
// Consumers can run at any priced work boundary, including paths without an
// enqueue and work after the last enqueue. Component merges therefore reread
// this owner rather than treating an earlier value view as authoritative.
// The nil-port path preserves the standalone pure-value API.
func worldSoundQueueView(value [4]uint8, port *worldSoundPort) [4]uint8 {
	if port == nil {
		return value
	}
	return *port.owners.Queue
}

func enqueueWorldSound(queue *[4]uint8, code uint8, producer string, port *worldSoundPort) {
	if port == nil {
		_, _ = EnqueueSoundObserved(queue, code, nil)
		return
	}
	if port.err != nil {
		return
	}
	var hook SoundRequestHook
	if port.hook != nil {
		hook = func(op SoundRequestOperation, apply func() error) error {
			return port.hook(WorldSoundOperation{ObjectID: port.objectID, Producer: producer, Request: op}, port.owners, apply)
		}
	}
	_, port.err = EnqueueSoundObserved(port.owners.Queue, code, hook)
	// This is a temporary view. Later work can run a consumer without another
	// enqueue, so each component merge refreshes it from the canonical owner.
	*queue = worldSoundQueueView(*queue, port)
}
