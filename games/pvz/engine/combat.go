package engine

func (s *State) DamageZombie(id, amount int, sourceKind string, owner int) {
	for i := range s.Zombies {
		z := &s.Zombies[i]
		if z.ID != id || z.HP <= 0 {
			continue
		}
		if !CanDamageZombie(*z) {
			return
		}
		if z.Flags == nil {
			z.Flags = map[string]int64{}
		}
		if z.Kind == "zomboss" && z.State != "boss-head" && z.Flags["headOpen"] == 0 {
			return
		}
		bypass := false
		switch sourceKind {
		case "fume", "gloom", "cabbage", "kernel", "butter", "melon", "wintermelon", "cob", "starleft", "backwards", "spike", "chomper", "squash", "kelp":
			bypass = true
		}
		if sourceKind == "iceshroom" && CanChillZombie(*z) {
			duration := int64(80 + s.random(41))
			if z.SlowUntil > s.Tick {
				duration = int64(60 + s.random(21))
			}
			if s.isWater(z.Row) {
				duration = 60
			}
			if CanFreezeZombie(*z) {
				z.Flags["frozenUntil"] = max(z.Flags["frozenUntil"], s.Tick+duration)
			}
			z.SlowUntil = max(z.SlowUntil, s.Tick+400)
		}
		if sourceKind == "firepea" || sourceKind == "jalapeno" {
			z.SlowUntil = 0
			delete(z.Flags, "frozenUntil")
		}
		if sourceKind == "butter" {
			z.Flags["butterUntil"] = max(z.Flags["butterUntil"], s.Tick+80)
		}
		if (sourceKind == "wintermelon" || sourceKind == "icepea") && CanChillZombie(*z) {
			z.SlowUntil = max(z.SlowUntil, s.Tick+200)
		}
		shield := z.Kind == "screendoor" || z.Kind == "ladder" || z.Kind == "newspaper"
		if sourceKind == "spike" || bypass && shield {
			z.HP -= amount
		} else {
			s.damage(z, amount, sourceKind == "icepea" || sourceKind == "wintermelon")
		}
		return
	}
}
func CanDamageZombie(z Zombie) bool {
	if z.Kind == "bobsled" && z.Flags["sled"] == 1 && z.Flags["slot"] > 0 {
		return false
	}
	if z.Kind == "bungee" && z.State != "grabbing" && z.State != "bungee-grab" {
		return false
	}
	return true
}
func (s *State) damagePlant(id, amount int, attacker *Zombie) {
	at := s.plantIndex(id)
	if at < 0 || amount <= 0 {
		return
	}
	p := s.Plants[at]
	if p.Layer == "body" {
		if shell := s.layerAt(p.Row, p.Col, "shell"); shell >= 0 {
			at = shell
			p = s.Plants[at]
		}
	}
	s.Plants[at].HP -= amount
	p = s.Plants[at]
	s.emit("plant-hit", p.Row, PlantX(p), p.Owner, p.Kind, p.ID, amount)
	if attacker != nil {
		s.OnPlantEaten(p, attacker)
	}
	if p.HP <= 0 && p.Layer == "support" {
		s.dropUnsupported(p.Row, p.Col)
	}
}
func (s *State) canBreach(z Zombie) bool {
	if z.HP <= 0 || z.X >= 0 || z.Allegiance == "plants" || z.Digging || z.Kind == "bungee" || z.Kind == "zomboss" || z.Kind == "yeti" && z.State == "escaping" {
		return false
	}
	return z.Altitude > 0 || s.Mowers[z.Row].State != "ready"
}
func (s *State) transformImitater(index int) {
	if index < 0 || index >= len(s.Plants) {
		return
	}
	old := s.Plants[index]
	if old.TargetKind == "coffeebean" {
		body := s.bodyAt(old.Row, old.Col)
		if body >= 0 && body != index {
			s.Plants[body].Flags["wakeAt"] = s.Tick + 20
		}
		s.Plants[index].HP = 0
		s.emit("transform", old.Row, PlantX(old), old.Owner, "coffeebean", old.ID, 0)
		return
	}
	c, ok := PlantSpec(old.TargetKind)
	if !ok {
		return
	}
	p := s.newPlant(c, old.Row, old.Col, old.Owner)
	p.ID = old.ID
	p.Imitated = true
	s.Plants[index] = p
	s.emit("transform", p.Row, PlantX(p), p.Owner, p.Kind, p.ID, 0)
}
func (s *State) wakeCoffee(index int) {
	p := &s.Plants[index]
	p.Asleep = false
	delete(p.Flags, "wakeAt")
	s.emit("wake", p.Row, PlantX(*p), p.Owner, p.Kind, p.ID, 0)
}
