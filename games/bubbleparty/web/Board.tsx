import { useCallback, useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import {
  LatestInput,
  NEUTRAL,
  isBombKey,
  keyMove,
  type Move,
} from "./input";
import { HEIGHT, WIDTH, drawBoard, type DrawPoint } from "./canvas";
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
  return view.serverAt + Math.max(0, performance.now() - (clock.receivedAt || performance.now()));
}

export function Board(props: BoardProps<View>) {
  const { room, data: view, act, assets, sound, audio, clock } = props;
  const me = view && room.you >= 0 ? view.players[room.you] : undefined;
  const enabled = !!view && view.canAct && !room.spectating && !!me?.alive;
  const [error, setError] = useState("");
  const input = useRef(new LatestInput());
  const sending = useRef(false);
  const latest = useRef(props);
  latest.current = props;
  const currentMove = useRef<Move>("none");
  const display = useRef(new Map<number, DrawPoint>());
  const canvas = useRef<HTMLCanvasElement>(null);
  const seen = useRef(new Set<number>());
  const initialized = useRef("");
  const phaseSeen = useRef("");
  const musicOn = useRef(false);
  const bombTimer = useRef<number | undefined>(undefined);
  useNow();

  const sendOne = useCallback((force: boolean) => {
    const p = latest.current;
    if (sending.current || document.hidden || !p.data) return;
    const request = input.current.next(performance.now(), force);
    if (!request) return;
    const { token, ...action } = request;
    sending.current = true;
    void p
      .act(action)
      .then(() => input.current.complete(token))
      .catch(() => {
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
      scope: room.id,
      epoch: view.epoch,
      seq: me?.seq ?? 0,
      enabled,
    });
  }, [room.id, view?.epoch, me?.seq, enabled, view]);

  useEffect(() => {
    const id = window.setInterval(() => sendOne(false), 40);
    return () => window.clearInterval(id);
  }, [sendOne]);

  const setMove = useCallback(
    (move: Move) => {
      if (!enabled) return;
      currentMove.current = move;
      input.current.set({ move, bomb: false });
      sendOne(true);
    },
    [enabled, sendOne],
  );

  const placeBomb = useCallback(() => {
    if (!enabled) return;
    input.current.set({ move: currentMove.current, bomb: true });
    sendOne(true);
    if (bombTimer.current) window.clearTimeout(bombTimer.current);
    bombTimer.current = window.setTimeout(() => {
      input.current.set({ move: currentMove.current, bomb: false });
      sendOne(true);
    }, 130);
  }, [enabled, sendOne]);

  useEffect(() => {
    const down = (event: KeyboardEvent) => {
      if (!enabled) return;
      const move = keyMove(event.key);
      if (move) {
        event.preventDefault();
        setMove(move);
        return;
      }
      if (isBombKey(event.key)) {
        event.preventDefault();
        if (!event.repeat) placeBomb();
      }
    };
    const up = (event: KeyboardEvent) => {
      const move = keyMove(event.key);
      if (!move || currentMove.current !== move) return;
      setMove("none");
    };
    const release = () => {
      if (bombTimer.current) window.clearTimeout(bombTimer.current);
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
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", release);
      document.removeEventListener("visibilitychange", release);
    };
  }, [enabled, setMove, placeBomb, sendOne]);

  useEffect(() => {
    let raf = 0;
    const reduced = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    const frame = () => {
      const current = latest.current.data;
      const ctx = canvas.current?.getContext("2d");
      if (current && ctx) {
        if (!initialized.current) {
          for (const p of current.players) display.current.set(p.seat, { x: p.x, y: p.y });
        }
        for (const p of current.players) {
          const at = display.current.get(p.seat);
          if (!at || reduced) {
            display.current.set(p.seat, { x: p.x, y: p.y });
          } else {
            at.x += (p.x - at.x) * 0.28;
            at.y += (p.y - at.y) * 0.28;
            if (Math.abs(p.x - at.x) + Math.abs(p.y - at.y) < 0.02) {
              at.x = p.x;
              at.y = p.y;
            }
          }
        }
        drawBoard(ctx, current, display.current, clockNow(current, latest.current.clock), !!reduced);
      }
      raf = requestAnimationFrame(frame);
    };
    raf = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(raf);
  }, []);

  useEffect(() => {
    if (!view) return;
    const scope = `${room.id}:${view.epoch}`;
    if (initialized.current !== scope) {
      initialized.current = scope;
      seen.current = new Set(view.events.map((event) => event.id));
      phaseSeen.current = view.phase;
      display.current = new Map(view.players.map((p) => [p.seat, { x: p.x, y: p.y }]));
      currentMove.current = "none";
      input.current.reset(me?.seq ?? 0);
      return;
    }
    if (!sound || document.hidden) return;
    let explosions = 0;
    for (const event of view.events) {
      if (seen.current.has(event.id)) continue;
      seen.current.add(event.id);
      switch (event.type) {
        case "place":
          void audio.playWithFallback(sources(assets, "place.wav"));
          break;
        case "explode":
          if (explosions++ < 2) void audio.playWithFallback(sources(assets, "explode.wav"));
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
          const winners = me?.team === event.value ? "win.wav" : "lose.wav";
          void audio.playWithFallback(sources(assets, winners));
          audio.stopMusic();
          break;
        }
      }
    }
  }, [view, room.id, assets, sound, audio, me?.team, me?.seq]);

  useEffect(() => {
    if (!view || !sound) return;
    const scope = `${room.id}:${view.epoch}:${view.phase}`;
    if (phaseSeen.current === view.phase) return;
    const previous = phaseSeen.current;
    phaseSeen.current = view.phase;
    if (view.phase === "countdown") {
      void audio.playWithFallback(sources(assets, "countdown.wav"));
    } else if (view.phase === "playing" && previous === "countdown") {
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
      if (bombTimer.current) window.clearTimeout(bombTimer.current);
      audio.stop();
      audio.stopMusic();
      musicOn.current = false;
    },
    [audio],
  );

  if (!view) {
    return <p className="bubble-loading">正在铺设小区草坪…</p>;
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
        : view.canAct
          ? "可以行动"
          : "等待其他玩家";

  const press = (move: Move) => (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    event.currentTarget.setPointerCapture?.(event.pointerId);
    setMove(move);
  };
  const release = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    if (currentMove.current !== "none") setMove("none");
  };

  return (
    <main
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
          width={WIDTH * 2}
          height={HEIGHT * 2}
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
      <div className="bubble-mine">
        <span>
          {me ? `你 · ${teamNames[me.team]} · ${me.alive ? "存活" : "已淘汰"}` : "观战"}
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
      {!view.spectator && (
        <div className="bubble-controls" aria-label="操作">
          <div className="bubble-dpad" data-bubbleparty-dpad>
            <button
              className="up"
              disabled={!enabled}
              aria-label="向上"
              onPointerDown={press("up")}
              onPointerUp={release}
              onPointerCancel={release}
              onPointerLeave={release}
            >
              ↑
            </button>
            <button
              className="left"
              disabled={!enabled}
              aria-label="向左"
              onPointerDown={press("left")}
              onPointerUp={release}
              onPointerCancel={release}
              onPointerLeave={release}
            >
              ←
            </button>
            <span className="hub" aria-hidden="true" />
            <button
              className="right"
              disabled={!enabled}
              aria-label="向右"
              onPointerDown={press("right")}
              onPointerUp={release}
              onPointerCancel={release}
              onPointerLeave={release}
            >
              →
            </button>
            <button
              className="down"
              disabled={!enabled}
              aria-label="向下"
              onPointerDown={press("down")}
              onPointerUp={release}
              onPointerCancel={release}
              onPointerLeave={release}
            >
              ↓
            </button>
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
          : "WASD／方向键移动，空格或 J 放水泡。水泡会连爆并破坏木箱；掉进爆心会被淘汰。"}
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
