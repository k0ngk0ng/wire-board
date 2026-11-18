import { createRoot } from "react-dom/client";
import { useState } from "react";
import { I18nProvider } from "../web/I18n";
import { AssetStore } from "../web/assets";
import { ShopPanel, HatchPanel } from "../web/ShopPanel";
import type { Action, Assets, Catalog, Pokemon, View } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; input: { keys: string[]; pokemon: Pokemon; gold: number }; parameters: { language: string; theme: string; font: string; variant: string } };
const { language, font, theme, variant } = data.parameters;
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(font);
const gold = variant === "low" ? 0 : data.input.gold;
const stock = data.input.keys.map((key, index) => variant === "sold" && (index === 0 || index === 4) ? "" : key);
const prices = { eggPrice: variant === "sold" ? 50000 : 5432, remainingEggs: variant === "sold" ? 0 : 87 };
const shop = stock.map((key, index) => {
  const item = data.catalog.items[key];
  return { stock: index, itemID: key, name: item?.name, description: item?.description, sprite: item?.sprite, tier: item?.tier, price: item?.price, affordable: Boolean(item && gold >= Number(item.price)), equipable: Boolean(item?.equipable) };
});
const art = [data.catalog.species[data.input.pokemon.species].sprite.base, "src/assets/images/icons/egg.png", "src/assets/images/items/poke.png", ...shop.map(item => item.sprite)];
await Promise.all(art.filter((path): path is string => typeof path === "string").map(path => store.image(path)));
const actions: Action[] = [];
Reflect.set(window, "shopActions", actions);
const assets: Assets = { catalog: data.catalog, images: store.images, urls: store.urls, ready: true, version: 0 };
const view = { gold, paused: false, progression: { ...prices, shop, settings: { language: Number(language), theme, font, mapEffectsMode: variant === "still" ? 0 : 1 } } } as unknown as View;
function Fixture() {
  const [hatching, setHatching] = useState(variant === "hatch");
  const close = () => { actions.push({ type: "close" }); setHatching(false); };
  return <section className="pokepath-board"><div className="pokepath-classic-scene"><I18nProvider language={Number(language)} dictionary={data.dictionary} catalog={data.catalog}><ShopPanel view={view} assets={assets} store={store} disabled={false} onAct={action => actions.push(action)} onClose={close} />{hatching && <HatchPanel pokemon={data.input.pokemon} assets={assets} store={store} font={font} onClose={close} />}</I18nProvider></div></section>;
}
createRoot(document.getElementById("root")!).render(<Fixture />);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
