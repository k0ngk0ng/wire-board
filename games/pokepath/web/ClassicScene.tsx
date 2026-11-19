import { hasRouteResult } from "./ResultPanel";
import { ChallengeClock } from "./ChallengeClock";
import { challengeLocksPanel } from "./ChallengePanel";
import { useLanguage, useLocalize } from "./I18n";
import { formatGameGold } from "./number-format";
import { useEffect, useRef, useState, type ReactNode, type Ref } from "react";
import type { Action, Assets, Point, Pokemon, View } from "./types";
import { fixedTargets, targetOptions } from "./target-modes";
import { DamageMonitor, type DamageMode } from "./DamageMonitor";
import { FastPicker } from "./FastPicker";
import type { AssetStore } from "./assets";
import { Stage, type StageHandle } from "./Stage";
import type { PlacementQuery, RangeQuery } from "./placement";
import { Portrait, ResourceImage, RichText, type Panel, text } from "./Panels";

const sections: { panel: Panel; icon: string; label: string }[] = [{ panel: "profile", icon: "profile", label: "训练家" }, { panel: "box", icon: "box", label: "仓库" }, { panel: "inventory", icon: "inventory", label: "背包" }, { panel: "shop", icon: "shop", label: "商店" }, { panel: "routes", icon: "map", label: "路线" }, { panel: "challenge", icon: "challenge", label: "挑战" }, { panel: "dex", icon: "dex", label: "图鉴" }, { panel: "damage", icon: "damageDealt", label: "伤害统计" }, { panel: "settings", icon: "menu", label: "设置" }];

export type ClassicSceneProps = { view: View; assets: Assets; store?: AssetStore; selected?: number; placing: boolean; allRanges?: boolean; damageOpen?: boolean; damageMode?: DamageMode; onDamageToggle?: () => void; onDamageMode?: (mode: DamageMode) => void; range?: number; onPointerPoint?: (point: Point) => void; onPointerCancel?: () => void; panel: Panel; disabled: boolean; stage: Ref<StageHandle>; onAct: (action: Action) => void; onSelect: (id: number) => void; onDetails: (id: number) => void; onAim?: (id: number) => void; onInspect?: (id: number) => Pokemon | undefined; onPanel: (panel: Panel) => void; placementPreview?: PlacementQuery; placementRange?: RangeQuery; onPoint: (point: Point, fromController?: boolean, touch?: boolean) => void; onHover?: (id?: number) => void; onReady: () => void; modalOpen?: boolean; children?: ReactNode };

export function ClassicScene({ view, assets, store, selected, placing, allRanges, damageOpen, damageMode, onDamageToggle, onDamageMode, range, onPointerPoint, onPointerCancel, panel, disabled, stage, onAct, onSelect, onDetails, onAim, onInspect, onPanel, placementPreview, placementRange, onPoint, onHover, onReady, modalOpen, children }: ClassicSceneProps) {
  const localize = useLocalize();
  const language = useLanguage();
  const [localDamageOpen, setLocalDamageOpen] = useState(false);
  const [localDamageMode, setLocalDamageMode] = useState<DamageMode>("trueDamage");
  const showDamage = damageOpen ?? localDamageOpen;
  const toggleDamage = onDamageToggle ?? (() => setLocalDamageOpen(value => !value));
  const viewport = useRef<HTMLDivElement>(null);
  const [enemyKey, setEnemyKey] = useState<string>();
  const [picker, setPicker] = useState<{ slot: number; mode: "pokemon" | "item"; pokemon?: Pokemon }>();
  useEffect(() => {
    const close = (event: PointerEvent) => { if (!(event.target as HTMLElement).closest("[data-fast-picker],[data-fast-opener]")) setPicker(undefined); };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, []);
  const wave = assets.route?.waves[String(view.wave)];
  const preview = [...new Set(wave?.preview ?? [])];
  const enemy = assets.catalog?.enemies[enemyKey ?? preview[0]];
  const activeEnemy = view.enemies.find(candidate => candidate.key === (enemyKey ?? preview[0]));
  const icon = (key: string) => store?.url(`src/assets/images/icons/${key}.png`);
  const capacities = [40, 160, 320, 540];
  const slots = view.teamSlots ?? 6;
  useEffect(() => {
    const element = viewport.current;
    if (!element) return;
    const resize = () => element.style.setProperty("--pp-scene-scale", String(Math.min(element.clientWidth / 1238, element.clientHeight / 674)));
    const observer = new ResizeObserver(resize);
    observer.observe(element);
    resize();
    return () => observer.disconnect();
  }, []);
  useEffect(() => { setEnemyKey(undefined); }, [view.route, view.wave]);
  const routeName = assets.route?.name ?? assets.catalog?.routes[String(view.route)]?.name ?? "路線 1-1";
  const lockedSection = (panel: Panel) => challengeLocksPanel(view, panel);
  const inputsBlocked = Boolean(view.progression?.draft?.active) || hasRouteResult(view.phase) || Boolean(modalOpen);
  const inChallenge = Boolean(view.progression?.challenge?.active);
  return localize(<div className="pokepath-classic-viewport" ref={viewport}><div className="pokepath-classic-scene" data-logical-width="1238" data-logical-height="674" data-game-controls onKeyDown={event => { if (event.key === "Escape") setPicker(undefined); }}>
    <aside className="pokepath-trainer" inert={inputsBlocked}>
      <header className="pokepath-trainer-heading"><span className="pokepath-trainer-portrait">{store && <ResourceImage path={`src/assets/images/portraits/${view.progression?.portrait ?? 0}.png`} store={store} />}</span><strong className="pokepath-trainer-name" data-i18n-literal>{(view.progression?.profileName ?? "Player").toUpperCase()}</strong><span className="pokepath-trainer-stars">⭐{view.stars}</span><span className="pokepath-trainer-ribbons">{store && <img src={icon("ribbon1")} alt="" />}{view.ribbons}</span><span className="pokepath-trainer-gold" aria-label={`共享金币 ${view.gold}`}>${formatGameGold(view.gold, language)}</span><span className="pokepath-trainer-hearts" aria-label={`共享生命 ${view.health}`} role="img">{Array.from({ length: view.maxHealth ?? 14 }, (_, index) => <span key={index} style={{ backgroundImage: store ? `url("${icon(index < view.health ? "heart" : "no-heart")}")` : undefined }} />)}</span></header>
      <div className="pokepath-classic-party">{Array.from({ length: 10 }, (_, index) => {
        const pokemon = view.team[index];
        const specie = pokemon ? assets.catalog?.species[pokemon.species] : undefined;
        const availableTargets = pokemon?.targetModes?.length ? targetOptions.filter(option => pokemon.targetModes!.includes(option[0])) : [];
        const targetIndex = pokemon ? availableTargets.findIndex(option => option[0] === pokemon.target) : -1;
        const targetLocked = !availableTargets.length;
        const changeTarget = (delta: number) => { if (pokemon && !targetLocked) onAct({ type: "target", pokemonID: pokemon.id, mode: availableTargets[(targetIndex + delta + availableTargets.length) % availableTargets.length][0] }); };
        const shape = pokemon?.combatRangeType ?? specie?.rangeType ?? "circle";
        const symbol = ({ cross: "+", xShape: "X", vShape: "V", star: "*", horizontalLine: "−", verticalLine: "|", donut: "O", circle: "O" } as Record<string, string>)[shape] ?? "O";
        const bombardment = specie?.attackType === "bombardment";
        const deployed = pokemon && view.towers.some(tower => tower.pokemonID === pokemon.id);
        return <article key={pokemon?.id ?? `slot-${index}`} data-party-slot={index} data-pokemon-id={pokemon?.id} className={`pokepath-classic-card${pokemon ? " occupied" : index < slots ? " empty" : " locked"}${pokemon?.id === selected ? " selected" : ""}`} style={specie?.color ? { backgroundColor: `${specie.color}33` } : undefined} onClick={event => { if (pokemon && !(event.target as Element).closest("button,input,select,[data-fast-picker]")) onSelect(pokemon.id); }}>
          <strong className="pokepath-card-name" style={pokemon ? { color: specie?.color } : undefined}>{pokemon ? pokemon.alias || specie?.name || pokemon.species : index < slots ? "空位" : "已锁定"}</strong>
          {pokemon ? <><span className="pokepath-card-level">Lv {pokemon.level}</span>{view.progression?.settings.indicatorField && <span className={`pokepath-card-field ${deployed ? "deployed" : "undeployed"}`} aria-label={deployed ? "已部署" : "未部署"}>{deployed ? "✓" : "✕"}</span>}{view.progression?.settings.indicatorShape && <span className="pokepath-card-shape" title={shape}>{specie?.attackType === "area" ? "A" : specie?.attackType === "aura" ? "~" : specie?.attackType === "bombardment" ? "B" : specie?.attackType === "stream" ? ">" : symbol}</span>}{view.progression?.settings.fastTarget && <div className="pokepath-card-fast-target"><button type="button" aria-label="上一个快速目标" disabled={disabled || targetLocked} onClick={() => changeTarget(-1)}>❮</button><span>{fixedTargets[pokemon.target] ?? targetOptions.find(option => option[0] === pokemon.target)?.[1] ?? pokemon.target}</span><button type="button" aria-label="下一个快速目标" disabled={disabled || targetLocked} onClick={() => changeTarget(1)}>❯</button></div>}<button type="button" className="pokepath-card-portrait" aria-label={`选择${specie?.name ?? pokemon.species}`} onClick={() => onSelect(pokemon.id)}><Portrait pokemon={pokemon} assets={assets} store={store} /></button><div className="pokepath-card-buttons"><button type="button" className="deploy" aria-label={deployed ? `收回${specie?.name ?? pokemon.species}` : `部署${specie?.name ?? pokemon.species}`} disabled={disabled} onClick={() => deployed ? onAct({ type: "retire", pokemonID: pokemon.id }) : onSelect(pokemon.id)} style={{ backgroundImage: store ? `url("${icon(deployed ? "pokeball-open" : "pokeball-close")}"), linear-gradient(var(--pp-uiPokemonButton1a, #b23d27), var(--pp-uiPokemonButton1b, #9d3829))` : undefined }} /><button type="button" className="info" aria-label={`查看${specie?.name ?? pokemon.species}详情`} onClick={() => onDetails(pokemon.id)}>i</button><button type="button" className="upgrade" aria-label={bombardment ? `瞄准${specie?.name ?? pokemon.species}` : `升级${specie?.name ?? pokemon.species}`} disabled={disabled || (bombardment ? !deployed : pokemon.level >= 100 || view.gold < (pokemon.upgradeCost ?? 0))} onClick={() => bombardment ? onAim?.(pokemon.id) : onAct({ type: "upgrade", pokemonID: pokemon.id, count: 1 })}>{bombardment ? "◎" : "+1"}</button><button type="button" className="item" data-fast-opener aria-label={`为${specie?.name ?? pokemon.species}装备道具`} disabled={disabled} onClick={() => setPicker(previous => previous?.slot === index && previous.mode === "item" ? undefined : { slot: index, mode: "item", pokemon: onInspect?.(pokemon.id) ?? pokemon })}>+</button></div></> : index < slots ? <button type="button" className="pokepath-card-add" data-fast-opener aria-label="添加伙伴" onClick={() => setPicker(previous => previous?.slot === index && previous.mode === "pokemon" ? undefined : { slot: index, mode: "pokemon" })}>+</button> : <span className="pokepath-card-requirement"><span className="pokepath-card-lock" aria-hidden="true">🔒</span><br />⭐{capacities[index - 6]}</span>}
          {picker?.slot === index && (picker.mode === "pokemon" && !pokemon || picker.mode === "item" && picker.pokemon?.id === pokemon?.id) && <FastPicker mode={picker.mode} pokemon={picker.pokemon} view={view} assets={assets} store={store} disabled={disabled} onAct={onAct} onClose={() => setPicker(undefined)} />}
        </article>;
      })}</div>
    </aside>
    <div className="pokepath-central" inert={inputsBlocked}>
      <div className="pokepath-classic-top"><div className="pokepath-formations-save">{Array.from({ length: 5 }, (_, slot) => <button type="button" key={slot} title={`保存编队 ${slot + 1}`} aria-label={`保存编队 ${slot + 1}`} disabled={disabled} onClick={() => onAct({ type: "saveTeam", slot })}>#{slot + 1}</button>)}</div><div className="pokepath-classic-route"><div>{routeName}<br />波次 {view.wave}</div><span>⭐{view.records?.[String(view.route)] ?? 0}</span></div><div className="pokepath-formations-load">{Array.from({ length: 5 }, (_, slot) => <button type="button" key={slot} title={`套用编队 ${slot + 1}`} aria-label={`套用编队 ${slot + 1}`} disabled={disabled || !view.progression?.formations?.[slot]?.members?.length} onClick={() => onAct({ type: "loadTeam", slot })}>#{slot + 1}</button>)}</div></div>
      <Stage ref={stage} view={view} assets={assets} selected={placing ? selected : range} placing={placing} allRanges={allRanges} onPointerPoint={onPointerPoint} onPointerCancel={onPointerCancel} placementPreview={placementPreview} placementRange={placementRange} onPoint={onPoint} onHover={onHover} onReady={onReady} />
      <div className="pokepath-classic-bottom" data-challenge-active={inChallenge || undefined}>{view.progression?.canManageWaves && <div className="pokepath-wave-manager">{[-10, -1].map(value => <button type="button" key={value} aria-label={`波次 ${value}`} disabled={disabled || view.waveActive} onClick={() => onAct({ type: "selectWave", mode: "delta", value })}>{value === -10 ? "≪" : "＜"}</button>)}<input aria-label="选择波次" type="number" min={1} max={assets.catalog?.routes[String(view.route)]?.waveCount ?? 100} value={view.wave} disabled={disabled || view.waveActive} onChange={event => onAct({ type: "selectWave", value: Number(event.target.value) })} />{[1, 10].map(value => <button type="button" key={value} aria-label={`波次 +${value}`} disabled={disabled || view.waveActive} onClick={() => onAct({ type: "selectWave", mode: "delta", value })}>{value === 10 ? "≫" : "＞"}</button>)}<button type="button" aria-label="重复波次" aria-pressed={view.progression.waveRepeat} disabled={disabled} onClick={() => onAct({ type: "repeatWave", enabled: !view.progression?.waveRepeat })}>↻</button><button type="button" aria-label="镜像波次" aria-pressed={view.progression.waveMirror} disabled={disabled} onClick={() => onAct({ type: "mirrorWave", enabled: !view.progression?.waveMirror })}>↔</button></div>}{inChallenge ? <ChallengeClock elapsedMS={view.progression?.challengeElapsedMS ?? 0} /> : !view.progression?.custom && <span className="pokepath-wave-income">WAVE ${Number(view.waveGold ?? 0)} | ${view.waveElapsedMS ? Math.floor(Number(view.waveGold ?? 0) * 60000 / view.waveElapsedMS) : 0}/min</span>}<div className="pokepath-terrain-counts">{[1, 2, 3, 4].map(terrain => <span key={terrain} data-terrain={terrain}>{view.towers.filter(tower => view.tiles.find(tile => tile.id === tower.tileID)?.terrain === terrain).length}/{view.tiles.filter(tile => tile.terrain === terrain).length}</span>)}</div></div>
    </div>
    <aside className="pokepath-monitor" inert={inputsBlocked}><nav className="pokepath-classic-menu" aria-label="游戏面板" style={store ? { backgroundImage: `url("${store.url("src/assets/images/textures/texture1.png")}")` } : undefined}>{sections.map(section => <button type="button" key={section.panel} data-classic-section={section.panel} aria-label={section.label} title={section.panel === "challenge" && lockedSection(section.panel) ? view.progression?.challengeReason || section.label : section.label} disabled={lockedSection(section.panel)} aria-pressed={section.panel === "damage" ? showDamage : panel === section.panel} onClick={() => section.panel === "damage" ? toggleDamage() : onPanel(panel === section.panel ? "team" : section.panel)}>{store ? <img src={icon(section.icon)} alt="" /> : <span aria-hidden="true">□</span>}</button>)}</nav>
      <section className="pokepath-enemy-info"><div className="pokepath-enemy-preview">{preview.map(key => { const spec = assets.catalog?.enemies[key]; return <button type="button" key={key} aria-label={`查看${text(spec?.name)}`} onClick={() => setEnemyKey(key)}>{store && spec?.sprite.base && <img src={store.url(spec.sprite.base)} alt="" />}<span>x{wave?.sequence.filter(value => value === key).length ?? 0}</span></button>; })}</div><h3>{text(enemy?.name)}</h3><dl>{[["生命", activeEnemy?.maxHealth ?? Number(enemy?.health ?? 0)], ["护甲", activeEnemy?.maxArmor ?? Number(enemy?.armor ?? 0)], ["速度", Number(enemy?.speed ?? 0) / 60], ["力量", Number(enemy?.power ?? 0)], ["再生", `${Number(enemy?.regeneration ?? 0)}/S`], ["眩晕", enemy?.canStun ? "弱" : "强"], ["减速", enemy?.canSlow ? "弱" : "强"], ["燃烧", enemy?.canBurn ? "弱" : "强"], ["中毒", enemy?.canPoison ? "弱" : "强"], ["隐形", enemy?.invisible ? "是" : "否"], ["金币", `$${Number(enemy?.gold ?? 0)}`]].map(([label, value]) => <div key={label} className={value === "弱" ? "weak" : value === "强" || label === "隐形" && value === "否" ? "strong" : ""}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>{Boolean(enemy?.passiveId) && assets.catalog?.passives[String(enemy?.passiveId)] && <section className="pokepath-enemy-passive"><strong>{text(assets.catalog.passives[String(enemy?.passiveId)].name)}</strong><p><RichText value={assets.catalog.passives[String(enemy?.passiveId)].description} enabled={view.progression?.settings.colorTags} /></p></section>}</section>
      {showDamage && <DamageMonitor view={view} assets={assets} store={store} mode={damageMode ?? localDamageMode} onMode={onDamageMode ?? setLocalDamageMode} />}
      <div className="pokepath-classic-controls" data-game-controls><button type="button" className="pause" aria-label={view.paused ? "继续" : "暂停"} disabled={disabled} onClick={() => onAct({ type: view.paused ? "resume" : "pause" })}>{view.paused ? "▶" : "||"}</button><button type="button" className="auto" aria-label="自动波次" aria-pressed={view.autoWave} disabled={disabled} onClick={() => onAct({ type: "auto" })}>自动</button><button type="button" className="speed" aria-label="速度" title={`${view.speed}×`} data-pokepath-action="speed" disabled={disabled} onClick={() => onAct({ type: "speed" })}>🚀</button><button type="button" className="wave" aria-label="下一波" disabled={disabled || view.waveActive || view.paused || view.health <= 0} onClick={() => onAct({ type: "wave" })}>{view.waveActive ? "抵御中" : "下一波"}</button></div>
    </aside>
    {children}
  </div></div>);
}
