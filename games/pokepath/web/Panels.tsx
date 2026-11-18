import { BoxPanel, type BoxDisplay } from "./BoxPanel";
import { ShopPanel } from "./ShopPanel";
import { useLocalize, useTranslate, useLanguage } from "./I18n";
import { useEffect, useRef, useState } from "react";
import type { Action, Assets, Challenge, Pokemon, View } from "./types";
import { ChallengePanel } from "./ChallengePanel";
import { SettingsPanel } from "./SettingsPanel";
import { ProfilePanel } from "./ProfilePanel";
import { DexPanel, InventoryPanel, RoutesPanel } from "./CollectionPanels";
import type { AssetStore } from "./assets";
import type { GameControllerBindings } from "../../../web/src/game-api/module";

export type Panel = "team" | "profile" | "damage" | "shop" | "box" | "inventory" | "routes" | "challenge" | "dex" | "settings" | "editor";
export const panels: { id: Panel; label: string }[] = [{ id: "profile", label: "训练家" }, { id: "damage", label: "伤害统计" }, { id: "team", label: "编队" }, { id: "shop", label: "商店" }, { id: "box", label: "仓库" }, { id: "inventory", label: "背包" }, { id: "routes", label: "路线" }, { id: "challenge", label: "挑战" }, { id: "dex", label: "图鉴" }, { id: "editor", label: "编辑器" }, { id: "settings", label: "设置" }];
export const text = (value?: string | string[]) => (Array.isArray(value) ? value[8] ?? value[0] : value ?? "").replace(/\[\/?[A-Z_]+\]/g, "");

export function RichText({ value, enabled = true, classic = false, literal = false, tagHints = false }: { value?: string | string[]; enabled?: boolean; classic?: boolean; literal?: boolean; tagHints?: boolean }) {
  const translate = useTranslate();
  const language = useLanguage();
  const source = Array.isArray(value) ? value[language] || value[0] || value[8] || "" : literal ? value ?? "" : translate(value ?? "");
  if (!enabled) return source.replace(/\[\/?[A-Z_]+\]/g, "");
  const colors: Record<string, string> = classic ? { POWER: "#ff5252", SPEED: "#42a5f5", RANGE: "#66bb6a", CRITICAL: "#ffca28", CRIT: "#ffca28", SHAPE: "#ab47bc", TERRAIN: "#8d6e63", POKEMON: "#588ced", BURN: "#fc3d03", POISON: "#4caf50", STUN: "#ffca28", SLOW: "#42a5f5", ABILITY: "#26a69a", ITEM: "#ffd166", ADMIN: "#ae3ec9", GOLD: "#70ac4c", CURSE: "#ab47bc", STATUS: "#ab47bc", STATS: "#26a69a", MONEY: "#70ac4c", HP: "#f06292", ARMOR: "#9c9797", TOWER: "#25a4db", ENEMY: "#de0929", ROUTE: "#8fcf21", MAP: "#cf1f5f", TEAM: "#1ccdd6", WEATHER: "#f0feff", PROJECTILE: "#d67c1c", SCORCH: "#87150f", NIGHTMARE: "#989899", STEAM: "#26a69a", WEB: "#26a69a", ETC: "#9c9797", STAR: "#e7b936", RIBBON: "#e0244a", SECRET: "#ab47bc", EGG: "#5c9e6f", FIELD: "#409552", GRASS: "#2f6f48", WATER: "#2c75b2", MOUNTAIN: "#8c6b5e", HEART: "#d7443e", RICOCHET: "#66bb6a", SPREAD: "#ab47bc", HONEY: "#26a69a", RAIN: "#0993e8", BADRAIN: "#553eed", GOODRAIN: "#ada926", SUN: "#c7a73e", BADSUN: "#a81e0c", HAIL: "#12bddb", SANDSTORM: "#d49b61", SANDAVALANCHE: "#bf7830", STRONGWINDS: "#6484a3", RANDOMSTATUS: "#ab47bc", STARRY: "#fcba03", FOG: "#b4b3ba", CANDY: "#588ced", SPIDER: "#4caf50", APPLE: "#fc3d03", MAGMA: "#911f45", RB: "#e34da2", BF: "#2bb368", ER: "#c4a93b", SW: "#5888cc", VR: "#6b5ab5", PERS: "#26a69a" } : { POWER: "#ff4d34", SPEED: "#68d4ff", RANGE: "#70ac4c", CRITICAL: "#f6dc4a", CRIT: "#f6dc4a", SHAPE: "#cc70d4", TERRAIN: "#af8f7d", POKEMON: "#ff8463", BURN: "#f97316", POISON: "#b57ad4", STUN: "#e8cf55", SLOW: "#6bbce7", ABILITY: "#c393db", ITEM: "#e9c35a", ADMIN: "#ae3ec9", GOLD: "#70ac4c" };
  const tags: string[] = [];
  return source.split(/(\[\/?[A-Z_]+\])/g).map((part, index) => {
    const marker = part.match(/^\[(\/?)([A-Z_]+)\]$/);
    if (marker) { if (marker[1]) { const at = tags.lastIndexOf(marker[2]); if (at >= 0) tags.splice(at); } else tags.push(marker[2]); return null; }
    const color = tags.map(tag => colors[tag]).filter(Boolean).at(-1);
    const admin = classic && tags.at(-1) === "ADMIN";
    const hint = tagHints && tags.some(tag => ["BURN", "POISON", "SLOW", "STUN", "SCORCH", "NIGHTMARE", "CURSE", "CRITICAL", "RICOCHET", "SPREAD", "RAIN", "SUN", "HAIL", "SANDSTORM", "SANDAVALANCHE", "STRONGWINDS", "STARRY", "FOG", "CANDY", "SPIDER", "APPLE", "MAGMA", "RB", "BF", "ER", "SW", "VR", "PERS"].includes(tag));
    return color ? <span key={index} className={admin ? "pokepath-rich-admin" : hint ? "pokepath-dex-tag-hint" : undefined} style={{ color: admin ? undefined : color, fontWeight: classic ? "bold" : undefined, textTransform: classic ? "uppercase" : undefined }}>{part}</span> : part;
  });
}

export function ResourceImage({ path, store }: { path: string; store: AssetStore }) {
  const localize = useLocalize();
  const element = useRef<HTMLImageElement>(null);
  const [url, setURL] = useState<string | undefined>(() => store.images.has(store.path(path)) ? store.url(path) : undefined);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let alive = true;
    setFailed(false);
    setURL(store.images.has(store.path(path)) ? store.url(path) : undefined);
    const load = () => { void store.image(path).then(() => { if (alive) setURL(store.url(path)); }).catch(() => { if (alive) setFailed(true); }); };
    const target = element.current?.parentElement;
    let observer: IntersectionObserver | undefined;
    if (store.images.has(store.path(path)) || !target || typeof IntersectionObserver === "undefined") load();
    else {
      observer = new IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { observer?.disconnect(); load(); } }, { rootMargin: "120px" });
      observer.observe(target);
    }
    return () => { alive = false; observer?.disconnect(); };
  }, [path, store]);
  return localize(failed ? <span className="pokepath-portrait-placeholder" role="img" aria-label="素材加载失败">◈</span> : <img ref={element} src={url} data-resource={path} alt="" />);
}

export function Portrait({ pokemon, assets, store }: { pokemon: Pokemon; assets: Assets; store?: AssetStore }) {
  const path = pokemon.sprite?.base ?? assets.catalog?.species[pokemon.species]?.sprite.base;
  return path && store ? <ResourceImage path={path} store={store} /> : <span className="pokepath-portrait-placeholder" aria-hidden="true">◈</span>;
}

export function Inspector({ pokemon, view, assets, store, disabled, onAct, onDeploy }: { pokemon?: Pokemon; view: View; assets: Assets; store?: AssetStore; disabled: boolean; onAct: (action: Action) => void; onDeploy: () => void }) {
  if (!pokemon) return <div className="pokepath-selection-empty">选择伙伴，查看属性与部署位置。</div>;
  const species = assets.catalog?.species[pokemon.species];
  const ability = species ? assets.catalog?.abilities[species.abilityId] : undefined;
  const deployed = view.towers.some(tower => tower.pokemonID === pokemon.id);
  return <section className="pokepath-inspector" data-selected-pokemon={pokemon.id}>
    <div className="pokepath-inspector-heading"><Portrait pokemon={pokemon} assets={assets} store={store} /><div><h3>{pokemon.alias || species?.name || pokemon.species}</h3><span>等级 {pokemon.level}</span></div></div>
    <dl className="pokepath-stats"><div><dt>攻击</dt><dd>{Math.round(pokemon.stats.power)}</dd></div><div><dt>间隔</dt><dd>{(pokemon.stats.attackMS / 1000).toFixed(2)} 秒</dd></div><div><dt>射程</dt><dd>{Math.round(pokemon.stats.range)}</dd></div></dl>
    {ability && <div className="pokepath-ability"><strong>{text(ability.name)}</strong><p><RichText value={ability.description} enabled={view.progression?.settings.colorTags} /></p></div>}
    <div className="pokepath-primary-actions"><button type="button" onClick={onDeploy} disabled={disabled} aria-label="部署">{deployed ? "移动" : "部署"}</button><button type="button" className="primary" aria-label="升级" disabled={disabled || pokemon.level >= 100 || view.gold < (pokemon.upgradeCost ?? 0)} onClick={() => onAct({ type: "upgrade", pokemonID: pokemon.id, count: 1 })}>升级{pokemon.upgradeCost ? <small>{pokemon.upgradeCost} 金币</small> : null}</button>{deployed && <button type="button" aria-label="收回" disabled={disabled} onClick={() => onAct({ type: "retire", pokemonID: pokemon.id })}>收回</button>}</div>
    <label className="pokepath-target">攻击目标<select aria-label="攻击目标" value={pokemon.target} disabled={disabled} onChange={event => onAct({ type: "target", pokemonID: pokemon.id, mode: event.target.value })}>{[["first", "最靠前"], ["last", "最后方"], ["highHP", "生命最多"], ["lowHP", "生命最少"], ["highArmor", "护甲最高"], ["noArmor", "无护甲"], ["faster", "速度最快"], ["slower", "速度最慢"], ["invisible", "隐身优先"], ["notBurned", "未灼伤"], ["notPoisoned", "未中毒"], ["random", "随机"]].map(([key, label]) => <option key={key} value={key}>{label}</option>)}</select></label>
    {pokemon.item && <div className="pokepath-equipped"><span>{text(assets.catalog?.items[pokemon.item]?.name) || pokemon.item}</span><button type="button" disabled={disabled} onClick={() => onAct({ type: "unequip", pokemonID: pokemon.id })}>卸下</button></div>}
    <div className="pokepath-secondary-actions"><button type="button" disabled={disabled} onClick={() => onAct({ type: "team", pokemonID: pokemon.id, mode: "box" })}>移至仓库</button><button type="button" disabled={disabled} onClick={() => onAct({ type: "favorite", pokemonID: pokemon.id, enabled: !pokemon.favorite })}>{pokemon.favorite ? "取消收藏" : "收藏"}</button></div>
  </section>;
}

export function GamePanel({ panel, view, assets, store, selected, disabled, controllerBindings, challengePlan, onChallengePlan, boxDisplay, onBoxDisplay, onAct, onSelect, onDetails, onPanel, onClose }: { panel: Panel; view: View; assets: Assets; store?: AssetStore; selected?: number; disabled: boolean; controllerBindings?: GameControllerBindings; challengePlan?: Challenge; onChallengePlan?: (plan: Challenge) => void; boxDisplay?: BoxDisplay; onBoxDisplay?: (display: BoxDisplay) => void; onAct: (action: Action) => void; onSelect: (id: number) => void; onDetails?: (id: number) => void; onPanel?: (panel: Panel) => void; onClose: () => void }) {
  const localize = useLocalize();
  const sectionProps = { view, assets, store, selected, disabled, onAct, onSelect, onDetails, onPanel, onClose };
  if (panel === "challenge") return <ChallengePanel {...sectionProps} plan={challengePlan} onPlan={onChallengePlan} />;
  if (panel === "settings") return localize(<SettingsPanel {...sectionProps} controllerBindings={controllerBindings} />);
  if (panel === "profile") return localize(<ProfilePanel {...sectionProps} />);
  if (panel === "inventory") return localize(<InventoryPanel {...sectionProps} />);
  if (panel === "routes") return localize(<RoutesPanel {...sectionProps} />);
  if (panel === "dex") return localize(<DexPanel {...sectionProps} />);
  if (panel === "shop") return <ShopPanel {...sectionProps} />;
  if (panel === "box") return <BoxPanel {...sectionProps} display={boxDisplay} onDisplay={onBoxDisplay} />;
  return null;
}
