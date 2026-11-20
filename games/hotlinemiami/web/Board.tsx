import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { useGameController } from "../../../web/src/controllers/use-game-controller";
import { loadBattleAssets, type LoadedAssets } from "./assets";
import { emptyInput, InputChannel } from "./input";
import { EventCursor, MotionPrediction } from "./prediction";
import { BattleRenderer } from "./renderer";
import { AttackPresentation } from "./attack";
import { ActionFeedback } from "./feedback";
import { aimAngle, preparePredictedInput } from "./camera-input";
import { TouchControls } from "./TouchControls";
import { isMeleeWeapon, mapNames, playerColors, weaponNames, type GameEvent, type GameInput, type Player, type View } from "./types";
import "./style.css";

type Metrics = {
  inputs: { at: number; type: string; trusted: boolean }[];
  responses: { inputAt: number; visibleAt: number; type: string; seq?: number }[];
  frames: number[];
};

type CanvasState = HTMLCanvasElement & {
  __hotlineFrame?: Record<string, unknown>;
  __hotlineMetrics?: Metrics;
};

function sameInput(a: GameInput, b: GameInput) {
  return a.moveX === b.moveX && a.moveY === b.moveY && a.aim === b.aim && a.fire === b.fire && !a.throw && !a.pickup && !a.finish && !a.interact;
}

export function Board(props: BoardProps<View>) {
  const latest = useRef(props);
  latest.current = props;
  const root = useRef<HTMLDivElement>(null);
  const canvas = useRef<CanvasState>(null);
  const loaded = useRef<LoadedAssets | undefined>(undefined);
  const renderer = useRef<BattleRenderer | undefined>(undefined);
  const prediction = useRef<MotionPrediction | undefined>(undefined);
  const attack = useRef<AttackPresentation | undefined>(undefined);
  const feedback = useRef(new ActionFeedback());
  const channel = useRef<InputChannel | undefined>(undefined);
  const timeline = useRef(new EventCursor());
  const previous = useRef<View | undefined>(undefined);
  const received = useRef({ view: props.data, at: performance.now() });
  const pointer = useRef({ down: false, aim: props.data.players[props.room.you]?.aim ?? 0, x: 0, y: 0, screenX: 0, screenY: 0, positioned: false });
  const touch = useRef({ moveX: 0, moveY: 0, aimX: 0, aimY: 0, fire: false });
  const oldActions = useRef(new Set<string>());
  const last = useRef({ input: emptyInput(), at: 0 });
  const flash = useRef(0);
  const metrics = useRef<Metrics>({ inputs: [], responses: [], frames: [] });
  const awaitingResponse = useRef<{ at: number; type: string } | undefined>(undefined);
  const lastDrawn = useRef<{ x: number; y: number; aim: number; flash: number; pulse: string } | undefined>(undefined);
  const process = useRef<(force?: boolean) => void>(() => {});
  const initialized = useRef(false);
  const drawCount = useRef(0);
  const wasEnabled = useRef(false);
  const lastCancel = useRef(0);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState("");
  const [uiNow, setUiNow] = useState(0);
  const [feed, setFeed] = useState<GameEvent[]>([]);
  const [attempt, setAttempt] = useState(0);
  const { room, data } = props;
  const seat = room.you;
  const player = data.players[seat];
  const identity = `${room.id}:${data.epoch}:${seat}:${room.seats[seat]?.id ?? "watch"}`;

  function canControl() {
    const current = latest.current;
    const currentPlayer = current.data.players[current.room.you];
    return !!loaded.current && !current.busy && !current.clock.paused && !document.hidden && !current.room.spectating && current.room.you >= 0 && !current.room.seats[current.room.you]?.autoPlay && !current.room.seats[current.room.you]?.left && !current.data.finished && current.data.canAct && !!currentPlayer?.alive && (current.inputWindow ?? 0) > 0;
  }

  const controller = useGameController({
    gameId: "hotlinemiami",
    userId: room.seats[seat]?.id ?? room.seats[0]?.id ?? "",
    scope: identity,
    root,
    enabled: canControl,
    changed: () => process.current(true),
    error: setError,
  });

  if (!channel.current) channel.current = new InputChannel(action => latest.current.act({ ...action }), setError);

  function recordNative(event: Event) {
    const at = performance.now();
    const item = { at, type: event.type, trusted: event.isTrusted };
    metrics.current.inputs.push(item);
    if (metrics.current.inputs.length > 512) metrics.current.inputs.shift();
    if (canControl()) awaitingResponse.current = item;
  }

  function cancel(send = true) {
    pointer.current.down = false;
    touch.current = { moveX: 0, moveY: 0, aimX: 0, aimY: 0, fire: false };
    controller.current?.clear();
    oldActions.current.clear();
    last.current.input = emptyInput();
    if (send) channel.current?.release();
    prediction.current?.clear(performance.now());
    attack.current?.clear();
    feedback.current.clear();
    awaitingResponse.current = undefined;
  }

  function issue(input: GameInput, now: number) {
    const action = channel.current?.push(input, prepared => {
      const assets = loaded.current;
      if (!prediction.current || !assets) return;
      const point = pointer.current.positioned ? { x: pointer.current.screenX, y: pointer.current.screenY } : undefined;
      preparePredictedInput(prediction.current, prepared, now, point, { width: assets.rules.width / 256, height: assets.rules.height / 256 });
      if (point) pointer.current.aim = prepared.aim;
    });
    if (!action) return;
    feedback.current.push(action, now);
    if (input.fire && attack.current?.fire(action.seq, prediction.current?.predictedFrame ?? latest.current.data.frame)) {
      flash.current = now + 65;
    }
    last.current.input = { ...input, aim: action.aim, throw: false, pickup: false, finish: false, interact: false };
    last.current.at = now;
  }

  process.current = (force = false) => {
    const now = performance.now();
    const current = latest.current;
    const currentPlayer = current.data.players[current.room.you];
    const enabled = canControl();
    prediction.current?.setEnabled(enabled, now);
    channel.current?.update({ identity: `${current.room.id}:${current.data.epoch}:${current.room.you}:${current.room.seats[current.room.you]?.id ?? "watch"}`, epoch: current.data.epoch, life: currentPlayer?.life ?? 0, receivedSeq: currentPlayer?.receivedSeq ?? 0, enabled, window: current.inputWindow ?? 0, frame: current.data.frame });
    if (wasEnabled.current && !enabled) cancel(false);
    wasEnabled.current = enabled;
    const actions = controller.current?.sample(now) ?? new Set<string>();
    if (controller.current && controller.current.cancelVersion !== lastCancel.current) {
      lastCancel.current = controller.current.cancelVersion;
      pointer.current.down = false;
      touch.current = { moveX: 0, moveY: 0, aimX: 0, aimY: 0, fire: false };
    }
    if (!canControl() || !currentPlayer) return;
    const values = controller.current?.values;
    const level = (name: string) => Math.round((values?.get(name) ?? (actions.has(name) ? 1 : 0)) * 1000);
    const moveX = Math.max(-1000, Math.min(1000, level("right") - level("left") + touch.current.moveX));
    const moveY = Math.max(-1000, Math.min(1000, level("down") - level("up") + touch.current.moveY));
    const aimX = level("aimRight") - level("aimLeft") + touch.current.aimX;
    const aimY = level("aimDown") - level("aimUp") + touch.current.aimY;
    if (aimX || aimY) {
      pointer.current.aim = aimAngle(aimX, aimY);
      pointer.current.positioned = false;
    } else if (pointer.current.positioned && renderer.current) {
      const local = prediction.current?.sample(now) ?? currentPlayer;
      const point = renderer.current.pointFromPointer(pointer.current.screenX, pointer.current.screenY, local);
      pointer.current.x = point.x;
      pointer.current.y = point.y;
      pointer.current.aim = aimAngle(Math.round(point.x * 256) - local.x, Math.round(point.y * 256) - local.y);
    }
    const input: GameInput = {
      moveX, moveY, aim: pointer.current.aim,
      fire: pointer.current.down || touch.current.fire || actions.has("fire"),
      throw: actions.has("throw") && !oldActions.current.has("throw"),
      pickup: actions.has("pickup") && !oldActions.current.has("pickup"),
      finish: actions.has("finish") && !oldActions.current.has("finish"),
      interact: actions.has("interact") && !oldActions.current.has("interact"),
    };
    oldActions.current = actions;
    if (!sameInput(input, last.current.input) || now - last.current.at >= (input.moveX || input.moveY || input.fire ? 33 : 100) || force && now - last.current.at >= 16) issue(input, now);
  };

  useLayoutEffect(() => {
    const now = performance.now();
    const prior = received.current.view;
    if (prior.frame < data.frame) previous.current = prior;
    if (prior.frame <= data.frame) received.current = { view: data, at: now };
    if (prediction.current && player) prediction.current.confirm(player, data.frame, now, identity, JSON.stringify(data.doors), JSON.stringify(data.brokenGlass ?? []));
    if (player) attack.current?.confirm(player);
    const fresh = timeline.current.consume(identity, data.events);
    renderer.current?.ingest(fresh);
    if (fresh.length) {
      const kills = fresh.filter(event => event.type === "kill" || event.type === "death");
      if (kills.length) setFeed(previousFeed => [...previousFeed, ...kills].slice(-3));
      if (props.sound) {
        for (const event of fresh) {
          const sounds = loaded.current?.sounds;
          const key = event.type === "fire" ? event.weapon ?? "pistol" : event.type;
          const url = sounds?.get(key);
          if (url) void props.audio.effect(`hotline:${event.id % 8}`, [url], { playbackID: `${data.epoch}:${event.id}` });
        }
      }
    }
  }, [data, identity]);

  useEffect(() => {
    const abort = new AbortController();
    let active = true;
    initialized.current = false;
    setReady(false);
    setError("");
    const current = latest.current;
    void loadBattleAssets(current.assets, current.data.mapId, current.data.players.map(candidate => candidate.character), abort.signal, (percent, label, bytes) => latest.current.onLoadProgress?.(percent, label, bytes), current.data.map).then(assets => {
      if (!active || !canvas.current) return;
      loaded.current = assets;
      renderer.current = new BattleRenderer(canvas.current, assets);
      prediction.current = new MotionPrediction(assets.motion);
      attack.current = new AttackPresentation(assets.weapons, assets.unarmed);
      const currentPlayer = latest.current.data.players[latest.current.room.you];
      if (currentPlayer) prediction.current.confirm(currentPlayer, latest.current.data.frame, performance.now(), identity, JSON.stringify(latest.current.data.doors));
      if (currentPlayer) attack.current.confirm(currentPlayer);
      setReady(true);
      canvas.current.focus({ preventScroll: true });
    }).catch(error => {
      if (!active || abort.signal.aborted) return;
      const message = error instanceof Error ? error.message : "战场初始化失败";
      setError(message);
      latest.current.onLoadError?.(message);
    });
    return () => {
      active = false;
      abort.abort();
      cancel(false);
      loaded.current = undefined;
      renderer.current = undefined;
      prediction.current = undefined;
      attack.current = undefined;
    };
  }, [data.mapId, identity, attempt]);

  useEffect(() => {
    const blur = () => cancel();
    const hidden = () => { if (document.hidden) cancel(); };
    const key = (event: KeyboardEvent) => {
      if (root.current?.contains(event.target as Node) || event.target === document.body) recordNative(event);
      if (event.code === "Escape") cancel();
    };
    window.addEventListener("blur", blur);
    window.addEventListener("keydown", key, true);
    window.addEventListener("keyup", key, true);
    document.addEventListener("visibilitychange", hidden);
    let frame = 0, active = true, lastUI = 0;
    const draw = (now: number) => {
      if (!active) return;
      process.current();
      const current = latest.current;
      const currentPlayer = current.data.players[current.room.you];
      const local = currentPlayer && prediction.current ? prediction.current.sample(now) : currentPlayer;
      if (local && canControl()) {
        if (pointer.current.positioned && renderer.current) {
          const point = renderer.current.pointFromPointer(pointer.current.screenX, pointer.current.screenY, local);
          pointer.current.x = point.x;
          pointer.current.y = point.y;
          pointer.current.aim = aimAngle(Math.round(point.x * 256) - local.x, Math.round(point.y * 256) - local.y);
        }
        local.aim = pointer.current.aim;
      }
      if (renderer.current && canvas.current) {
        const between = previous.current ? Math.max(1, current.data.frame - previous.current.frame) * 1000 / 60 : 33;
        const blend = Math.min(1, Math.max(0, (now - received.current.at) / between));
        const point = pointer.current.positioned ? { x: pointer.current.screenX, y: pointer.current.screenY } : undefined;
        const pulses = feedback.current.sample(now, local?.life ?? 0, !!local?.alive);
        const pulse = pulses.map(item => `${item.seq}:${item.kind}`).join(",");
        const screen = renderer.current.draw(current.data, local, previous.current, blend, current.room.you, current.room.seats.map(candidate => candidate.name), flash.current, now, point, pulses);
        metrics.current.frames.push(now);
        if (metrics.current.frames.length > 1200) metrics.current.frames.shift();
        const old = lastDrawn.current;
        const changed = local && old && (local.x !== old.x || local.y !== old.y || local.aim !== old.aim || (flash.current > now && flash.current !== old.flash) || !!pulse && pulse !== old.pulse);
        if (awaitingResponse.current && changed) {
          metrics.current.responses.push({ inputAt: awaitingResponse.current.at, visibleAt: performance.now(), type: awaitingResponse.current.type, seq: channel.current?.seq });
          if (metrics.current.responses.length > 512) metrics.current.responses.shift();
          awaitingResponse.current = undefined;
        }
        if (local) lastDrawn.current = { x: local.x, y: local.y, aim: local.aim, flash: flash.current, pulse };
        canvas.current.__hotlineFrame = {
          now, local: local ? { ...local, ...screen } : null,
          frame: current.data.frame, serverFrame: current.data.frame, predictedFrame: prediction.current?.predictedFrame ?? current.data.frame,
          receivedSeq: currentPlayer?.receivedSeq ?? 0, appliedSeq: currentPlayer?.appliedSeq ?? 0,
          attackFlashUntil: flash.current, inputEnabled: canControl(), inputWindow: current.inputWindow ?? 0, pending: prediction.current?.pendingCount ?? 0,
          drawCount: ++drawCount.current,
          feedback: pulses,
        };
        canvas.current.__hotlineMetrics = metrics.current;
        if (!initialized.current) {
          initialized.current = true;
          latest.current.onLoadProgress?.(100, "战场已就绪");
          latest.current.onReady?.();
        }
      }
      if (now - lastUI > 100) { setUiNow(now); lastUI = now; }
      frame = requestAnimationFrame(draw);
    };
    frame = requestAnimationFrame(draw);
    return () => {
      active = false;
      cancelAnimationFrame(frame);
      window.removeEventListener("blur", blur);
      window.removeEventListener("keydown", key, true);
      window.removeEventListener("keyup", key, true);
      document.removeEventListener("visibilitychange", hidden);
    };
  }, [identity]);

  useEffect(() => () => { channel.current?.dispose(); }, []);

  useEffect(() => {
    const url = loaded.current?.music;
    if (!ready || !props.sound || props.audioEnabled === false || !url) return;
    void props.audio.music([url], { loop: true, startOffset: latest.current.data.frame / 60, playbackID: `${latest.current.data.epoch}:music` });
    return () => props.audio.stopMusic();
  }, [ready, props.sound, props.audioEnabled, identity]);

  function pointerMove(event: ReactPointerEvent<HTMLCanvasElement>) {
    if (event.pointerType === "touch" || !renderer.current || !canvas.current) return;
    const rect = canvas.current.getBoundingClientRect();
    pointer.current.screenX = (event.clientX - rect.left) / rect.width;
    pointer.current.screenY = (event.clientY - rect.top) / rect.height;
    pointer.current.positioned = true;
    const local = prediction.current?.sample(performance.now()) ?? latest.current.data.players[latest.current.room.you];
    if (local) {
      const point = renderer.current.pointFromPointer(pointer.current.screenX, pointer.current.screenY, local);
      pointer.current.x = point.x;
      pointer.current.y = point.y;
      pointer.current.aim = aimAngle(Math.round(point.x * 256) - local.x, Math.round(point.y * 256) - local.y);
    }
    recordNative(event.nativeEvent);
    process.current(true);
  }

  function edge(action: "throw" | "pickup" | "finish" | "interact") {
    if (!canControl()) return;
    issue({ ...last.current.input, [action]: true }, performance.now());
  }

  const remainingFrames = Math.max(0, data.endFrame - data.frame);
  const seconds = Math.ceil(remainingFrames / 60);
  const timer = `${Math.trunc(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
  const enabled = ready && !props.busy && !room.spectating && !room.seats[seat]?.autoPlay && !data.finished && !!player?.alive && (props.inputWindow ?? 0) > 0;
  const respawn = player && !player.alive ? Math.max(1, Math.ceil((player.respawnFrame - data.frame) / 60)) : 0;
  return <div ref={root} className="hm-board" data-hotlinemiami-board data-ready={ready ? "true" : "false"}>
    <div className="hm-battle-heading">
      <div><span className="hm-map-label">{mapNames[data.mapId] ?? "混战"}</span><strong>{room.spectating ? "观战中" : player && !player.alive ? "等待复活" : "自由混战"}</strong></div>
      <time className={seconds <= 30 ? "hm-timer hm-timer-final" : "hm-timer"} aria-label={`剩余 ${timer}`}>{timer}</time>
      <div className="hm-ammo"><strong>{player ? weaponNames[player.weapon] ?? player.weapon : "自由视角"}</strong><span>{player ? isMeleeWeapon(player.weapon) ? "近战" : `${player.ammo} 发` : `${data.players.length} 人`}</span></div>
    </div>
    <div className="hm-score-strip" aria-label="实时比分">
      {data.players.map(candidate => <div key={candidate.seat} className={candidate.seat === seat ? "hm-score hm-score-self" : "hm-score"} style={{ "--hm-player-color": playerColors[candidate.seat % 4] } as React.CSSProperties}>
        <i aria-hidden="true" /><span>{room.seats[candidate.seat]?.name ?? "玩家"}</span><strong>{candidate.score}</strong>
      </div>)}
    </div>
    <div className="hm-surface">
      <canvas ref={canvas} data-hotlinemiami-canvas tabIndex={0} aria-label="迈阿密热线战场，WASD 移动，鼠标瞄准，左键攻击" onContextMenu={event => event.preventDefault()}
        onPointerMove={pointerMove}
        onPointerDown={event => {
          if (event.pointerType === "touch") return;
          canvas.current?.focus({ preventScroll: true });
          pointerMove(event);
          if (!canControl()) return;
          event.preventDefault();
          canvas.current?.setPointerCapture(event.pointerId);
          if (event.button === 2) edge("throw");
          else if (event.button === 0) { pointer.current.down = true; process.current(true); }
        }}
        onPointerUp={event => { if (event.button === 0) { recordNative(event.nativeEvent); pointer.current.down = false; process.current(true); } }}
        onPointerCancel={() => cancel()} onLostPointerCapture={() => { pointer.current.down = false; process.current(true); }} />
      {!ready && !error && <div className="hm-canvas-notice">正在布置战场</div>}
      {!!respawn && !data.finished && <div className="hm-death-notice"><strong>倒下了</strong><span>{respawn} 秒后重返赛场</span></div>}
      {ready && !enabled && !props.busy && !data.finished && player?.alive && !room.spectating && !room.seats[seat]?.autoPlay && <div className="hm-connection-notice">正在恢复连接</div>}
      {feed.length > 0 && <div className="hm-kill-feed" aria-live="polite">{feed.filter(event => data.frame - event.frame < 240).map(event => <span key={event.id}>{room.seats[event.seat]?.name ?? "玩家"}<b>击倒</b>{room.seats[event.target ?? -1]?.name ?? "对手"}</span>)}</div>}
    </div>
    <TouchControls disabled={!enabled} native={recordNative} move={stick => { touch.current.moveX = stick.x; touch.current.moveY = stick.y; process.current(true); }} aim={stick => { touch.current.aimX = stick.x; touch.current.aimY = stick.y; touch.current.fire = stick.x * stick.x + stick.y * stick.y >= 360000; process.current(true); }} edge={edge} />
    <div className="hm-bottom-line"><span>{player && player.protectedUntil > data.frame ? "出生保护 · 攻击后解除" : player && player.downUntil > data.frame ? "倒地中" : <><span className="hm-desktop-hint">WASD 移动 · 左键攻击 · 右键投掷 · E 拾取</span><span className="hm-touch-hint">右摇杆外推攻击 · 拾取与投掷换武器</span></>}</span><span>击杀 +1</span></div>
    {error && <div className="hm-error" role="alert">{error}{!ready && <button type="button" onClick={() => setAttempt(value => value + 1)}>重试</button>}</div>}
    <output className="hm-sr-only" aria-live="off">{uiNow ? "战场运行中" : "准备战场"}</output>
  </div>;
}
