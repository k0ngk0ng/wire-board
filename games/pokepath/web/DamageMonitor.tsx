import { useEffect, useRef } from "react";
import { useLocalize, useLanguage } from "./I18n";
import { Portrait } from "./Panels";
import type { AssetStore } from "./assets";
import type { Assets, View } from "./types";

export type DamageMode = "trueDamage" | "overdamage";
export function DamageMonitor({ view, assets, store, mode, onMode }: { view: View; assets: Assets; store?: AssetStore; mode: DamageMode; onMode: (mode: DamageMode) => void }) {
  const localize = useLocalize();
  const language = useLanguage();
  const numeric = (value: number) => Math.floor(value).toString().replace(/\B(?=(\d{3})+(?!\d))/g, [0, 6, 7].includes(language) ? "," : language === 2 ? " " : ".");
  const previous = useRef({ epoch: view.epoch, route: view.route, wave: view.wave, bars: new Map<number, number>(), percentages: new Map<number, number>() });
  const history = previous.current;
  if (history.epoch !== view.epoch || history.route !== view.route) { history.epoch = view.epoch; history.route = view.route; history.wave = view.wave; history.bars.clear(); history.percentages.clear(); }
  else if (history.wave !== view.wave) { history.wave = view.wave; history.bars = new Map(history.percentages); history.percentages.clear(); }
  const total = mode === "trueDamage" ? view.totalDamage ?? 0 : view.totalOverdamage ?? 0;
  useEffect(() => { history.percentages = new Map(view.team.map(pokemon => [pokemon.id, total > 0 ? Math.ceil((mode === "trueDamage" ? pokemon.damage ?? 0 : pokemon.overdamage ?? 0) / total * 100) : 0])); }, [view, mode, total, history]);
  return localize(<div className="pokepath-damage-monitor" data-damage-mode={mode}><section className="pokepath-damage-rows" aria-label="伤害统计">{Array.from({ length: 10 }, (_, index) => {
    const pokemon = view.team[index];
    const species = pokemon && assets.catalog?.species[pokemon.species];
    const value = pokemon ? mode === "trueDamage" ? pokemon.damage ?? 0 : pokemon.overdamage ?? 0 : 0;
    const percent = total > 0 && value > 0 ? Math.ceil(value / total * 100) : 0;
    const helping = [19, 83, 101, 111, 144, 173].includes(species?.id ?? -1);
    const suffering = species?.id === 252;
    return <article key={index} data-damage-row={index} data-pokemon-id={pokemon?.id}>{pokemon && <><span className="pokepath-damage-portrait"><Portrait pokemon={pokemon} assets={assets} store={store} /></span><span className="pokepath-damage-number">{helping ? "正在协助" : suffering ? "它正在受苦" : numeric(value)}<small>{helping || suffering ? ":)" : `(${percent}%)`}</small></span><span className="pokepath-damage-bar"><span className="previous" style={{ width: `${history.bars.get(pokemon.id) ?? 0}%`, backgroundColor: `${species?.color ?? "#fff"}4d` }} /><span style={{ width: `${percent}%`, backgroundColor: species?.color }} /></span></>}</article>;
  })}</section><button type="button" className="pokepath-damage-mode" onClick={() => onMode(mode === "trueDamage" ? "overdamage" : "trueDamage")}>{mode === "trueDamage" ? "真实伤害" : "超额伤害"}</button></div>);
}
