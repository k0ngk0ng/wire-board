import { createRoot } from "react-dom/client";
import { GamePanel } from "../web/Panels";
import { I18nProvider } from "../web/I18n";
import { AssetStore } from "../web/assets";
import type { Action, Catalog, View } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; input: { view: View }; parameters: { language: string; theme: string; font: string; variant: string } };
const { view } = data.input;
const variant = data.parameters.variant;
if (variant === "active") {
  view.progression!.challenge = { levelCap: 50, slotLimit: 6, toughEnemies: 50, draft: true, noItems: true, permadeath: true, mirror: true, active: true };
  view.progression!.canChallenge = false;
  view.waveActive = true;
}
if (variant === "ready") {
  view.progression!.currentRibbons = 5;
  view.progression!.ribbons = { 0: { levelCap: 2, slotLimit: 1, mirror: 1, noItems: 1 } };
  view.progression!.claims = { 0: [true, true, false, true] };
  view.box = [{ id: 17, species: "bidoof", level: 1, item: "", target: "first", tileID: -1, stats: { power: 1, attackMS: 1, range: 1, criticalPct: 0 } }];
}
if (variant === "empty") {
  view.progression!.currentRibbons = 0;
  view.progression!.ribbons = {};
}
view.progression!.settings = { ...view.progression!.settings, language: Number(data.parameters.language), theme: data.parameters.theme, font: data.parameters.font };
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(data.parameters.font);
const rewards = data.catalog.routes[String(view.route)] as { challengeRewards?: string[] };
const paths = ["src/assets/images/icons/ribbon1.png", "src/assets/images/icons/ribbon3.png", "src/assets/images/icons/egg.png", ...(rewards.challengeRewards ?? []).map(key => data.catalog.items[key]?.sprite).filter((path): path is string => typeof path === "string")];
await Promise.all(paths.map(path => store.image(path)));
const actions: (Action | { type: "close" })[] = [];
Reflect.set(window, "challengeActions", actions);
createRoot(document.getElementById("root")!).render(<section className="pokepath-board"><div className="pokepath-classic-scene"><I18nProvider language={Number(data.parameters.language)} dictionary={data.dictionary} catalog={data.catalog}><GamePanel panel="challenge" view={view} assets={{ catalog: data.catalog, images: store.images, urls: store.urls, ready: true, version: 0 }} store={store} disabled={variant === "spectator"} onSelect={() => {}} onAct={action => actions.push(action)} onClose={() => actions.push({ type: "close" })} /></I18nProvider></div></section>);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
