import { useLocalize } from "./I18n";
import type { CSSProperties } from "react";
import { fixedTargets, targetOptions } from "./target-modes";
import { Portrait, ResourceImage, RichText, text } from "./Panels";
import type { AssetStore } from "./assets";
import type { Action, Assets, Pokemon, View } from "./types";


const terrains: Record<number, string> = { 0: "場地", 1: "草原", 2: "草", 3: "水", 4: "山", 5: "化石" };
const darker = (color: string) => `#${color.replace("#", "").match(/../g)?.map(value => Math.floor(parseInt(value, 16) * 0.75).toString(16).padStart(2, "0")).join("") ?? "a33635"}`;

export function PokemonDetails({ pokemon, view, assets, store, disabled, inspectionOnly, onAct, onClose, onSelect, onInventory, onAim }: { pokemon: Pokemon; view: View; assets: Assets; store?: AssetStore; disabled: boolean; inspectionOnly?: boolean; onAct: (action: Action) => void; onClose: () => void; onSelect: (id: number) => void; onInventory: () => void; onAim?: (id: number) => void }) {
  const localize = useLocalize();
  const statsView = pokemon.combatStats ?? pokemon.stats;
  const species = assets.catalog?.species[pokemon.species];
  const ability = species ? assets.catalog?.abilities[species.abilityId] : undefined;
  const growth = view.progression?.growth?.[String(pokemon.id)];
  const item = assets.catalog?.items[pokemon.item];
  const owned = [...view.team, ...view.box];
  const index = owned.findIndex(member => member.id === pokemon.id);
  const navigate = (delta: number) => onSelect(owned[(index + delta + owned.length) % owned.length].id);
  const availableTargets = pokemon.targetModes?.length ? targetOptions.filter(option => pokemon.targetModes!.includes(option[0])) : [];
  const targetFixed = !availableTargets.length || !view.team.some(candidate => candidate.id === pokemon.id);
  const target = availableTargets.findIndex(option => option[0] === pokemon.target);
  const changeTarget = (delta: number) => onAct({ type: "target", pokemonID: pokemon.id, mode: availableTargets[(target + delta + availableTargets.length) % availableTargets.length][0] });
  const stats = [["力量", Math.round(statsView.power), "#ff4d34"], ["充能", `${(statsView.attackMS / 1000).toFixed(2)}s`, "#68d4ff"], ["暴擊", `${Number(statsView.criticalPct.toFixed(2))}%`, "#f6dc4a"], ["射程", Math.round(statsView.range), "#70ac4c"], ["形狀", ({ circle: "圓形", donut: "環形", ring: "環形", cross: "十字", xShape: "X 形", horizontalLine: "橫線", verticalLine: "縱線", line: "直線", star: "星形" } as Record<string, string>)[pokemon.combatRangeType ?? species?.rangeType ?? "circle"] ?? "圓形", "#cc70d4"], ["地形", species?.terrains.map(value => terrains[value] ?? String(value)).join(" / ") ?? "—", "#af8f7d"]];
  const content = <>
    <div className="pokepath-details-title"><button type="button" aria-label="上一个伙伴" onClick={() => navigate(-1)}>❮</button><h2>{pokemon.alias || species?.name || pokemon.species} [{pokemon.level}]</h2><button type="button" aria-label="下一个伙伴" onClick={() => navigate(1)}>❯</button></div>
    <dl className="pokepath-details-stats">{stats.map(([label, value, color]) => <div key={label}><dt style={{ color: String(color) }}>{label}</dt><dd>{value}</dd></div>)}<div><dt>目標</dt><dd className="pokepath-details-target"><button type="button" aria-label="上一个攻击目标" disabled={disabled || Boolean(targetFixed)} onClick={() => changeTarget(-1)}>❮</button><select aria-label="攻击目标" value={pokemon.target} disabled={disabled || Boolean(targetFixed)} onChange={event => onAct({ type: "target", pokemonID: pokemon.id, mode: event.target.value })}>{!availableTargets.some(option => option[0] === pokemon.target) && <option value={pokemon.target}>{fixedTargets[pokemon.target] ?? targetOptions.find(option => option[0] === pokemon.target)?.[1] ?? pokemon.target}</option>}{availableTargets.map(([key, label]) => <option key={key} value={key}>{label}</option>)}</select><button type="button" aria-label="下一个攻击目标" disabled={disabled || Boolean(targetFixed)} onClick={() => changeTarget(1)}>❯</button></dd></div></dl>
    <section className="pokepath-details-ability"><strong>{text(ability?.name) || "能力"}</strong><p><RichText value={ability?.description} enabled={view.progression?.settings.colorTags} /></p></section>
    <section className="pokepath-details-item"><strong>{text(item?.name) || "無道具"}</strong>{item && <div className="pokepath-details-item-portrait">{typeof item.sprite === "string" && store && <ResourceImage path={item.sprite} store={store} />}</div>}<p>{item ? <RichText value={item.description} enabled={view.progression?.settings.colorTags} /> : "當前未裝備任何道具。"}</p><button type="button" aria-label={item ? "卸下道具" : "装备道具"} disabled={disabled} onClick={() => item ? onAct({ type: "unequip", pokemonID: pokemon.id }) : onInventory()}>{item ? "−" : "+"}</button></section>
    {view.attackModes?.towers?.[String(pokemon.id)]?.hasAim && <button type="button" className="pokepath-details-aim" aria-label="瞄准攻击区域" disabled={disabled} onClick={() => onAim?.(pokemon.id)}>◎</button>}
    <div className="pokepath-details-upgrades">{[1, 5, 10].map(count => { const cost = pokemon.upgradeCosts?.[String(count)] ?? (count === 1 ? pokemon.upgradeCost : undefined); return <button type="button" key={count} data-upgrade-count={count} aria-label={`升级 ${count} 级`} disabled={disabled || pokemon.level >= 100 || cost === undefined || cost > view.gold} onClick={() => onAct({ type: "upgrade", pokemonID: pokemon.id, count })}>升級{count === 1 ? "" : ` X${count}`}<br />(${cost ?? "—"})</button>; })}</div>
    {growth && (growth.forms.length > 0 || growth.appearances.length > 0 || growth.canReset || growth.canDNA || growth.canShiny || growth.canCustomSkin) && <div className="pokepath-details-growth">
      {growth.forms.length > 0 && <select aria-label="战斗形态" value={pokemon.species} disabled={disabled} onChange={event => onAct({ type: "form", pokemonID: pokemon.id, mode: event.target.value })}>{growth.forms.map(key => <option key={key} value={key}>{assets.catalog?.species[key]?.name ?? key}</option>)}</select>}
      {growth.appearances.length > 0 && <select aria-label="外观形态" value={view.progression?.appearance?.[String(pokemon.id)] ?? pokemon.species} disabled={disabled} onChange={event => onAct({ type: "appearance", pokemonID: pokemon.id, mode: event.target.value })}>{growth.appearances.map(key => <option key={key} value={key}>{assets.catalog?.species[key]?.name ?? key}</option>)}</select>}
      {growth.canDNA && <select aria-label="复制伙伴 DNA" value="" disabled={disabled} onChange={event => onAct({ type: "dna", pokemonID: pokemon.id, targetID: Number(event.target.value) })}><option value="" disabled>複製 DNA</option>{view.team.filter(candidate => candidate.id !== pokemon.id).map(candidate => <option key={candidate.id} value={candidate.id}>{assets.catalog?.species[candidate.species]?.name ?? candidate.species}</option>)}</select>}
      {growth.canReset && <button type="button" disabled={disabled} onClick={() => onAct({ type: "reset", pokemonID: pokemon.id })}>重置等級</button>}
      {growth.canShiny && <button type="button" aria-pressed={!view.progression?.hiddenShiny?.[String(pokemon.id)]} disabled={disabled} onClick={() => onAct({ type: "shiny", pokemonID: pokemon.id, enabled: Boolean(view.progression?.hiddenShiny?.[String(pokemon.id)]) })}>異色</button>}
      {growth.canCustomSkin && <button type="button" aria-pressed={view.progression?.customSkins?.[String(pokemon.id)] ?? false} disabled={disabled} onClick={() => onAct({ type: "customSkin", pokemonID: pokemon.id, enabled: !view.progression?.customSkins?.[String(pokemon.id)] })}>造型</button>}
    </div>}
    <span className="pokepath-details-index">LV {species?.evolutionLevel ?? "—"}</span>
    <div className="pokepath-details-mobile-portrait"><Portrait pokemon={pokemon} assets={assets} store={store} /></div>
  </>;
  return localize(<div className="pokepath-details-overlay" onClick={onClose}><section className="pokepath-details" role="dialog" aria-modal="true" aria-label="单位数据" data-details-pokemon={pokemon.id} onClick={event => event.stopPropagation()} style={{ "--pp-unit-color": darker(species?.color ?? "#ff8463") } as CSSProperties}>
    <header><strong>單位數據</strong><button type="button" data-controller-cancel aria-label="关闭单位详情" onClick={onClose}>X</button></header>
    {inspectionOnly ? <div className="pokepath-details-readonly" inert>{content}</div> : content}
  </section></div>);
}
