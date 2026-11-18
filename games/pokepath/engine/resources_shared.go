package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
)

var activeReferenceCatalog atomic.Pointer[Catalog]
var referenceCatalogOnce sync.Once
var referenceCatalogInstall sync.Mutex

func CatalogData() *Catalog {
	referenceCatalogOnce.Do(func() { activeReferenceCatalog.Store(initialCatalog()) })
	return activeReferenceCatalog.Load()
}

func mergeReferenceMap[K comparable, V any](current, incoming map[K]V) map[K]V {
	merged := make(map[K]V, len(current)+len(incoming))
	for key, value := range current {
		merged[key] = value
	}
	for key, value := range incoming {
		merged[key] = value
	}
	return merged
}

func validateReferenceCore(catalog *Catalog) error {
	if catalog == nil || catalog.Version != "1.7.3" || len(catalog.Source.SourceSHA256) != 64 {
		return errors.New("规则事实目录版本不兼容")
	}
	if len(catalog.Species) != 517 || len(catalog.Enemies) != 347 || len(catalog.Routes) != 47 || len(catalog.Items) != 197 || len(catalog.Abilities) != 254 || len(catalog.Passives) != 85 || len(catalog.Weather) != 11 || len(catalog.Projectiles) != 62 || len(catalog.Collection) != 257 || len(catalog.Achievements) != 39 {
		return errors.New("规则事实目录不完整")
	}
	if len(catalog.Songs) != 41 || len(catalog.Playlist) != 41 {
		return errors.New("音乐事实目录不完整")
	}
	seenSongs := map[string]bool{}
	for order, key := range catalog.Playlist {
		song, exists := catalog.Songs[key]
		if !exists || seenSongs[key] || song.Key != key || song.ID != order || len(song.Names) != 10 || song.Audio != "src/assets/sounds/music/"+key+".mp3" {
			return errors.New("音乐曲序或音源无效")
		}
		for _, symbol := range key {
			if symbol < 'a' || symbol > 'z' {
				if symbol < '0' || symbol > '9' {
					return errors.New("音乐标识无效")
				}
			}
		}
		seenSongs[key] = true
	}
	for key, species := range catalog.Species {
		if species.Key != key || len(species.Terrains) == 0 || species.Sprite.Frames < 1 || species.Sprite.Rows < 1 {
			return fmt.Errorf("伙伴事实不完整：%s", key)
		}
		if _, ok := catalog.Abilities[species.AbilityID]; !ok {
			return fmt.Errorf("伙伴能力不存在：%s", key)
		}
		if _, ok := catalog.Projectiles[species.ProjectileID]; !ok {
			return fmt.Errorf("伙伴弹道不存在：%s", key)
		}
		if species.EvolutionKey != "" {
			if _, ok := catalog.Species[species.EvolutionKey]; !ok {
				return fmt.Errorf("伙伴进化不存在：%s", key)
			}
		}
		prices := catalog.UpgradeCosts[species.CostScale]
		if len(prices) != 101 || prices[0] != 0 {
			return fmt.Errorf("升级价格事实不完整：%s", species.CostScale)
		}
	}
	for key, enemy := range catalog.Enemies {
		if enemy.Health <= 0 || !finiteNumber(enemy.Health) || enemy.Speed < 0 || !finiteNumber(enemy.Speed) {
			return fmt.Errorf("敌人事实不完整：%s", key)
		}
		if enemy.PassiveID != "" {
			if _, ok := catalog.Passives[enemy.PassiveID]; !ok {
				return fmt.Errorf("敌人被动不存在：%s", key)
			}
		}
	}
	for key, route := range catalog.Routes {
		if strconv.Itoa(route.ID) != key || route.Width <= 0 || route.Height <= 0 {
			return fmt.Errorf("地图元数据不完整：%s", key)
		}
		if song, exists := catalog.Songs[route.MusicKey]; !exists || song.ID != route.MusicID {
			return fmt.Errorf("地图音乐引用无效：%s", key)
		}
		if len(route.Paths) > 0 || len(route.Waves) > 0 {
			if err := validateReferenceRoute(catalog, route); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateReferenceRoute(catalog *Catalog, route RouteSpec) error {
	if _, exists := catalog.Routes[strconv.Itoa(route.ID)]; !exists || route.Width <= 0 || route.Height <= 0 || len(route.Paths) == 0 || len(route.Placements) == 0 || len(route.Waves) == 0 {
		return errors.New("地图规则尚未完整加载")
	}
	if song, exists := catalog.Songs[route.MusicKey]; !exists || song.ID != route.MusicID {
		return errors.New("地图音乐引用无效")
	}
	if route.ID == 35 && len(route.ChessPlacements) != 16 || route.ID != 35 && len(route.ChessPlacements) != 0 {
		return errors.New("地图棋子事实不完整")
	}
	chessSpecies := map[byte]string{'h': "kingdra", 'q': "vespiquen", 'b': "bisharp", 'k': "kingambit", 'p': "pawniard", 't': "stakataka"}
	chessKeys := map[string]bool{}
	for _, piece := range route.ChessPlacements {
		if piece.Key == "" || chessKeys[piece.Key] || chessSpecies[piece.Key[0]] != piece.Species || !finiteNumber(piece.X) || !finiteNumber(piece.Y) || piece.X < 0 || piece.Y < 0 || piece.X >= float64(route.Width) || piece.Y >= float64(route.Height) || math.Mod(piece.X, 24) != 0 || math.Mod(piece.Y, 24) != 0 {
			return errors.New("地图棋子事实无效")
		}
		if _, exists := catalog.Species[piece.Species]; !exists {
			return errors.New("地图棋子物种不存在")
		}
		chessKeys[piece.Key] = true
	}
	for _, points := range append(append([][]Point{}, route.Paths...), route.MirrorPaths...) {
		if len(points) < 2 {
			return errors.New("地图路径不完整")
		}
		for _, point := range points {
			if math.IsNaN(point.X) || math.IsNaN(point.Y) || math.IsInf(point.X, 0) || math.IsInf(point.Y, 0) {
				return errors.New("地图坐标无效")
			}
		}
	}
	occupied := map[int]bool{}
	for _, tile := range route.Placements {
		if occupied[tile.ID] || tile.ID < 0 || tile.Terrain < 1 || tile.Terrain > 4 || tile.X < 0 || tile.Y < 0 || tile.X >= float64(route.Width) || tile.Y >= float64(route.Height) {
			return errors.New("地图部署位置无效")
		}
		occupied[tile.ID] = true
	}
	for _, wave := range route.Waves {
		if len(wave.Sequence) == 0 {
			return errors.New("地图波次不完整")
		}
		for _, key := range wave.Preview {
			if _, exists := catalog.Enemies[key]; !exists {
				return errors.New("地图预览引用未知敌人")
			}
		}
		for _, key := range wave.Sequence {
			if key == nil {
				continue
			}
			if _, exists := catalog.Enemies[*key]; !exists {
				return errors.New("地图波次引用未知敌人")
			}
		}
	}
	return nil
}

func InstallCatalog(raw []byte) error {
	if len(raw) == 0 || len(raw) > 16<<20 {
		return errors.New("规则目录大小无效")
	}
	var incoming Catalog
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return errors.New("规则目录格式无效")
	}
	if err := validateReferenceCore(&incoming); err != nil {
		return err
	}
	referenceCatalogInstall.Lock()
	defer referenceCatalogInstall.Unlock()
	previous := CatalogData()
	if previous.Version != "" && previous.Source.SourceSHA256 != incoming.Source.SourceSHA256 {
		return errors.New("不能混用不同版本的规则目录")
	}
	incoming.Routes = mergeReferenceMap(previous.Routes, incoming.Routes)
	for key, old := range previous.Routes {
		if len(old.Paths) > 0 && len(incoming.Routes[key].Paths) == 0 {
			incoming.Routes[key] = old
		}
	}
	activeReferenceCatalog.Store(&incoming)
	return nil
}

func InstallRoute(raw []byte) error {
	if len(raw) == 0 || len(raw) > 16<<20 {
		return errors.New("地图数据大小无效")
	}
	var route RouteSpec
	if err := json.Unmarshal(raw, &route); err != nil {
		return errors.New("地图数据格式无效")
	}
	referenceCatalogInstall.Lock()
	defer referenceCatalogInstall.Unlock()
	previous := CatalogData()
	if previous.Version == "" {
		return errors.New("请先加载规则核心目录")
	}
	if err := validateReferenceRoute(previous, route); err != nil {
		return err
	}
	updated := *previous
	updated.Routes = mergeReferenceMap(previous.Routes, map[string]RouteSpec{strconv.Itoa(route.ID): route})
	activeReferenceCatalog.Store(&updated)
	return nil
}
