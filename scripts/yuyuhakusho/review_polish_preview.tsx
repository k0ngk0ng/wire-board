// Targeted visual/input fixture around the actual shared controller and cover.
import { createRoot } from "../../web/node_modules/react-dom/client";
import { useState } from "react";
import { Cover } from "../../games/yuyuhakusho/web/Cover";
import { MDController } from "../../web/src/controllers/MDController";
import { Preparation } from "../../games/yuyuhakusho/web/Preparation";
const profile = { revision: 0, turboHz: 10, keyboard: {
  up: "KeyW", down: "KeyS", left: "KeyA", right: "KeyD", mode: "KeyF", start: "Space",
  a: "KeyJ", b: "KeyK", c: "KeyL", x: "KeyU", y: "KeyI", z: "KeyO",
} };
const events: unknown[] = [];
Object.assign(window, { mdPolishEvents: events });
function Preview() {
  const [mode, setMode] = useState<"play" | "configure">("play");
  const [options, setOptions] = useState({ arena: 0, teamBattle: false, characters: [3, 9] });
  return <main style={{ maxWidth: 720, margin: "24px auto", padding: 16 }}>
    <div data-cover-preview style={{ maxWidth: 640, margin: "0 auto 24px" }}><Cover /></div>
    <MDController profile={profile} mode={mode} onRemap={button => events.push({ remap: button })}
      sink={{ pressPointer: (id, buttons) => events.push({ id, buttons }), releasePointer: (id, cancelled) => events.push({ id, cancelled }) }} />
    <button id="toggle-configure" onClick={() => setMode(mode === "play" ? "configure" : "play")}>{mode === "play" ? "配置" : "游玩"}</button>
    <Preparation room={{ id: "preview", name: "preview", kind: "yuyuhakusho", host: "human", status: "waiting", version: 1, you: 0, capacity: 2, seats: [{ id: "human", name: "玩家", ready: false }, { id: "bot", name: "电脑", ready: true, bot: true }] }} options={options} host busy={false} assets="" prepare={action => {
      events.push(action);
      if (action.type === "arena") setOptions(previous => ({ ...previous, arena: Number(action.arena) }));
      if (action.type === "character") setOptions(previous => ({ ...previous, characters: previous.characters.map((id, seat) => seat === action.seat ? Number(action.character) : id) }));
    }} />
  </main>;
}
createRoot(document.getElementById("root")!).render(<Preview />);
