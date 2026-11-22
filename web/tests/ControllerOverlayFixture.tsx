import React, { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { createPortal } from "react-dom";
import { FCController } from "../src/controllers/FCController";
import { MDController } from "../src/controllers/MDController";
import { TeamHud, TouchPad } from "../../games/battlecity/web/Board";
import { GameFullscreen, type GameFullscreenHandle } from "../src/ui/room-fullscreen";
import { useChatPosition } from "../src/ui/chat-position";
import "../../games/contra/web/runtime.css";
import "../../games/basketball/web/network.css";
import "../../games/yuyuhakusho/web/network.css";
import "../../games/battlecity/web/style.css";

const profile = { revision: 0, turboHz: 10, keyboard: { up: "KeyW", down: "KeyS", left: "KeyA", right: "KeyD", select: "KeyF", mode: "KeyF", start: "Space", a: "KeyK", b: "KeyJ", c: "KeyL", x: "KeyU", y: "KeyI", z: "KeyO", turboA: "KeyI", turboB: "KeyU" } };
const probe = { actions: [] as string[], move: "none", fire: false };
Object.assign(window, { controllerOverlayProbe: probe });

function Game({ fullscreen }: { fullscreen: boolean }) {
  const contacts = useRef(new Map<number, readonly string[]>());
  useEffect(() => { Object.assign(window, { controllerOverlayReady: true }); }, []);
  const sink = { pressPointer: (id: number, buttons: readonly string[]) => { contacts.current.set(id, buttons); probe.actions = [...contacts.current.values()].flatMap(value => [...value]); }, releasePointer: (id: number) => { contacts.current.delete(id); probe.actions = [...contacts.current.values()].flatMap(value => [...value]); } };
  const kind = new URLSearchParams(location.search).get("kind");
  const canvas = <canvas width={256} height={240} className={kind === 'battlecity' ? 'bc-canvas' : undefined} style={{ background: "linear-gradient(#428aad,#5a8b3e 65%,#283e28)" }} />;
  const style = fullscreen ? undefined : { height: "calc(100dvh - 20px)" };
  if (kind === "contra") return <div className="contra-runtime touch-controller-stage" data-overlay-stage style={style}><div className="contra-runtime-screen touch-controller-picture">{canvas}</div><div className="contra-runtime-controller touch-controller-overlay"><FCController mode="play" profile={profile} sink={sink} /></div></div>;
  if (kind === "basketball") return <div className="basketball-board touch-controller-stage" data-overlay-stage style={style}><div className="basketball-screen"><p className="basketball-status">比赛进行中</p><div className="basketball-stage touch-controller-picture">{canvas}</div></div><aside className="basketball-controls touch-controller-overlay"><FCController mode="play" profile={profile} sink={sink} /></aside></div>;
  if (kind === "yuyuhakusho") return <div className="yuyu-board touch-controller-stage" data-overlay-stage style={style}><div className="yuyu-screen touch-controller-picture">{canvas}</div><div className="yuyu-controller touch-controller-overlay"><MDController mode="play" profile={profile} sink={sink} /></div></div>;
  const room = { seats: Array.from({ length: 6 }, (_, seat) => ({ id: 'player-' + seat, name: '队员 ' + (seat + 1) })) } as unknown as Parameters<typeof TeamHud>[0]['room'];
  const view = { teamSize: 3, tanks: Array.from({ length: 6 }, (_, seat) => ({ team: Math.floor(seat / 3), alive: true, hp: 3, maxHp: 3, kills: 0, class: 'balanced' })) } as unknown as Parameters<typeof TeamHud>[0]['view'];
  return <div className="battlecity-board" data-battlecity-phase="playing" data-overlay-stage><div className="bc-status">比赛进行中</div><TeamHud room={room} view={view} team={0} serverNow={0} viewer={0} /><div className="bc-field-wrap">{canvas}</div><TeamHud room={room} view={view} team={1} serverNow={0} viewer={0} /><div className="bc-controls"><TouchPad enabled onMove={value => { probe.move = value; }} onFire={value => { probe.fire = value; }} /></div></div>;
}

function FixtureChat({ target }: { target: HTMLElement }) {
  const toggle = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLElement>(null);
  const position = useChatPosition(toggle, panel, target, false);
  return createPortal(<div className="table-chat" style={position.style}><button ref={toggle} type="button" className="chat-toggle" aria-label="聊天" {...position.handlers}>聊</button></div>, target);
}

function App() {
  const fullscreen = useRef<GameFullscreenHandle>(null);
  const [target, setTarget] = useState<HTMLElement | null>(null);
  return <><button id="fixture-fullscreen" style={{ position: 'fixed', top: 0, right: 0, zIndex: 999, display: target ? 'none' : undefined }} onClick={() => fullscreen.current?.enter()}>全屏</button><GameFullscreen ref={fullscreen} scope="controller-overlay" enabled onReadyChange={() => {}} onChatTargetChange={setTarget}><Game fullscreen={!!target} /></GameFullscreen>{target && <FixtureChat target={target} />}</>;
}

createRoot(document.getElementById("root")!).render(<App />);
