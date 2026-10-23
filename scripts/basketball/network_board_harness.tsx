import {
  StreamInputs,
  mergeStreamState,
  mergeStreamGame,
} from "../../web/src/realtime-stream";
import { createRoot } from "react-dom/client";
import { OptionsForm } from "../../games/basketball/web/module";
import type { BasketballOptions } from "../../games/basketball/web/module";
import { Board } from "../../games/basketball/web/Board";
import { ControllerSettings } from "../../web/src/controllers/Settings";
import { useEffect, useMemo, useState } from "react";
import type { GameRoom, GameAudioPort } from "../../web/src/game-api/module";
import { GameAudio } from "../../web/src/audio";
import basketballAssets from "../../games/basketball/assets/release-manifest.json";

// The room and actions use the real authenticated host. Only the outer page
// and audio sink are fixture plumbing; no world fields are changed here.
const records: {
  actions: any[];
  checkpoints: { at: number; url: string }[];
  packets: { at: number; bytes: number; type: string; full: boolean }[];
  sent: { at: number; action: unknown; ackAt?: number }[];
  errors: string[];
  room?: GameRoom;
  ackDelay: number;
  userID: string;
  music: { sources: readonly string[]; options: unknown }[];
  denyFirstMusic?: boolean;
  mediaEvents: Array<{
    at: number;
    type: string;
    element: number;
    src: string;
    currentSrc: string;
    paused: boolean;
    readyState: number;
    networkState: number;
    error: number | null;
  }>;
} = {
  packets: [],
  sent: [],
  checkpoints: [],
  actions: [],
  errors: [],
  ackDelay: 0,
  userID: "",
  music: [],
  mediaEvents: [],
};
(window as any).networkBoard = records;
const fixtureFetch = window.fetch;
window.fetch = (input, init) => {
  const url = String(input);
  if (/\/sync(?:$|\?)/.test(url) && !url.includes("probe=1"))
    records.checkpoints.push({ at: performance.now(), url });
  return fixtureFetch(input, init);
};
(window as any).fullStateProbe = async () =>
  (await api("/rooms/" + records.room!.id + "/sync?probe=1")).data.checkpoint
    .state;

window.addEventListener("error", (event) => records.errors.push(event.message));
window.addEventListener("unhandledrejection", (event) =>
  records.errors.push(String(event.reason)),
);
async function api(path: string, body?: unknown) {
  const r = await fetch("/api" + path, {
    method: body ? "POST" : "GET",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  const v = await r.json();
  if (!r.ok) throw Error(v.error ?? String(r.status));
  return v;
}
let serial = 0;
const ordered = new StreamInputs();
const streamMode = new URL(location.href).searchParams.get("stream") === "1";
const mediaMode = new URL(location.href).searchParams.get("audio") === "media";
const mirroredAssets =
  "/__oss/" + new URL(basketballAssets.assetsBaseURL).pathname.slice(1);
if (mediaMode) {
  // Observe native media lifecycle; do not replace Audio, play(), load() or
  // event delivery. The fixture serves verified original bytes over loopback.
  const ids = new WeakMap<HTMLAudioElement, number>();
  let nextID = 0;
  for (const type of [
    "loadstart",
    "loadedmetadata",
    "canplay",
    "playing",
    "pause",
    "emptied",
    "abort",
    "error",
  ]) {
    document.addEventListener(
      type,
      (event) => {
        const media = event.target;
        if (
          !(media instanceof HTMLAudioElement) ||
          !("platformMusic" in media.dataset)
        )
          return;
        if (!ids.has(media)) ids.set(media, ++nextID);
        records.mediaEvents.push({
          at: performance.now(),
          type,
          element: ids.get(media)!,
          src: media.getAttribute("src") ?? "",
          currentSrc: media.currentSrc,
          paused: media.paused,
          readyState: media.readyState,
          networkState: media.networkState,
          error: media.error?.code ?? null,
        });
      },
      true,
    );
  }
}
let stateValue: any, activeSocket: WebSocket | undefined;
(window as any).disconnectInputStream = () => activeSocket?.close();
const silence: GameAudioPort = {
  play: async () => true,
  playWithFallback: async () => true,
  stop: () => {},
  pause: () => {},
  resume: async () => true,
  music: async (sources, options) => {
    records.music.push({ sources, options });
    if (records.music.length === 1) {
      records.denyFirstMusic = true;
      return false;
    }
    return true;
  },
  stopMusic: () => {},
  loop: async () => true,
  stopLoop: () => {},
};
function App() {
  const [state, setState] = useState<any>(),
    [settings, setSettings] = useState(false);
  const [audio] = useState(() => (mediaMode ? new GameAudio() : undefined));
  const gameAudio = useMemo<GameAudioPort>(() => {
    if (!audio || !state?.room) return silence;
    const port = audio.channel(`room:${state.room.id}`);
    return {
      ...port,
      music: (sources, options) => {
        records.music.push({ sources, options });
        return port.music(sources, options);
      },
    };
  }, [audio, state?.room?.id]);
  useEffect(() => {
    audio?.mount();
    return () => audio?.dispose();
  }, [audio]);
  const [clockEpoch, setClockEpoch] = useState(0);
  const [capacity, setCapacity] = useState(1);
  const [options, setOptions] = useState<BasketballOptions>({
    court: "new-york",
    mode: "solo",
  });
  const accept = (v: any) => {
    stateValue = v;
    records.room = v.room;
    records.userID = v.user.id;
    setState(v);
  };
  const refresh = async () => {
    const v = await api("/state");
    accept(mergeStreamState(stateValue, v, performance.now()));
  };
  useEffect(() => {
    if (!streamMode) return;
    let alive = true,
      timer: ReturnType<typeof setTimeout>,
      connected = false;
    const connect = () => {
      const socket = new WebSocket(
        location.origin.replace(/^http/, "ws") + "/api/ws?stream=1",
      );
      activeSocket = socket;
      socket.onopen = () => ordered.attach(socket);
      socket.onmessage = (event) => {
        const packet = JSON.parse(String(event.data));
        records.packets.push({
          at: performance.now(),
          bytes: new TextEncoder().encode(String(event.data)).length,
          type: packet.type,
          full:
            !!packet.game?.data?.runtime ||
            !!packet.state?.room?.game?.data?.runtime,
        });
        if (packet.type === "ack") {
          ordered.ack(packet);
          return;
        }
        if ((window as any).freezeBasketballAuthority) return;
        if (packet.type === "state") {
          if (
            connected ||
            (stateValue?.serverInstance &&
              stateValue.serverInstance !== packet.state.serverInstance)
          )
            setClockEpoch((n) => n + 1);
          connected = false;
          accept(mergeStreamState(stateValue, packet.state, performance.now()));
        } else if (packet.type === "game") {
          const merged = mergeStreamGame(stateValue, packet, performance.now());
          if (merged) accept(merged);
        }
      };
      socket.onclose = () => {
        ordered.close();
        if (alive) {
          connected = true;
          timer = setTimeout(connect, 100);
        }
      };
    };
    connect();
    return () => {
      alive = false;
      clearTimeout(timer);
      activeSocket?.close();
      ordered.close();
    };
  }, []);
  useEffect(() => {
    if (streamMode) return;
    let alive = true;
    const run = async () => {
      try {
        await refresh();
      } catch (e) {
        records.errors.push(String(e));
      }
      if (alive) setTimeout(run, 80);
    };
    void run();
    return () => {
      alive = false;
    };
  }, []);
  const act = async (action: Record<string, unknown>) => {
    const room = records.room!;
    records.actions.push(action);
    const nonce = "browser-network-" + ++serial;
    const sent = {
      at: performance.now(),
      action,
      ackAt: undefined as number | undefined,
    };
    records.sent.push(sent);
    if (streamMode && ordered.connected)
      await ordered.send({
        roomId: room.id,
        version: room.version,
        serverInstance: stateValue.serverInstance,
        nonce,
        action,
      });
    else
      await api("/rooms/" + room.id, {
        type: "action",
        version: room.version,
        nonce,
        action,
      });
    sent.ackAt = performance.now();
    if (records.ackDelay)
      await new Promise((r) => setTimeout(r, records.ackDelay));
  };
  (window as any).switchNetworkUser = async (name: string) => {
    await api("/login", { name, password: "poker-test-password" });
    await refresh();
  };
  return (
    <main
      onPointerDownCapture={() => {
        void audio?.unlock();
      }}
      onKeyDownCapture={() => {
        void audio?.unlock();
      }}
    >
      <h1>热血篮球 · 联机验收</h1>
      <details id="room-options-preview">
        <summary>创建选项验收</summary>
        <label>
          座位
          <select
            aria-label="座位"
            value={capacity}
            onChange={(e) => setCapacity(+e.target.value)}
          >
            <option value={1}>1 人</option>
            <option value={2}>2 人</option>
          </select>
        </label>
        <OptionsForm
          value={options}
          capacity={capacity}
          onChange={setOptions}
          disabled={false}
        />
        <output id="mode-config">{JSON.stringify(options)}</output>
      </details>
      <button id="settings" onClick={() => setSettings(true)}>
        控制器设置
      </button>
      {state?.room && (
        <Board
          room={state.room}
          data={state.room.game.data}
          act={act}
          command={async () => {}}
          busy={false}
          assets={mediaMode ? mirroredAssets : ""}
          sound={mediaMode}
          audio={gameAudio}
          inputWindow={ordered.connected ? 8 : 1}
          clock={{
            serverNow: state.serverNow,
            receivedAt: state.receivedAt ?? performance.now(),
            epoch: clockEpoch,
          }}
        />
      )}
      {settings && (
        <ControllerSettings
          userId={state.user.id}
          onClose={() => setSettings(false)}
        />
      )}
    </main>
  );
}
void api("/login", { name: "牌局管理员", password: "poker-test-password" })
  .then(() => createRoot(document.querySelector("#root")!).render(<App />))
  .catch((e) => {
    records.errors.push(String(e));
    document.body.textContent = String(e);
  });
