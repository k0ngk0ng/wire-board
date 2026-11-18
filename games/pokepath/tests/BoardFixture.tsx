import { useState } from "react";
import { createRoot } from "react-dom/client";
import { ControllerNavigation } from "../../../web/src/controllers/navigation";
import { Board } from "../web/Board";
import { loadNativeRuntime } from "../web/native-runtime";
import type { BoardProps } from "../../../web/src/game-api/module";
import type { Action, RuleState, View } from "../web/types";
import "../web/style.css";
import "../web/classic.css";

const base = new URL("/pokepath/v1/", location.href);
const native = await loadNativeRuntime<RuleState, View, Action>({ baseURL: new URL("native/", base), catalogBaseURL: base });
let state = native.create({ starter: "charmander" }, 1, 1, 0);
const team = state.team as { id: number; species: string; item: string; tileID: number }[];
const { facts, ...drifloonSprite } = (native.catalog.species as Record<string, { sprite: Record<string, unknown> }>).drifloon.sprite;
state.team = [{ ...team[0], id: 917, species: "drifloon", item: "blimpKeys", sprite: drifloonSprite }, { ...team[0], id: 73 }];
state.inventory = [{ id: "blimpKeys", equippedBy: 917 }];
state.nextID = 918;
const initial = native.project(state);
for (const [index, pokemon] of initial.team.entries()) {
  const tile = initial.tiles.filter(tile => tile.terrain === 1)[index];
  state = native.apply(state, 0, { type: "deploy", pokemonID: pokemon.id, tileID: tile.id, epoch: initial.epoch });
}
const navigation = new ControllerNavigation(document.getElementById("root")!);
const trace = { navigate: (action: string) => { navigation.sample(new Set([action]), performance.now()); navigation.sample(new Set(), performance.now()); }, actions: [] as Action[], sounds: [] as unknown[], state: () => state };
(window as typeof window & { boardFixture: typeof trace }).boardFixture = trace;
const acceptedInputs: string[] = [];
const project = () => ({ ...native.project(state), snapshot: native.clone(state), acceptedInputs: [...acceptedInputs] });
const audio = { play: async (...values: unknown[]) => { trace.sounds.push(values); }, music: async () => {}, stop: () => {} };
function Fixture() {
  const [view, setView] = useState(project);
  const props = { data: view, room: { id: "board-input", name: "输入验证", kind: "pokepath", you: 0, spectating: false, loading: false, seats: [{ id: "fixture", name: "Fixture", autoPlay: false }], game: { finished: false } }, sound: true, audio, assets: "", clock: { serverNow: 0, receivedAt: 0 }, act: async (action: Action) => {
    trace.actions.push(action);
    const { clientInputID, ...ruleAction } = action;
    state = native.apply(state, 0, ruleAction);
    if (typeof clientInputID === "string") acceptedInputs.push(clientInputID);
    setView(project());
  }, command: async () => {}, onReady: () => { document.documentElement.dataset.ready = "true"; } } as unknown as BoardProps<View>;
  return <Board {...props} />;
}
createRoot(document.getElementById("root")!).render(<Fixture />);
