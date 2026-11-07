package scoreclock

import (
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/videoclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func (w *battleWorldClock) refreshSceneDisplay() error {
	p := w.p
	var ready bool
	if err := p.step(engine.MainWorkOperation{Phase: "read-scene-resource-flags", CPUCycles: 12}, func() error { ready = w.world.Palette.Flags&1 != 0; return nil }); err != nil {
		return err
	}
	if err := p.spend("test-scene-resource-ready", 10); err != nil {
		return err
	}
	if err := p.branch("scene-resource-unready", !ready, false); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "publish-scene-resource-ready", CPUCycles: 16}, func() error {
		if ready {
			w.world.Palette.Flags = 0x81
		} else {
			w.world.Palette.Flags = 0
		}
		return nil
	}); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "scene-cached-display", CPUCycles: 20}, func() error { w.graphics.CachedDisplay = ready; return nil }); err != nil {
		return err
	}
	return p.spend("return-scene-display", 16)
}
func (w *battleWorldClock) refreshSceneRegisters() error {
	p := w.p
	g := w.graphics
	if err := p.spend("scene-register-port", 12); err != nil {
		return err
	}
	// These are typed layout/configuration values, not a source instruction or
	// RAM command stream. Live IRQ, display, reload and backdrop have one owner.
	horizontal := uint32(4)
	if w.world.InterruptMode == 4 {
		horizontal |= 16
	}
	values := []uint32{0x8000 | horizontal, 0x8100 | uint32(w.displayValue(false)), 0x8200 | uint32(g.Layout.Planes[0]>>10), 0x8300 | uint32(g.Layout.Window>>10), 0x8400 | uint32(g.Layout.Planes[1]>>13), 0x8500 | uint32(g.Layout.Sprites>>9), 0x8700 | uint32(w.world.BackdropColor), 0x8a00 | uint32(g.Layout.CounterBase), 0x8b00 | uint32(w.world.ScrollMode), 0x8c81, 0x8d00 | uint32(g.Layout.HScroll>>10), 0x9000 | uint32(g.Layout.PlaneSize), 0x9100 | uint32(g.Layout.WindowHorizontal), 0x9200 | uint32(g.Layout.WindowVertical)}
	for _, value := range values {
		if err := w.control("refresh-scene-register", 16, value, 2); err != nil {
			return err
		}
	}
	return p.spend("return-scene-registers", 16)
}
func (w *battleWorldClock) refreshPalette() error {
	p := w.p
	if err := p.spend("palette-transfer-setup", 12, 12, 10); err != nil {
		return err
	}
	if err := w.control("enable-palette-transfer", 8, 0x8100|uint32(w.displayValue(true)), 2); err != nil {
		return err
	}
	if err := w.control("palette-transfer-length", 20, 0x93409400, 4); err != nil {
		return err
	}
	for _, value := range []uint32{0x9500, 0x96f0, 0x977f, 0x8f02} {
		if err := w.control("palette-transfer-source", 12, value, 2); err != nil {
			return err
		}
	}
	if err := p.spend("palette-transfer-destination", 24); err != nil {
		return err
	}
	if err := w.control("palette-transfer-address-high", 16, 0xc000, 2); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "palette-transfer-address-low", CPUCycles: 16}, func() error {
		if err := p.io(BattleSampleIO{Kind: "vdp-control", Write: true, Value: 0x80, Width: 2}); err != nil {
			return err
		}
		return w.dma(videoclock.DMACRAM, 64, uint32(p.s.clock.Cycle-p.context.FrameOrigin))
	}); err != nil {
		return err
	}
	if err := w.control("restore-palette-display", 16, 0x8100|uint32(w.displayValue(false)), 2); err != nil {
		return err
	}
	return p.spend("return-palette-transfer", 16)
}
func (w *battleWorldClock) battleSceneRoutine() error {
	p := w.p
	if w.world.VideoMode != 2 {
		return ErrBattleWorldProfile
	}
	if err := p.spend("scene-routine-dispatch", 12, 12, 12, 8); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "scene-disable-cached-display", CPUCycles: 20}, func() error { w.graphics.CachedDisplay = false; return nil }); err != nil {
		return err
	}
	if err := w.control("scene-disable-display", 24, 0x8100|uint32(w.displayValue(false)), 2); err != nil {
		return err
	}
	if err := w.control("scene-backdrop-address", 28, 0xc0000000, 4); err != nil {
		return err
	}
	if err := w.data("scene-backdrop-clear", 20, 0); err != nil {
		return err
	}
	if err := w.control("scene-sample-reload", 20, 0x8a09, 2); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "scene-sample-phase-reset", CPUCycles: 16}, func() error { p.context.Phase = 0; return nil }); err != nil {
		return err
	}
	var missed bool
	if err := p.step(engine.MainWorkOperation{Phase: "scene-missed-test", CPUCycles: 16}, func() error { missed = w.flags.MissedWorld; return nil }); err != nil {
		return err
	}
	if err := p.branch("scene-missed-branch", missed, false); err != nil {
		return err
	}
	if !missed {
		if err := p.step(engine.MainWorkOperation{Phase: "save-backdrop-color", CPUCycles: 24}, func() error { w.graphics.SavedColors[0] = w.world.Palette.Current[0]; return nil }); err != nil {
			return err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "save-sprite-marker", CPUCycles: 24}, func() error { w.graphics.SavedColors[1] = w.graphics.SpriteMarker; return nil }); err != nil {
			return err
		}
	} else {
		if err := w.control("scene-missed-marker-address", 28, 0x74020003, 4); err != nil {
			return err
		}
		if err := w.data("scene-missed-marker", 28, w.graphics.SavedColors[1]); err != nil {
			return err
		}
	}
	return p.spend("return-battle-scene", 16)
}
func (w *battleWorldClock) refreshSceneRoutine() error {
	p := w.p
	if err := p.spend("scene-mode-read", 12, 12); err != nil {
		return err
	}
	equal := w.world.VideoMode == w.graphics.PreviousVideoMode
	if err := p.branch("scene-mode-equal", equal, false); err != nil {
		return err
	}
	if !equal {
		return ErrBattleWorldProfile
	}
	if err := p.spend("call-scene-routine", 18); err != nil {
		return err
	}
	if err := w.battleSceneRoutine(); err != nil {
		return err
	}
	if err := p.spend("scene-IRQ-enabled-test", 16); err != nil {
		return err
	}
	enabled := w.world.InterruptMode == 4
	if err := p.branch("scene-IRQ-enabled", enabled, false); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "scene-IRQ-ready", CPUCycles: 16}, func() error { w.graphics.SceneIRQReady = enabled; return nil }); err != nil {
		return err
	}
	return p.spend("return-scene-routine", 16)
}

func (w *battleWorldClock) handleVertical() error {
	if !w.pendingVertical {
		return nil
	}
	callerMask := w.verticalReturnMask
	w.pendingVertical = false
	p := w.p
	if err := p.step(engine.MainWorkOperation{Phase: "mask-vertical-IRQ", CPUCycles: 20}, func() error { p.context.Mask = 7; return nil }); err != nil {
		return err
	}
	if err := p.worldBus(w.device, true); err != nil {
		return err
	}
	if err := p.worldGate(w.device, true); err != nil {
		return err
	}
	var paused bool
	if err := p.step(engine.MainWorkOperation{Phase: "vertical-pause-read", CPUCycles: 16}, func() error { paused = w.world.Clock.AudioPause == 255; return nil }); err != nil {
		return err
	}
	if err := p.branch("vertical-paused-bus", paused, true); err != nil {
		return err
	}
	if !paused {
		if err := p.worldBus(w.device, false); err != nil {
			return err
		}
	}
	if err := p.step(engine.MainWorkOperation{Phase: "save-vertical-temporaries", CPUCycles: 112, InternalRefresh: true, TailCycles: 8}, nil); err != nil {
		return err
	}
	for {
		var value uint8
		if err := p.step(engine.MainWorkOperation{Phase: "vertical-status-read", CPUCycles: 16}, func() error {
			value = p.video.StatusBits()
			return p.io(BattleSampleIO{Kind: "vdp-status", Value: uint32(value), Width: 2})
		}); err != nil {
			return err
		}
		if err := p.spend("vertical-blank-mask", 8); err != nil {
			return err
		}
		if err := p.branch("vertical-blank-wait", value&8 == 0, false); err != nil {
			return err
		}
		if value&8 != 0 {
			break
		}
	}
	if err := p.spend("call-vertical-scene", 18); err != nil {
		return err
	}
	if err := p.spend("vertical-mode-route", 12, 10); err != nil {
		return err
	}
	var missed bool
	if err := p.step(engine.MainWorkOperation{Phase: "vertical-world-missed", CPUCycles: 16}, func() error { missed = w.flags.MissedWorld; return nil }); err != nil {
		return err
	}
	if err := p.branch("vertical-world-missed-branch", missed, false); err != nil {
		return err
	}
	if !missed {
		if err := p.spend("call-scene-display-refresh", 18); err != nil {
			return err
		}
		if err := w.refreshSceneDisplay(); err != nil {
			return err
		}
		if err := p.spend("call-scene-register-refresh", 20); err != nil {
			return err
		}
		if err := w.refreshSceneRegisters(); err != nil {
			return err
		}
		if err := p.spend("call-palette-refresh", 18); err != nil {
			return err
		}
		if err := w.refreshPalette(); err != nil {
			return err
		}
		if err := p.spend("call-scene-mode-refresh", 20); err != nil {
			return err
		}
		if err := w.refreshSceneRoutine(); err != nil {
			return err
		}
	} else {
		if err := p.spend("call-previous-scene-refresh", 20, 12); err != nil {
			return err
		}
		if err := p.branch("previous-scene-ready", w.graphics.SceneIRQReady, false); err != nil {
			return err
		}
		if w.graphics.SceneIRQReady {
			if err := w.battleSceneRoutine(); err != nil {
				return err
			}
		} else {
			if err := p.spend("return-previous-scene", 16); err != nil {
				return err
			}
		}
	}
	var transfer bool
	if err := p.step(engine.MainWorkOperation{Phase: "vertical-transfer-active-test", CPUCycles: 16}, func() error { transfer = w.flags.TransferActive; return nil }); err != nil {
		return err
	}
	if err := p.branch("vertical-transfer-active", transfer, false); err != nil {
		return err
	}
	if !transfer {
		if err := w.releaseGate(); err != nil {
			return err
		}
	}
	if err := p.spend("return-vertical-scene", 16); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "restore-vertical-temporaries", CPUCycles: 112, InternalRefresh: true, TailCycles: 12}, nil); err != nil {
		return err
	}
	return w.restoreMask("return-vertical-IRQ", 20, callerMask)
}
