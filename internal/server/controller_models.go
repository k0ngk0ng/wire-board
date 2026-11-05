package server

// Expose only explicitly wired independent modules. FC/MD remain legacy layout
// presets; an independent game receives its own mutable persistence key.
func (s *Server) controllerModels() []controllerModel {
	models := controllerModels()
	for _, kind := range s.gameKinds {
		module, independent := s.modules[kind]
		if !independent || module.Controls == nil {
			continue
		}
		spec := module.Controls
		model := controllerModel{ID: "game:" + kind, Name: s.gameNames[kind], GameID: kind, Layout: spec.Layout, legacyProfile: spec.LegacyProfile}
		if spec.Layout == "classic-2" || spec.Layout == "md-6" {
			for _, base := range controllerModels() {
				if base.ID == spec.Layout {
					model.Buttons = base.Buttons
					model.Defaults = base.Defaults
					model.StandardGamepad = base.StandardGamepad
					model.TurboTargets = base.TurboTargets
				}
			}
		} else {
			model.Defaults = controllerProfile{Keyboard: map[string]string{}, KeyboardAlt: map[string][]string{}, TurboHz: 10}
			model.StandardGamepad = map[string][]controllerGamepadBinding{}
			model.TurboTargets = map[string]string{}
			for _, action := range spec.Actions {
				if action.Continuous {
					model.Continuous = append(model.Continuous, action.ID)
				}
				model.Buttons = append(model.Buttons, controllerButton{action.ID, action.Label})
				model.Defaults.Keyboard[action.ID] = ""
				if len(action.Keys) > 0 {
					model.Defaults.Keyboard[action.ID] = action.Keys[0]
				}
				if len(action.Keys) > 1 {
					model.Defaults.KeyboardAlt[action.ID] = append([]string(nil), action.Keys[1:]...)
				}
				for _, binding := range action.Gamepad {
					index := binding.Index
					model.StandardGamepad[action.ID] = append(model.StandardGamepad[action.ID], controllerGamepadBinding{Type: binding.Type, Index: &index, Direction: binding.Direction})
				}
				if action.TurboTarget != "" {
					model.TurboTargets[action.ID] = action.TurboTarget
				}
			}
		}
		if len(model.Buttons) > 0 {
			models = append(models, model)
		}
	}
	return models
}
