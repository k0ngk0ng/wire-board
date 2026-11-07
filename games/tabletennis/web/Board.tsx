import {
  PhysicalController,
  mergeLogicalButtons,
} from "../../../web/src/controllers/gamepad";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { api } from "../../../web/src/ui/board-host";
import {
  KeyboardController,
  type ControllerSettingsData,
  type ControllerProfile,
} from "../../../web/src/controllers/input";
import { StrokeChannel } from "./input";
import { PaddleMotion } from "./motion";
import {
  BallDisplay,
  contactPoint,
  canReach,
  returnWindowOpen,
  sweptPointerContact,
} from "./prediction";
import {
  paddleAtScreen,
  paddleReachRadius,
  pointerOverBall,
} from "./view-space";
import { PaddleStream } from "./paddle-stream";
import { localPoint } from "./timeline";
import { EventTimeline, gameNow } from "./timeline";
import type { View, Stroke, Vec } from "./types";
import type { CourtRenderer } from "./renderer";
import "./style.css";

export function Board(props: BoardProps<View>) {
  const ballDisplay = useRef(new BallDisplay());
  const currentIdentity = useRef("");
  const focusedIdentity = useRef("");
  const racketScreenPoint = useRef<{ x: number; y: number } | null>(null);
  const lastPointerSample = useRef<{
    point: { x: number; y: number };
    at: number;
    key: string;
  } | null>(null);
  const movement = useRef<PaddleStream | null>(null);
  const motion = useRef(new PaddleMotion());
  const latest = useRef(props);
  latest.current = props;
  const canvas = useRef<HTMLCanvasElement>(null),
    engine = useRef<CourtRenderer | null>(null),
    channel = useRef<StrokeChannel | null>(null),
    keyboard = useRef<KeyboardController | null>(null),
    gamepad = useRef<PhysicalController | null>(null),
    events = useRef(new EventTimeline());
  const gesture = useRef({ x: 0, y: 0, down: false }),
    pointer = useRef<{ id: number; touch: boolean } | null>(null),
    swing = useRef<
      | { at: number; stroke: Stroke; contact?: Vec; connected?: boolean }
      | undefined
    >(undefined),
    held = useRef(new Set<string>());
  const [error, setError] = useState(""),
    [ready, setReady] = useState(false),
    [now, setNow] = useState(0);
  const { room, data: d, clock } = props;
  const watching = !!room.spectating || room.you < 0;
  const seat = watching ? 0 : room.you;
  const team = d.options.teams[seat] ?? 0;
  const identity = `${room.id}:${d.epoch}:${room.you}:${room.seats[room.you]?.id ?? "watch"}:${clock.epoch ?? 0}:${!!room.seats[room.you]?.autoPlay}`;
  currentIdentity.current = identity;
  const ballKey = `${identity}:${d.rally}:${d.shot}`;
  useEffect(() => {
    if (
      !ready ||
      props.busy ||
      watching ||
      room.seats[room.you]?.autoPlay ||
      focusedIdentity.current === identity
    )
      return;
    focusedIdentity.current = identity;
    if (document.activeElement === document.body)
      canvas.current?.focus({ preventScroll: true });
  }, [ready, props.busy, watching, identity]);
  const canControl = () => {
    const p = latest.current;
    return (
      !p.busy &&
      !p.clock.paused &&
      gameNow(p.clock, performance.now()) - p.data.serverAt <= 750 &&
      !p.room.spectating &&
      p.room.you >= 0 &&
      !p.room.seats[p.room.you]?.autoPlay &&
      !p.room.seats[p.room.you]?.left &&
      p.data.phase !== "finished"
    );
  };
  if (!channel.current)
    channel.current = new StrokeChannel({
      send: (action) => latest.current.act({ ...action }),
      onError: (message) => {
        ballDisplay.current.reset();
        setError(message);
      },
    });
  if (!movement.current)
    movement.current = new PaddleStream((action) => latest.current.act(action));
  const canTrack = () =>
    ready &&
    !watching &&
    !latest.current.room.seats[latest.current.room.you]?.autoPlay &&
    latest.current.data.phase !== "finished";
  function clearPointer() {
    pointer.current = null;
    lastPointerSample.current = null;
    motion.current.clear();
    movement.current?.clear();
    gesture.current = { x: 0, y: 0, down: false };
  }
  function cancel(emit = false) {
    clearPointer();
    keyboard.current?.clear();
    gamepad.current?.clear();
    held.current.clear();
    channel.current?.cancel(emit);
    ballDisplay.current.reset();
  }
  useLayoutEffect(() => {
    motion.current.clear();
    lastPointerSample.current = null;
    movement.current?.clear();
  }, [clock.paused]);
  useLayoutEffect(() => {
    movement.current!.update(
      `${identity}:${d.ends}`,
      d.epoch,
      d.ends,
      d.paddles[room.you]?.motionSeq ?? 0,
    );
    channel.current!.update({
      identity,
      epoch: d.epoch,
      rally: d.rally,
      shot: d.shot,
      seq: d.controls.seq,
      enabled: canControl() && d.controls.canSwing,
      serverAt: d.serverAt,
      retrySeq: d.controls.retrySeq,
    });
    if (d.controls.retrySeq && d.controls.retrySeq === d.controls.seq)
      ballDisplay.current.reset();
  }, [props, identity, ballKey]);
  function placedStroke(
    stroke: Stroke,
    at: number,
    screenPoint = racketScreenPoint.current,
  ): Stroke {
    const p = latest.current;
    const contact = contactPoint(p.data, p.room.you >= 0 ? p.room.you : 0, at);
    const rect = canvas.current?.getBoundingClientRect();
    const side = (p.data.options.teams[p.room.you] ?? 0) ^ p.data.ends;
    const own = p.data.paddles[p.room.you];
    const position =
      screenPoint && rect
        ? paddleAtScreen(
            screenPoint,
            Math.max(
              0,
              Math.min(p.data.reachDepth ?? 1.7, localPoint(contact, side).z),
            ),
            rect.width,
            rect.height,
          )
        : { x: (own?.x ?? 0) * 0.9, y: own?.y ?? 0.9, z: own?.z ?? 1.7 };
    return {
      ...stroke,
      racketX: position.x / 0.9,
      racketY: position.y,
      racketZ: position.z,
      racketRadius:
        screenPoint && rect
          ? paddleReachRadius(screenPoint, position.z, rect.width, rect.height)
          : 0.22,
    };
  }
  function strike(
    stroke: Stroke,
    hit?: { point: { x: number; y: number }; at: number },
  ) {
    const p = latest.current;
    const at = hit?.at ?? gameNow(p.clock, performance.now());
    stroke = placedStroke(stroke, at, hit?.point ?? racketScreenPoint.current);
    if (!canControl() || !p.data.controls.canSwing) return;
    if (at < p.data.controls.openAt) {
      setError("等球靠近再挥拍");
      return;
    }
    if (p.data.phase === "rally" && at > p.data.controls.closeAt) return;
    setError("");
    if (channel.current?.swing(stroke, at)) {
      movement.current?.clear();
      swing.current = {
        at: performance.now(),
        stroke,
        contact: contactPoint(p.data, p.room.you, at),
        connected: canReach(p.data, p.room.you, stroke, at),
      };
      ballDisplay.current.start(
        p.data,
        p.room.you,
        stroke,
        at,
        currentIdentity.current,
      );
    }
  }
  function capturePointer(
    point: { x: number; y: number },
    x: number,
    y: number,
    time: number,
    down = false,
  ) {
    const p = latest.current,
      rect = canvas.current?.getBoundingClientRect(),
      at = gameNow(p.clock, time);
    racketScreenPoint.current = point;
    motion.current.move(x, y, time);
    if (!canControl() || !rect) {
      lastPointerSample.current = null;
      return;
    }
    const key = `${currentIdentity.current}:${p.data.rally}:${p.data.shot}`,
      previous = lastPointerSample.current;
    lastPointerSample.current = { point, at, key };
    const live = motion.current.sample(time, Math.min(rect.width, rect.height));
    if (previous?.key === key && live.moving && p.data.controls.canSwing) {
      const hit = sweptPointerContact(
        p.data,
        p.room.you,
        previous,
        { point, at },
        rect.width,
        rect.height,
      );
      if (hit) strike(live.stroke, hit);
    }
    if (
      down &&
      pointer.current?.touch &&
      p.data.controls.canSwing &&
      returnWindowOpen(p.data, at, {
        moving: false,
        settled: false,
        touching: true,
      }) &&
      visibleContact(at)
    )
      strike({ kind: "block", aim: 0, power: 0.35 }, { point, at });
  }
  function visibleContact(at: number): boolean {
    const p = latest.current,
      rect = canvas.current?.getBoundingClientRect();
    if (!racketScreenPoint.current || !rect) return false;
    const side = (p.data.options.teams[p.room.you] ?? 0) ^ p.data.ends;
    return pointerOverBall(
      racketScreenPoint.current,
      localPoint(contactPoint(p.data, p.room.you, at), side),
      rect.width,
      rect.height,
    );
  }
  useEffect(() => {
    let alive = true;
    keyboard.current = null;
    gamepad.current?.dispose();
    gamepad.current = null;
    let revision = -1;
    const applyProfile = (profile: ControllerProfile) => {
      if (!alive || profile.revision < revision) return;
      revision = profile.revision;
      cancel();
      keyboard.current = new KeyboardController(profile);
      if (gamepad.current) gamepad.current.configure(profile);
      else gamepad.current = new PhysicalController(profile);
    };
    const user = room.seats[room.you]?.id;
    const change = (event: Event) => {
      const detail = (
        event as CustomEvent<{
          userId: string;
          model: string;
          profile: ControllerProfile;
        }>
      ).detail;
      if (detail?.userId === user && detail.model === "game:tabletennis") {
        applyProfile(detail.profile);
      }
    };
    window.addEventListener("controller-settings-changed", change);
    if (!watching)
      void api("/controllers?game=tabletennis")
        .then((settings: ControllerSettingsData) => {
          if (alive && settings.profiles["game:tabletennis"])
            applyProfile(settings.profiles["game:tabletennis"]);
        })
        .catch(() => {
          if (alive) setError("控制器设置暂时无法读取，可继续使用鼠标或触屏");
        });
    return () => {
      alive = false;
      window.removeEventListener("controller-settings-changed", change);
      gamepad.current?.dispose();
      gamepad.current = null;
      keyboard.current?.clear();
    };
  }, [room.seats[room.you]?.id, watching]);
  useEffect(() => {
    const p = latest.current;
    events.current.reset(p.data.events, gameNow(p.clock, performance.now()));
    swing.current = undefined;
    racketScreenPoint.current = null;
    lastPointerSample.current = null;
    motion.current.clear();
    ballDisplay.current.reset();
    keyboard.current?.clear();
    gamepad.current?.clear();
    pointer.current = null;
    gesture.current = { x: 0, y: 0, down: false };
    held.current.clear();
  }, [identity]);
  useEffect(() => {
    const c = canvas.current;
    if (!c) return;
    let live = true,
      raf = 0,
      local: CourtRenderer | undefined,
      first = true,
      lastUI = 0,
      stale = false;
    setReady(false);
    props.onLoadProgress?.(15, "准备球馆");
    const reduced = matchMedia("(prefers-reduced-motion: reduce)");
    const keyRoot = c.closest<HTMLElement>("[data-fullscreen-board]") ?? c;
    const press = (event: KeyboardEvent) => {
      if (
        event.target instanceof Element &&
        event.target.closest(
          'input,textarea,select,button,[contenteditable="true"],[role="dialog"]',
        )
      )
        return;
      if (event.key === "Escape") {
        cancel(canControl());
        return;
      }
      if (!canControl()) return;
      clearPointer();
      keyboard.current?.press(event.code, performance.now(), event.repeat);
      if (keyboard.current?.sample(performance.now()).size)
        event.preventDefault();
    };
    const release = (event: KeyboardEvent) =>
      keyboard.current?.release(event.code);
    keyRoot.addEventListener("keydown", press);
    keyRoot.addEventListener("keyup", release);
    const focusOut = (event: FocusEvent) => {
      if (
        event.relatedTarget instanceof Node &&
        keyRoot.contains(event.relatedTarget)
      )
        return;
      clearPointer();
      keyboard.current?.clear();
      gamepad.current?.clear();
      held.current.clear();
    };
    keyRoot.addEventListener("focusout", focusOut);
    const observer = new ResizeObserver(() => local?.resize());
    observer.observe(c.parentElement!);
    const lost = (e: Event) => {
      e.preventDefault();
      cancel();
      latest.current.onLoadError?.("球场画面已中断，请重试加载");
    };
    c.addEventListener("webglcontextlost", lost);
    void import("./renderer")
      .then(({ createCourt }) => {
        if (!live) return;
        local = createCourt(
          c,
          latest.current.data.options.court,
          latest.current.room.seats.length,
        );
        engine.current = local;
        latest.current.onLoadProgress?.(75, "初始化球拍与球台");
        const frame = (time: number) => {
          if (!live || !local) return;
          const p = latest.current,
            at = gameNow(p.clock, time);
          if (!p.clock.paused && at - p.data.serverAt > 750) {
            if (!stale) cancel();
            stale = true;
          } else stale = false;
          const liveMotion = motion.current.sample(
            time,
            Math.min(c.clientWidth, c.clientHeight),
          );
          gesture.current = {
            x: Math.max(-1, Math.min(1, liveMotion.velocity.x)),
            y: Math.max(-1, Math.min(1, liveMotion.velocity.y)),
            down: liveMotion.active,
          };
          if (
            canControl() &&
            p.data.controls.canSwing &&
            liveMotion.active &&
            visibleContact(at)
          ) {
            const stroke = placedStroke(liveMotion.stroke, at);
            const readyToServe =
              p.data.phase === "serve" &&
              liveMotion.moving &&
              gameNow(p.clock, liveMotion.at) >= p.data.phaseUntil &&
              canReach(p.data, p.room.you, stroke, at);
            const readyToReturn =
              returnWindowOpen(p.data, at, {
                ...liveMotion,
                touching: pointer.current?.touch,
              }) && canReach(p.data, p.room.you, stroke, at);
            if (readyToServe || readyToReturn) strike(stroke);
          }
          // Reserve the HTTP fallback's single slot for reliable strokes.
          if (canControl() && liveMotion.active && (p.inputWindow ?? 1) > 1) {
            const placed = placedStroke(liveMotion.stroke, at);
            movement.current?.offer(
              {
                x: placed.racketX ?? 0,
                y: placed.racketY ?? 0.9,
                z: placed.racketZ ?? 1.7,
              },
              at,
            );
            movement.current?.flush(at);
          } else movement.current?.clear();
          const shownBall = ballDisplay.current.sample(
            p.data,
            at,
            currentIdentity.current,
          );
          local.render({
            ball: shownBall,
            predicted: ballDisplay.current.active,
            inputEnabled:
              canControl() &&
              p.data.controls.canSwing &&
              !!channel.current?.canSwing,
            racketScreenPoint: racketScreenPoint.current,
            data: p.data,
            seat: p.room.you,
            time: at,
            now: time,
            gesture: gesture.current,
            swing: swing.current,
            reduced: reduced.matches,
          });
          if (first) {
            first = false;
            setReady(true);
            latest.current.onLoadProgress?.(100, "球场已就绪");
            latest.current.onReady?.();
          }
          if (time - lastUI > 80) {
            lastUI = time;
            setNow(at);
          }
          if (p.sound && !p.clock.paused) {
            for (const e of events.current.take(p.data.events, at)) {
              const clip =
                e.type === "hit"
                  ? e.kind === "smash"
                    ? "smash"
                    : "hit"
                  : e.type === "bounce"
                    ? "bounce"
                    : e.type === "net"
                      ? "net"
                      : null;
              if (clip) {
                const path = "/tabletennis/v1/" + clip + ".mp3";
                void p.audio.playWithFallback(
                  p.assets
                    ? [p.assets.replace(/\/$/, "") + path, path]
                    : [path],
                );
              }
            }
          } else events.current.reset(p.data.events, at);
          const keyboardEnabled =
            canControl() &&
            (document.activeElement === c ||
              document.activeElement === keyRoot);
          const physical =
            gamepad.current?.sample(time, canControl()) ?? new Set<string>();
          if (physical.size && (pointer.current || lastPointerSample.current))
            clearPointer();
          if (canControl() && keyboard.current) {
            const keys = mergeLogicalButtons(
              keyboardEnabled ? keyboard.current.sample(time) : [],
              physical,
            );
            const axis =
              (keys.has("right") ? 1 : 0) - (keys.has("left") ? 1 : 0);
            if (axis)
              racketScreenPoint.current = {
                x: Math.max(
                  0,
                  Math.min(
                    1,
                    (racketScreenPoint.current?.x ?? 0.5) + axis * 0.0225,
                  ),
                ),
                y: racketScreenPoint.current?.y ?? 0.7,
              };
            const vertical =
              (keys.has("down") ? 1 : 0) - (keys.has("up") ? 1 : 0);
            if (vertical)
              racketScreenPoint.current = {
                x: racketScreenPoint.current?.x ?? 0.5,
                y: Math.max(
                  0,
                  Math.min(
                    1,
                    (racketScreenPoint.current?.y ?? 0.7) + vertical * 0.0225,
                  ),
                ),
              };
            for (const key of ["a", "b", "select", "start"])
              if (keys.has(key) && !held.current.has(key)) {
                strike({
                  kind: key === "a" ? "drive" : key === "b" ? "chop" : "block",
                  aim: keys.has("left") ? -0.7 : keys.has("right") ? 0.7 : 0,
                  power: keys.has("up") ? 0.95 : keys.has("down") ? 0.35 : 0.65,
                  lift: keys.has("up") ? 1 : key === "a" ? 0.7 : 0,
                });
              }
            held.current = keys;
          }
          raf = requestAnimationFrame(frame);
        };
        raf = requestAnimationFrame(frame);
      })
      .catch(() => {
        if (live)
          latest.current.onLoadError?.(
            "球馆无法初始化，请检查浏览器图形支持后重试",
          );
      });
    const blur = () => cancel();
    const visibility = () => {
      if (document.hidden) cancel();
      else {
        const current = latest.current;
        events.current.reset(
          current.data.events,
          gameNow(current.clock, performance.now()),
        );
        ballDisplay.current.reset();
      }
    };
    window.addEventListener("blur", blur);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      live = false;
      cancelAnimationFrame(raf);
      observer.disconnect();
      keyRoot.removeEventListener("keydown", press);
      keyRoot.removeEventListener("keyup", release);
      keyRoot.removeEventListener("focusout", focusOut);
      c.removeEventListener("webglcontextlost", lost);
      window.removeEventListener("blur", blur);
      document.removeEventListener("visibilitychange", visibility);
      cancel();
      local?.dispose();
      engine.current = null;
    };
  }, [d.options.court, d.epoch]);
  const countdown = Math.max(
    0,
    Math.ceil((d.phaseUntil - (now || d.serverAt)) / 1000),
  );
  const serveRemaining = Math.max(
    0,
    Math.ceil((d.serveDeadline - (now || d.serverAt)) / 1000),
  );
  const serveExpired =
    d.phase === "serve" && d.serveDeadline > 0 && serveRemaining === 0;
  const pointEvent = [...d.events].reverse().find((e) => e.type === "point");
  const pointReason = (
    {
      net: "触网",
      wrong_side: "落在己方台",
      wrong_service_half: "发球落点不正确",
      out: "出界",
      double_bounce: "二次落台",
      miss: "漏接",
      serve_timeout: "发球超时",
    } as Record<string, string>
  )[pointEvent?.kind ?? ""];
  const status =
    d.phase === "countdown"
      ? `${countdown} 秒后${d.events.at(-1)?.type === "let" ? "擦网重发" : "开始"}`
      : d.phase === "serve"
        ? serveExpired
          ? "等待判定"
          : d.expected === room.you
            ? `你发球 · ${d.score.every((n) => n >= 10) ? "1/1" : `${((d.score[0] + d.score[1]) % 2) + 1}/2`}`
            : `${room.seats[d.expected]?.name ?? "对方"}发球 · ${d.score.every((n) => n >= 10) ? "1/1" : `${((d.score[0] + d.score[1]) % 2) + 1}/2`}`
        : d.phase === "serve_toss"
          ? "抛球"
          : d.phase === "point_break"
            ? `${pointReason ? pointReason + " · " : ""}${d.pointWinner === team ? "本队" : "对方"}得分`
            : d.phase === "game_break"
              ? `下一局 ${countdown}`
              : d.phase === "finished"
                ? "比赛结束"
                : d.expected === room.you
                  ? "接球"
                  : `${room.seats[d.expected]?.name ?? "对方"}接球`;
  return (
    <section
      className="tt-board"
      data-tabletennis-board
      data-phase={d.phase}
      data-ready={ready}
      data-rally={d.rally}
      data-shot={d.shot}
      data-seq={d.controls.seq}
    >
      <div className="tt-score">
        <div>
          <strong>
            {room.seats
              .filter((_, i) => d.options.teams[i] === team)
              .map((p) => p.name)
              .join(" · ")}
          </strong>
          <small>已赢 {d.gamesWon[team]} 局</small>
        </div>
        <div className="tt-score-center">
          <small>
            第 {d.game} 局 · {room.seats.length === 4 ? "双打" : "单打"}
          </small>
          <b>
            {d.score[team]}
            <i>:</i>
            {d.score[1 - team]}
          </b>
        </div>
        <div className="tt-opponents">
          <strong>
            {room.seats
              .filter((_, i) => d.options.teams[i] !== team)
              .map((p) => p.name)
              .join(" · ")}
          </strong>
          <small>已赢 {d.gamesWon[1 - team]} 局</small>
        </div>
      </div>
      <div className="tt-scene">
        <canvas
          ref={canvas}
          data-pointer-tracking={canTrack()}
          tabIndex={0}
          role="application"
          aria-label="第一人称乒乓球，直接在画面划动挥拍"
          onPointerDown={(e) => {
            if (!canTrack() || pointer.current || e.button > 0) return;
            e.currentTarget.focus({ preventScroll: true });
            e.currentTarget.setPointerCapture(e.pointerId);
            const rect = e.currentTarget.getBoundingClientRect();
            const point = {
              x: (e.clientX - rect.left) / rect.width,
              y: (e.clientY - rect.top) / rect.height,
            };
            pointer.current = {
              id: e.pointerId,
              touch: e.pointerType !== "mouse",
            };
            capturePointer(
              point,
              e.clientX,
              e.clientY,
              e.timeStamp > 0 && e.timeStamp <= performance.now() + 50
                ? e.timeStamp
                : performance.now(),
              true,
            );
            gesture.current = { x: 0, y: 0, down: true };
            setError("");
          }}
          onPointerMove={(e) => {
            if (
              !canTrack() ||
              (pointer.current && pointer.current.id !== e.pointerId)
            )
              return;
            if (canTrack() && (pointer.current || e.pointerType === "mouse")) {
              const rect = e.currentTarget.getBoundingClientRect();
              const point = {
                x: (e.clientX - rect.left) / rect.width,
                y: (e.clientY - rect.top) / rect.height,
              };
              capturePointer(
                point,
                e.clientX,
                e.clientY,
                e.timeStamp > 0 && e.timeStamp <= performance.now() + 50
                  ? e.timeStamp
                  : performance.now(),
              );
            }
          }}
          onPointerUp={(e) => {
            const p = pointer.current;
            if (!p || p.id !== e.pointerId) return;
            pointer.current = null;
            if (e.pointerType !== "mouse") clearPointer();
          }}
          onPointerLeave={() => {
            if (!pointer.current) clearPointer();
          }}
          onPointerCancel={(e) => {
            if (!pointer.current || pointer.current.id === e.pointerId)
              clearPointer();
          }}
          onLostPointerCapture={() => {
            if (pointer.current) clearPointer();
          }}
        />
        <div className="tt-status" role="status">
          <span>
            {watching ? "观战 · " : ""}
            {status}
          </span>
          {d.phase === "serve" && d.serveDeadline > 0 && !serveExpired && (
            <b data-tabletennis-serve-clock>{serveRemaining}秒</b>
          )}
        </div>
        {error && (
          <div className="tt-error" role="alert">
            {error}
          </div>
        )}
      </div>
    </section>
  );
}
