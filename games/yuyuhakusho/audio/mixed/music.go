package mixed

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"

func (r *Runtime) MusicDescriptor(id string) (pcmcommands.Descriptor, bool) {
	return r.pcm.MusicDescriptor(id)
}

func (r *Runtime) MusicFlags(at uint64) (uint8, error) { return r.pcm.MusicFlags(at) }

func (r *Runtime) PublishAcceptedAt(at uint64, input pcmcommands.Input) error {
	return r.atomic(func(next *Runtime) error { return next.pcm.PublishAcceptedAt(at, input) })
}
