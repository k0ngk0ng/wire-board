package pokepath

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func controls() *gameapi.ControllerDefinition {
	button := func(index int) []gameapi.ControllerPhysicalBinding {
		return []gameapi.ControllerPhysicalBinding{{Type: "button", Index: index}}
	}
	definition := &gameapi.ControllerDefinition{Layout: "navigation", Actions: []gameapi.ControllerAction{
		{ID: "profile", Label: "训练家", Keys: []string{"KeyQ"}, Gamepad: button(8)},
		{ID: "routes", Label: "路线", Keys: []string{"KeyW"}, Gamepad: button(6)},
		{ID: "box", Label: "仓库", Keys: []string{"KeyE"}, Gamepad: button(3)},
		{ID: "shop", Label: "商店", Keys: []string{"KeyS"}, Gamepad: button(2)},
		{ID: "damage", Label: "伤害统计", Keys: []string{"KeyZ"}, Gamepad: button(11)},
		{ID: "ranges", Label: "全部攻击范围", Keys: []string{"KeyX"}, Gamepad: button(10)},
		{ID: "speed", Label: "切换速度", Keys: []string{"ShiftLeft", "ShiftRight"}, Gamepad: button(7)},
		{ID: "previousDetails", Label: "上一个伙伴", Keys: []string{"KeyA"}},
		{ID: "nextDetails", Label: "下一个伙伴", Keys: []string{"KeyD"}},
	}}
	for slot := 0; slot < 10; slot++ {
		definition.Actions = append(definition.Actions, gameapi.ControllerAction{ID: fmt.Sprintf("select%d", slot), Label: fmt.Sprintf("选择队伍第 %d 位", slot+1), Keys: []string{fmt.Sprintf("Digit%d", (slot+1)%10)}})
	}
	for slot := 0; slot < 5; slot++ {
		definition.Actions = append(definition.Actions, gameapi.ControllerAction{ID: fmt.Sprintf("load%d", slot), Label: fmt.Sprintf("载入编队 %d", slot+1), Keys: []string{fmt.Sprintf("F%d", slot+1)}})
	}
	return definition
}
