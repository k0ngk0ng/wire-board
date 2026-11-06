package mixed

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"

func (r *Runtime) OrdinaryDescriptor(id string) (pcmcommands.Descriptor, bool) {
	return r.pcm.OrdinaryDescriptor(id)
}

func (r *Runtime) ReadArbitration(at uint64, field pcmcommands.ArbitrationField) (uint8, error) {
	return r.pcm.ReadArbitration(at, field)
}
