package resourcebridge

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"

// OrdinaryDescriptor resolves one accepted logical code to immutable metadata;
// diagnostic/duck 7F is absent from the normal ordinary resource closure.
func (l *Library) OrdinaryDescriptor(code uint8) (pcmcommands.Descriptor, bool) {
	if l == nil || l.pcm == nil {
		return pcmcommands.Descriptor{}, false
	}
	d, ok := l.pcm.Ordinary(code)
	if !ok {
		return pcmcommands.Descriptor{}, false
	}
	return pcmcommands.Descriptor{ID: d.ID, PayloadID: d.SampleID, NativeRate: d.NativeRate, Flags: d.Flags, RoutingOrPan: d.RoutingOrPan, Protocol: pcmcommands.OrdinaryProtocol}, true
}
