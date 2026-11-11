package starfort

import "errors"

func (s *State) weatherV2() bool       { return s.WeatherRules == 2 }
func (s *State) sleeping(e Enemy) bool { return s.weatherV2() && e.SleepingUntil > s.Tick }

// Weather rolls use the server clock and saved RNG, once per enemy per second.
func (s *State) stepWeather() {
	if !s.weatherV2() || (s.World.Event.ID != "fog" && s.World.Event.ID != "starlight") {
		return
	}
	for i := range s.Enemies {
		e := &s.Enemies[i]
		if e.HP <= 0 || s.Tick < e.WeatherCheckAt {
			continue
		}
		e.WeatherCheckAt = s.Tick + 1000/StepMS
		if s.Tick < e.WeatherReadyAt || !s.skillRoll(10) {
			continue
		}
		e.WeatherReadyAt = s.Tick + 15000/StepMS
		kind := "confused"
		if s.World.Event.ID == "fog" {
			e.ConfusedUntil = s.Tick + 3000/StepMS
			e.Repairing, e.RepairSinceMS = 0, 0
		} else {
			kind = "sleep"
			e.SleepingUntil = s.Tick + 3000/StepMS
		}
		s.event(Event{Type: kind, TargetID: e.ID, X: e.X, Y: e.Y, Kind: e.Kind, Owner: -1})
	}
}

func (s *State) weatherMiss(t Tower, index int) bool {
	if !s.weatherV2() || s.World.Event.ID != "frost" || t.Kind == "frost" {
		return false
	}
	e := &s.Enemies[index]
	if hasGuard(e.MissGuards, t.ID, s.Tick) || !s.skillRoll(10) {
		return false
	}
	e.MissGuards = putGuard(s.freshGuards(e.MissGuards), t.ID, s.Tick+10000/StepMS)
	s.event(Event{Type: "miss", TowerID: t.ID, TargetID: e.ID, X: e.X, Y: e.Y, Kind: t.Kind, Owner: t.Owner})
	return true
}

// Summons and split children begin with independent control and weather state.
func clearWeather(e *Enemy) {
	e.SleepingUntil, e.ConfusedUntil, e.WeatherReadyAt, e.WeatherCheckAt = 0, 0, 0, 0
	e.MissGuards = nil
	e.BossGuard = false
}

type MedicalControl struct {
	Count   int  `json:"count"`
	Heal    int  `json:"heal"`
	Enabled bool `json:"enabled"`
}

func (s *State) medicalControl(player int) *MedicalControl {
	if !s.weatherV2() || s.World.MedicalPacks == 0 {
		return nil
	}
	return &MedicalControl{Count: s.World.MedicalPacks, Heal: min(s.BaseMaxHP-s.BaseHP, 2+s.CompletedWaves/20), Enabled: player >= 0 && player < len(s.Players) && s.Phase == "prepare" && s.CompletedWaves > 0 && s.BaseHP < s.BaseMaxHP}
}
func (s *State) useMedical(player int) error {
	c := s.medicalControl(player)
	if c == nil || !c.Enabled {
		return errors.New("医疗包只能在波间整备且星核缺血时使用")
	}
	s.World.MedicalPacks--
	s.BaseHP += c.Heal
	p := s.World.Map.Base
	s.event(Event{Type: "objective", Kind: "medical", Owner: player, X: p.X, Y: p.Y, Amount: float64(c.Heal)})
	s.note("使用医疗包，星核恢复 %d 点生命", c.Heal)
	return nil
}

func (s *State) validWeatherV2() error {
	if s.World.MedicalPacks < 0 || s.World.MedicalPacks > maxValue || s.World.GuardLeakRemainder < 0 || s.World.GuardLeakRemainder > 1 || (!s.weatherV2() && (s.World.MedicalPacks != 0 || s.World.GuardLeakRemainder != 0)) {
		return errors.New("无效医疗包或守卫漏怪记录")
	}
	for _, e := range s.Enemies {
		if e.SleepingUntil < 0 || e.ConfusedUntil < 0 || e.WeatherReadyAt < 0 || e.WeatherCheckAt < 0 || (e.BossGuard && e.Kind != "shield") || (!s.weatherV2() && (e.SleepingUntil != 0 || e.ConfusedUntil != 0 || e.WeatherReadyAt != 0 || e.WeatherCheckAt != 0 || e.BossGuard || len(e.MissGuards) > 0)) {
			return errors.New("无效天气控制状态")
		}
	}
	return nil
}
