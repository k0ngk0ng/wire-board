import { createRoot } from "react-dom/client";
import { useState } from "react";
import { DexPanel } from "../web/CollectionPanels";
import { I18nProvider } from "../web/I18n";
import { AssetStore } from "../web/assets";
import type { Action, Catalog, View } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; input: { view: View; keys: { collection: string[]; enemies: string[]; tutorials: string[] } }; parameters: { language: string; theme: string; font: string; variant: string } };
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(data.parameters.font);
const catalog = { ...data.catalog, collection: data.input.keys.collection, enemies: Object.fromEntries(data.input.keys.enemies.map(key => [key, data.catalog.enemies[key]])), tutorials: Object.fromEntries(data.input.keys.tutorials.map(key => [key, data.catalog.tutorials![key]])) };
const paths = [...catalog.collection.map(key => catalog.species[key].sprite.base), ...Object.values(catalog.enemies).map(definition => definition.sprite.base), ...Object.values(catalog.tutorials).map(definition => definition.sprite)];
await Promise.all(paths.filter((path): path is string => typeof path === "string" && path.length > 0).map(path => store.image(path)));
const actions: Action[] = [];
Reflect.set(window, "dexActions", actions);
function Fixture() {
  const [stage, setStage] = useState(0);
  const view = { ...data.input.view, items: [...data.input.view.items, ...(data.parameters.variant === "nox" ? [{ id: "noxDex" }] : [])], progression: { ...data.input.view.progression!, unownStage: stage, settings: { ...data.input.view.progression!.settings, font: data.parameters.font, language: Number(data.parameters.language) } } };
  const act = (action: Action) => {
    actions.push(action);
    if (action.type === "unown" && action.targetID === 0 && ["yes", "是"].includes(String(action.name).trim().toLowerCase())) setStage(1);
  };
  return <section className="pokepath-board"><div className="pokepath-classic-scene"><I18nProvider language={Number(data.parameters.language)} dictionary={data.dictionary} catalog={catalog}><DexPanel view={view} assets={{ catalog, images: store.images, urls: store.urls, ready: true, version: 0 }} store={store} disabled={false} onAct={act} onClose={() => {}} /></I18nProvider></div></section>;
}

createRoot(document.getElementById("root")!).render(<Fixture />);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
