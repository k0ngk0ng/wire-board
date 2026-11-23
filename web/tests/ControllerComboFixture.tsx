import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { FCController } from "../src/controllers/FCController";
import { MDController } from "../src/controllers/MDController";
import { GameFullscreen, type GameFullscreenHandle } from "../src/ui/room-fullscreen";
import { RuntimeInput } from "../../games/contra/web/runtime-input";
import { MDInput } from "../../games/yuyuhakusho/web/input";
import "../../games/contra/web/runtime.css";
import "../../games/yuyuhakusho/web/network.css";

const md = new URLSearchParams(location.search).get("kind") === "md";
const keyboard = md
  ? { up: "KeyW", down: "KeyS", left: "KeyA", right: "KeyD", mode: "KeyF", start: "Space", a: "KeyJ", b: "KeyK", c: "KeyL", x: "KeyU", y: "KeyI", z: "KeyO" }
  : { up: "KeyW", down: "KeyS", left: "KeyA", right: "KeyD", select: "KeyF", start: "Space", a: "KeyK", b: "KeyJ", turboA: "KeyI", turboB: "KeyU" };
const initial = { revision: 0, turboHz: 10, keyboard: Object.fromEntries(Object.entries(keyboard).filter((entry): entry is [string, string] => typeof entry[1] === "string")) };
const probe = { ready: false, mask: 0, samples: [] as number[], events: [] as { type: string; trusted: boolean }[] };
Object.assign(window, { controllerComboProbe: probe });
for (const type of ["touchstart", "touchmove", "touchend", "touchcancel"]) document.addEventListener(type, event => { probe.events.push({ type, trusted: event.isTrusted }); });

function App() {
  const root = useRef<HTMLDivElement>(null);
  const input = useRef<RuntimeInput | MDInput | null>(null);
  const fullscreen = useRef<GameFullscreenHandle>(null);
  const [profile, setProfile] = useState(initial);
  const [disabled, setDisabled] = useState(false);
  const [configure, setConfigure] = useState(false);
  const [mounted, setMounted] = useState(true);
  useEffect(() => {
    const source = md ? new MDInput(root.current!, initial, mask => { probe.mask = mask; }) : new RuntimeInput(initial, root.current!, () => { probe.mask = 0; });
    input.current = source;
    source.setEnabled(true);
    let frame = 0;
    const sample = () => {
      const mask = source instanceof MDInput ? source.sample() : source.sample(performance.now());
      if (mask !== probe.mask) probe.samples.push(mask);
      probe.mask = mask;
      frame = requestAnimationFrame(sample);
    };
    frame = requestAnimationFrame(sample);
    probe.ready = true;
    return () => { cancelAnimationFrame(frame); source.dispose(); input.current = null; };
  }, []);
  useEffect(() => { input.current?.configure(profile); }, [profile]);
  useEffect(() => { input.current?.setEnabled(!disabled && !configure); }, [disabled, configure]);
  const sink = { pressPointer: (id: number, buttons: readonly string[]) => input.current?.pressPointer(id, buttons), releasePointer: (id: number, cancelled: boolean) => input.current?.releasePointer(id, cancelled) };
  const controller = md ? <MDController profile={profile} mode={configure ? "configure" : "play"} disabled={disabled} sink={sink} /> : <FCController profile={profile} mode={configure ? "configure" : "play"} disabled={disabled} sink={sink} />;
  return <>
    <header style={{ height: 50 }}>
      <button id="fullscreen" onClick={() => fullscreen.current?.enter()}>全屏</button>
      <button id="disable" onClick={() => setDisabled(value => !value)}>禁用</button>
      <button id="profile" onClick={() => setProfile(value => ({ ...value, revision: value.revision + 1 }))}>配置</button>
      <button id="configure" onClick={() => setConfigure(value => !value)}>配置图</button>
      <button id="unmount" onClick={() => setMounted(value => !value)}>卸载</button>
    </header>
    <GameFullscreen ref={fullscreen} scope="controller-combos" enabled onReadyChange={() => {}}>
      <div ref={root} className={`${md ? "yuyu-board" : "contra-runtime"} touch-controller-stage`} data-game-focus tabIndex={0} style={{ height: "calc(100dvh - 50px)", position: "relative", maxWidth: "none", background: "#101c21" }}>
        <div className={`${md ? "yuyu-screen" : "contra-runtime-screen"} touch-controller-picture`} style={{ height: "100%" }}><canvas width={md ? 320 : 256} height={md ? 224 : 240} style={{ background: "linear-gradient(#427995,#4c7450 68%,#1e3427)" }} /></div>
        {mounted && <div className={configure ? "controller-settings" : `${md ? "yuyu-controller" : "contra-runtime-controller"} touch-controller-overlay`}>{controller}</div>}
      </div>
    </GameFullscreen>
  </>;
}

createRoot(document.getElementById("root")!).render(<App />);
