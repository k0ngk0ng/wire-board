import { createRoot } from "react-dom/client";
import { useState } from "react";
import { AssetStore } from "../web/assets";
import { I18nProvider } from "../web/I18n";
import { GamePanel } from "../web/Panels";
import type { Action, Assets, Catalog, Pokemon, View } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; input: { team: Pokemon[]; box: Pokemon[]; chosen: number }; parameters: { language: string; theme: string; font: string } };
const { language, theme, font } = data.parameters;
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(font);
const art = [...data.input.team, ...data.input.box].flatMap(pokemon => [data.catalog.species[pokemon.species].sprite.base, data.catalog.items[pokemon.item]?.sprite]);
await Promise.all([...art, "src/assets/images/icons/size.png", "src/assets/images/icons/shiny.png"].filter((path): path is string => typeof path === "string").map(path => store.image(path)));
const assets: Assets = { catalog: data.catalog, images: store.images, urls: store.urls, version: 0, ready: true };
const actions: Action[] = [];
Reflect.set(window, "boxActions", actions);

function Fixture() {
  const [selected, choose] = useState(data.input.chosen);
  const [team, setTeam] = useState(data.input.team);
  const [box, setBox] = useState(data.input.box);
  const view = { team, box, towers: [], paused: false, progression: { settings: { language: Number(language), theme, font, indicatorShape: true }, challenge: {}, teamCapacity: 6, profileStats: { pokemonOwned: team.length + box.length, shinyAmount: [...team, ...box].filter(pokemon => pokemon.shiny).length } } } as unknown as View;
  const act = (action: Action) => {
    actions.push(action);
    const replace = (entries: Pokemon[]) => entries.map(pokemon => pokemon.id !== action.pokemonID ? pokemon : { ...pokemon, ...(action.type === "favorite" ? { favorite: action.enabled } : action.type === "rename" ? { alias: String(action.name) } : {}) });
    if (action.type === "favorite" || action.type === "rename") { setTeam(replace); setBox(replace); }
    if (action.type === "team" && action.mode === "box") {
      const removed = team.find(pokemon => pokemon.id === action.pokemonID);
      if (removed) { setTeam(team.filter(pokemon => pokemon.id !== removed.id)); setBox([...box, removed]); }
    }
    if (action.type === "team" && action.mode === "team") {
      const added = box.find(pokemon => pokemon.id === action.pokemonID);
      if (added) { setBox(box.filter(pokemon => pokemon.id !== added.id)); setTeam([...team, added]); }
    }
  };
  return <section className="pokepath-board"><div className="pokepath-classic-scene"><I18nProvider language={Number(language)} dictionary={data.dictionary} catalog={data.catalog}><GamePanel panel="box" view={view} assets={assets} store={store} selected={selected} disabled={false} onAct={act} onSelect={choose} onDetails={id => actions.push({ type: "inspect", pokemonID: id })} onClose={() => {}} /><div className="pokepath-box-fixture-party" style={{ position: "absolute", left: 0, top: 110, width: 120, height: 100 }} data-party-slot="0" data-pokemon-id={team[0]?.id} /></I18nProvider></div></section>;
}

createRoot(document.getElementById("root")!).render(<Fixture />);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
