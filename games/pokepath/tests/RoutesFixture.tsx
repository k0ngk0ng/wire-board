import { createRoot } from "react-dom/client";
import { RoutesPanel } from "../web/CollectionPanels";
import { I18nProvider } from "../web/I18n";
import { AssetStore } from "../web/assets";
import type { Action, Catalog, View } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; input: { view: View }; parameters: { language: string; theme: string; font: string; variant: string } };
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(data.parameters.font);
const { view } = data.input;
if (data.parameters.variant === "active") view.waveActive = true;
if (data.parameters.variant === "hidden") {
  view.progression!.secretMaps = {};
  view.items = [];
}
view.progression!.settings = { ...view.progression!.settings, language: Number(data.parameters.language), theme: data.parameters.theme, font: data.parameters.font };
const paths = [...Object.values(data.catalog.routes).map(route => route.background), "src/assets/images/icons/ribbon1.png"];
await Promise.all(paths.map(path => store.image(path)));
const actions: (Action | { panel: string })[] = [];
Reflect.set(window, "routesActions", actions);
createRoot(document.getElementById("root")!).render(<section className="pokepath-board"><div className="pokepath-classic-scene"><I18nProvider language={Number(data.parameters.language)} dictionary={data.dictionary} catalog={data.catalog}><RoutesPanel view={view} assets={{ catalog: data.catalog, images: store.images, urls: store.urls, ready: true, version: 0 }} store={store} disabled={false} onAct={action => actions.push(action)} onPanel={panel => actions.push({ panel })} onClose={() => {}} /></I18nProvider></div></section>);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
