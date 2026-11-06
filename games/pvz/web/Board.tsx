import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { BoardProps } from "../../../web/src/game-api/module";
import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { useGameOverlayRoot } from "../../../web/src/ui/game-overlay-root";
import { Modal } from "../../../web/src/ui/board-host";
import { assetURL, ensureUnits, loadArt, unitReady, type Art } from "./art";
import { CardArt } from "./CardArt";
import { CardBook } from "./CardBook";
import { hitStage, paint, STAGE, type Hover } from "./paint";
import { layerName, mapFor, plantsAt, slopeAt, stageFor, terrainReason } from "./scene";
import { createPredictor } from "./runtime";
import { useSceneMusic } from "./music";
import { PvzEffects } from "./effects";
import { ResourceDownloads, type DownloadProgress } from "./downloads";
import { roundRow } from "./outcome";
import { neededUnits } from "./entry-assets";
import type { PvzAction, Spec, View } from "./types";

type Predictor = Awaited<ReturnType<typeof createPredictor>>;
function complete(view: View | undefined): view is View { return !!view && Array.isArray(view.players) && Array.isArray(view.plants); }
function formatClock(seconds: number) { const n = Math.max(0, Math.ceil(seconds)); return `${Math.floor(n / 60)}:${String(n % 60).padStart(2, "0")}`; }


export function Board(props: BoardProps<View>) {
  const overlayRoot = useGameOverlayRoot();
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
  const effects = useRef<PvzEffects | null>(null);
  const inFlight = useRef(new Set<number>());
  const inputAt = useRef(0);
  const inputSequence = useRef(0);
  const isReady = useRef(false);
  const renderEnabled = useRef(false);
  const controlsEnabled = useRef(false);
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
  const [bookOpen, setBookOpen] = useState(false);
  const [layers, setLayers] = useState<Hover>(null);
  const [abilityPlant, setAbilityPlant] = useState<number | null>(null);
  const [loadingUnits, setLoadingUnits] = useState(false);
  useSceneMusic(props, view, art, ready);
  const player = view.players?.[props.room.you];
  const watching = !!props.room.spectating || props.room.you < 0;
  const side = player?.side ?? "plants";
  const cards = (player?.deck ?? []).map(kind => view.catalog?.[side]?.find(card => card.kind === kind)).filter((card): card is Spec => !!card);
  const map = mapFor(view), stage = stageFor(view, art);
  const sceneID = props.data.map?.id ?? map.id;
  const unitKey = neededUnits(view).sort().join("|");
  const automated = !!props.room.seats[props.room.you]?.autoPlay;
  const usable = ready && !loadingUnits && !watching && !automated && !!view.canAct && ["prepare", "battle"].includes(view.phase) && !(side === "zombies" && view.phase === "prepare");
  controlsEnabled.current = usable && !bookOpen && !layers && !loadError;
  const displayNow = props.clock.serverNow + Math.max(0, now - props.clock.receivedAt);
  const remaining = Math.max(0, (view.phaseUntil - displayNow) / 1000);

  const choose = (kind: string | null, useShovel = false) => {
    paintState.current.selected = kind; paintState.current.shovel = useShovel;
    setAbilityPlant(null); setLayers(null);
    setSelected(kind); setShovel(useShovel); setError("");
  };
  useControllerCancel(controllerRoot, () => { setLayers(null); choose(null); });

  useEffect(() => {
    const controller = new AbortController();
    let retired = false;
    setReady(false); isReady.current = false; renderEnabled.current = false; setArt(null); setLoadError("");
    let downloaded:DownloadProgress|undefined;
    const report = (value: number, label: string) => {
      if (retired) return;
      setProgress(value); latest.current.onLoadProgress?.(value, label, downloaded);
    };
    const downloads=new ResourceDownloads(latest.current.loadSizes??{},p=>{downloaded=p;report(p.percent,"加载本局场景与规则程序")});
    Promise.all([
      loadArt(props.assets, controller.signal, report, sceneID, neededUnits(authoritative.current),downloads),
      createPredictor(props.assets,downloads),
    ]).then(([loadedArt, predictor]) => {
      if (retired) { predictor.dispose(); return; }
      runtime.current = predictor;
      if (complete(authoritative.current)) predictor.update(authoritative.current);
      predictor.update(latest.current.data);
      const initial = predictor.sample(performance.now());
      if (complete(initial)) { frame.current = initial; setView(initial); }
      setArt(loadedArt);
      renderEnabled.current = true;
      report(95, "初始化草坪画面");
    }).catch(reason => {
      if (retired || reason?.name === "AbortError") return;
      const message = reason instanceof Error ? reason.message : "游戏资源加载失败";
      setLoadError(message); latest.current.onLoadError?.(message);
    });
    return () => {
      retired = true; renderEnabled.current = false; controller.abort(); runtime.current?.dispose(); runtime.current = null;
    };
  }, [props.assets, attempt, sceneID]);

  useEffect(() => {
    if (!art || !complete(view)) return;
    let retired = false;
    const kinds = unitKey.split("|");
    const missing = kinds.some(kind => !unitReady(art, kind));
    if (!missing) { setLoadingUnits(false); return; }
    const visible=[...(view.plants??[]),...(view.zombies??[])].some(entity=>!unitReady(art,entity.kind));
    setLoadingUnits(visible);
    void ensureUnits(art, kinds).then(() => { if (!retired) setLoadingUnits(false); }).catch(reason => {
      if (retired) return;
      setLoadError(reason instanceof Error ? reason.message : "角色素材加载失败");
    });
    return () => { retired = true; };
  }, [art, unitKey]);

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
      setBookOpen(false);
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
      if (element && context && complete(scene) && renderEnabled.current) {
        const rect = element.getBoundingClientRect();
        const ratio = Math.min(2, devicePixelRatio || 1);
        const w = Math.max(1, Math.round(rect.width * ratio)), h = Math.max(1, Math.round(rect.height * ratio));
        if (element.width !== w || element.height !== h) { element.width = w; element.height = h; }
        context.setTransform(w / STAGE.width, 0, 0, h / STAGE.height, 0, 0);
        const p = latest.current;
        const canAct = controlsEnabled.current && !p.room.spectating && p.room.you >= 0 && !p.room.seats[p.room.you]?.autoPlay && scene.canAct && !p.clock.paused;
        paint(context, scene, art, { ...paintState.current, actor: p.room.you, canAct, now: time, reducedMotion: media.matches });
        Object.assign(element, { __pvzFrame: { tick: scene.tick, epoch: scene.epoch, phase: scene.phase, scene: mapFor(scene).id, rows: mapFor(scene).rows, wireMode: scene.wireMode, renderAt: time, pixelAt: performance.now(), plants: scene.plants.length, zombies: scene.zombies.length, projectiles: scene.projectiles.length, poses: art.drawnPoses.slice(), selected: paintState.current.selected, inputEnabled: canAct, player: p.room.you, lastInputAt: inputAt.current, lastActionSeq: inputSequence.current } });
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
    if (!art) return;
    const dispatch = new PvzEffects(props.audio, art.base, art.manifest.audio ?? {});
    effects.current = dispatch;
    const changed = () => dispatch.silence(frame.current.epoch, frame.current.events ?? []);
    document.addEventListener("visibilitychange", changed);
    return () => { document.removeEventListener("visibilitychange", changed); dispatch.dispose(); effects.current = null; };
  }, [art, props.audio]);
  useEffect(() => {
    if (!art || !ready || !complete(view)) return;
    effects.current?.consume(view.epoch, view.events ?? [], props.sound, !document.hidden);
  }, [view, props.sound, art, ready]);

  useEffect(() => {
    const clear = () => { choose(null); paintState.current.hover = null; };
    window.addEventListener("blur", clear);
    document.addEventListener("visibilitychange", clear);
    return () => { window.removeEventListener("blur", clear); document.removeEventListener("visibilitychange", clear); };
  }, []);

  const submit = async (intent: Omit<PvzAction, "epoch" | "seq">) => {
    const p = latest.current, current = frame.current;
    const own = current.players?.[p.room.you];
    if (!isReady.current || p.room.spectating || p.room.you < 0 || !own || p.room.seats[p.room.you]?.autoPlay || !current.canAct || p.clock.paused) throw new Error("当前不能操作");
    if (inFlight.current.size >= 8) throw new Error("正在确认操作，请稍候");
    const action: PvzAction = { ...intent, epoch: current.epoch, seq: ++nextSeq.current };
    inFlight.current.add(action.seq); inputAt.current = performance.now(); inputSequence.current = action.seq;
    const rejectUnsent = (reason: unknown) => {
      inFlight.current.delete(action.seq);
      const latestView = runtime.current?.sample(performance.now());
      const checkpoint = latestView?.snapshot as { players?: { seq: number }[] } | undefined;
      const confirmed = checkpoint?.players?.[p.room.you]?.seq ?? authoritative.current.players?.[p.room.you]?.seq ?? 0;
      nextSeq.current = Math.max(confirmed, latestView?.players?.[p.room.you]?.seq ?? own.seq, ...inFlight.current);
      const message = reason instanceof Error ? reason.message : "当前位置或阶段不允许此操作";
      setError(message); return new Error(message);
    };
    let accepted = false;
    try { accepted = runtime.current?.apply(action, p.room.you) ?? false; }
    catch (reason) { throw rejectUnsent(reason); }
    if (!accepted) throw rejectUnsent(new Error(runtime.current?.lastError || "当前位置或阶段不允许此操作"));
    try {
      const predicted = runtime.current?.sample(performance.now());
      if (predicted && complete(predicted)) { frame.current = predicted; setView(predicted); }
      setError("");
      await p.act({ ...action });
    } catch (reason) {
      runtime.current?.reject(action.seq);
      const corrected = runtime.current?.sample(performance.now());
      if (corrected && complete(corrected)) { frame.current = corrected; setView(corrected); }
      setError(reason instanceof Error ? reason.message : "操作未成功，请重试");
      throw reason;
    } finally { inFlight.current.delete(action.seq); }
  };

  const actAt = (row: number, col: number) => {
    const p = latest.current, current = frame.current, own = current.players?.[p.room.you];
    if (!own || !usable || bookOpen) return;
    const stack = plantsAt(current, row, col);
    if (abilityPlant !== null) {
      void submit({ type: "ability", plantId: abilityPlant, row, col }).then(() => choose(null)).catch(() => {});
      return;
    }
    if (paintState.current.shovel) {
      if (stack.length > 1) { setLayers({ row, col }); return; }
      const plant = stack[0];
      if (!plant) return;
      if (plant.owner !== p.room.you) { setError("只能铲除自己的植物"); return; }
      void submit({ type: "shovel", plantId: plant.id }).catch(() => {});
    } else {
      const kind = paintState.current.selected;
      if (!kind) { if (stack.length && own.side === "plants") setLayers({ row, col }); return; }
      const spec = current.catalog[own.side].find(card => card.kind === (kind === "imitater" ? own.imitaterKind : kind));
      if (!spec) return;
      const staged=current.catalog[own.side].find(card=>card.kind===kind);
      if((staged?.unlockAt??0)>current.tick){setError(`开战后 ${staged!.unlockAfterSeconds} 秒解锁`);return;}
      if(art&&!unitReady(art,kind)){setError("角色素材加载中，请稍候");return;}
      const invalid = terrainReason(spec, mapFor(current));
      if (invalid) { setError(invalid); return; }
      if (own.money < spec.cost) { setError(own.side === "plants" ? "阳光不足" : "脑力不足"); return; }
      if ((own.cooldowns[kind] ?? 0) > current.tick) { setError("卡牌尚未冷却"); return; }
      void submit({ type: own.side === "plants" ? "plant" : "spawn", kind, row, ...(own.side === "plants" || spec.target === "cell" || spec.target === "plant" ? { col } : {}), ...(kind === "imitater" ? { imitateKind: own.imitaterKind } : {}) }).catch(() => {});
    }
  };

  function renderCard(card: Spec) {
    const effective = card.kind === "imitater" ? view.catalog.plants.find(spec => spec.kind === player?.imitaterKind) ?? card : card;
    const locked=(card.unlockAt??0)>view.tick;
    const cooldown = Math.max(0, ((player?.cooldowns[card.kind] ?? 0) - view.tick) * 50);
    const unaffordable = (player?.money ?? 0) < effective.cost;
    const chosen = selected === card.kind && !shovel;
    const total = Math.max(1, effective.cooldown ?? ((effective.cooldownTicks ?? 20) * 50));
    const invalid = terrainReason(effective, map);
    return <button type="button" key={card.kind} className={`pvz-seed${card.kind === "imitater" ? " pvz-imitater-card" : ""}${chosen ? " pvz-selected" : ""}${unaffordable ? " pvz-unaffordable" : ""}`} data-pvz-card={card.kind} aria-pressed={chosen} aria-label={`${card.name}，${effective.cost}${side === "plants" ? "阳光" : "脑力"}${cooldown ? `，冷却 ${Math.ceil(cooldown / 1000)} 秒` : ""}${invalid ? `，${invalid}` : ""}`} title={invalid || (card.kind === "imitater" ? `模仿${effective.name}` : card.description)} disabled={!usable || !!invalid} onClick={() => choose(chosen ? null : card.kind)}>
      <span className="pvz-seed-name">{card.name}</span>
      {art && <CardArt art={art} kind={card.kind === "imitater" ? player?.imitaterKind || card.kind : card.kind} />}
      <span className="pvz-seed-cost">{effective.cost}</span>
      {cooldown > 0 && <span className="pvz-seed-cooldown" aria-hidden="true" style={{ height: `${Math.min(100, cooldown / total * 100)}%` }}><b>{locked?`解锁 ${formatClock(cooldown/1000)}`:Math.ceil(cooldown/1000)}</b></span>}
    </button>;
  }

  return <div ref={controllerRoot} className={`pvz-board pvz-side-${side}`} data-pvz-board data-ready={ready} data-phase={view.phase} data-scene={map.id}>
    <div className="pvz-hud">
      <div className="pvz-wallet"><span className={`pvz-resource-icon pvz-resource-${side}`} aria-hidden="true" /><strong>{player?.money ?? "—"}</strong><small>{watching ? "观战" : side === "plants" ? "阳光" : "脑力"}</small></div>
      <div className="pvz-round-clock"><span>第 {view.round ?? 1} / 2 回合</span><strong data-pvz-clock>{view.phase === "prepare" ? `布阵 ${formatClock(remaining)}` : view.phase === "intermission" ? `交换攻守 ${formatClock(remaining)}` : view.phase === "finished" ? "比赛结束" : formatClock(remaining)}</strong></div>
      <div className="pvz-role"><strong>{watching ? "观战" : side === "plants" ? "植物防守" : "僵尸进攻"}</strong>{automated && <span>托管中</span>}<button type="button" aria-label="卡册" disabled={!ready || !art} onClick={() => setBookOpen(true)}>卡册</button></div>
    </div>
    <div className={`pvz-seed-rack${cards.length > 8 ? " pvz-deck-many" : ""}`} data-game-controls data-controller-group="cards">
      {cards.map(renderCard)}
      {side === "plants" && <button type="button" className={`pvz-shovel${shovel ? " pvz-selected" : ""}`} aria-label="铲子" aria-pressed={shovel} disabled={!usable} onClick={() => choose(null, !shovel)}>
        {art?.manifest.images?.shovel ? <img src={assetURL(art.base, art.manifest.images.shovel)} alt="" /> : <svg viewBox="0 0 50 70" aria-hidden="true"><path d="M22 6h8v35h-8z" fill="#9b6b3c"/><path d="M14 37h24l-4 23-8 8-9-8z" fill="#adb6b3" stroke="#45524b" strokeWidth="3"/><path d="M17 5h19v9H17z" fill="none" stroke="#704e31" strokeWidth="5"/></svg>}<span>铲子</span>
      </button>}
    </div>
    <div className="pvz-scene" data-game-controls>
      <canvas ref={canvas} data-pvz-canvas data-controller-ignore width={800} height={600} tabIndex={-1} role="img" aria-label={side === "plants" ? "草坪，先选择植物卡，再选择格子种植。" : "草坪，先选择僵尸卡，再选择要进攻的行。"}
        onPointerMove={event => { paintState.current.hover = hitStage(event.clientX, event.clientY, event.currentTarget.getBoundingClientRect(), frame.current, art); }}
        onPointerLeave={() => { paintState.current.hover = null; }}
        onPointerDown={event => { if (event.button !== 0) return; event.preventDefault(); event.currentTarget.focus({ preventScroll: true }); const hit = hitStage(event.clientX, event.clientY, event.currentTarget.getBoundingClientRect(), frame.current, art); if (hit) { paintState.current.hover = hit; actAt(hit.row, hit.col); } }}
      />
      {(!ready || loadingUnits || loadError) && <div className="pvz-loading" role={loadError ? "alert" : "status"}>
        {loadError ? <><p>{loadError}</p><button type="button" onClick={() => setAttempt(value => value + 1)}>重新加载</button></> : <><strong>草坪准备中</strong><progress value={progress} max={100} aria-label="植物大战僵尸加载进度" /></>}
      </div>}
      {ready && view.phase === "intermission" && <div className="pvz-phase-banner"><strong>交换攻守</strong><span>{formatClock(remaining)}</span></div>}
      {ready && !watching && side === "zombies" && view.phase === "prepare" && <div className="pvz-phase-banner pvz-phase-small"><strong>植物正在布阵</strong><span>{formatClock(remaining)}</span></div>}
      <div ref={grid} className="pvz-grid-controls" role="grid" aria-label="草坪格子" data-controller-group="board">{Array.from({ length: map.rows }, (_, row) => side === "plants" || ["cell", "plant"].includes(view.catalog?.zombies.find(card => card.kind === selected)?.target ?? "") ? Array.from({ length: map.cols }, (_, col) => <button key={`${row}-${col}`} type="button" data-pvz-cell={`${row}:${col}`} aria-label={`第 ${row + 1} 行第 ${col + 1} 列`} style={{ left: `${(stage.left + col * stage.cellWidth) / stage.width * 100}%`, top: `${(stage.top + row * stage.cellHeight + (stage.roofSlope[col] ?? 0)) / stage.height * 100}%`, width: `${stage.cellWidth / stage.width * 100}%`, height: `${stage.cellHeight / stage.height * 100}%` }} disabled={!usable} onFocus={() => { paintState.current.hover = { row, col }; }} onClick={() => actAt(row, col)} />) : <button key={row} type="button" aria-label={`向第 ${row + 1} 行出兵`} style={{ left: "95.3%", top: `${(stage.top + row * stage.cellHeight) / stage.height * 100}%`, width: "4.7%", height: `${stage.cellHeight / stage.height * 100}%` }} disabled={!usable} onFocus={() => { paintState.current.hover = { row, col: 8 }; }} onClick={() => actAt(row, 8)} />)}<div className="pvz-pickups" data-controller-group="pickups">{view.pickups?.map(pickup => <button type="button" key={pickup.id} data-pvz-pickup={pickup.id} data-owner={pickup.owner} aria-label={`收取${pickup.amount}${pickup.kind === "sun" ? "阳光" : "金币"}`} disabled={!ready || watching || automated || view.players[pickup.owner]?.team !== player?.team} style={{ left: `${(stage.left + pickup.x / 1000 * stage.cellWidth) / stage.width * 100}%`, top: `${(stage.top + pickup.y / 1000 * stage.cellHeight + slopeAt(pickup.x, stage.roofSlope)) / stage.height * 100}%`, width: `${36 / stage.width * 100}%`, height: `${36 / stage.height * 100}%` }} onClick={() => { void submit({ type: "collect", pickupId: pickup.id }).catch(() => {}); }} />)}</div></div>
    </div>
    <div className="pvz-below"><span className="pvz-action-error" role="status" aria-live="polite">{error || (abilityPlant !== null ? "选择炮击目标" : "")}</span><span className="pvz-round-history">{view.rounds?.map((round, i) => { const row=roundRow(round,i);return <span key={i}>{row.attack}进攻 · {row.result} {row.time}</span>;})}</span></div>
    {bookOpen && art && <CardBook view={view} art={art} actor={watching ? -1 : props.room.you} canEdit={!watching && !automated} onClose={() => setBookOpen(false)} onSave={(kinds, imitateKind) => submit({ type: "deck", kinds, imitateKind })} />}
    {layers && createPortal(<Modal title="植物管理" className="pvz-layer-modal" onClose={() => setLayers(null)}><div data-controller-group="plant-layers" className="pvz-layer-list">{plantsAt(view, layers.row, layers.col).map(plant => <div key={plant.id}><strong>{view.catalog.plants.find(spec => spec.kind === plant.kind)?.name ?? plant.kind}<small>{layerName(plant)} · {props.room.seats[plant.owner]?.name}</small></strong><span>{plant.hp} / {plant.maxHP}{plant.asleep ? " · 休眠" : ""}</span><button type="button" disabled={!usable || plant.owner !== props.room.you} onClick={() => { void submit({ type: "shovel", plantId: plant.id }).then(() => setLayers(null)).catch(() => {}); }}>铲除</button>{plant.kind === "cobcannon" && <button type="button" disabled={!usable || plant.owner !== props.room.you || plant.asleep || plant.readyAt > view.tick || (plant.chargeMS ?? 0) > view.tick * 50} onClick={() => { choose(null); setAbilityPlant(plant.id); paintState.current.selected = "__ability"; setLayers(null); }}>{Math.max(plant.readyAt * 50, plant.chargeMS ?? 0) > view.tick * 50 ? `装填 ${Math.ceil((Math.max(plant.readyAt * 50, plant.chargeMS ?? 0) - view.tick * 50) / 1000)} 秒` : "瞄准"}</button>}</div>)}</div></Modal>, overlayRoot)}
  </div>;
}
