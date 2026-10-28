package contra

import "errors"

// StepCampaignEntrance is one original DB1C phase. It streams into the existing
// queue/collision/map owners. The enclosing Campaign calls AfterPresentation
// only when this function returns true; no frame, palette or display commit is
// performed here. Rejected work leaves every owner unchanged.
func StepCampaignEntrance(w *ActiveWorld, resources *LevelResources) (bool, error) {
	if w == nil || resources == nil || w.Combat.World.GameRoutine != 5 || w.Combat.World.LevelRoutine != 3 || w.Combat.World.Level >= 8 || w.Combat.World.ScrollType == 0 && w.Terrain.TransitionRemaining == 0 {
		return false, errors.New("unsupported campaign entrance context")
	}
	next := *w
	next.Presentation.Queue = w.Presentation.Queue.Clone()
	s := &next.Terrain
	ctx := TerrainScrollContext{World: &next.Combat.World, Terrain: &next.Combat.Terrain, Header: &next.Header, ScreenCleared: &next.Combat.ScreenCleared, Queue: &next.Presentation.Queue, Resources: resources, Work: &next.Work}
	info, err := resources.Stage(int(next.Combat.World.Level))
	if err != nil {
		return false, err
	}
	if next.Combat.World.SceneType != info.SceneType || next.Combat.World.ScrollType != info.ScrollType {
		return false, errors.New("entrance scene does not match resources")
	}
	if next.Combat.World.ScrollType != 0 {
		if err := s.tileRow(ctx); err != nil {
			return false, err
		}
		if err := s.attributeRow(ctx); err != nil {
			return false, err
		}
		s.TileAddress -= 32
		s.Axis--
		done := int8(s.Axis) < 0
		if done {
			next.Combat.World.Screen = 0
			next.Combat.World.ScreenScroll = 0
			s.MapOffset = 0
			s.Axis = 29
			s.TileAddress = 0x23a0
			if err := resources.LoadMap(int(next.Combat.World.Level), s.Variant, 0, 0, &s.Supertiles); err != nil {
				return false, err
			}
			s.MapOffset = 64
			if err := resources.LoadMap(int(next.Combat.World.Level), s.Variant, 1, 64, &s.Supertiles); err != nil {
				return false, err
			}
		}
		*w = next
		return done, nil
	}

	if err := s.tileColumn(ctx); err != nil {
		return false, err
	}
	if err := s.attributeColumn(ctx); err != nil {
		return false, err
	}
	s.TileAddress = (s.TileAddress & 0xff00) | uint16(uint8(s.TileAddress)+1)
	s.Axis++
	s.TransitionRemaining--
	done := s.TransitionRemaining == 0
	if !done && next.Combat.World.SceneType == 0 && s.Axis >= 32 {
		s.Axis = 0
		s.TileAddress = 0x2400
		s.AttributeAddress = (s.AttributeAddress & 0xff) | 0x2700
		s.MapOffset = 64
		if err := resources.LoadMap(int(next.Combat.World.Level), s.Variant, int(next.Combat.World.Screen+1), int(s.MapOffset), &s.Supertiles); err != nil {
			return false, err
		}
	}
	*w = next
	return done, nil
}
