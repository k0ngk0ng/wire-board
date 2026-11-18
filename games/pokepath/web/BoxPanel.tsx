import { useLayoutEffect, useRef, useState, type CSSProperties, type PointerEvent } from "react";
import { useFact, useLanguage, useTranslate } from "./I18n";
import { useResourceURL } from "./resource-url";
import type { AssetStore } from "./assets";
import type { Action, Assets, Catalog, Pokemon, Species, View } from "./types";

const sizes = [44, 50, 58, 67];
const sortKeys = ["team", "alphabetical", "level", "ability", "grass", "water", "mountain", "power", "speed", "critical", "range", "shiny", "acquisition"] as const;
const tabs = ["allTab", "fieldTab", "grassTab", "waterTab", "mountainTab", "fossilTab"];
const fossils = new Set([58, 59, 63, 64, 65, 66, 94, 140, 136, 151, 247]);
const searchGroups: Record<string, number[]> = {
  stun: [13, 72, 133, 136, 162, 204, 210, 183, 222, 192], slow: [54, 64, 80, 128, 170, 195], ricochet: [72], heal: [83, 103, 109, 231, 225, 207], curse: [16, 180, 73], burn: [0, 73, 74, 107, 120, 181, 197, 226, 228, 211, 214], poison: [169, 196, 202, 6, 203, 222], armor: [8, 151, 138], splash: [54, 55, 109], nightmare: [69, 91, 154], paradox: [178, 179, 238, 239], "iron thorns": [178], "sandy shocks": [179]
};
const speciesTerms: Record<string, string[]> = { stun: ["vikavolt", "tapuKoko", "kyogre"], slow: ["grubbin", "charjabug", "kyogre", "tapuFini", "glalie", "snorunt"], cone: ["charizard", "drampa", "flutterMane", "glalie", "snorunt", "megaCharizardY"] };
const icons = "src/assets/images/icons/";
const word = (value: unknown, language: number) => Array.isArray(value) ? String(value[language] ?? value[0] ?? "") : typeof value === "string" ? value : "";
const baseName = (name: string) => name.replace(/^m-/i, "");
const rgbDark = (value: string) => /^#[\da-f]{6}$/i.test(value) ? `rgb(${[1, 3, 5].map(index => Math.floor(Number.parseInt(value.slice(index, index + 2), 16) * .6)).join(", ")})` : value;

type Entry = { pokemon: Pokemon; specie?: Species; name: string; ability: string; team: boolean; terrains: number[] };
type Drag = { id: number; pointer: number; source: HTMLButtonElement; originX: number; originY: number; slots: number[]; active: boolean };
type Ghost = { id: number; x: number; y: number; scale: number };
type Sort = (typeof sortKeys)[number];
export type BoxDisplay = { sort: number; size: number };

function entryFor(pokemon: Pokemon, team: boolean, catalog: Catalog | undefined, language: number): Entry {
  const specie = catalog?.species[pokemon.species];
  const ability = specie && catalog?.abilities?.[specie.abilityId];
  return { pokemon, specie, name: word(specie?.names ?? specie?.name, language) || pokemon.species, ability: word(ability?.names ?? ability?.name, language), team, terrains: specie?.terrains ?? [] };
}

function matches(entry: Entry, query: string) {
  const { pokemon, specie, name, ability } = entry;
  if (baseName(name.toLowerCase()).includes(query) || pokemon.alias?.toLowerCase().includes(query) || ability.toLowerCase().includes(query)) return true;
  if (query === "shiny" && pokemon.shiny) return true;
  if (["area", "aoe"].includes(query) && specie?.attackType === "area") return true;
  const shapes: Record<string, string> = { x: "xShape", v: "vShape", star: "star", cross: "cross", "+": "cross", "-": "horizontalLine" };
  if (shapes[query] && shapes[query] === specie?.rangeType) return true;
  if (["bombardment", "orbital", "aura"].some(type => type.includes(query) && type === specie?.attackType)) return true;
  if (specie && Object.entries(searchGroups).some(([tag, ids]) => tag.includes(query) && ids.includes(specie.id))) return true;
  if (Object.entries(speciesTerms).some(([tag, keys]) => tag.includes(query) && keys.includes(pokemon.species))) return true;
  if ("beam".includes(query) && ["vikavolt", "tapuKoko", "kyogre"].includes(specie?.abilityId ?? "")) return true;
  if (["sword", "soj"].includes(query) && [165, 175, 176, 233].includes(specie?.id ?? -1)) return true;
  if (["forces", "nature", "fon"].includes(query) && [220, 227, 230].includes(specie?.id ?? -1)) return true;
  if (["-+", "+-"].includes(query) && ["minun", "plusle"].includes(pokemon.species)) return true;
  if (["ub", "ultra", "beast", "ultrabeast"].includes(query) && ["stakataka", "nihilego", "pheromosa"].includes(pokemon.species)) return true;
  return "mega".includes(query) && Boolean(specie?.facts?.mega || specie?.facts?.base);
}

function compare(mode: Sort, a: Entry, b: Entry) {
  if (mode === "team" && a.team !== b.team) return Number(b.team) - Number(a.team);
  if (mode === "shiny") return Number(Boolean(b.pokemon.shiny)) - Number(Boolean(a.pokemon.shiny));
  const favorite = Number(Boolean(b.pokemon.favorite)) - Number(Boolean(a.pokemon.favorite));
  if (favorite) return favorite;
  switch (mode) {
    case "alphabetical": return baseName(a.name).localeCompare(baseName(b.name));
    case "level": return b.pokemon.level - a.pokemon.level;
    case "ability": return a.ability.localeCompare(b.ability);
    case "grass": case "water": case "mountain": {
      const terrain = { grass: 2, water: 3, mountain: 4 }[mode];
      return Number(b.terrains.includes(terrain)) - Number(a.terrains.includes(terrain));
    }
    case "power": return b.pokemon.stats.power - a.pokemon.stats.power;
    case "speed": return a.pokemon.stats.attackMS - b.pokemon.stats.attackMS;
    case "critical": return b.pokemon.stats.criticalPct - a.pokemon.stats.criticalPct;
    case "range": return b.pokemon.stats.range - a.pokemon.stats.range;
    case "acquisition": return b.pokemon.id - a.pokemon.id;
    default: return 0;
  }
}

function unitLabel(entry: Entry, mode: Sort, cap?: number) {
  const { stats, level } = entry.pokemon;
  if (mode === "level") return `Lv ${cap || level}`;
  if (mode === "ability") return <span style={{ lineHeight: "7px" }}>{entry.ability}</span>;
  if (mode === "grass" || mode === "water" || mode === "mountain") return entry.terrains.flatMap(terrain => {
    const mark = ({ 2: ["G", "#409552"], 3: ["W", "#5ea1d9"], 4: ["M", "#b89184"] } as Record<number, string[]>)[terrain];
    return mark ? [<span key={terrain} style={{ color: mark[1], fontSize: 7, letterSpacing: 2 }}>{mark[0]}</span>] : [];
  });
  if (mode === "power") return stats.power;
  if (mode === "speed") return (stats.attackMS / 1000).toFixed(2);
  if (mode === "critical") return `${stats.criticalPct.toFixed(1)}%`;
  if (mode === "range") return stats.range;
  return entry.name;
}

function attackShape({ pokemon, specie }: Entry) {
  const id = specie?.abilityId ?? "";
  if (["icyWind", "dazzlingGleam"].includes(id)) return ["cone", "C"];
  if (["secretSword", "intrepidSword"].includes(id) || pokemon.species === "samurottHisui") return ["sweep", "S"];
  if (["boomburst", "liquidVoice", "drumBeating", "originPulse", "charmedCharm", "emptyLight"].includes(id)) return ["waves", "W"];
  const attack = specie?.attackType;
  const range = specie?.rangeType ?? "circle";
  const kind = attack === "stream" ? "stream" : range !== "circle" ? range : attack === "area" || attack === "aura" ? attack : "circle";
  const symbols: Record<string, string> = { cross: "+", xShape: "X", vShape: "V", star: "*", horizontalLine: "-", area: "A", aura: "~", bombardment: "B", stream: ">" };
  return [kind, symbols[attack ?? ""] ?? symbols[range] ?? "O"];
}

function BoxCard({ entry, store, assets, selected, indicator, size, label, dim, clearTint, moving, onChoose, onTransfer, onStart, onMove, onEnd, onCancel }: { entry: Entry; store?: AssetStore; assets: Assets; selected: boolean; indicator: boolean; size: number; label: ReturnType<typeof unitLabel>; dim: boolean; clearTint: boolean; moving: boolean; onChoose: () => void; onTransfer: () => void; onStart: (event: PointerEvent<HTMLButtonElement>) => void; onMove: (event: PointerEvent<HTMLButtonElement>) => void; onEnd: (event: PointerEvent<HTMLButtonElement>) => void; onCancel: () => void }) {
  const node = useRef<HTMLButtonElement>(null);
  const [visible, setVisible] = useState(false);
  useLayoutEffect(() => {
    const element = node.current;
    if (!element || visible) return;
    if (typeof IntersectionObserver === "undefined") { setVisible(true); return; }
    const observer = new IntersectionObserver(records => { if (records.some(record => record.isIntersecting)) { setVisible(true); observer.disconnect(); } }, { root: element.parentElement, rootMargin: "80px" });
    observer.observe(element);
    return () => observer.disconnect();
  }, [visible]);
  const path = entry.pokemon.sprite?.base ?? entry.specie?.sprite.base;
  const item = assets.catalog?.items?.[entry.pokemon.item]?.sprite;
  const iconURL = useResourceURL(visible && typeof item === "string" ? item : undefined, store);
  const shinyURL = useResourceURL(entry.pokemon.shiny ? icons + "shiny.png" : undefined, store);
  const url = useResourceURL(visible ? path : undefined, store);
  const dimensions = path && store?.images.get(store.path(path));
  const ratio = 1 + size * .1;
  const backgroundSize = dimensions ? `${dimensions.naturalWidth * ratio}px ${dimensions.naturalHeight * ratio}px` : undefined;
  const [shape, marker] = attackShape(entry);
  return <button ref={node} type="button" className="pokepath-box-unit" data-box-pokemon={entry.pokemon.id} data-box-member={entry.team ? entry.specie?.id === 70 ? "special" : "team" : undefined} data-box-current={selected || undefined} data-box-dim={dim || undefined} data-box-clear-tint={clearTint || undefined} data-box-moving={moving || undefined} data-box-inverted={entry.pokemon.item === "inverter" || undefined} data-resource={path} aria-label={entry.name} aria-pressed={selected} style={{ backgroundImage: url && !moving ? `url("${url}")` : undefined, backgroundSize }} onClick={onChoose} onDoubleClick={onTransfer} onPointerDown={onStart} onPointerMove={onMove} onPointerUp={onEnd} onPointerCancel={onCancel} onLostPointerCapture={onCancel}>
    <span className="pokepath-box-unit-label">{label}</span>{entry.pokemon.favorite && <span className="pokepath-box-unit-fav" aria-hidden="true">⭐</span>}{shinyURL && <span className="pokepath-box-unit-shiny" aria-hidden="true" style={{ backgroundImage: `url("${shinyURL}")` }} />}{indicator && <span className="pokepath-box-unit-shape" data-box-attack-shape={shape} aria-hidden="true">{marker}</span>}{iconURL && <span className="pokepath-box-unit-item" aria-hidden="true" style={{ backgroundImage: `url("${iconURL}")` }} />}
  </button>;
}

function RenameBox({ entry, font, onClose, onAct }: { entry: Entry; font?: string; onClose: () => void; onAct: (action: Action) => void }) {
  const fact = useFact();
  const translate = useTranslate();
  const [name, setName] = useState("");
  const label = fact("text.changeName.title").toUpperCase() || "改名";
  const color = rgbDark(entry.specie?.color ?? "#a33635");
  const send = (restore: boolean) => { if (restore || name.trim()) onAct({ type: "rename", pokemonID: entry.pokemon.id, name: restore ? "" : name }); onClose(); };
  return <div className="pokepath-details-overlay pokepath-box-rename-overlay" onClick={event => { if (event.target === event.currentTarget) onClose(); }}>
    <form className="pokepath-box-rename" data-box-font={font} role="dialog" aria-modal="true" aria-label={label} style={{ "--pp-box-rename-color": color } as CSSProperties} onSubmit={event => { event.preventDefault(); send(false); }} onKeyDown={event => { if (event.key === "Escape") { event.stopPropagation(); onClose(); } }}>
      <div className="pokepath-box-rename-head">{label}</div><div className="pokepath-box-rename-body">
        <input type="text" maxLength={10} autoFocus data-box-rename-input aria-label={translate("伙伴昵称")} value={name} placeholder={entry.pokemon.alias || entry.name.toUpperCase()} onChange={event => setName(event.target.value)} />
        <button type="button" data-box-rename-restore onClick={() => send(true)}>{fact("text.changeName.restore").toUpperCase() || "恢复"}</button><button type="submit" data-box-rename-accept>{fact("text.changeName.rename").toUpperCase() || "确认"}</button>
      </div>
    </form>
  </div>;
}

export function BoxPanel({ view, assets, store, selected, disabled, display, onDisplay, onAct, onSelect, onDetails, onClose }: { view: View; assets: Assets; store?: AssetStore; selected?: number; disabled: boolean; display?: BoxDisplay; onDisplay?: (display: BoxDisplay) => void; onAct: (action: Action) => void; onSelect: (id: number) => void; onDetails?: (id: number) => void; onClose: () => void }) {
  const fact = useFact();
  const language = useLanguage();
  const translate = useTranslate();
  const [query, setQuery] = useState("");
  const [localDisplay, setLocalDisplay] = useState<BoxDisplay>({ sort: 0, size: 0 });
  const { sort, size } = display ?? localDisplay;
  const changeDisplay = onDisplay ?? setLocalDisplay;
  const [terrain, setTerrain] = useState(0);
  const [rename, setRename] = useState<number>();
  const [ghost, setGhost] = useState<Ghost>();
  const order = useRef<{ query: string; ids: number[] } | undefined>(undefined);
  const tintedSlots = useRef(Infinity);
  const pendingDrag = useRef<Drag | undefined>(undefined);
  const skipClick = useRef(false);
  const mode = sortKeys[sort];
  const settings = view.progression?.settings;
  const owned = [...view.team.map(pokemon => entryFor(pokemon, true, assets.catalog, language)), ...view.box.map(pokemon => entryFor(pokemon, false, assets.catalog, language))];
  const search = query.toLowerCase();
  const previous = order.current?.query === search ? new Map(order.current.ids.map((id, index) => [id, index])) : undefined;
  const visible = owned.filter(entry => matches(entry, search)).sort((a, b) => compare(mode, a, b) || (previous?.get(a.pokemon.id) ?? Infinity) - (previous?.get(b.pokemon.id) ?? Infinity));
  useLayoutEffect(() => {
    order.current = { query: search, ids: visible.map(entry => entry.pokemon.id) };
    tintedSlots.current = Math.min(tintedSlots.current, visible.length);
  });
  const chosen = owned.find(entry => entry.pokemon.id === selected) ?? visible[0] ?? owned[0];
  const naming = owned.find(entry => entry.pokemon.id === rename);
  const stats = view.progression?.profileStats;
  const collectionSize = Math.max(0, (assets.catalog?.collection?.length ?? 257) - 1);
  const cap = view.progression?.challenge?.levelCap;
  const capacity = view.progression?.teamCapacity ?? view.teamSlots ?? 6;
  const writable = !disabled && !view.paused;
  const sizeURL = useResourceURL(icons + "size.png", store);
  const moving = ghost && owned.find(entry => entry.pokemon.id === ghost.id);
  const ghostURL = useResourceURL(moving ? moving.pokemon.sprite?.base ?? moving.specie?.sprite.base : undefined, store);
  const str = (key: string, fallback: string) => (fact(key) || fallback).toUpperCase();
  const canTransfer = (entry?: Entry) => writable && Boolean(entry && (entry.team || view.team.length < capacity));
  const transfer = (entry?: Entry) => { if (entry && canTransfer(entry)) onAct({ type: "team", pokemonID: entry.pokemon.id, mode: entry.team ? "box" : "team" }); };
  const cancel = () => { pendingDrag.current = undefined; setGhost(undefined); };
  const start = (event: PointerEvent<HTMLButtonElement>, entry: Entry) => {
    skipClick.current = false;
    if (!writable || event.button !== 0 || !event.isPrimary || event.pointerType === "touch") return;
    pendingDrag.current = { id: entry.pokemon.id, pointer: event.pointerId, source: event.currentTarget, originX: event.clientX, originY: event.clientY, slots: view.team.map(pokemon => pokemon.id), active: false };
    event.currentTarget.setPointerCapture(event.pointerId);
  };
  const move = (event: PointerEvent<HTMLButtonElement>) => {
    const drag = pendingDrag.current;
    if (!drag || drag.pointer !== event.pointerId) return;
    if (!writable) { cancel(); return; }
    if (!drag.active && Math.hypot(event.clientX - drag.originX, event.clientY - drag.originY) <= 6) return;
    const scene = drag.source.closest<HTMLElement>(".pokepath-classic-scene");
    const rect = scene?.getBoundingClientRect();
    if (!scene || !rect || !scene.clientWidth) return;
    const scale = rect.width / scene.clientWidth;
    drag.active = true;
    skipClick.current = true;
    setGhost({ id: drag.id, x: (event.clientX - rect.left) / scale, y: (event.clientY - rect.top) / scale, scale });
  };
  const end = (event: PointerEvent<HTMLButtonElement>) => {
    const drag = pendingDrag.current;
    if (!drag || drag.pointer !== event.pointerId) return;
    cancel();
    if (!drag.active || !writable || drag.slots.includes(drag.id)) return;
    const slot = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>("[data-party-slot]");
    if (!slot || !drag.source.closest(".pokepath-classic-scene")?.contains(slot)) return;
    const index = Number(slot.dataset.partySlot);
    if (!Number.isInteger(index) || index < 0 || index >= capacity) return;
    const targetID = drag.slots[index] ?? 0;
    if (Number(slot.dataset.pokemonId ?? 0) !== targetID) return;
    onAct({ type: "teamSlot", pokemonID: drag.id, slot: index, targetID });
  };
  return <>
    <section className="pokepath-central-panel pokepath-classic-box" data-box-font={settings?.font} role="dialog" aria-label={fact("text.box.title") || "仓库"} inert={Boolean(naming && writable)}><div className="pokepath-box-stats"><div data-box-count>POKEDEX: {stats?.pokemonOwned ?? owned.length}/{collectionSize}</div><div data-box-count>SHINYDEX: {stats?.shinyAmount ?? owned.filter(entry => entry.pokemon.shiny).length}/{collectionSize}</div></div>
      <button type="button" className="pokepath-box-name" data-box-name disabled={!writable || !chosen} onClick={() => chosen && setRename(chosen.pokemon.id)} style={{ "--pp-box-name-color": chosen?.specie?.color ?? "#a33635" } as CSSProperties}><span>{chosen ? `${(chosen.pokemon.alias || chosen.name).toUpperCase()} [${cap || chosen.pokemon.level}]` : str("text.box.title", "仓库")}</span></button>
      <button type="button" data-box-favorite className="pokepath-box-favorite" aria-label={translate("收藏伙伴")} aria-pressed={Boolean(chosen?.pokemon.favorite)} disabled={!writable || !chosen} onClick={() => chosen && onAct({ type: "favorite", pokemonID: chosen.pokemon.id, enabled: !chosen.pokemon.favorite })}>⭐</button>
      <button type="button" data-box-size className="pokepath-box-size" aria-label={translate("调整仓库大小")} style={sizeURL ? { backgroundImage: `url("${sizeURL}")` } : undefined} onClick={() => changeDisplay({ sort, size: (size + 1) % sizes.length })} />
      <div className="pokepath-box-toolbar"><button type="button" data-box-command="team" disabled={chosen?.team || !canTransfer(chosen)} onClick={() => transfer(chosen)}><span>{str("text.box.add", "添加单位")}</span></button><button type="button" data-box-command="box" disabled={!chosen?.team || !writable} onClick={() => transfer(chosen)}><span>{str("text.box.remove", "移除单位")}</span></button><button type="button" data-box-command="data" disabled={!chosen} onClick={() => chosen && onDetails?.(chosen.pokemon.id)}><span>{str("text.ui.info", "数据")}</span></button><button type="button" data-box-command="all" disabled={!writable || !view.team.length} onClick={() => onAct({ type: "teamAll", mode: "box" })}><span>{str("text.box.removeAll", "全部移除")}</span></button></div>
      <div className="pokepath-box-grid" style={{ "--pp-box-size": `${sizes[size]}px`, "--pp-box-scale": 1 + size * .1 } as CSSProperties}>{visible.map((entry, index) => <BoxCard key={entry.pokemon.id} entry={entry} store={store} assets={assets} selected={entry.pokemon.id === chosen?.pokemon.id} label={unitLabel(entry, mode, cap)} indicator={Boolean(settings?.indicatorShape)} size={size} moving={ghost?.id === entry.pokemon.id} clearTint={!entry.team && index >= tintedSlots.current} dim={terrain === 5 ? !fossils.has(entry.specie?.id ?? -1) : terrain > 0 && !entry.terrains.includes(terrain)} onChoose={() => { if (skipClick.current) { skipClick.current = false; return; } onSelect(entry.pokemon.id); }} onTransfer={() => transfer(entry)} onStart={event => start(event, entry)} onMove={move} onEnd={end} onCancel={cancel} />)}</div>
      <input type="text" data-box-search className="pokepath-box-search" maxLength={10} aria-label={translate("搜索伙伴")} value={query} onChange={event => setQuery(event.target.value)} onKeyDown={event => { if (event.key === "Enter" && visible.length === 1) { event.preventDefault(); onSelect(visible[0].pokemon.id); transfer(visible[0]); } }} />
      <div className="pokepath-box-filter"><button type="button" data-box-previous aria-label={translate("上一种仓库排序")} onClick={() => changeDisplay({ size, sort: (sort + sortKeys.length - 1) % sortKeys.length })}>&lt;</button><span data-box-sort>{str(`text.box.${mode}`, mode)}</span><button type="button" data-box-next aria-label={translate("下一种仓库排序")} onClick={() => changeDisplay({ size, sort: (sort + 1) % sortKeys.length })}>&gt;</button></div>
      <div className="pokepath-box-terrains">{tabs.map((tab, index) => <button type="button" key={tab} data-box-terrain={index} aria-pressed={terrain === index} onClick={() => setTerrain(index)}><span>{str(`text.box.${tab}`, tab)}</span></button>)}</div>
      <button type="button" data-controller-cancel className="pokepath-central-close" aria-label={translate("关闭仓库")} onClick={onClose}>×</button>
    </section>
    {ghost && ghostURL && <div className="pokepath-box-ghost" aria-hidden="true" style={{ left: ghost.x, top: ghost.y, width: 60 / ghost.scale, height: 60 / ghost.scale, backgroundImage: `url("${ghostURL}")` }} />}
    {naming && writable && <RenameBox entry={naming} font={settings?.font} onClose={() => setRename(undefined)} onAct={onAct} />}
  </>;
}
