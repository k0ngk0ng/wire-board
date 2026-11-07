package contra

import "fmt"

// Numeric kinds >=16 are stage-local. Keep that choice beside the original
// common table instead of merging unrelated actors into one generic behavior.
func (s *EnemySystem) stepEntity(slot int, c EnemyStepContext) (BossDoorSignal, error) {
	kind := s.Slots[slot].Kind
	var err error
	if kind < 16 {
		switch kind {
		case 0:
			err = s.StepPickup(slot, c)
		case 1:
			err = s.StepBullet(slot, c)
		case 2:
			err = s.StepWeaponBox(slot, c)
		case 3:
			err = s.StepCapsule(slot, c)
		case 4:
			err = s.StepTurret(slot, c)
		case 5:
			err = s.StepRunner(slot, c)
		case 6:
			err = s.StepSniper(slot, c)
		case 7:
			err = s.StepRedTurret(slot, c)
		case 8, 10:
			err = s.StepBaseCommonWall(slot, c)
		case 11:
			err = s.StepMortarProjectile(slot, c)
		case 12:
			err = s.StepScuba(slot, c)
		case 14:
			err = s.StepTurretMan(slot, c)
		case 15:
			err = s.StepTurretManBullet(slot, c)
		default:
			err = fmt.Errorf("unused common enemy type %d", kind)
		}
		return BossDoorNoSignal, err
	}
	switch c.Level {
	case 0:
		switch kind {
		case 16:
			err = s.StepBossCannon(slot, c)
		case 17:
			return s.StepBossDoor(slot, c)
		case 18:
			err = s.StepBridge(slot, c)
		default:
			err = fmt.Errorf("unknown Jungle type %d", kind)
		}
	case 1, 3:
		switch kind {
		case 16, 27, 28, 29:
			return s.StepBaseBoss(slot, c)
		case 17, 18:
			err = s.StepBaseThrownWeapon(slot, c)
		case 19:
			err = s.StepBaseWallTurret(slot, c)
		case 20:
			err = s.StepBaseWallCore(slot, c)
		case 21, 22, 23, 24:
			err = s.StepBaseIndoorSoldier(slot, c)
		case 25, 26, 32:
			err = s.StepBaseGenerator(slot, c)
		case 30, 31:
			err = s.StepBaseRedBlueSoldier(slot, c)
		default:
			err = fmt.Errorf("unknown Base type %d", kind)
		}
	case 2:
		if kind >= 16 && kind <= 19 {
			err = s.StepWaterfallHazard(slot, c)
		} else if kind == 20 || kind == 21 {
			if c.Scene.BossScrollComplete == nil || c.Scene.Palette == nil {
				return BossDoorNoSignal, fmt.Errorf("missing Waterfall scene state")
			}
			boss := WaterfallBossContext{ScrollComplete: *c.Scene.BossScrollComplete, PaletteTimer: c.Scene.Palette.FadeTimer, DestroyedArms: &s.BossPartsDestroyed,
				OnDefeated: func() error {
					if e := c.OnBossControl(BossControl{Kind: BossAudioReset}); e != nil {
						return e
					}
					return c.OnBossControl(BossControl{Kind: BossBeginDefeat})
				},
				OnClearDelay: func(delay uint8) error { return c.OnBossControl(BossControl{Kind: BossExitReady, Delay: delay}) }}
			if kind == 20 {
				err = s.StepDragonMouth(slot, c, boss)
			} else {
				err = s.StepDragonArm(slot, c, boss)
			}
		} else {
			err = fmt.Errorf("unknown Waterfall type %d", kind)
		}
	case 4:
		switch kind {
		case 16:
			err = s.StepIceGenerator(slot, c)
		case 17:
			err = s.StepIceGrenade(slot, c)
		case 18:
			err = s.StepSnowTank(slot, c)
		case 19:
			err = s.StepIceJoint(slot, c)
		case 20:
			err = s.StepSnowCarrier(slot, c)
		case 21:
			err = s.StepMiniUFO(slot, c)
		case 22:
			err = s.StepSnowBomb(slot, c)
		default:
			err = fmt.Errorf("unknown Snow type %d", kind)
		}
	case 5:
		switch kind {
		case 16, 17, 18:
			err = s.StepFlame(slot, c)
		case 19:
			err = s.StepGiantSoldier(slot, c)
		case 20:
			err = s.StepSpikedDisk(slot, c)
		default:
			err = fmt.Errorf("unknown Energy Zone type %d", kind)
		}
	case 6:
		switch kind {
		case 16:
			err = s.StepClaw(slot, c)
		case 17, 18:
			err = s.StepSpikeWall(slot, c)
		case 19:
			err = s.StepCartGenerator(slot, c)
		case 20, 21:
			err = s.StepCart(slot, c)
		case 22:
			err = s.StepArmoredDoor(slot, c)
		case 23:
			err = s.StepMortarLauncher(slot, c)
		case 24:
			err = s.StepHangarSoldierGenerator(slot, c)
		default:
			err = fmt.Errorf("unknown Hangar type %d", kind)
		}
	case 7:
		switch kind {
		case 16:
			err = s.StepAlienGuardian(slot, c, c.AlienBoss)
		case 17:
			err = s.StepAlienFetus(slot, c)
		case 18:
			err = s.StepAlienMouth(slot, c)
		case 19:
			err = s.StepAlienBlob(slot, c)
		case 20:
			err = s.StepAlienSpider(slot, c)
		case 21:
			err = s.StepAlienSpawner(slot, c)
		case 22:
			err = s.StepAlienHeart(slot, c, c.AlienBoss)
		default:
			err = fmt.Errorf("unknown Alien's Lair type %d", kind)
		}
	default:
		err = fmt.Errorf("unknown enemy stage %d", c.Level)
	}
	return BossDoorNoSignal, err
}
