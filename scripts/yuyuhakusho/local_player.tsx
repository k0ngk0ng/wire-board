import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { MDController } from "../../web/src/controllers/MDController";
import { KeyboardController, type ControllerProfile } from "../../web/src/controllers/input";
import "./local_player.css";

type Player = { slot: number; character_id: number; computer: boolean; input_channel: number; active: boolean; health: number; energy: number; X: number; Y: number; Pose: number; Action: number };
type View = { tick: number; logic_frame: number; ready: boolean; paused: boolean; completed: boolean; winner: number; window_complete: boolean; players: Player[] };
type Reply = { ok: boolean; error?: string; view?: View; snapshot?: string; sprite_pixels?: number };
declare global {
  interface Window {
    Go: new () => { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };
    yuyuhakushoWasm: (request: string, pixels?: Uint8Array) => string;
    yyResearch: typeof research;
  }
}

const profile: ControllerProfile = { revision: 0, turboHz: 10, keyboard: {
  up: "KeyW", down: "KeyS", left: "KeyA", right: "KeyD", mode: "KeyF", start: "Space",
  b: "KeyJ", a: "KeyK", c: "KeyL", x: "KeyU", y: "KeyI", z: "KeyO",
} };
const keyboard = new KeyboardController(profile);
const pointers = new Map<number, readonly string[]>();
const bits: Record<string, number> = { up: 1, down: 2, left: 4, right: 8, b: 16, c: 32, a: 64, start: 128, z: 256, y: 512, x: 1024, mode: 2048 };
const characterNames: Record<number, string> = { 0: "幻海", 1: "藏马", 2: "桑原", 3: "幽助", 6: "阵", 8: "户愚吕兄", 9: "酎", 10: "户愚吕弟" };
function sample() {
  const held = keyboard.sample(performance.now());
  for (const buttons of pointers.values()) for (const button of buttons) held.add(button);
  return [...held].sort();
}
function clearInput() { keyboard.clear(); pointers.clear(); }
const sink = {
  pressPointer(id: number, buttons: readonly string[]) { pointers.set(id, [...buttons]); },
  releasePointer(id: number) { pointers.delete(id); },
};
const research = { assetsReady: false, view: null as View | null, pixelSignature: 0, trusted: [] as boolean[], errors: [] as string[], sample };
window.yyResearch = research;

function call(request: object, pixels?: Uint8Array): Reply {
  const response = JSON.parse(window.yuyuhakushoWasm(JSON.stringify(request), pixels)) as Reply;
  if (!response.ok) throw Error(response.error ?? "独立对局运行失败");
  if (response.view) research.view = response.view;
  return response;
}
function editable(target: EventTarget | null) {
  return target instanceof Element && !!target.closest("input,select,textarea,[contenteditable=true]");
}
function signature(bytes: Uint8Array) {
  let hash = 2166136261;
  for (let i = 0; i < bytes.length; i++) hash = Math.imul(hash ^ bytes[i], 16777619);
  return hash >>> 0;
}

function App() {
  const canvas = useRef<HTMLCanvasElement>(null);
  const running = useRef(false);
  const [ready, setReady] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState("");
  const [players, setPlayers] = useState(2);
  const [character, setCharacter] = useState(9);
  const [opponent, setOpponent] = useState(9);
  const [team, setTeam] = useState(false);
  const [watch, setWatch] = useState(false);
  const [view, setView] = useState<View | null>(null);

  useEffect(() => {
    let mounted = true;
    const targets = new Set(Object.values(profile.keyboard));
    const trusted = (event: Event) => { research.trusted.push(event.isTrusted); research.trusted.splice(0, Math.max(0, research.trusted.length - 128)); };
    const down = (event: KeyboardEvent) => {
      trusted(event);
      if (!running.current || editable(event.target) || event.metaKey || event.ctrlKey || event.altKey || !targets.has(event.code)) return;
      event.preventDefault();
      keyboard.press(event.code, performance.now(), event.repeat);
    };
    const up = (event: KeyboardEvent) => keyboard.release(event.code);
    const focus = (event: FocusEvent) => { if (editable(event.target)) clearInput(); };
    const visibility = () => { if (document.hidden) clearInput(); };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", clearInput);
    window.addEventListener("focusin", focus);
    window.addEventListener("pointerdown", trusted);
    document.addEventListener("visibilitychange", visibility);
    const context = canvas.current?.getContext("2d", { alpha: false });
    if (!context) { setError("无法初始化比赛画面"); return; }
    context.fillStyle = "#000";
    context.fillRect(0, 0, 320, 224);
    const pixels = new Uint8Array(320 * 224 * 4);
    const image = context.createImageData(320, 224);
    const downloaded: Record<string, { done: number; total: number }> = {};
    const updateProgress = () => {
      const values = Object.values(downloaded);
      const total = values.reduce((sum, value) => sum + value.total, 0);
      const done = values.reduce((sum, value) => sum + value.done, 0);
      if (mounted && total > 0) setProgress(Math.min(95, Math.round(done / total * 95)));
    };
    async function asset(url: string) {
      const response = await fetch(url);
      if (!response.ok || !response.body) throw Error("无法加载比赛资源");
      downloaded[url] = { done: 0, total: Number(response.headers.get("Content-Length")) || 0 };
      const chunks: Uint8Array[] = [];
      const reader = response.body.getReader();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        chunks.push(value);
        downloaded[url].done += value.length;
        updateProgress();
      }
      const bytes = new Uint8Array(downloaded[url].done);
      let offset = 0;
      for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
      return bytes;
    }
    async function unpack(bytes: Uint8Array) {
      const buffer = new ArrayBuffer(bytes.length);
      new Uint8Array(buffer).set(bytes);
      const stream = new Blob([buffer]).stream().pipeThrough(new DecompressionStream("gzip"));
      return JSON.parse(await new Response(stream).text());
    }
    void (async () => {
      try {
        const [wasm, ruleBytes, spriteBytes, arenaBytes, hudBytes] = await Promise.all([
          asset("/player.wasm"), asset("/rules.json.gz"), asset("/sprites.json.gz"), asset("/arena.json.gz"), asset("/hud.json.gz"),
        ]);
        const [rules, sprites, arena, hud] = await Promise.all([unpack(ruleBytes), unpack(spriteBytes), unpack(arenaBytes), unpack(hudBytes)]);
        const go = new window.Go();
        const { instance } = await WebAssembly.instantiate(wasm, go.importObject);
        void go.run(instance).catch(fail);
        if (typeof window.yuyuhakushoWasm !== "function") throw Error("比赛程序未完成初始化");
        call({ Op: "load", Rules: rules, Sprites: sprites, Arena: arena, HUD: hud });
        if (mounted) { research.assetsReady = true; setProgress(100); setReady(true); }
      } catch (reason) { fail(reason); }
    })();
    function fail(reason: unknown) {
      running.current = false; clearInput();
      const message = reason instanceof Error ? reason.message : String(reason);
      research.errors.push(message);
      if (mounted) setError(message);
    }
    let previous = 0, accumulator = 0, frameID = 0, lastUI = 0;
    const step = 1000 / 59.92274340431231;
    const loop = (at: number) => {
      frameID = requestAnimationFrame(loop);
      if (!running.current || document.hidden) { previous = at; accumulator = 0; return; }
      accumulator += Math.min(at - (previous || at), step * 4);
      previous = at;
      try {
        let changed = false;
        while (accumulator >= step) {
          const held = sample().reduce((mask, button) => mask | (bits[button] ?? 0), 0);
          const response = call({ Op: "step", Held: [held] }, pixels);
          accumulator -= step; changed = true;
          if (response.view?.completed) { running.current = false; setView(response.view); break; }
        }
        if (changed) {
          image.data.set(pixels); context.putImageData(image, 0, 0);
          research.pixelSignature = signature(pixels);
          if (at - lastUI > 100 && research.view) { setView(research.view); lastUI = at; }
        }
      } catch (reason) { fail(reason); }
    };
    frameID = requestAnimationFrame(loop);
    return () => {
      mounted = false; running.current = false; clearInput(); cancelAnimationFrame(frameID);
      window.removeEventListener("keydown", down); window.removeEventListener("keyup", up);
      window.removeEventListener("blur", clearInput); window.removeEventListener("focusin", focus);
      window.removeEventListener("pointerdown", trusted); document.removeEventListener("visibilitychange", visibility);
    };
  }, []);

  function start() {
    clearInput();
    try {
      const slots = players === 2 ? [0, 3] : players === 3 ? [0, 1, 3] : [0, 1, 2, 3];
      call({ Op: "prepare", Options: { Seed: 0x12345678, Difficulty: 6, TeamBattle: team && players === 4,
        Players: slots.map((slot, color) => ({ Slot: slot, CharacterID: slot === 0 ? character : opponent, Color: color, Computer: watch || slot !== 0, HumanChannel: 0 })),
      } });
      const response = call({ Op: "ready" });
      setView(response.view ?? null); setError(""); running.current = true;
      canvas.current?.focus();
    } catch (reason) { setError(reason instanceof Error ? reason.message : String(reason)); }
  }
  const colors = ["#476689", "#b17742", "#4f8269", "#8a5681"];
  return <main className="yy-player">
    <header className="yy-heading"><h1>魔强统一战</h1><span>本地研究</span></header>
    <div className="yy-options">
      <label>人数<select aria-label="人数" value={players} onChange={event => { setPlayers(Number(event.target.value)); clearInput(); }}><option value={2}>2 人</option><option value={3}>3 人</option><option value={4}>4 人</option></select></label>
      <label>角色<select id="player-character" aria-label="角色" value={character} onChange={event => { setCharacter(Number(event.target.value)); clearInput(); }}>{Object.entries(characterNames).map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></label>
      <label>电脑<select id="opponent-character" aria-label="电脑角色" value={opponent} onChange={event => { setOpponent(Number(event.target.value)); clearInput(); }}>{Object.entries(characterNames).map(([id, name]) => <option key={id} value={id}>{name}</option>)}</select></label>
      {players === 4 && <label className="yy-check"><input type="checkbox" checked={team} onChange={event => setTeam(event.target.checked)} />组队</label>}
      <label className="yy-check"><input type="checkbox" checked={watch} onChange={event => setWatch(event.target.checked)} />电脑演示</label>
      <button id="start-match" type="button" onClick={start} disabled={!ready}>{view ? "重新开始" : "开始对局"}</button>
    </div>
    <section className="yy-display">
      <canvas ref={canvas} width={320} height={224} tabIndex={0} aria-label="魔强统一战比赛画面" />
      {!ready && !error && <div className="yy-loading"><progress value={progress} max={100} /><span>{progress === 95 ? "正在初始化" : `加载中 ${progress}%`}</span></div>}
    </section>
    {error && <div className="yy-error" role="alert">{error}<button onClick={() => location.reload()}>重新加载</button></div>}
    {view && <div className="yy-players">{view.players.map((player, index) => <div className="yy-player-card" key={player.slot} style={{ borderTopColor: colors[index] }}>
      <span>{player.computer ? "电脑" : "你"}<small>{characterNames[player.character_id]}</small></span><strong>{Math.max(0, player.health)}</strong>
      <meter aria-label={`${player.computer ? "电脑" : "你的"}生命`} min={0} max={223} value={Math.max(0, player.health)} />
    </div>)}</div>}
    {view?.paused && <p className="yy-status">已暂停</p>}
    {view?.completed && <p className="yy-status">对局结束</p>}
    <div className="yy-pad"><MDController profile={profile} mode="play" sink={sink} disabled={!ready || !view || view.completed} /></div>
  </main>;
}

createRoot(document.getElementById("root")!).render(<App />);
