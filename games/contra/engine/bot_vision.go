package contra

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"sort"
)

// BotVisionKnowledge is immutable visual knowledge prepared independently of a
// running match. Template labels describe recognizable artwork, never a hidden
// slot or a live enemy Kind. Transparent template pixels contain no information.
type BotVisionLabel struct {
	Class                    string
	Family                   string
	Player                   int
	Kind, Sprite, Appearance uint8
	Phase                    string
	Stage, Scene, Choice     BotPublicNumber
}
type BotVisionTemplate struct {
	Width, Height, AnchorX, AnchorY int
	RGBA                            []uint8
	Label                           BotVisionLabel
}
type BotVisionTerrain struct {
	Pattern []uint8
	Kind    TerrainKind
}
type BotVisionKnowledge struct {
	Sprites []BotVisionTemplate
	Terrain []BotVisionTerrain
}
type botVisionPoint struct {
	x, y   int
	color  uint32
	detail bool
	offset uint16
}
type botVisionPattern struct {
	width, height, anchorX, anchorY int
	points                          []botVisionPoint
	bitPattern                      *botBitPattern
	detailCount                     int
	minimumDetails                  int
	partialPossible                 bool
	colorAnchors                    []botVisionPoint
	colorCounts                     []botPatternColor
	cornerAnchors                   [4][]botVisionAnchorChoice
	label                           BotVisionLabel
	signature                       [32]uint8
	playerMask                      uint8
	nonPlayerAlias                  bool
}
type botTerrainMeaning struct {
	kind      TerrainKind
	ambiguous bool
}
type BotVision struct {
	sprites        []botVisionPattern
	terrain        map[[64]uint8]botTerrainMeaning
	terrainPatches map[botTerrainPatchKey]botTerrainMeaning
}

func NewBotVision(source BotVisionKnowledge) (*BotVision, error) {
	v := &BotVision{terrain: map[[64]uint8]botTerrainMeaning{}, terrainPatches: map[botTerrainPatchKey]botTerrainMeaning{}}
	aliases := map[[32]uint8]int{}
	for _, s := range source.Sprites {
		if s.Width < 1 || s.Height < 1 || s.Width > 128 || s.Height > 128 || len(s.RGBA) != s.Width*s.Height*4 || s.AnchorX < -128 || s.AnchorX > s.Width+128 || s.AnchorY < -128 || s.AnchorY > s.Height+128 {
			return nil, errors.New("invalid visual template dimensions")
		}
		if s.Label.Class == "player" && (s.Label.Player < 0 || s.Label.Player > 1) {
			return nil, errors.New("invalid visual player label")
		}
		p := botVisionPattern{width: s.Width, height: s.Height, anchorX: s.AnchorX, anchorY: s.AnchorY, label: s.Label}
		if s.Label.Class == "player" {
			p.playerMask = 1 << uint(s.Label.Player)
		} else {
			p.nonPlayerAlias = true
		}
		if p.label.Class != "player" {
			p.label.Player = -1
		}
		canonical := make([]byte, 2+len(s.RGBA))
		canonical[0], canonical[1] = uint8(s.Width), uint8(s.Height)
		for y := 0; y < s.Height; y++ {
			for x := 0; x < s.Width; x++ {
				at := (y*s.Width + x) * 4
				alpha := s.RGBA[at+3]
				if alpha != 0 && alpha != 255 {
					return nil, errors.New("visual template must use discrete transparency")
				}
				if alpha == 0 {
					continue
				}
				copy(canonical[2+at:2+at+4], s.RGBA[at:at+4])
				color := botPixelRGB(s.RGBA, at)
				detail := botVisibleDetailColor(color)
				p.points = append(p.points, botVisionPoint{x: x, y: y, color: color, detail: detail, offset: uint16(y*256 + x)})
				if detail {
					p.detailCount++
				}
			}
		}
		if len(p.points) < 3 {
			return nil, errors.New("visual template is not identifiable")
		}
		prepareBotPatternAnchors(&p)
		p.bitPattern = compileBotBitPattern(&p)
		p.signature = sha256.Sum256(canonical)
		if i, ok := aliases[p.signature]; ok {
			old := &v.sprites[i]
			old.playerMask |= p.playerMask
			old.nonPlayerAlias = old.nonPlayerAlias || p.nonPlayerAlias
			if old.label.Class != p.label.Class || old.label.Player != p.label.Player || old.label.Kind != p.label.Kind || old.label.Family != p.label.Family {
				old.label.Class = "ambiguous"
			}
			old.label.Sprite = min(old.label.Sprite, p.label.Sprite)
			old.label.Appearance = min(old.label.Appearance, p.label.Appearance)
			old.anchorX = min(old.anchorX, p.anchorX)
			old.anchorY = min(old.anchorY, p.anchorY)
			if old.label.Phase != p.label.Phase {
				old.label.Phase = ""
			}
			if old.label.Stage != p.label.Stage {
				old.label.Stage = BotPublicNumber{}
			}
			if old.label.Scene != p.label.Scene {
				old.label.Scene = BotPublicNumber{}
			}
			if old.label.Choice != p.label.Choice {
				old.label.Choice = BotPublicNumber{}
			}
		} else {
			aliases[p.signature] = len(v.sprites)
			v.sprites = append(v.sprites, p)
		}
	}
	sort.Slice(v.sprites, func(i, j int) bool { return bytes.Compare(v.sprites[i].signature[:], v.sprites[j].signature[:]) < 0 })
	for _, s := range source.Terrain {
		if len(s.Pattern) != 256 || (s.Kind != TerrainEmpty && s.Kind != TerrainPlatform && s.Kind != TerrainWater && s.Kind != TerrainSolid) {
			return nil, errors.New("invalid visual terrain template")
		}
		key, ok := botPatternKey(s.Pattern)
		if !ok {
			continue
		}
		if old, exists := v.terrain[key]; exists && old.kind != s.Kind {
			old.ambiguous = true
			v.terrain[key] = old
		} else if !exists {
			v.terrain[key] = botTerrainMeaning{kind: s.Kind}
		}
		v.addTerrainPatches(s)
	}
	return v, nil
}

func botPixelRGB(p []uint8, at int) uint32 {
	return uint32(p[at])<<16 | uint32(p[at+1])<<8 | uint32(p[at+2])
}

type botVisionMatch struct {
	pattern         *botVisionPattern
	left, top, x, y int
	score           int
	exact           bool
}

// ObserveBotFrame accepts only the final visible image, public HUD and fixed
// artwork knowledge. Its result is independent of every hidden rule owner.
func ObserveBotFrame(frame BotVisualFrame, vision *BotVision) (BotObservation, error) {
	pixels := frame.Pixels()
	hud := frame.HUD()
	if vision == nil || len(pixels) != 256*240*4 || hud.Seats < 1 || hud.Seats > 2 || frame.Actor() < 0 || frame.Actor() >= hud.Seats {
		return BotObservation{}, errors.New("invalid visual observation input")
	}
	out := BotObservation{Tick: frame.Tick(), Actor: frame.Actor(), Seats: hud.Seats, Phase: hud.Phase}
	if hud.Stage.Known {
		out.StageKnown, out.Stage = true, hud.Stage.Value
	}
	if hud.Scene.Known {
		out.SceneKnown, out.Scene = true, hud.Scene.Value
	}
	if hud.Choice.Known {
		out.ChoiceKnown, out.Choice = true, hud.Choice.Value
	}
	if hud.Paused.Known {
		out.PausedKnown, out.Paused = true, hud.Paused.Value
	}
	for i, p := range hud.Players {
		if i >= hud.Seats || !p.Occupied {
			continue
		}
		if p.Lives.Known {
			out.Players[i].LivesKnown, out.Players[i].Lives = true, p.Lives.Value
		}
		if p.Weapon.Known {
			out.Players[i].WeaponKnown, out.Players[i].Weapon = true, p.Weapon.Value
		}
		if p.Alive.Known {
			out.Players[i].AliveKnown, out.Players[i].Alive = true, p.Alive.Value
		}
	}
	locations := map[uint32][]int{}
	colors := make([]uint32, 256*240)
	for i := 0; i < 256*240; i++ {
		color := botPixelRGB(pixels, i*4)
		colors[i] = color
		locations[color] = append(locations[color], i)
	}
	if len(locations) < 2 {
		return out, nil
	}
	pixelCounts := newBotPixelCounts(locations)
	planes := botRGBPlanesFromLocations(locations)
	var matches []botVisionMatch
	occupiedMask := uint8(0)
	for i, p := range hud.Players {
		if i < hud.Seats && p.Occupied {
			occupiedMask |= 1 << uint(i)
		}
	}
	tested := make([]uint32, 256*240)
	for i := range vision.sprites {
		pattern := &vision.sprites[i]
		if pattern.label.Class == "ambiguous" && (pattern.playerMask == 0 || pattern.nonPlayerAlias) {
			continue
		}
		if pattern.playerMask != 0 && pattern.playerMask&occupiedMask == 0 {
			continue
		}
		partial := pattern.playerMask != 0
		anchors, anchorCount := botVisualAnchors(pattern, locations, partial)
		candidateCount := anchorCount
		if partial && !pattern.partialPossible {
			// A complete match necessarily contains the rarest anchor. Other
			// anchors cannot introduce a valid candidate when partial matches
			// are mathematically impossible. Preserve the two-anchor gate.
			if anchorCount < 2 {
				continue
			}
			candidateCount, partial = 1, false
		}
		for _, anchor := range anchors[:candidateCount] {
			for _, candidate := range locations[anchor.color] {
				x, y := candidate%256-anchor.x, candidate/256-anchor.y
				if x < 0 || y < 0 || x+pattern.width > 256 || y+pattern.height > 240 {
					continue
				}
				key := y*256 + x
				if tested[key] == uint32(i+1) {
					continue
				}
				tested[key] = uint32(i + 1)
				if partial {
					votes := 0
					for _, a := range anchors[:anchorCount] {
						if colors[key+int(a.offset)] == a.color {
							votes++
						}
					}
					if votes < 2 {
						continue
					}
					if !pixelCounts.possible(pattern, x, y, true) {
						continue
					}
				}
				score, match := botMatchBitPattern(colors, planes, pattern.bitPattern, pattern, x, y, partial)
				if match {
					matches = append(matches, botVisionMatch{pattern, x, y, x + pattern.anchorX, y + pattern.anchorY, score, score == pattern.detailCount*20+len(pattern.points)})
				}
			}
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.exact != b.exact {
			return a.exact
		}
		if a.score != b.score {
			return a.score > b.score
		}
		if a.y != b.y {
			return a.y < b.y
		}
		if a.x != b.x {
			return a.x < b.x
		}
		return bytes.Compare(a.pattern.signature[:], b.pattern.signature[:]) < 0
	})
	var playerOcclusion *[256 * 240 / 64]uint64
	var terrainOcclusion *[256 * 240 / 64]uint64
	var accepted []botVisionMatch
	for _, m := range matches {
		duplicate := false
		for _, old := range accepted {
			if old.pattern.label.Class != m.pattern.label.Class {
				continue
			}
			if m.pattern.label.Class == "player" && old.pattern.label.Player != m.pattern.label.Player {
				continue
			}
			overlapX := min(old.left+old.pattern.width, m.left+m.pattern.width) - max(old.left, m.left)
			overlapY := min(old.top+old.pattern.height, m.top+m.pattern.height) - max(old.top, m.top)
			if overlapX > 0 && overlapY > 0 && overlapX*overlapY*2 >= min(old.pattern.width*old.pattern.height, m.pattern.width*m.pattern.height) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		accepted = append(accepted, m)
		label := m.pattern.label
		visiblePlayers := m.pattern.playerMask & occupiedMask
		if visiblePlayers != 0 && !m.pattern.nonPlayerAlias {
			label.Class = "player-candidate"
			// Head-only artwork can occur inside an NPC body. Occupancy can
			// exclude another player, but cannot prove this head is a player;
			// retain it for association with an earlier visible body instead.
			headOnly := label.Sprite >= 24 && label.Sprite <= 29
			if visiblePlayers == 1 && !headOnly && m.exact {
				label.Class, label.Player = "player", 0
			}
			if visiblePlayers == 2 && !headOnly && m.exact {
				label.Class, label.Player = "player", 1
			}
		}
		if label.Phase != "" && out.Phase == "" {
			out.Phase = label.Phase
		}
		if label.Stage.Known && !out.StageKnown {
			out.StageKnown, out.Stage = true, label.Stage.Value
		}
		if label.Scene.Known && !out.SceneKnown {
			out.SceneKnown, out.Scene = true, label.Scene.Value
		}
		if label.Choice.Known && !out.ChoiceKnown {
			out.ChoiceKnown, out.Choice = true, label.Choice.Value
		}
		switch label.Class {
		case "player-candidate":
			out.PlayerCandidates = append(out.PlayerCandidates, BotPlayerCandidate{Mask: visiblePlayers, Exact: m.exact, Player: BotPlayer{X: m.x, Y: m.y, Visible: true, Alive: true, Sprite: label.Sprite, Appearance: label.Appearance}})
			if out.Phase == "" {
				out.Phase = "battle"
			}
		case "player":
			p := &out.Players[label.Player]
			if label.Player >= out.Seats || !hud.Players[label.Player].Occupied {
				continue
			}
			if p.Visible {
				out.PlayerCandidates = append(out.PlayerCandidates, BotPlayerCandidate{Mask: 1 << uint(label.Player), Exact: m.exact, Player: BotPlayer{X: m.x, Y: m.y, Visible: true, Alive: true, Sprite: label.Sprite, Appearance: label.Appearance}})
				continue
			}
			if m.exact {
				if playerOcclusion == nil {
					playerOcclusion = new([256 * 240 / 64]uint64)
				}
				for _, point := range m.pattern.points {
					at := (m.top+point.y)*256 + m.left + point.x
					playerOcclusion[at/64] |= uint64(1) << uint(at%64)
				}
			}
			p.Visible, p.X, p.Y, p.Sprite, p.Appearance = true, m.x, m.y, label.Sprite, label.Appearance
			if !p.AliveKnown {
				p.Alive = true
			}
			if out.Phase == "" {
				out.Phase = "battle"
			}
		case "scenery":
			// Only the fixed seam artwork may hide terrain. Keep its opaque
			// pixels separate from player-only moving-support recognition.
			if m.exact && label.Sprite == 196 && label.Kind == 0 && m.pattern.playerMask == 0 {
				if terrainOcclusion == nil {
					terrainOcclusion = new([256 * 240 / 64]uint64)
				}
				for _, point := range m.pattern.points {
					at := (m.top+point.y)*256 + m.left + point.x
					terrainOcclusion[at/64] |= uint64(1) << uint(at%64)
				}
			}
		case "enemy":
			out.Enemies = append(out.Enemies, BotEnemy{X: m.x, Y: m.y, Kind: label.Kind, Sprite: label.Sprite, Appearance: label.Appearance, Family: label.Family})
		}
	}
	applyBotVisionMarkers(accepted, hud, &out)
	vision.recoverPlayerCoveredSupports(colors, playerOcclusion, &out)
	sort.Slice(out.Enemies, func(i, j int) bool {
		a, b := out.Enemies[i], out.Enemies[j]
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		if a.X != b.X {
			return a.X < b.X
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Sprite < b.Sprite
	})
	for i := range out.Enemies {
		out.Enemies[i].ID = i
	}
	vision.observeTerrain(pixels, &out)
	if terrainOcclusion != nil && out.Phase == "battle" {
		if playerOcclusion != nil {
			for i, word := range playerOcclusion {
				terrainOcclusion[i] |= word
			}
		}
		vision.recoverCoveredTerrain(pixels, terrainOcclusion, &out)
	} else {
		vision.recoverPlayerCoveredTerrain(pixels, playerOcclusion, &out)
	}
	if out.StageKnown && out.Stage == 2 {
		out.Vertical = true
	}
	return out, nil
}

func botVisibleDetailColor(color uint32) bool { return max(color>>16, (color>>8)&255, color&255) > 64 }

func botPatternKey(pattern []uint8) ([64]uint8, bool) {
	var out [64]uint8
	if len(pattern) != 256 {
		return out, false
	}
	labels := map[uint8]uint8{}
	for i, v := range pattern {
		label, ok := labels[v]
		if !ok {
			if len(labels) == 4 {
				return out, false
			}
			label = uint8(len(labels))
			labels[v] = label
		}
		out[i/4] |= label << uint(6-2*(i%4))
	}
	return out, len(labels) > 1
}
func botImageCell(pixels []uint8, x, y int) ([64]uint8, bool) {
	var out [64]uint8
	if x < 0 || x+16 > 256 || y < 0 || y+16 > 240 {
		return out, false
	}
	var colors [4]uint32
	count := 0
	for yy := 0; yy < 16; yy++ {
		for xx := 0; xx < 16; xx++ {
			c := botPixelRGB(pixels, ((y+yy)*256+x+xx)*4)
			label := 0
			for label < count && colors[label] != c {
				label++
			}
			if label == count {
				if count == 4 {
					return out, false
				}
				colors[count] = c
				count++
			}
			i := yy*16 + xx
			out[i/4] |= uint8(label) << uint(6-2*(i%4))
		}
	}
	return out, count > 1
}
func (v *BotVision) observeTerrain(pixels []uint8, out *BotObservation) {
	if len(v.terrain) == 0 {
		return
	}
	runs := botSameColorRuns(pixels)
	best, phaseX, phaseY := 0, 0, 0
	for oy := 0; oy < 16; oy++ {
		for ox := 0; ox < 16; ox++ {
			var samples [20][2]int
			count := 0
			for y := oy + 32; y+16 <= 224; y += 48 {
				for x := ox + 16; x+16 <= 256; x += 48 {
					if botUniformCell(pixels, runs, x, y) {
						continue
					}
					samples[count] = [2]int{x, y}
					count++
				}
			}
			// Uniform samples never score. A tie cannot replace the earlier
			// phase, so no remaining work is useful once this bound is <=best.
			score := 0
			for i := 0; i < count && score+count-i > best; i++ {
				if key, ok := botImageCell(pixels, samples[i][0], samples[i][1]); ok {
					if _, yes := v.terrain[key]; yes {
						score++
					}
				}
			}
			if score > best {
				best, phaseX, phaseY = score, ox, oy
			}
		}
	}
	if best < 2 {
		return
	}
	out.GridKnown = true
	out.ViewX, out.ViewY = uint8((16-phaseX)%16), uint8((16-phaseY)%16)
	for y := range out.Terrain {
		for x := range out.Terrain[y] {
			px, py := x*16-int(out.ViewX), y*16-int(out.ViewY)
			if botUniformCell(pixels, runs, px, py) {
				continue
			}
			if kind, known := v.visualTerrainCell(pixels, px, py); known {
				out.Terrain[y][x], out.TerrainKnown[y][x] = kind, true
			}
		}
	}
}
