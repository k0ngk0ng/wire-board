package pal

import "github.com/k0ngk0ng/wire-board/internal/gameapi"

func controls() *gameapi.ControllerDefinition {
	button := func(i int) gameapi.ControllerPhysicalBinding {
		return gameapi.ControllerPhysicalBinding{Type: "button", Index: i}
	}
	axis := func(i, d int) gameapi.ControllerPhysicalBinding {
		return gameapi.ControllerPhysicalBinding{Type: "axis", Index: i, Direction: d}
	}
	return &gameapi.ControllerDefinition{Layout: "game", Actions: []gameapi.ControllerAction{
		{ID: "up", Label: "向上", Keys: []string{"ArrowUp", "KeyW"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(12), axis(1, -1)}},
		{ID: "down", Label: "向下", Keys: []string{"ArrowDown", "KeyS"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(13), axis(1, 1)}},
		{ID: "left", Label: "向左", Keys: []string{"ArrowLeft", "KeyA"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(14), axis(0, -1)}},
		{ID: "right", Label: "向右", Keys: []string{"ArrowRight", "KeyD"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(15), axis(0, 1)}},
		{ID: "confirm", Label: "确认", Keys: []string{"Enter", "Space", "KeyK"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(0)}},
		{ID: "cancel", Label: "返回／菜单", Keys: []string{"Backspace", "KeyJ"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(1), button(9)}},
		{ID: "status", Label: "人物状态", Keys: []string{"KeyC"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(3)}},
		{ID: "defend", Label: "防御", Keys: []string{"KeyG"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(2)}},
		{ID: "item", Label: "使用道具", Keys: []string{"KeyE"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(5)}},
		{ID: "throw", Label: "投掷道具", Keys: []string{"KeyT"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(7)}},
		{ID: "repeat", Label: "重复上次行动", Keys: []string{"KeyR"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(4)}},
		{ID: "force", Label: "自动选招", Keys: []string{"KeyF"}, Gamepad: []gameapi.ControllerPhysicalBinding{button(6)}},
		{ID: "flee", Label: "逃跑", Keys: []string{"KeyQ"}},
	}}
}
