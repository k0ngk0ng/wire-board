import { createRoot } from "react-dom/client";
import { OptionsForm } from "../../games/basketball/web/module";
import type { BasketballOptions } from "../../games/basketball/web/module";
import { Board } from "../../games/basketball/web/Board";
import { ControllerSettings } from "../../web/src/controllers/Settings";
import { useEffect, useState } from "react";
import type { GameRoom, GameAudioPort } from "../../web/src/game-api/module";

// The room and actions use the real authenticated host. Only the outer page
// and audio sink are fixture plumbing; no world fields are changed here.
const records: {
  actions: any[];
  errors: string[];
  room?: GameRoom;
  ackDelay: number;
  userID: string;
  music: { sources: readonly string[]; options: unknown }[];
  denyFirstMusic?: boolean;
} = { actions: [], errors: [], ackDelay: 0, userID: "", music: [] };
(window as any).networkBoard = records;
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
  const [capacity, setCapacity] = useState(1);
  const [options, setOptions] = useState<BasketballOptions>({
    court: "new-york",
    mode: "solo",
  });
  const refresh = async () => {
    const v = await api("/state");
    records.room = v.room;
    records.userID = v.user.id;
    setState(v);
  };
  useEffect(() => {
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
    await api("/rooms/" + room.id, {
      type: "action",
      version: room.version,
      nonce: "browser-network-" + ++serial,
      action,
    });
    if (records.ackDelay)
      await new Promise((r) => setTimeout(r, records.ackDelay));
  };
  (window as any).switchNetworkUser = async (name: string) => {
    await api("/login", { name, password: "poker-test-password" });
    await refresh();
  };
  return (
    <main>
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
          assets=""
          sound={false}
          audio={silence}
          clock={{ serverNow: state.serverNow, receivedAt: performance.now() }}
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
