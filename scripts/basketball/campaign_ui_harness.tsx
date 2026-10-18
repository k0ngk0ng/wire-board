// Isolated real React acceptance driven by retained authenticated server views.
// No simulated game rules or claimed full match; UI requests are recorded.
import { useState } from "react";
import { createRoot } from "react-dom/client";
import {
  OptionsForm,
  type BasketballOptions,
} from "../../games/basketball/web/module";
import { Preparation } from "../../games/basketball/web/Preparation";
import { Board } from "../../games/basketball/web/Board";
import { BasketballNetworkClient } from "../../games/basketball/web/network-client";
import { CampaignPanel } from "../../games/basketball/web/CampaignPanel";
import type { BasketballNetworkView } from "../../games/basketball/web/network-scene";
import type { GameRoom } from "../../web/src/game-api/module";
import "../../web/src/style.css";
const source = await (await fetch("/fixtures.json")).json();
const actions: Record<string, unknown>[] = [];
// Deferred client creation tests the real Board's loading/modal boundary only.
// It does not simulate Wasm, gameplay or accepted controller input.
const clientBindings: boolean[] = [];
Object.assign(window, { campaignClientBindings: clientBindings });
BasketballNetworkClient.create = async () => {
  await new Promise<void>((resolve) =>
    Object.assign(window, { resolveCampaignClient: resolve }),
  );
  return {
    update: (_data: unknown, _version: number, enabled: boolean) =>
      clientBindings.push(enabled),
    configure: () => {},
    dispose: () => {},
    controllerSink: { pressPointer: () => {}, releasePointer: () => {} },
  } as unknown as BasketballNetworkClient;
};
const originalFetch = window.fetch;
window.fetch = async (input, init) =>
  String(input) === "/api/controllers"
    ? new Response(
        JSON.stringify({
          profiles: {
            "classic-2": {
              revision: 1,
              turboHz: 10,
              keyboard: {
                up: "KeyW",
                down: "KeyS",
                left: "KeyA",
                right: "KeyD",
                select: "KeyF",
                start: "Space",
                b: "KeyJ",
                a: "KeyK",
                turboB: "KeyU",
                turboA: "KeyI",
              },
            },
          },
        }),
        { headers: { "Content-Type": "application/json" } },
      )
    : originalFetch(input, init);
const silentAudio = {
  play: async () => true,
  playWithFallback: async () => true,
  stop: () => {},
  pause: () => {},
  resume: async () => true,
  music: async () => true,
  stopMusic: () => {},
  loop: async () => true,
  stopLoop: () => {},
};
function Harness() {
  const [mode, setMode] = useState("waiting"),
    [busy, setBusy] = useState(false),
    [spectating, setSpectating] = useState(false);
  const [options, setOptions] = useState<BasketballOptions>(
    source.waiting.options,
  );
  const [data, setData] = useState<BasketballNetworkView>(source.halftime);
  const [open, setOpen] = useState(false);
  const room: GameRoom = {
    ...source.waiting,
    you: spectating ? -1 : 0,
    spectating,
    status: mode === "waiting" ? "waiting" : "playing",
  };
  Object.assign(window, {
    campaignFixture: {
      source,
      actions,
      mode,
      setMode,
      busy,
      setBusy,
      spectating,
      setSpectating,
      options,
      setOptions,
      data,
      setData,
      open,
    },
  });
  const view = {
    ...data,
    canPrepare: !spectating && data.canPrepare,
    campaign: data.campaign
      ? { ...data.campaign, canAct: !spectating && data.campaign.canAct }
      : undefined,
  };
  return (
    <main className="campaign-fixture">
      <h1>热血篮球 · 战役准备验收</h1>
      {mode === "board" ? (
        <Board
          room={room}
          data={view}
          act={async (a) => {
            actions.push(a);
          }}
          command={async () => {}}
          assets=""
          busy={busy}
          sound={false}
          audio={silentAudio}
          clock={{ serverNow: data.serverAt, receivedAt: performance.now() }}
        />
      ) : mode === "creation" ? (
        <form>
          <OptionsForm
            value={options}
            onChange={setOptions}
            capacity={options.mode === "solo" ? 1 : 2}
            disabled={busy}
          />
          <button type="submit">创建</button>
        </form>
      ) : mode === "waiting" ? (
        <Preparation
          room={room}
          options={options}
          host={!spectating}
          busy={busy}
          assets="/missing-cdn"
          prepare={(a) => {
            actions.push(a);
          }}
        />
      ) : (
        <CampaignPanel
          data={view}
          busy={busy}
          assets="/missing-cdn"
          act={async (a) => {
            actions.push(a);
          }}
          clock={{ serverNow: data.serverAt, receivedAt: performance.now() }}
          onOpenChange={setOpen}
        />
      )}
    </main>
  );
}
createRoot(document.querySelector("#root")!).render(<Harness />);
