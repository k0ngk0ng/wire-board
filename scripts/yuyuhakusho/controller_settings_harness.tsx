// Acceptance fixture: real ControllerSettings and PhysicalController with the
// real authenticated HTTP API. The host wrapper is not the production main UI.
import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { ControllerSettings } from "../../web/src/controllers/Settings";
import { PhysicalController } from "../../web/src/controllers/gamepad";
import type { ControllerSettingsData } from "../../web/src/controllers/input";
import { api } from "../../web/src/ui/board-host";
import "../../web/src/style.css";

declare global {
  interface Window {
    controllerAcceptanceEvents: unknown[];
    controllerAcceptanceProbe?: { revision: number; at: number; held: string };
  }
}
window.controllerAcceptanceEvents = [];
window.addEventListener("controller-settings-changed", (event) =>
  window.controllerAcceptanceEvents.push((event as CustomEvent).detail),
);
function Probe({ data }: { data: ControllerSettingsData }) {
  const [held, setHeld] = useState("");
  useEffect(() => {
    const model = data.models.find((m) => m.id === "game:yuyuhakusho")!;
    const physical = new PhysicalController(data.profiles[model.id], model);
    let frame = 0;
    const tick = (at: number) => {
      const value = [...physical.sample(at)].sort().join(",");
      window.controllerAcceptanceProbe = {
        revision: data.profiles[model.id].revision,
        at,
        held: value,
      };
      setHeld(value);
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(frame);
      physical.dispose();
    };
  }, [data]);
  return (
    <output id="sampler-output" aria-label="公共物理采样验证">
      {held}
    </output>
  );
}
function App() {
  const [user, setUser] = useState<{ id: string; name: string }>();
  const [open, setOpen] = useState(false),
    [data, setData] = useState<ControllerSettingsData>();
  const [error, setError] = useState("");
  async function refresh() {
    const state = await api("/state");
    setUser(state.user);
    setData(await api("/controllers"));
  }
  useEffect(() => {
    void refresh().catch(() => {});
  }, []);
  async function login(name: string) {
    setOpen(false);
    try {
      await api("/logout", {});
      await api("/login", { name, password: "test-password-123" }).catch(() =>
        api("/register", {
          name,
          password: "test-password-123",
          invite: "controller-test",
        }),
      );
      await refresh();
      setError("");
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <main style={{ padding: 20 }}>
      <h1>控制器设置本地验收</h1>
      <button id="user-a" onClick={() => void login("ControllerBrowserA")}>
        测试用户 A
      </button>
      <button id="user-b" onClick={() => void login("ControllerBrowserB")}>
        测试用户 B
      </button>
      <button id="open-settings" disabled={!user} onClick={() => setOpen(true)}>
        控制器设置
      </button>
      <p id="current-user">{user?.name}</p>
      <p role="alert">{error}</p>
      {data && <Probe data={data} />}
      {open && user && (
        <ControllerSettings
          key={user.id}
          userId={user.id}
          gameId="yuyuhakusho"
          availableGames={["yuyuhakusho"]}
          onClose={() => {
            setOpen(false);
            void refresh();
          }}
        />
      )}
    </main>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
