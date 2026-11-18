import { createRoot } from "react-dom/client";
import { useState } from "react";
import { InventoryPanel } from "../web/CollectionPanels";
import { I18nProvider } from "../web/I18n";
import { AssetStore } from "../web/assets";
import type { Action, Catalog, View } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; input: { view: View }; parameters: { language: string; theme: string; font: string; readonly?: string } };
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(data.parameters.font);
await Promise.all([...data.input.view.team.map(pokemon => data.catalog.species[pokemon.species].sprite.base), ...data.input.view.items.map(item => data.catalog.items[item.id].sprite), "src/assets/images/icons/size.png"].filter((path): path is string => typeof path === "string").map(path => store.image(path)));
const actions: Action[] = [];
Reflect.set(window, "inventoryActions", actions);

function Fixture() {
  const [view, setView] = useState({ ...data.input.view, progression: { ...data.input.view.progression!, settings: { ...data.input.view.progression!.settings, language: Number(data.parameters.language), theme: data.parameters.theme, font: data.parameters.font } } });
  const act = (action: Action) => {
    actions.push(action);
    if (action.type === "swapItems") setView(previous => {
      const items = [...previous.items];
      const first = items.findIndex(item => item.id === action.itemID);
      const second = items.findIndex(item => item.id === action.name);
      if (first >= 0 && second >= 0) [items[first], items[second]] = [items[second], items[first]];
      return { ...previous, items };
    });
  };
  return <section className="pokepath-board"><div className="pokepath-classic-scene">{view.team.map((pokemon, index) => <div key={pokemon.id} data-party-slot={index} data-pokemon-id={pokemon.id} style={{ position: "absolute", left: 0, top: 103 + index * 114, width: 129, height: 114, background: "#253528" }} />)}<I18nProvider language={Number(data.parameters.language)} dictionary={data.dictionary} catalog={data.catalog}><InventoryPanel view={view} assets={{ catalog: data.catalog, images: store.images, urls: store.urls, ready: true, version: 0 }} store={store} selected={917} disabled={data.parameters.readonly === "true"} onAct={act} onSelect={() => {}} onClose={() => {}} /></I18nProvider></div></section>;
}

createRoot(document.getElementById("root")!).render(<Fixture />);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
