package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
)

type controllerButton struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type controllerProfile struct {
	Revision int64             `json:"revision"`
	Keyboard map[string]string `json:"keyboard"`
	TurboHz  int               `json:"turboHz"`
}

type controllerModel struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Buttons  []controllerButton `json:"buttons"`
	Defaults controllerProfile  `json:"defaults"`
}

// Models describe logical layouts, not game rules. Physical gamepad identities
// can gain a separate binding source without sharing one user's keyboard map.
func controllerModels() []controllerModel {
	makeModel := func(id, name string, fourButtons bool) controllerModel {
		buttons := []controllerButton{{"up", "上"}, {"down", "下"}, {"left", "左"}, {"right", "右"}, {"select", "选择（Select）"}, {"start", "开始（Start）"}, {"b", "B"}, {"a", "A"}, {"turboB", "Turbo B"}, {"turboA", "Turbo A"}}
		keys := map[string]string{"up": "KeyW", "down": "KeyS", "left": "KeyA", "right": "KeyD", "select": "KeyF", "start": "Space", "b": "KeyJ", "a": "KeyK", "turboB": "KeyU", "turboA": "KeyI"}
		if fourButtons {
			buttons = append(buttons, controllerButton{"x", "X"}, controllerButton{"y", "Y"}, controllerButton{"l", "L"}, controllerButton{"r", "R"})
			keys["x"], keys["y"], keys["l"], keys["r"] = "KeyL", "KeyH", "KeyQ", "KeyE"
		}
		return controllerModel{ID: id, Name: name, Buttons: buttons, Defaults: controllerProfile{Keyboard: keys, TurboHz: 10}}
	}
	return []controllerModel{makeModel("classic-2", "经典双键（FC）", false), makeModel("classic-4", "经典四键（SFC）", true)}
}

var controllerKeyCode = regexp.MustCompile(`^(Key[A-Z]|Digit[0-9]|Numpad[0-9]|Arrow(Up|Down|Left|Right)|Space|Enter|Tab|Backspace|Shift(Left|Right)|Control(Left|Right)|Alt(Left|Right)|Bracket(Left|Right)|Semicolon|Quote|Comma|Period|Slash|Backslash|Minus|Equal|Backquote|F([1-9]|1[0-2]))$`)

func validateControllerProfile(model controllerModel, p controllerProfile) error {
	if p.Revision < 0 || p.Revision > 1<<53-1 || p.TurboHz < 2 || p.TurboHz > 15 || len(p.Keyboard) != len(model.Buttons) {
		return errors.New("控制器配置不完整，连发速度应为每秒 2～15 次")
	}
	seen := map[string]bool{}
	for _, button := range model.Buttons {
		code := p.Keyboard[button.ID]
		if !controllerKeyCode.MatchString(code) {
			return errors.New("请选择有效的键盘按键")
		}
		if seen[code] {
			return errors.New("每个按键只能对应一个操作")
		}
		seen[code] = true
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
	models := controllerModels()
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
	for _, candidate := range controllerModels() {
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
	d := json.NewDecoder(r.Body)
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
