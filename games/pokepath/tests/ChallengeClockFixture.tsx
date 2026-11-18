import { createRoot } from "react-dom/client";
import { ChallengeClock } from "../web/ChallengeClock";
import { I18nProvider } from "../web/I18n";
import { AssetStore } from "../web/assets";
import type { Catalog } from "../web/types";

const data = Reflect.get(window, "oracleData") as { catalog: Catalog; dictionary: unknown; parameters: { elapsed: string; language: string; font: string; active: string } };
const language = Number(data.parameters.language);
const store = new AssetStore(new URL("/pokepath/v1/", location.href), () => {});
await store.initialize();
await store.font(data.parameters.font);
const active = data.parameters.active === "yes";
createRoot(document.getElementById("root")!).render(<section className="pokepath-board"><div className="pokepath-classic-scene"><I18nProvider language={language} dictionary={data.dictionary} catalog={data.catalog}><div className="pokepath-central-panel"><div className="pokepath-classic-bottom" data-challenge-active={active || undefined}>{active && <ChallengeClock elapsedMS={Number(data.parameters.elapsed)} />}</div></div></I18nProvider></div></section>);
await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
