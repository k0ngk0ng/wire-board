import { createRoot } from "react-dom/client";
import { DraftPanel } from "../web/DraftPanel";
import { I18nProvider } from "../web/I18n";
import { AssetStore } from "../web/assets";
import type { Action, Catalog, View } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; input: { view: View }; parameters: { language: string; theme: string; font: string; variant: string } };
const { view } = data.input;
const progression = view.progression!;
if (data.parameters.variant === "short") {
  progression.draft.offers = progression.draft.offers.slice(0, 2);
  progression.draft.round = 5;
  progression.draft.canReroll = false;
}
progression.settings = { ...progression.settings, language: Number(data.parameters.language), theme: data.parameters.theme, font: data.parameters.font };
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(data.parameters.font);
await Promise.all(view.box.map(pokemon => pokemon.sprite?.base ?? data.catalog.species[pokemon.species]?.sprite.base).filter((path): path is string => Boolean(path)).map(path => store.image(path)));
const actions: Action[] = [];
Reflect.set(window, "draftReceipts", actions);
createRoot(document.getElementById("root")!).render(<section className="pokepath-board"><div className="pokepath-classic-scene"><I18nProvider language={Number(data.parameters.language)} dictionary={data.dictionary} catalog={data.catalog}><DraftPanel view={view} assets={{ catalog: data.catalog, images: store.images, urls: store.urls, ready: true, version: 0 }} store={store} disabled={data.parameters.variant === "spectator"} onAct={action => actions.push(action)} onDetails={id => actions.push({ type: "details", pokemonID: id, blocked: true })} /></I18nProvider></div></section>);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
