import { useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { I18nProvider } from "../web/I18n";
import { ClassicScene } from "../web/ClassicScene";
import { GamePanel, type Panel } from "../web/Panels";
import { PokemonDetails } from "../web/PokemonDetails";
import { AssetStore } from "../web/assets";
import { loadNativeRuntime } from "../web/native-runtime";
import { pokemonAt } from "../web/battle-renderer";
import type { StageHandle } from "../web/Stage";
import type { Action, Catalog, Route, RuleState, View } from "../web/types";
import "../web/style.css";
import "../web/classic.css";

const base = new URL("/pokepath/v1/", location.href);
const store = new AssetStore(base, () => {});
await store.initialize();
const catalog = await store.json<Catalog>("catalog.json");
const uiText = await store.json("ui-text.json");
let route = await store.json<Route>("routes/0.json");
const native = await loadNativeRuntime<RuleState, View, Action>({ baseURL: new URL("native/", base), catalogBaseURL: base, resolveResource: path => store.url(path), readResource: (path, expected) => store.readExternal(path, expected) });
let state = native.create({ starter: "charmander" }, 1, 1, 0);
const initial = native.project(state);
await store.prepare(initial, catalog, route);

function Fixture() {
  const [view, setView] = useState(initial);
  const [selected, setSelected] = useState(initial.team[0]?.id);
  const [placing, setPlacing] = useState(false);
  const [range, setRange] = useState<number>();
  const [details, setDetails] = useState<number>();
  const [panel, setPanel] = useState<Panel>("team");
  const [error, setError] = useState("");
  const stage = useRef<StageHandle>(null);
  const assets = { catalog, route, images: store.images, urls: store.urls, version: view.sequence, ready: true };
  const act = async (action: Action) => {
    try {
      if (action.type === "route" && action.route !== undefined) { await native.ensureRoute(action.route); route = await store.json<Route>(`routes/${action.route}.json`); }
      state = native.apply(state, 0, { ...action, epoch: view.epoch });
      const next = native.project(state);
      await store.prepare(next, catalog, route);
      setView(next);
      setError("");
    } catch (cause) { setError(String(cause)); }
  };
  const pokemon = [...view.team, ...view.box].find(pokemon => pokemon.id === details);
  return <section className="pokepath-board" data-fixture-ready onKeyDown={event => { if (event.key === "Escape") { setPanel("team"); setDetails(undefined); setPlacing(false); } }}><I18nProvider language={view.progression?.settings.language ?? 8} dictionary={uiText} catalog={catalog}><ClassicScene view={view} assets={assets} store={store} selected={selected} placing={placing} range={range} panel={panel} disabled={false} stage={stage} onAct={action => void act(action)} onSelect={id => { setSelected(id); setPlacing(true); }} onDetails={setDetails} onPanel={next => { if (next === "routes" && panel !== "routes") void act({ type: "mapOpen" }); setPanel(next); }} onPoint={point => {
    const placed = pokemonAt(view, point);
    if (!placing && placed) { setDetails(placed.id); return; }
    const tile = view.tiles.find(tile => Math.hypot(tile.x + 12 - point.x, tile.y + 12 - point.y) < 18);
    if (placing && selected !== undefined && tile) { void act({ type: "deploy", pokemonID: selected, tileID: tile.id }); setPlacing(false); }
  }} onHover={setRange} onReady={() => { document.documentElement.dataset.ready = "true"; }}>{panel !== "team" && <GamePanel key={panel} panel={panel} view={view} assets={assets} store={store} selected={selected} disabled={false} onAct={action => void act(action)} onSelect={setSelected} onDetails={setDetails} onPanel={next => { if (next === "routes" && panel !== "routes") void act({ type: "mapOpen" }); setPanel(next); }} onClose={() => setPanel("team")} />}{pokemon && <PokemonDetails pokemon={pokemon} view={view} assets={assets} store={store} disabled={false} onAct={action => void act(action)} onClose={() => setDetails(undefined)} onSelect={setDetails} onInventory={() => { setDetails(undefined); setPanel("inventory"); }} />}{error && <div role="alert">{error}</div>}</ClassicScene></I18nProvider></section>;
}

createRoot(document.getElementById("root")!).render(<Fixture />);
