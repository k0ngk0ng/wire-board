package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type controllerButton struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type controllerProfile struct {
	KeyboardAlt   map[string][]string                 `json:"keyboardAlt,omitempty"`
	Revision      int64                               `json:"revision"`
	Keyboard      map[string]string                   `json:"keyboard"`
	TurboHz       int                                 `json:"turboHz"`
	Gamepads      map[string]controllerGamepadProfile `json:"gamepads,omitempty"`
	GamepadDevice *string                             `json:"gamepadDevice,omitempty"`
}

type controllerGamepadBinding struct {
	Type      string `json:"type"`
	Index     *int   `json:"index"`
	Direction int    `json:"direction,omitempty"`
}
type controllerGamepadProfile struct {
	Deadzone float64                               `json:"deadzone"`
	Bindings map[string][]controllerGamepadBinding `json:"bindings"`
}

func validGamepadKey(key string) bool {
	kind, name, ok := strings.Cut(key, ":")
	return ok && (kind == "standard" || kind == "raw") && strings.TrimSpace(name) != "" && len(key) <= 1024 && strings.IndexFunc(key, func(r rune) bool { return r < 32 || r == 127 }) < 0
}

type controllerModel struct {
	Continuous      []string                              `json:"continuous,omitempty"`
	GameID          string                                `json:"gameId,omitempty"`
	Layout          string                                `json:"layout"`
	StandardGamepad map[string][]controllerGamepadBinding `json:"standardGamepad,omitempty"`
	TurboTargets    map[string]string                     `json:"turboTargets,omitempty"`
	legacyProfile   string
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Buttons         []controllerButton `json:"buttons"`
	Defaults        controllerProfile  `json:"defaults"`
}

// Models describe logical layouts, not game rules. Physical devices retain
// separate bindings within each user/model profile.
func controllerModels() []controllerModel {
	makeModel := func(id, name string, fourButtons bool) controllerModel {
		buttons := []controllerButton{{"up", "上"}, {"down", "下"}, {"left", "左"}, {"right", "右"}, {"select", "选择（Select）"}, {"start", "开始（Start）"}, {"b", "B"}, {"a", "A"}, {"turboB", "Turbo B"}, {"turboA", "Turbo A"}}
		keys := map[string]string{"up": "KeyW", "down": "KeyS", "left": "KeyA", "right": "KeyD", "select": "KeyF", "start": "Space", "b": "KeyJ", "a": "KeyK", "turboB": "KeyU", "turboA": "KeyI"}
		if fourButtons {
			buttons = append(buttons, controllerButton{"x", "X"}, controllerButton{"y", "Y"}, controllerButton{"l", "L"}, controllerButton{"r", "R"})
			keys["x"], keys["y"], keys["l"], keys["r"] = "KeyL", "KeyH", "KeyQ", "KeyE"
		}
		return controllerModel{ID: id, Name: name, Layout: id, Buttons: buttons, Defaults: controllerProfile{Keyboard: keys, TurboHz: 10}}
	}
	return []controllerModel{makeModel("classic-2", "经典双键（FC）", false), makeModel("classic-4", "经典四键（SFC）", true)}
}

var controllerKeyCode = regexp.MustCompile(`^(Key[A-Z]|Digit[0-9]|Numpad[0-9]|Arrow(Up|Down|Left|Right)|Space|Enter|Tab|Backspace|Delete|Home|End|PageUp|PageDown|Shift(Left|Right)|Control(Left|Right)|Alt(Left|Right)|Bracket(Left|Right)|Semicolon|Quote|Comma|Period|Slash|Backslash|Minus|Equal|Backquote|F([1-9]|1[0-2]))$`)

func validateControllerProfile(model controllerModel, p controllerProfile) error {
	if p.Revision < 0 || p.Revision > 1<<53-1 || p.TurboHz < 2 || p.TurboHz > 15 || len(p.Keyboard) != len(model.Buttons) {
		return errors.New("控制器配置不完整，连发速度应为每秒 2～15 次")
	}
	seen := map[string]bool{}
	for _, button := range model.Buttons {
		code, exists := p.Keyboard[button.ID]
		if !exists {
			return errors.New("控制器操作已更新，请重新读取设置")
		}
		if code == "" {
			continue
		}
		if !controllerKeyCode.MatchString(code) {
			return errors.New("请选择有效的键盘按键")
		}
		if seen[code] {
			return errors.New("每个按键只能对应一个操作")
		}
		seen[code] = true
	}
	device := ""
	if p.GamepadDevice != nil {
		device = *p.GamepadDevice
	}
	if len(p.Gamepads) > 16 || device != "" && device != "none" && !validGamepadKey(device) {
		return errors.New("手柄型号配置不正确，最多保存 16 种实体手柄")
	}
	allowed := map[string]bool{}
	for _, button := range model.Buttons {
		allowed[button.ID] = true
	}
	for action, codes := range p.KeyboardAlt {
		if !allowed[action] || len(codes) > 4 {
			return errors.New("键盘替代按键配置不正确")
		}
		for _, code := range codes {
			if !controllerKeyCode.MatchString(code) || seen[code] {
				return errors.New("每个键盘按键只能绑定一次")
			}
			seen[code] = true
		}
	}
	for key, pad := range p.Gamepads {
		if !validGamepadKey(key) || pad.Deadzone < 0.1 || pad.Deadzone > 0.8 || pad.Bindings == nil || len(pad.Bindings) > len(model.Buttons) {
			return errors.New("手柄配置不完整，摇杆死区应为 10%～80%")
		}
		used := map[struct {
			kind             string
			index, direction int
		}]bool{}
		for button, bindings := range pad.Bindings {
			if !allowed[button] || bindings == nil || len(bindings) > 4 {
				return errors.New("手柄逻辑按键或绑定数量不正确")
			}
			for _, binding := range bindings {
				if binding.Index == nil || *binding.Index < 0 {
					return errors.New("请选择有效的手柄按钮或轴")
				}
				switch binding.Type {
				case "button":
					if *binding.Index > 63 || binding.Direction != 0 {
						return errors.New("手柄按钮编号不正确")
					}
				case "axis":
					if *binding.Index > 15 || binding.Direction != 1 && binding.Direction != -1 {
						return errors.New("手柄轴或方向不正确")
					}
				default:
					return errors.New("手柄绑定类型不正确")
				}
				id := struct {
					kind             string
					index, direction int
				}{binding.Type, *binding.Index, binding.Direction}
				if used[id] {
					return errors.New("同一个手柄按钮或轴方向只能对应一个操作")
				}
				used[id] = true
			}
		}
	}
	return nil
}

func (s *Server) initControllerProfiles() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS controller_profiles (
user_id TEXT NOT NULL, model TEXT NOT NULL, settings BLOB NOT NULL,
revision INTEGER NOT NULL, PRIMARY KEY(user_id, model));`)
	return err
}

func (s *Server) readControllerProfile(user string, model controllerModel) (controllerProfile, error) {
	profile := model.Defaults
	var data []byte
	var revision int64
	err := s.db.QueryRow("SELECT settings,revision FROM controller_profiles WHERE user_id=? AND model=?", user, model.ID).Scan(&data, &revision)
	if errors.Is(err, sql.ErrNoRows) && model.legacyProfile != "" {
		// Migrate once, never share a mutable layout with an independent game.
		_, err = s.db.Exec(`INSERT INTO controller_profiles(user_id,model,settings,revision) SELECT user_id,?,settings,revision FROM controller_profiles WHERE user_id=? AND model=? ON CONFLICT(user_id,model) DO NOTHING`, model.ID, user, model.legacyProfile)
		if err != nil {
			return profile, err
		}
		err = s.db.QueryRow("SELECT settings,revision FROM controller_profiles WHERE user_id=? AND model=?", user, model.ID).Scan(&data, &revision)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return profile, nil
	}
	if err != nil {
		return profile, err
	}
	// Decode into a fresh map: defaults are also returned as reset presets.
	profile = controllerProfile{}
	if err = json.Unmarshal(data, &profile); err != nil {
		return profile, err
	}
	profile.Revision = revision
	return profile, validateControllerProfile(model, profile)
}

func (s *Server) controllers(w http.ResponseWriter, r *http.Request) {
	user, ok := s.needUser(w, r)
	if !ok {
		return
	}
	models := s.controllerModels()
	if kind := r.URL.Query().Get("game"); kind != "" {
		filtered := []controllerModel{}
		for _, model := range models {
			if model.GameID == kind {
				filtered = append(filtered, model)
			}
		}
		if len(filtered) == 0 {
			fail(w, 404, "游戏操作方案不存在")
			return
		}
		models = filtered
	}
	profiles := map[string]controllerProfile{}
	for _, model := range models {
		profile, err := s.readControllerProfile(user.ID, model)
		if err != nil {
			fail(w, 500, "读取控制器设置失败")
			return
		}
		profiles[model.ID] = profile
	}
	respond(w, 200, map[string]any{"models": models, "profiles": profiles})
}

func (s *Server) saveController(w http.ResponseWriter, r *http.Request) {
	user, ok := s.needUser(w, r)
	if !ok {
		return
	}
	var model controllerModel
	for _, candidate := range s.controllerModels() {
		if candidate.ID == r.PathValue("model") {
			model = candidate
			break
		}
	}
	if model.ID == "" {
		fail(w, 404, "控制器型号不存在")
		return
	}
	var profile controllerProfile
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	d.DisallowUnknownFields()
	if d.Decode(&profile) != nil || d.Decode(new(any)) != io.EOF {
		fail(w, 400, "控制器配置格式不正确")
		return
	}
	if err := validateControllerProfile(model, profile); err != nil {
		fail(w, 400, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.readControllerProfile(user.ID, model)
	if err != nil {
		fail(w, 500, "读取控制器设置失败")
		return
	}
	if current.Revision != profile.Revision {
		fail(w, 409, "设置已在其他页面更新，请重新加载后再修改")
		return
	}
	// Old keyboard-only clients do not know about physical devices. An omitted
	// map preserves them; an explicit empty map is a deliberate device reset.
	if profile.Gamepads == nil {
		profile.Gamepads = current.Gamepads
	}
	if profile.GamepadDevice == nil {
		profile.GamepadDevice = current.GamepadDevice
	}
	if profile.KeyboardAlt == nil {
		profile.KeyboardAlt = current.KeyboardAlt
	}
	profile.Revision++
	data, err := json.Marshal(profile)
	if err == nil {
		_, err = s.db.Exec(`INSERT INTO controller_profiles(user_id,model,settings,revision) VALUES(?,?,?,?)
ON CONFLICT(user_id,model) DO UPDATE SET settings=excluded.settings,revision=excluded.revision`, user.ID, model.ID, data, profile.Revision)
	}
	if err != nil {
		fail(w, 500, "保存控制器设置失败")
		return
	}
	respond(w, 200, profile)
}
