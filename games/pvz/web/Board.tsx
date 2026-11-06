import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { assetURL, loadArt, spriteFrame, type Art } from "./art";
import { hitStage, paint, STAGE, type Hover } from "./paint";
import { createPredictor } from "./runtime";
import type { PvzAction, Spec, View } from "./types";

type Predictor = Awaited<ReturnType<typeof createPredictor>>;
function complete(view: View | undefined): view is View { return !!view && Array.isArray(view.players) && Array.isArray(view.plants); }
function formatClock(seconds: number) { const n = Math.max(0, Math.ceil(seconds)); return `${Math.floor(n / 60)}:${String(n % 60).padStart(2, "0")}`; }

function CardArt({ art, kind }: { art: Art; kind: string }) {
  const canvas = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const context = canvas.current?.getContext("2d");
    if (!context) return;
    context.clearRect(0, 0, 100, 90);
    const selected = spriteFrame(art, kind, 0);
    if (!selected) return;
    const { image, frame } = selected;
    const scratch = document.createElement("canvas"); scratch.width = frame.w; scratch.height = frame.h;
    const scan = scratch.getContext("2d", { willReadFrequently: true });
    let crop = { x: 0, y: 0, w: frame.w, h: frame.h };
    if (scan) {
      scan.drawImage(image, frame.x, frame.y, frame.w, frame.h, 0, 0, frame.w, frame.h);
      try {
        const pixels = scan.getImageData(0, 0, frame.w, frame.h).data;
        let x1 = frame.w, y1 = frame.h, x2 = -1, y2 = -1;
        for (let y = 0; y < frame.h; y++) for (let x = 0; x < frame.w; x++) if (pixels[(y * frame.w + x) * 4 + 3] > 4) { x1 = Math.min(x1, x); y1 = Math.min(y1, y); x2 = Math.max(x2, x); y2 = Math.max(y2, y); }
        if (x2 >= x1) crop = { x: x1, y: y1, w: x2 - x1 + 1, h: y2 - y1 + 1 };
      } catch { /* Source CORS is checked by publishing; retain full-frame preview on failure. */ }
    }
    const scale = Math.min(80 / crop.w, 82 / crop.h);
    const w = crop.w * scale, h = crop.h * scale;
    context.drawImage(image, frame.x + crop.x, frame.y + crop.y, crop.w, crop.h, (100 - w) / 2, 86 - h, w, h);
  }, [art, kind]);
  return <canvas ref={canvas} width={100} height={90} aria-hidden="true" />;
}

export function Board(props: BoardProps<View>) {
  const latest = useRef(props); latest.current = props;
  const canvas = useRef<HTMLCanvasElement>(null);
  const controllerRoot = useRef<HTMLDivElement>(null);
  const grid = useRef<HTMLDivElement>(null);
  const runtime = useRef<Predictor | null>(null);
  const authoritative = useRef<View>(props.data);
  const frame = useRef<View>(props.data);
  const paintState = useRef({ selected: null as string | null, shovel: false, hover: null as Hover });
  const nextSeq = useRef(0);
  const lastIdentity = useRef("");
  const eventCursor = useRef({ epoch: "", id: -1 });
  const inFlight = useRef(new Set<number>());
  const inputAt = useRef(0);
  const inputSequence = useRef(0);
  const isReady = useRef(false);
  const [view, setView] = useState(props.data);
  const [art, setArt] = useState<Art | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [shovel, setShovel] = useState(false);
  const [error, setError] = useState("");
  const [loadError, setLoadError] = useState("");
  const [ready, setReady] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [now, setNow] = useState(0);
  const [progress, setProgress] = useState(0);
  const player = view.players?.[props.room.you];
  const watching = !!props.room.spectating || props.room.you < 0;
  const side = player?.side ?? "plants";
  const cards = view.catalog?.[side] ?? [];
  const automated = !!props.room.seats[props.room.you]?.autoPlay;
  const usable = ready && !watching && !automated && !!view.canAct && ["prepare", "battle"].includes(view.phase) && !(side === "zombies" && view.phase === "prepare");
  const displayNow = props.clock.serverNow + Math.max(0, now - props.clock.receivedAt);
  const remaining = Math.max(0, (view.phaseUntil - displayNow) / 1000);

  const choose = (kind: string | null, useShovel = false) => {
    paintState.current.selected = kind; paintState.current.shovel = useShovel;
    setSelected(kind); setShovel(useShovel); setError("");
  };
  useControllerCancel(controllerRoot, () => choose(null));

  useEffect(() => {
    const controller = new AbortController();
    let retired = false;
    setReady(false); isReady.current = false; setLoadError("");
    const report = (value: number, label: string) => {
      if (retired) return;
      setProgress(value); latest.current.onLoadProgress?.(value, label);
    };
    Promise.all([
      loadArt(props.assets, controller.signal, report),
      createPredictor(props.assets),
    ]).then(([loadedArt, predictor]) => {
      if (retired) { predictor.dispose(); return; }
      runtime.current = predictor;
      if (complete(authoritative.current)) predictor.update(authoritative.current);
      predictor.update(latest.current.data);
      const initial = predictor.sample(performance.now());
      if (complete(initial)) { frame.current = initial; setView(initial); }
      setArt(loadedArt);
      report(95, "初始化草坪画面");
    }).catch(reason => {
      if (retired || reason?.name === "AbortError") return;
      const message = reason instanceof Error ? reason.message : "游戏资源加载失败";
      setLoadError(message); latest.current.onLoadError?.(message);
    });
    return () => {
      retired = true; controller.abort(); runtime.current?.dispose(); runtime.current = null;
    };
  }, [props.assets, attempt]);

  useEffect(() => {
    if (complete(props.data)) authoritative.current = props.data;
    runtime.current?.update(props.data);
    const current = runtime.current?.sample(performance.now()) ?? props.data;
    if (complete(current)) { frame.current = current; setView(current); }
    const identity = `${props.room.id}:${current.epoch}:${props.room.you}:${automated}`;
    if (lastIdentity.current !== identity) {
      lastIdentity.current = identity;
      inFlight.current.clear(); nextSeq.current = current.players?.[props.room.you]?.seq ?? 0;
      choose(null); paintState.current.hover = null;
    } else nextSeq.current = Math.max(nextSeq.current, current.players?.[props.room.you]?.seq ?? 0);
  }, [props.data, props.room.id, props.room.you, automated]);

  useEffect(() => {
    if (!art) return;
    let id = 0, lastHUD = 0;
    const media = matchMedia("(prefers-reduced-motion: reduce)");
    const resize = () => {
      const element = canvas.current, scene = element?.parentElement;
      if (!element || !scene) return;
      if (element.closest("[data-room-fullscreen]")) {
        const box = scene.getBoundingClientRect();
        const style = getComputedStyle(scene);
        const availableWidth = box.width - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
        const width = Math.min(availableWidth, box.height * STAGE.width / STAGE.height);
        element.style.width = `${width}px`; element.style.height = `${width * STAGE.height / STAGE.width}px`;
      } else { element.style.width = "100%"; element.style.height = "auto"; }
      if (grid.current) {
        grid.current.style.left = `${element.offsetLeft}px`; grid.current.style.top = `${element.offsetTop}px`;
        grid.current.style.width = `${element.clientWidth}px`; grid.current.style.height = `${element.clientHeight}px`;
      }
      const shovelButton = controllerRoot.current?.querySelector<HTMLButtonElement>(".pvz-shovel");
      if (shovelButton) {
        if (matchMedia("(max-width:650px)").matches && (!element.closest("[data-room-fullscreen]") || matchMedia("(orientation:portrait)").matches)) {
          shovelButton.style.top = `${scene.offsetTop + element.offsetTop + element.clientHeight - 47}px`;
          shovelButton.style.bottom = "auto";
        } else { shovelButton.style.top = ""; shovelButton.style.bottom = ""; }
      }
    };
    const observer = new ResizeObserver(resize);
    if (canvas.current?.parentElement) observer.observe(canvas.current.parentElement);
    if (controllerRoot.current) observer.observe(controllerRoot.current);
    resize();
    const render = (time: number) => {
      const current = runtime.current?.sample(time) ?? frame.current;
      if (complete(current)) frame.current = current;
      const scene = frame.current;
      const element = canvas.current;
      const context = element?.getContext("2d");
      if (element && context && complete(scene)) {
        const rect = element.getBoundingClientRect();
        const ratio = Math.min(2, devicePixelRatio || 1);
        const w = Math.max(1, Math.round(rect.width * ratio)), h = Math.max(1, Math.round(rect.height * ratio));
        if (element.width !== w || element.height !== h) { element.width = w; element.height = h; }
        context.setTransform(w / STAGE.width, 0, 0, h / STAGE.height, 0, 0);
        const p = latest.current;
        const canAct = !p.room.spectating && p.room.you >= 0 && !p.room.seats[p.room.you]?.autoPlay && scene.canAct && !p.clock.paused;
        paint(context, scene, art, { ...paintState.current, actor: p.room.you, canAct, now: time, reducedMotion: media.matches });
        Object.assign(element, { __pvzFrame: { tick: scene.tick, epoch: scene.epoch, phase: scene.phase, renderAt: time, pixelAt: performance.now(), plants: scene.plants.length, zombies: scene.zombies.length, projectiles: scene.projectiles.length, selected: paintState.current.selected, inputEnabled: canAct, player: p.room.you, lastInputAt: inputAt.current, lastActionSeq: inputSequence.current } });
        if (!isReady.current) {
          isReady.current = true; setReady(true); setProgress(100);
          p.onLoadProgress?.(100, "草坪已就绪"); p.onReady?.();
        }
      }
      if (time - lastHUD >= 100) { lastHUD = time; setNow(time); if (complete(current)) setView(current); }
      id = requestAnimationFrame(render);
    };
    id = requestAnimationFrame(render);
    return () => { cancelAnimationFrame(id); observer.disconnect(); };
  }, [art]);

  useEffect(() => {
    if (!art || !ready || !complete(view)) return;
    const events = view.events ?? [];
    const maxID = events.reduce((max, event) => Math.max(max, event.id), -1);
    if (eventCursor.current.epoch !== view.epoch) {
      eventCursor.current = { epoch: view.epoch, id: maxID }; return;
    }
    if (props.sound && !document.hidden) {
      const seen = new Set<string>();
      for (const event of events) {
        if (event.id <= eventCursor.current.id || seen.has(event.type)) continue;
        const sources = art.manifest.audio?.[event.type];
        if (sources) {
          seen.add(event.type);
          const paths = (Array.isArray(sources) ? sources : [sources]).map(path => assetURL(art.base, path));
          void props.audio.playWithFallback(paths);
        }
      }
    }
    eventCursor.current.id = Math.max(eventCursor.current.id, maxID);
  }, [view, props.sound, art, ready, props.audio]);

  useEffect(() => {
    const clear = () => { choose(null); paintState.current.hover = null; };
    window.addEventListener("blur", clear);
    document.addEventListener("visibilitychange", clear);
    return () => { window.removeEventListener("blur", clear); document.removeEventListener("visibilitychange", clear); };
  }, []);

  const actAt = (row: number, col: number) => {
    const p = latest.current, current = frame.current;
    const own = current.players?.[p.room.you];
    if (!isReady.current || p.room.spectating || p.room.you < 0 || !own || p.room.seats[p.room.you]?.autoPlay || !current.canAct || p.clock.paused) return;
    if (!["prepare", "battle"].includes(current.phase) || (own.side === "zombies" && current.phase === "prepare")) return;
    let action: PvzAction;
    if (paintState.current.shovel) {
      const plant = current.plants.find(item => item.row === row && item.col === col);
      if (!plant) return;
      if (plant.owner !== p.room.you) { setError("只能铲除自己的植物"); return; }
      action = { type: "shovel", epoch: current.epoch, seq: ++nextSeq.current, plantId: plant.id };
    } else {
      const kind = paintState.current.selected;
      if (!kind) return;
      const spec = current.catalog[own.side].find(card => card.kind === kind);
      if (!spec) return;
      if (own.money < spec.cost) { setError(own.side === "plants" ? "阳光不足" : "脑力不足"); return; }
      if ((own.cooldowns[kind] ?? 0) > current.tick) { setError("卡牌尚未冷却"); return; }
      if (own.side === "plants" && current.plants.some(plant => plant.row === row && plant.col === col)) { setError("这个格子已有植物"); return; }
      action = { type: own.side === "plants" ? "plant" : "spawn", epoch: current.epoch, seq: ++nextSeq.current, kind, row, ...(own.side === "plants" ? { col } : {}) };
    }
    if (inFlight.current.size >= 8) { setError("正在确认操作，请稍候"); return; }
    inFlight.current.add(action.seq);
    inputAt.current = performance.now(); inputSequence.current = action.seq;
    const accepted = runtime.current?.apply(action, p.room.you);
    if (accepted === false) { inFlight.current.delete(action.seq); setError("当前无法执行此操作"); return; }
    const predicted = runtime.current?.sample(performance.now());
    if (predicted && complete(predicted)) { frame.current = predicted; setView(predicted); }
    setError("");
    void p.act({ ...action }).catch(reason => {
      runtime.current?.reset(); runtime.current?.update(authoritative.current);
      setError(reason instanceof Error ? reason.message : "操作未成功，请重试");
    }).finally(() => inFlight.current.delete(action.seq));
  };

  function renderCard(card: Spec) {
    const cooldown = Math.max(0, ((player?.cooldowns[card.kind] ?? 0) - view.tick) * 50);
    const unaffordable = (player?.money ?? 0) < card.cost;
    const chosen = selected === card.kind && !shovel;
    const total = Math.max(1, card.cooldown ?? ((card.cooldownTicks ?? 20) * 50));
    return <button type="button" key={card.kind} className={`pvz-seed${chosen ? " pvz-selected" : ""}${unaffordable ? " pvz-unaffordable" : ""}`} data-pvz-card={card.kind} aria-pressed={chosen} aria-label={`${card.name}，${card.cost}${side === "plants" ? "阳光" : "脑力"}${cooldown ? `，冷却 ${Math.ceil(cooldown / 1000)} 秒` : ""}`} title={card.description} disabled={!usable} onClick={() => choose(chosen ? null : card.kind)}>
      <span className="pvz-seed-name">{card.name}</span>
      {art && <CardArt art={art} kind={card.kind} />}
      <span className="pvz-seed-cost">{card.cost}</span>
      {cooldown > 0 && <span className="pvz-seed-cooldown" aria-hidden="true" style={{ height: `${Math.min(100, cooldown / total * 100)}%` }}><b>{Math.ceil(cooldown / 1000)}</b></span>}
    </button>;
  }

  return <div ref={controllerRoot} className={`pvz-board pvz-side-${side}`} data-pvz-board data-ready={ready} data-phase={view.phase}>
    <div className="pvz-hud">
      <div className="pvz-wallet"><span className={`pvz-resource-icon pvz-resource-${side}`} aria-hidden="true" /><strong>{player?.money ?? "—"}</strong><small>{watching ? "观战" : side === "plants" ? "阳光" : "脑力"}</small></div>
      <div className="pvz-round-clock"><span>第 {view.round ?? 1} / 2 回合</span><strong data-pvz-clock>{view.phase === "prepare" ? `布阵 ${formatClock(remaining)}` : view.phase === "intermission" ? `交换攻守 ${formatClock(remaining)}` : view.phase === "finished" ? "比赛结束" : formatClock(remaining)}</strong></div>
      <div className="pvz-role"><strong>{watching ? "观战" : side === "plants" ? "植物防守" : "僵尸进攻"}</strong>{automated && <span>托管中</span>}</div>
    </div>
    <div className="pvz-seed-rack" data-game-controls data-controller-group="cards">
      {cards.map(renderCard)}
      {side === "plants" && <button type="button" className={`pvz-shovel${shovel ? " pvz-selected" : ""}`} aria-label="铲子" aria-pressed={shovel} disabled={!usable} onClick={() => choose(null, !shovel)}>
        {art?.manifest.images?.shovel ? <img src={assetURL(art.base, art.manifest.images.shovel)} alt="" /> : <svg viewBox="0 0 50 70" aria-hidden="true"><path d="M22 6h8v35h-8z" fill="#9b6b3c"/><path d="M14 37h24l-4 23-8 8-9-8z" fill="#adb6b3" stroke="#45524b" strokeWidth="3"/><path d="M17 5h19v9H17z" fill="none" stroke="#704e31" strokeWidth="5"/></svg>}<span>铲子</span>
      </button>}
    </div>
    <div className="pvz-scene" data-game-controls>
      <canvas ref={canvas} data-pvz-canvas data-controller-ignore width={800} height={600} tabIndex={-1} role="img" aria-label={side === "plants" ? "草坪，先选择植物卡，再选择格子种植。" : "草坪，先选择僵尸卡，再选择要进攻的行。"}
        onPointerMove={event => { paintState.current.hover = hitStage(event.clientX, event.clientY, event.currentTarget.getBoundingClientRect()); }}
        onPointerLeave={() => { paintState.current.hover = null; }}
        onPointerDown={event => { if (event.button !== 0) return; event.preventDefault(); event.currentTarget.focus({ preventScroll: true }); const hit = hitStage(event.clientX, event.clientY, event.currentTarget.getBoundingClientRect()); if (hit) { paintState.current.hover = hit; actAt(hit.row, hit.col); } }}
      />
      {!ready && <div className="pvz-loading" role={loadError ? "alert" : "status"}>
        {loadError ? <><p>{loadError}</p><button type="button" onClick={() => setAttempt(value => value + 1)}>重新加载</button></> : <><strong>草坪准备中</strong><progress value={progress} max={100} aria-label="植物大战僵尸加载进度" /></>}
      </div>}
      {ready && view.phase === "intermission" && <div className="pvz-phase-banner"><strong>交换攻守</strong><span>{formatClock(remaining)}</span></div>}
      {ready && !watching && side === "zombies" && view.phase === "prepare" && <div className="pvz-phase-banner pvz-phase-small"><strong>植物正在布阵</strong><span>{formatClock(remaining)}</span></div>}
      <div ref={grid} className="pvz-grid-controls" role="grid" aria-label="草坪格子" data-controller-group="board">{Array.from({ length: 5 }, (_, row) => side === "plants" ? Array.from({ length: 9 }, (_, col) => <button key={`${row}-${col}`} type="button" aria-label={`第 ${row + 1} 行第 ${col + 1} 列`} style={{ left: `${(STAGE.left + col * STAGE.cellWidth) / STAGE.width * 100}%`, top: `${(STAGE.top + row * STAGE.cellHeight) / STAGE.height * 100}%`, width: `${STAGE.cellWidth / STAGE.width * 100}%`, height: `${STAGE.cellHeight / STAGE.height * 100}%` }} disabled={!usable} onFocus={() => { paintState.current.hover = { row, col }; }} onClick={() => actAt(row, col)} />) : <button key={row} type="button" aria-label={`向第 ${row + 1} 行出兵`} style={{ left: "95.3%", top: `${(STAGE.top + row * STAGE.cellHeight) / STAGE.height * 100}%`, width: "4.7%", height: `${STAGE.cellHeight / STAGE.height * 100}%` }} disabled={!usable} onFocus={() => { paintState.current.hover = { row, col: 8 }; }} onClick={() => actAt(row, 8)} />)}</div>
    </div>
    <div className="pvz-below"><span className="pvz-action-error" role="status" aria-live="polite">{error}</span><span className="pvz-round-history">{view.rounds?.map((round, i) => <span key={i}>{round.attackTeam === 0 ? "A" : "B"} 队 · {round.breached ? formatClock(round.elapsedMS / 1000) : "守住"}</span>)}</span></div>
  </div>;
}
