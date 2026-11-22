import React, { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { MDController } from "../../../web/src/controllers/MDController";
import { MDInput } from "../web/input";
import "../web/network.css";

const initial = { revision: 0, turboHz: 10, keyboard: { up: "KeyW", down: "KeyS", left: "KeyA", right: "KeyD", mode: "KeyF", start: "Space", a: "KeyJ", b: "KeyK", c: "KeyL", x: "KeyU", y: "KeyI", z: "KeyO" } };
const probe = { mask: 0, events: [] as { type: string; trusted: boolean }[] };
Object.assign(window, { mdTouchProbe: probe });
for (const type of ["touchstart", "touchmove", "touchend", "touchcancel"]) document.addEventListener(type, event => { probe.events.push({ type, trusted: event.isTrusted }); });

function App() {
  const root = useRef<HTMLElement>(null);
  const input = useRef<MDInput | undefined>(undefined);
  const [disabled, setDisabled] = useState(false);
  const [profile, setProfile] = useState(initial);
  const [overlay, setOverlay] = useState(false);
  useEffect(() => {
    const source = new MDInput(root.current!, initial, mask => { probe.mask = mask; });
    input.current = source;
    source.setEnabled(true);
    let frame = 0;
    const sample = () => { probe.mask = source.sample(); frame = requestAnimationFrame(sample); };
    frame = requestAnimationFrame(sample);
    return () => { cancelAnimationFrame(frame); source.destroy(); input.current = undefined; };
  }, []);
  useEffect(() => { input.current?.setEnabled(!disabled); }, [disabled]);
  useEffect(() => { input.current?.configure(profile); }, [profile]);
  return <main ref={root} tabIndex={0} style={{ margin: 0, maxWidth: 840 }}>
    <button id="disable" onClick={() => setDisabled(value => !value)}>切换禁用</button>
    <button id="profile" onClick={() => setProfile(value => ({ ...value, revision: value.revision + 1 }))}>切换配置</button>
    <button id="overlay" onClick={() => setOverlay(value => !value)}>切换游戏浮层</button>
    {overlay ? <div className="yuyu-board" style={{ height: 'calc(100dvh - 50px)', maxWidth: 'none' }}><div className="yuyu-screen"><canvas width={320} height={224} /><div className="yuyu-controller"><MDController profile={profile} mode="play" disabled={disabled} sink={{ pressPointer: (id, buttons) => input.current?.pressPointer(id, buttons), releasePointer: (id, cancelled) => input.current?.releasePointer(id, cancelled) }} /></div></div></div> : <MDController profile={profile} mode="play" disabled={disabled} sink={{ pressPointer: (id, buttons) => input.current?.pressPointer(id, buttons), releasePointer: (id, cancelled) => input.current?.releasePointer(id, cancelled) }} />}
  </main>;
}

createRoot(document.getElementById("root")!).render(<App />);
