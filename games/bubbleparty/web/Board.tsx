import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import {
  LatestInput,
  HeldDirections,
  NEUTRAL,
  isBombKey,
  keyMove,
  type Move,
} from "./input";
import { DRAW_HEIGHT, DRAW_WIDTH, drawBoard, type DrawPoint } from "./canvas";
import { PlayerMotion } from "./prediction";
import { joystickIntent } from "./joystick";
import { AvatarPicker } from "./AvatarPicker";
import { avatarName } from "./avatar-art";
import { normalizeAvatar } from "./avatar-animation";
import { loadSprites } from "./sprites";
import { animationNow } from "./clock";
import { itemNames, mapNames, sources, teamNames, type View } from "./types";
import "./style.css";

function useNow(interval = 200) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), interval);
    return () => window.clearInterval(id);
  }, [interval]);
  return now;
}

function clockNow(view: View, clock: BoardProps["clock"]) {
  return animationNow(view.serverAt, clock, performance.now());
}

export function Board(props: BoardProps<View>) {
  const { room, data: view, act, assets, sound, audio, clock } = props;
  const me = view && room.you >= 0 ? view.players[room.you] : undefined;
  const trapped = !!me?.trappedUntil;
  const controlled = !!room.seats[room.you]?.autoPlay;
  const [avatarOpen, setAvatarOpen] = useState(false);
  const [stick, setStick] = useState({ x: 0, y: 0 });
  const [avatarSaving, setAvatarSaving] = useState(false);
  const preferenceSent = useRef("");
  const canControl =
    !!view &&
    view.canAct &&
    !room.spectating &&
    !!me?.alive &&
    !trapped &&
    !controlled;
  const enabled = canControl && !avatarOpen;
  const canChoose =
    !!view &&
    view.phase !== "finished" &&
    !!me?.alive &&
    !room.spectating &&
    !controlled;
  useEffect(() => {
    if (!canChoose) setAvatarOpen(false);
  }, [canChoose]);
  const preferenceKey = `bubbleparty.avatar:${room.seats[room.you]?.id ?? ""}`;
  const scope = `${room.id}:${view?.epoch ?? ""}:${clock.epoch ?? 0}:${room.you}`;
  const motion = useRef(new PlayerMotion());
  const [error, setError] = useState("");
  const input = useRef(new LatestInput());
  const sending = useRef(false);
  const latest = useRef(props);
  latest.current = props;
  const currentMove = useRef<Move>("none");
  const currentFacing = useRef(me?.dir ?? "down");
  const heldKeys = useRef(new HeldDirections());
  const padPointer = useRef<number | null>(null);
  const display = useRef(new Map<number, DrawPoint>());
  const canvas = useRef<HTMLCanvasElement>(null);
  const boardElement = useRef<HTMLElement>(null);
  const avatarWasOpen = useRef(false);
  useEffect(() => {
    if (
      avatarWasOpen.current &&
      !avatarOpen &&
      !document.querySelector('[role="dialog"]')
    )
      canvas.current?.focus({ preventScroll: true });
    avatarWasOpen.current = avatarOpen;
  }, [avatarOpen]);
  const seen = useRef(new Set<number>());
  const initialized = useRef("");
  const phaseSeen = useRef("");
  const musicOn = useRef(false);
  useNow();

  useEffect(() => {
    void loadSprites(assets);
  }, [assets]);

  useEffect(() => {
    setAvatarOpen(false);
  }, [room.id, view?.epoch]);
  useEffect(() => {
    if (!view || view.phase !== "countdown" || !canChoose) return;
    const identity = `${room.id}:${view.epoch}:${room.you}`;
    if (preferenceSent.current === identity) return;
    preferenceSent.current = identity;
    let saved: string | null = null;
    try {
      saved = localStorage.getItem(preferenceKey);
    } catch {}
    if (
      saved === null ||
      !Number.isInteger(Number(saved)) ||
      Number(saved) < 0 ||
      Number(saved) > 3
    )
      return;
    if (Number(saved) !== (me?.avatar ?? 0))
      void act({
        type: "avatar",
        epoch: view.epoch,
        avatar: Number(saved),
      }).catch((e) => setError(String(e instanceof Error ? e.message : e)));
  }, [view?.epoch, view?.phase, canChoose, room.id, room.you, preferenceKey]);

  // Enter the arena once on a narrow screen. Keep the shared player panels in
  // their normal place above it; snapshots must never keep scrolling the page.
  useEffect(() => {
    if (!view?.epoch || !window.matchMedia("(max-width: 600px)").matches)
      return;
    const frame = requestAnimationFrame(() =>
      boardElement.current?.scrollIntoView({
        block: "start",
        behavior: "instant",
      }),
    );
    return () => cancelAnimationFrame(frame);
  }, [room.id, view?.epoch]);

  useEffect(() => {
    currentFacing.current = me?.dir ?? "down";
    if (!enabled) {
      currentMove.current = "none";
      heldKeys.current.clear();
      padPointer.current = null;
      setStick({ x: 0, y: 0 });
      input.current.release();
    }
  }, [enabled]);

  const sendOne = useCallback((force: boolean) => {
    const p = latest.current;
    if (
      (sending.current && (p.inputWindow ?? 1) <= 1) ||
      document.hidden ||
      !p.data
    )
      return;
    const request = input.current.next(
      performance.now(),
      force,
      p.inputWindow ?? 1,
    );
    if (!request) return;
    const { token, ...action } = request;
    sending.current = true;
    const sentScope = `${p.room.id}:${p.data.epoch}:${p.clock.epoch ?? 0}:${p.room.you}`;
    void p
      .act(action)
      .then(() => input.current.complete(token))
      .catch(() => {
        const now = latest.current;
        if (
          sentScope !==
          `${now.room.id}:${now.data?.epoch}:${now.clock.epoch ?? 0}:${now.room.you}`
        )
          return;
        const current = latest.current.data;
        const seq =
          latest.current.room.you >= 0
            ? (current?.players[latest.current.room.you]?.seq ?? 0)
            : 0;
        input.current.reset(seq);
        currentMove.current = "none";
        setError("操作被当前对局拒绝，已重新同步。");
      })
      .finally(() => {
        sending.current = false;
      });
  }, []);

  useEffect(() => {
    if (!view) return;
    input.current.sync({
      scope,
      epoch: view.epoch,
      seq: me?.seq ?? 0,
      enabled: canControl,
    });
  }, [scope, view?.epoch, me?.seq, canControl, view]);

  useEffect(() => {
    const id = window.setInterval(() => sendOne(false), 40);
    return () => window.clearInterval(id);
  }, [sendOne]);

  const setMove = useCallback(
    (move: Move) => {
      if (!enabled) return;
      currentMove.current = move;
      if (move !== "none") currentFacing.current = move;
      input.current.set({ move, bomb: false });
      sendOne(true);
    },
    [enabled, sendOne],
  );

  const placeBomb = useCallback(() => {
    if (!enabled) return;
    input.current.set({ move: currentMove.current, bomb: true });
    sendOne(true);
    input.current.set({ move: currentMove.current, bomb: false });
  }, [enabled, sendOne]);

  useEffect(() => {
    const down = (event: KeyboardEvent) => {
      if (
        !enabled ||
        (event.target instanceof HTMLElement &&
          event.target.closest(
            "input,textarea,select,[contenteditable=true],[role=dialog]",
          ))
      )
        return;
      const move = keyMove(event.key);
      if (move) {
        event.preventDefault();
        setMove(heldKeys.current.press(event.key));
        return;
      }
      if (isBombKey(event.key)) {
        if (
          event.target instanceof HTMLElement &&
          event.target.closest("button,a") &&
          !event.target.closest(".bubble-controls")
        )
          return;
        event.preventDefault();
        if (!event.repeat) placeBomb();
      }
    };
    const up = (event: KeyboardEvent) => {
      const move = keyMove(event.key);
      if (!move) return;
      setMove(heldKeys.current.release(event.key));
    };
    const release = () => {
      heldKeys.current.clear();
      padPointer.current = null;
      setStick({ x: 0, y: 0 });
      currentMove.current = "none";
      input.current.release();
      input.current.set({ ...NEUTRAL });
      sendOne(true);
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", release);
    document.addEventListener("visibilitychange", release);
    return () => {
      heldKeys.current.clear();
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", release);
      document.removeEventListener("visibilitychange", release);
    };
  }, [enabled, setMove, placeBomb, sendOne]);

  useEffect(() => {
    let raf = 0;
    const reduced = window.matchMedia?.(
      "(prefers-reduced-motion: reduce)",
    ).matches;
    let lastFrame = performance.now();
    const frame = () => {
      const frameNow = performance.now();
      const blend = 1 - Math.exp(-Math.min(100, frameNow - lastFrame) / 35);
      lastFrame = frameNow;
      const current = latest.current.data;
      const ctx = canvas.current?.getContext("2d");
      if (current && ctx) {
        if (!initialized.current) {
          for (const p of current.players)
            display.current.set(p.seat, { x: p.px ?? p.x, y: p.py ?? p.y });
        }
        for (const p of current.players) {
          const props = latest.current;
          const own = p.seat === props.room.you && !props.room.spectating;
          if (own) {
            const active =
              current.canAct &&
              !props.room.seats[p.seat]?.autoPlay &&
              !document.hidden;
            const point = motion.current.frame(
              current,
              p.seat,
              currentMove.current,
              frameNow,
              `${props.room.id}:${current.epoch}:${props.clock.epoch ?? 0}:${p.seat}`,
              active,
            );
            display.current.set(p.seat, {
              ...point,
              move: active ? currentMove.current : "none",
              dir: active ? currentFacing.current : p.dir,
            });
            if (canvas.current) {
              canvas.current.dataset.renderX = String(point.x);
              canvas.current.dataset.renderY = String(point.y);
              canvas.current.dataset.serverX = String(p.px ?? p.x);
              canvas.current.dataset.serverY = String(p.py ?? p.y);
            }
            continue;
          }
          const at = display.current.get(p.seat);
          if (!at || reduced) {
            display.current.set(p.seat, { x: p.px ?? p.x, y: p.py ?? p.y });
          } else {
            const x = p.px ?? p.x,
              y = p.py ?? p.y;
            at.x += (x - at.x) * blend;
            at.y += (y - at.y) * blend;
            if (Math.abs(x - at.x) + Math.abs(y - at.y) < 0.005) {
              at.x = x;
              at.y = y;
            }
          }
        }
        drawBoard(
          ctx,
          current,
          display.current,
          clockNow(current, latest.current.clock),
          !!reduced,
          latest.current.assets,
        );
      }
      raf = requestAnimationFrame(frame);
    };
    raf = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(raf);
  }, []);

  useEffect(() => {
    if (!view) return;
    if (initialized.current !== scope) {
      initialized.current = scope;
      seen.current = new Set(view.events.map((event) => event.id));
      phaseSeen.current = view.phase;
      display.current = new Map(
        view.players.map((p) => [p.seat, { x: p.px ?? p.x, y: p.py ?? p.y }]),
      );
      currentMove.current = "none";
      heldKeys.current.clear();
      currentFacing.current = me?.dir ?? "down";
      padPointer.current = null;
      setStick({ x: 0, y: 0 });
      input.current.reset(me?.seq ?? 0);
      return;
    }
    if (!sound || document.hidden) {
      for (const event of view.events) seen.current.add(event.id);
      return;
    }
    let explosions = 0;
    for (const event of view.events) {
      if (seen.current.has(event.id)) continue;
      seen.current.add(event.id);
      switch (event.type) {
        case "place":
          void audio.playWithFallback(sources(assets, "place.wav"));
          break;
        case "explode":
          if (explosions++ < 2)
            void audio.playWithFallback(sources(assets, "explode.wav"));
          break;
        case "trap":
          void audio.playWithFallback(sources(assets, "trap.wav"));
          break;
        case "rescue":
          void audio.playWithFallback(sources(assets, "rescue.wav"));
          break;
        case "eliminate":
          void audio.playWithFallback(sources(assets, "eliminate.wav"));
          break;
        case "drop":
        case "item":
          void audio.playWithFallback(sources(assets, "item.wav"));
          break;
        case "shield":
          void audio.playWithFallback(sources(assets, "shield.wav"));
          break;
        case "sudden":
        case "shrink":
          void audio.playWithFallback(sources(assets, "sudden.wav"));
          break;
        case "finish": {
          const winners =
            room.game?.winners?.length === 0
              ? "draw.wav"
              : me?.team === event.value
                ? "win.wav"
                : "lose.wav";
          void audio.playWithFallback(sources(assets, winners));
          audio.stopMusic();
          break;
        }
      }
    }
  }, [view, scope, room.id, assets, sound, audio, me?.team, me?.seq]);

  useEffect(() => {
    if (!view) return;
    const previous = phaseSeen.current;
    phaseSeen.current = view.phase;
    if (!sound || document.hidden) {
      audio.stopMusic();
      musicOn.current = false;
      return;
    }
    if (view.phase === "countdown" && previous !== "countdown") {
      void audio.playWithFallback(sources(assets, "countdown.wav"));
    } else if (view.phase === "playing") {
      if (previous === "countdown")
        void audio.playWithFallback(sources(assets, "start.wav"));
      if (!musicOn.current) {
        musicOn.current = true;
        void audio.music(sources(assets, "battle.wav"));
      }
    } else if (view.phase === "finished") {
      audio.stopMusic();
      musicOn.current = false;
    }
  }, [view?.phase, view?.epoch, room.id, assets, sound, audio, view]);

  useEffect(
    () => () => {
      audio.stop();
      audio.stopMusic();
      musicOn.current = false;
    },
    [audio],
  );

  const openAvatars = () => {
    currentMove.current = "none";
    heldKeys.current.clear();
    padPointer.current = null;
    setStick({ x: 0, y: 0 });
    input.current.release();
    input.current.set({ ...NEUTRAL });
    sendOne(true);
    setError("");
    setAvatarOpen(true);
  };
  const chooseAvatar = async (avatar: number) => {
    if (!view || !canChoose || avatarSaving) return;
    const epoch = view.epoch;
    setAvatarSaving(true);
    setError("");
    try {
      await act({ type: "avatar", epoch, avatar });
      if (latest.current.data?.epoch !== epoch) return;
      try {
        localStorage.setItem(preferenceKey, String(avatar));
      } catch {}
      setAvatarOpen(false);
    } catch (e) {
      if (latest.current.data?.epoch === epoch)
        setError(e instanceof Error ? e.message : String(e));
    } finally {
      setAvatarSaving(false);
    }
  };
  if (!view) {
    return <p className="bubble-loading">正在加载泡泡堂战场…</p>;
  }

  const alive = [0, 1].map(
    (team) => view.players.filter((p) => p.alive && p.team === team).length,
  );
  const serverNow = clockNow(view, clock);
  const countdown =
    view.phase === "countdown"
      ? Math.max(0, Math.ceil((view.startAt - serverNow) / 1000))
      : 0;
  const left = Math.max(0, Math.ceil((view.matchEndsAt - serverNow) / 1000));
  const minutes = Math.floor(left / 60);
  const seconds = String(left % 60).padStart(2, "0");
  const status =
    view.phase === "finished"
      ? "对局结束"
      : view.spectator
        ? "观战中"
        : !me?.alive
          ? "已淘汰 · 观战中"
          : trapped
            ? "被水泡困住 · 等待队友救援"
            : controlled
              ? "电脑托管中"
              : view.canAct
                ? "可以行动"
                : "等待其他玩家";

  const releasePad = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (padPointer.current !== event.pointerId) return;
    event.preventDefault();
    padPointer.current = null;
    setStick({ x: 0, y: 0 });
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
    setMove("none");
  };
  const movePad = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!enabled || padPointer.current !== event.pointerId) return;
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    const intent = joystickIntent(
      event.clientX - rect.left - rect.width / 2,
      event.clientY - rect.top - rect.height / 2,
      rect.width * 0.34,
      currentMove.current,
    );
    setStick({ x: intent.x, y: intent.y });
    setMove(intent.move);
  };

  return (
    <main
      ref={boardElement}
      className="bubble-board"
      data-bubbleparty-phase={view.phase}
      data-bubbleparty-epoch={view.epoch}
      data-bubbleparty-map={view.mapId}
    >
      <section className="bubble-status" aria-live="polite">
        <div>
          <span className={`bubble-presence ${view.canAct ? "active" : ""}`} />
          <strong>{status}</strong>
          <small>{mapNames[view.mapId] ?? view.mapId}</small>
        </div>
        <div className="bubble-clock">
          {view.phase === "countdown"
            ? `${countdown} 秒后开始`
            : view.phase === "playing"
              ? `${minutes}:${seconds}`
              : "已结束"}
        </div>
      </section>
      {view.sudden && view.phase === "playing" && (
        <div className="bubble-sudden" role="status">
          突然死亡 · 安全区正在收缩 · 第 {view.safeInset} 圈
        </div>
      )}
      <div className="bubble-teams" aria-label="队伍存活">
        {[0, 1].map((team) => (
          <div key={team} className={`bubble-team team-${team}`}>
            <strong>{teamNames[team]}</strong>
            <span>{alive[team]} 人存活</span>
          </div>
        ))}
      </div>
      <div className="bubble-stage">
        <canvas
          ref={canvas}
          tabIndex={-1}
          width={DRAW_WIDTH * 2}
          height={DRAW_HEIGHT * 2}
          aria-label={`${mapNames[view.mapId] ?? view.mapId}水泡战场`}
          data-bubbleparty-canvas
        />
        {view.phase === "countdown" && (
          <div className="bubble-countdown" aria-hidden="true">
            <span>{countdown}</span>
            <small>准备水泡</small>
          </div>
        )}
      </div>
      <div className="bubble-mine" aria-live="polite">
        {trapped && (
          <strong className="bubble-trapped">
            救援剩余{" "}
            {Math.max(
              0,
              Math.ceil(((me?.trappedUntil ?? 0) - serverNow) / 1000),
            )}{" "}
            秒
          </strong>
        )}
        <span>
          {me
            ? `你 · ${teamNames[me.team]} · ${trapped ? "困泡中" : me.alive ? "存活" : "已淘汰"}`
            : "观战"}
        </span>
        {me && me.alive && (
          <span>
            水泡 {me.maxBombs} · 射程 {me.range}
            {me.shield ? " · 护盾" : ""}
            {me.eliminations ? ` · 淘汰 ${me.eliminations}` : ""}
            {me.crates ? ` · 木箱 ${me.crates}` : ""}
          </span>
        )}
      </div>
      {me && !view.spectator && view.phase !== "finished" && (
        <div className="bubble-appearance">
          <button
            type="button"
            data-bubbleparty-avatar-open
            aria-label="选择人物"
            disabled={!canChoose}
            onClick={openAvatars}
          >
            人物 · {avatarName(me.avatar)}
          </button>
        </div>
      )}
      {avatarOpen && me && (
        <AvatarPicker
          team={me.team}
          selected={normalizeAvatar(me.avatar)}
          assets={assets}
          saving={avatarSaving || !canChoose}
          error={error}
          onChoose={chooseAvatar}
          onClose={() => setAvatarOpen(false)}
        />
      )}
      {!view.spectator && (
        <div className="bubble-controls" aria-label="操作" data-game-controls>
          <div
            className="bubble-dpad bubble-joystick"
            role="group"
            aria-label="拖动摇杆移动，松手停止"
            aria-disabled={!enabled}
            data-active={padPointer.current !== null}
            data-bubbleparty-dpad
            onPointerDown={(event) => {
              if (!enabled || padPointer.current !== null || event.button !== 0)
                return;
              event.preventDefault();
              padPointer.current = event.pointerId;
              event.currentTarget.setPointerCapture(event.pointerId);
              movePad(event);
            }}
            onPointerMove={movePad}
            onPointerUp={releasePad}
            onPointerCancel={releasePad}
            onLostPointerCapture={releasePad}
          >
            <svg
              className="bubble-stick-guides"
              viewBox="0 0 100 100"
              aria-hidden="true"
            >
              <path d="M45 17l5-5 5 5M45 83l5 5 5-5M17 45l-5 5 5 5M83 45l5 5-5 5" />
            </svg>
            <span
              className="bubble-stick-thumb"
              aria-hidden="true"
              style={{
                transform: `translate(${stick.x * 38}px,${stick.y * 38}px)`,
              }}
            >
              <i />
            </span>
            <span className="bubble-stick-caption" aria-hidden="true">
              摇杆
            </span>
          </div>
          <button
            className="bubble-bomb"
            data-bubbleparty-bomb
            disabled={!enabled}
            onPointerDown={(event) => {
              event.preventDefault();
              placeBomb();
            }}
          >
            放水泡
            <small>空格 / J</small>
          </button>
        </div>
      )}
      <p className="bubble-hint">
        {view.spectator
          ? "观战只读；你可以发表情和聊天。"
          : "WASD／方向键移动，空格或 J 放水泡。触碰敌方困泡可击破、队友可救援；冲进未消退的水柱也会被困。"}
      </p>
      {error && (
        <div className="bubble-error" role="alert">
          {error}
        </div>
      )}
      {me && view.phase === "playing" && (
        <div className="bubble-log" aria-label="战场动态">
          {view.log.slice(-2).map((line, index) => (
            <span key={index}>{line}</span>
          ))}
        </div>
      )}
    </main>
  );
}
