package server

import (
	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func navigationController() *gameapi.ControllerDefinition {
	button := func(index int) gameapi.ControllerPhysicalBinding {
		return gameapi.ControllerPhysicalBinding{Type: "button", Index: index}
	}
	axis := func(index, direction int) gameapi.ControllerPhysicalBinding {
		return gameapi.ControllerPhysicalBinding{Type: "axis", Index: index, Direction: direction}
	}
	return &gameapi.ControllerDefinition{Layout: "navigation", Actions: []gameapi.ControllerAction{
		{ID: "navUp", Label: "向上选择", Keys: []string{"ArrowUp"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(12), axis(1, -1)}},
		{ID: "navDown", Label: "向下选择", Keys: []string{"ArrowDown"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(13), axis(1, 1)}},
		{ID: "navLeft", Label: "向左选择", Keys: []string{"ArrowLeft"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(14), axis(0, -1)}},
		{ID: "navRight", Label: "向右选择", Keys: []string{"ArrowRight"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(15), axis(0, 1)}},
		{ID: "confirm", Label: "选择／确认", Keys: []string{"Enter", "Space"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(0)}},
		{ID: "cancel", Label: "取消／返回", Keys: []string{"Backspace"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(1)}},
		{ID: "previousGroup", Label: "上一组操作", Keys: []string{"PageUp"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(4)}},
		{ID: "nextGroup", Label: "下一组操作", Keys: []string{"PageDown"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(5)}},
	}}
}
func (s *Server) controllerModels() []controllerModel {
	models := controllerModels()
	for _, kind := range s.gameKinds {
		spec := s.modules[kind].Controls
		if _, independent := s.modules[kind]; !independent {
			spec = games.ControllerDefinitionFor(kind)
		}
		if spec == nil {
			continue
		}
		if spec.Layout == "navigation" {
			base := navigationController()
			base.Actions = append(base.Actions, spec.Actions...)
			spec = base
		}
		model := controllerModel{ID: "game:" + kind, Name: s.gameNames[kind], GameID: kind, Layout: spec.Layout, legacyProfile: spec.LegacyProfile}
		if spec.Layout == "classic-2" || spec.Layout == "classic-4" {
			for _, base := range controllerModels() {
				if base.ID == spec.Layout {
					model.Buttons = base.Buttons
					model.Defaults = base.Defaults
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
		models = append(models, model)
	}
	return models
}
