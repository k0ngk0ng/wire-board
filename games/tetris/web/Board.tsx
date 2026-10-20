import { memo, useEffect, useMemo, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Controls } from "./Controls";
import {
  DAS_DELAY,
  DAS_INTERVAL,
  InputQueue,
  SOFT_DROP_DELAY,
  SOFT_DROP_INTERVAL,
  repeatTimes,
  type InputKind,
} from "./input";
import { animationFor, nextEvents, soundFor, type Seen } from "./events";
import { progressText, timeText } from "./format";
import { ghostY, predict } from "./prediction";
import { HIDDEN_ROWS, VISIBLE_ROWS, shapeCells } from "./shapes";
import {
  SPRINT_TARGET,
  modeNames,
  seatColors,
  sources,
  type Kind,
  type Options,
  type PlayerView,
  type View,
} from "./types";
import "./style.css";

type PieceLike = {
  kind: Kind;
  rot: number;
  x: number;
  y: number;
};

type Fx = { type: string; key: number; rows?: number[] };

const cellKey = (x: number, y: number) => `${x},${y}`;

function visibleCells(piece: PieceLike): Map<string, Kind> {
  const cells = new Map<string, Kind>();
  for (const pt of shapeCells(piece.kind, piece.rot)) {
    const x = piece.x + pt.x;
    const y = piece.y + pt.y - HIDDEN_ROWS;
    if (x >= 0 && x < 10 && y >= 0 && y < VISIBLE_ROWS)
      cells.set(cellKey(x, y), piece.kind);
  }
  return cells;
}

const BoardGrid = memo(function BoardGrid({
  board,
  active,
  ghost,
  fx,
  small = false,
}: {
  board: string[];
  active?: PieceLike;
  ghost?: number;
  fx?: Fx;
  small?: boolean;
}) {
  const activeCells = active ? visibleCells(active) : new Map<string, Kind>();
  const ghostCells = useMemo(() => {
    if (!active || ghost === undefined) return new Set<string>();
    return new Set(visibleCells({ ...active, y: ghost }).keys());
  }, [active?.kind, active?.rot, active?.x, ghost]);
  return (
    <div
      className={`tetris-grid ${small ? "is-small" : ""} ${
        fx ? `is-${fx.type}` : ""
      }`}
      data-tetris-grid={small ? "rival" : "self"}
      aria-hidden="true"
    >
      {Array.from({ length: VISIBLE_ROWS * 10 }, (_, i) => {
        const y = Math.floor(i / 10);
        const x = i % 10;
        const locked = board[y]?.[x] ?? "0";
        const kind = locked === "0" ? 0 : Number(locked);
        const activeKind = activeCells.get(cellKey(x, y)) ?? 0;
        const isGhost = !activeKind && ghostCells.has(cellKey(x, y));
        return (
          <span
            key={i}
            className={`tetris-cell${kind ? ` c${kind}` : ""}${
              activeKind ? ` active c${activeKind}` : ""
            }${isGhost ? " ghost" : ""}`}
          />
        );
      })}
      {fx?.type === "clear" &&
        fx.rows?.map((row) => (
          <span
            key={`clear-${row}`}
            className="tetris-clear-row"
            style={{
              top: `${(row / VISIBLE_ROWS) * 100}%`,
              height: `${100 / VISIBLE_ROWS}%`,
            }}
          />
        ))}
    </div>
  );
});

function MiniPiece({ kind, dim = false }: { kind: Kind; dim?: boolean }) {
  const cells = shapeCells(kind, 0);
  const minX = Math.min(...cells.map((p) => p.x));
  const minY = Math.min(...cells.map((p) => p.y));
  const filled = new Set(cells.map((p) => cellKey(p.x - minX, p.y - minY)));
  return (
    <span
      className={`tetris-mini-piece${dim ? " is-dim" : ""}`}
      aria-hidden="true"
    >
      {Array.from({ length: 16 }, (_, i) => {
        const x = i % 4;
        const y = Math.floor(i / 4);
        return (
          <i key={i} className={filled.has(cellKey(x, y)) ? `c${kind}` : ""} />
        );
      })}
    </span>
  );
}

function Preview({
  label,
  kinds,
  empty,
}: {
  label: string;
  kinds: Kind[];
  empty: string;
}) {
  return (
    <div className="tetris-preview" aria-label={label}>
      <span className="tetris-preview-label">{label}</span>
      <div className="tetris-preview-list">
        {kinds.length ? (
          kinds.map((kind, index) => (
            <MiniPiece key={`${kind}-${index}`} kind={kind} />
          ))
        ) : (
          <span className="tetris-preview-empty">{empty}</span>
        )}
      </div>
    </div>
  );
}

function Rival({
  player,
  roomName,
  mode,
  fx,
  you,
}: {
  player: PlayerView;
  roomName: string;
  mode: Options["mode"];
  fx?: Fx;
  you: number;
}) {
  return (
    <article
      className={`tetris-rival${player.alive ? "" : " is-eliminated"}`}
      data-tetris-rival={player.seat}
    >
      <header>
        <i
          aria-hidden="true"
          style={{ background: seatColors[player.seat % seatColors.length] }}
        />
        <strong>
          {roomName}
          {you === player.seat ? " · 你" : ""}
        </strong>
        <span>{player.alive ? "存活" : "已出局"}</span>
      </header>
      <BoardGrid board={player.board} active={player.active} fx={fx} small />
      <div className="tetris-rival-stats">
        <span>{progressText(mode, player.lines)}</span>
        <span>{player.score} 分</span>
        <span>待落 {player.pendingCount}</span>
        <span>
          目标 {player.targetSeat >= 0 ? `${player.targetSeat + 1} 号` : "—"}
        </span>
      </div>
    </article>
  );
}

const actionError = (error: unknown) =>
  error instanceof TypeError
    ? "网络连接中断，正在恢复操作"
    : error instanceof Error
      ? error.message
      : String(error);

const keyMap: Record<string, { kind: InputKind; held: boolean }> = {
  ArrowLeft: { kind: "left", held: true },
  KeyA: { kind: "left", held: true },
  ArrowRight: { kind: "right", held: true },
  KeyD: { kind: "right", held: true },
  ArrowUp: { kind: "rotateCW", held: false },
  KeyX: { kind: "rotateCW", held: false },
  KeyZ: { kind: "rotateCCW", held: false },
  ControlLeft: { kind: "rotateCCW", held: false },
  ControlRight: { kind: "rotateCCW", held: false },
  ArrowDown: { kind: "softDrop", held: true },
  KeyS: { kind: "softDrop", held: true },
  Space: { kind: "hardDrop", held: false },
  KeyC: { kind: "hold", held: false },
  ShiftLeft: { kind: "hold", held: false },
  ShiftRight: { kind: "hold", held: false },
};

const ignoredTarget = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  (target.matches("input,select,textarea") || target.isContentEditable);

export function Board(props: BoardProps<View>) {
  const { room, data: d, act, busy, clock, assets, audio, sound } = props;
  const you = room.you;
  const spectator = !!room.spectating || you < 0;
  const own = you >= 0 ? d.players[you] : undefined;
  const autoPlay = you >= 0 && !!room.seats[you]?.autoPlay;
  const eliminated =
    you >= 0 && (room.game?.eliminated?.includes(you) ?? false);
  const alive = own?.alive ?? false;
  const [now, setNow] = useState(() => performance.now());
  const [error, setError] = useState("");
  const [queueVersion, setQueueVersion] = useState(0);
  const [suspended, setSuspended] = useState(false);
  const [fx, setFx] = useState<Record<number, Fx>>({});
  const [reduced, setReduced] = useState(
    () =>
      typeof window !== "undefined" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  );
  const queue = useRef(new InputQueue(0));
  const latest = useRef(props);
  latest.current = props;
  const enabledRef = useRef(false);
  const suspendedRef = useRef(false);
  const sending = useRef(false);
  const held = useRef(new Map<string, { since: number; emitted: number }>());
  const touchHold = useRef<{
    kind: InputKind;
    since: number;
    emitted: number;
  } | null>(null);
  const seen = useRef<Seen>({ scope: "", id: 0 });
  const timers = useRef<number[]>([]);
  const ownSeq = own?.seq ?? 0;
  const scope = `${room.id}:${d.epoch}:${clock.epoch ?? 0}:${you}`;
  const scopeRef = useRef(scope);
  scopeRef.current = scope;
  const baseEnabled =
    room.status === "playing" &&
    !spectator &&
    !eliminated &&
    alive &&
    !autoPlay &&
    !busy &&
    d.phase === "playing";
  const enabled = baseEnabled && !suspended;
  enabledRef.current = baseEnabled;
  suspendedRef.current = suspended;
  const unconfirmed = queue.current.unconfirmed();
  const predicted = useMemo(
    () => predict(own?.board ?? [], own?.active, unconfirmed),
    [own?.board, own?.active, queueVersion, d.epoch],
  );
  const ghost = predicted && own ? ghostY(own.board, predicted) : undefined;
  const rivals =
    you >= 0 ? d.players.filter((player) => player.seat !== you) : d.players;

  useEffect(() => {
    const p = latest.current;
    queue.current.reset(
      p.room.you >= 0 ? (p.data.players[p.room.you]?.seq ?? 0) : 0,
    );
    setQueueVersion((value) => value + 1);
    held.current.clear();
    touchHold.current = null;
    const max = (p.data.events ?? []).reduce(
      (value, event) => Math.max(value, event.id),
      0,
    );
    seen.current = { scope, id: max };
    for (const id of timers.current) window.clearTimeout(id);
    timers.current = [];
    setFx({});
    setSuspended(false);
    suspendedRef.current = false;
    sending.current = false;
    setError("");
  }, [scope]);

  useEffect(() => {
    if (spectator) return;
    queue.current.acknowledge(ownSeq);
    setQueueVersion((value) => value + 1);
  }, [ownSeq, spectator]);

  useEffect(() => {
    if (baseEnabled) return;
    held.current.clear();
    touchHold.current = null;
    queue.current.reset(spectator ? 0 : ownSeq);
    setQueueVersion((value) => value + 1);
  }, [baseEnabled, ownSeq, spectator]);

  useEffect(() => {
    const id = window.setInterval(() => {
      if (
        !enabledRef.current ||
        suspendedRef.current ||
        (sending.current && (latest.current.inputWindow ?? 1) <= 1) ||
        document.hidden
      )
        return;
      const batch = queue.current.take();
      if (!batch) return;
      sending.current = true;
      setQueueVersion((value) => value + 1);
      const p = latest.current;
      const sentScope = scopeRef.current;
      void p
        .act({
          type: "inputs",
          epoch: p.data.epoch,
          seq: batch.seq,
          events: batch.events,
        })
        .catch((reason) => {
          if (scopeRef.current !== sentScope) return;
          const current = latest.current;
          queue.current.reset(
            current.room.you >= 0
              ? (current.data.players[current.room.you]?.seq ?? 0)
              : 0,
          );
          setQueueVersion((value) => value + 1);
          suspendedRef.current = true;
          setSuspended(true);
          setError(actionError(reason));
        })
        .finally(() => {
          sending.current = false;
        });
    }, 40);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    const id = window.setInterval(() => setNow(performance.now()), 100);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const change = () => setReduced(query.matches);
    query.addEventListener("change", change);
    return () => query.removeEventListener("change", change);
  }, []);

  const push = (kind: InputKind) => {
    if (!enabledRef.current) return;
    if (suspendedRef.current) wake();
    if (!queue.current.push(kind)) {
      suspendedRef.current = true;
      setSuspended(true);
      setError("操作过快，正在同步；请稍后继续。");
      return;
    }
    setQueueVersion((value) => value + 1);
  };

  const wake = () => {
    const p = latest.current;
    const seq = p.room.you >= 0 ? (p.data.players[p.room.you]?.seq ?? 0) : 0;
    queue.current.reset(seq);
    setQueueVersion((value) => value + 1);
    suspendedRef.current = false;
    sending.current = false;
    setSuspended(false);
    setError("");
  };

  const suspend = (message = "") => {
    held.current.clear();
    touchHold.current = null;
    suspendedRef.current = true;
    setSuspended(true);
    if (message) setError(message);
  };

  useEffect(() => {
    const id = window.setInterval(() => {
      if (!enabledRef.current || suspendedRef.current) {
        held.current.clear();
        touchHold.current = null;
        return;
      }
      const time = performance.now();
      for (const [code, state] of held.current) {
        const map = keyMap[code];
        if (!map?.held) continue;
        const delay = map.kind === "softDrop" ? SOFT_DROP_DELAY : DAS_DELAY;
        const interval =
          map.kind === "softDrop" ? SOFT_DROP_INTERVAL : DAS_INTERVAL;
        const due = repeatTimes(state.since, time, delay, interval);
        for (let i = state.emitted; i < due; i++) push(map.kind);
        state.emitted = Math.max(state.emitted, due);
      }
      const touch = touchHold.current;
      if (touch) {
        const delay = touch.kind === "softDrop" ? SOFT_DROP_DELAY : DAS_DELAY;
        const interval =
          touch.kind === "softDrop" ? SOFT_DROP_INTERVAL : DAS_INTERVAL;
        const due = repeatTimes(touch.since, time, delay, interval);
        for (let i = touch.emitted; i < due; i++) push(touch.kind);
        touch.emitted = Math.max(touch.emitted, due);
      }
    }, 16);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    const down = (event: KeyboardEvent) => {
      if (ignoredTarget(event.target)) return;
      if (suspendedRef.current) wake();
      if (!enabledRef.current) return;
      if (
        event.code === "Space" &&
        event.target instanceof HTMLElement &&
        event.target.matches("button")
      )
        return;
      const map = keyMap[event.code];
      if (!map) return;
      event.preventDefault();
      if (event.repeat) return;
      if (map.held) {
        if (!held.current.has(event.code)) {
          held.current.set(event.code, {
            since: performance.now(),
            emitted: 0,
          });
          push(map.kind);
        }
      } else {
        push(map.kind);
      }
    };
    const up = (event: KeyboardEvent) => {
      held.current.delete(event.code);
    };
    const blur = () => {
      suspend();
    };
    const visibility = () => {
      if (document.hidden) suspend();
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", blur);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", blur);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, [scope]);

  useEffect(() => {
    const result = nextEvents(d.events ?? [], seen.current, scope);
    seen.current = result.seen;
    if (result.events.length === 0) return;
    if (sound && !document.hidden) {
      for (const event of result.events) {
        const file = soundFor(event, you, spectator);
        if (file) void audio.playWithFallback(sources(assets, `${file}.wav`));
      }
    }
    if (reduced) return;
    const updates: Record<number, Fx> = {};
    for (const event of result.events) {
      const type = animationFor(event);
      if (type)
        updates[event.seat] = {
          type,
          key: event.id,
          rows: event.type === "clear" ? event.rows : undefined,
        };
    }
    if (Object.keys(updates).length === 0) return;
    setFx((old) => ({ ...old, ...updates }));
    for (const [seat, update] of Object.entries(updates)) {
      const id = window.setTimeout(() => {
        setFx((old) => {
          if (old[Number(seat)]?.key !== update.key) return old;
          const next = { ...old };
          delete next[Number(seat)];
          return next;
        });
      }, 280);
      timers.current.push(id);
    }
  }, [d.events, scope, sound, reduced, you, spectator, assets, audio]);

  useEffect(
    () => () => {
      for (const id of timers.current) window.clearTimeout(id);
      timers.current = [];
      audio.stop();
      held.current.clear();
      touchHold.current = null;
    },
    [audio],
  );

  const serverNow = clock.serverNow + now - clock.receivedAt;
  const remaining = Math.max(0, d.matchEndsAt - serverNow);
  const countdown = Math.max(0, Math.ceil((d.startAt - serverNow) / 1000));
  const lastEventID = (d.events ?? []).reduce(
    (value, event) => Math.max(value, event.id),
    0,
  );
  const status = spectator
    ? "正在观战"
    : d.phase === "finished"
      ? "对局结束"
      : d.phase === "countdown"
        ? "准备落块"
        : autoPlay
          ? "电脑托管中；取消托管后继续"
          : eliminated
            ? "你已出局，可以观战或离桌"
            : suspended
              ? "操作已暂停，正在同步"
              : "对局进行中";
  const touchStart = (kind: InputKind) => {
    if (!enabledRef.current) return;
    if (kind === "left" || kind === "right" || kind === "softDrop") {
      touchHold.current = { kind, since: performance.now(), emitted: 0 };
      push(kind);
    } else {
      push(kind);
    }
  };
  const touchEnd = () => {
    touchHold.current = null;
  };

  return (
    <section
      className={`tetris-board${spectator ? " is-spectator" : ""}`}
      data-tetris-phase={d.phase}
      data-tetris-seat={you}
      data-tetris-active-x={predicted?.x ?? ""}
      data-tetris-active-kind={predicted?.kind ?? ""}
      data-tetris-active-rotation={predicted?.rot ?? ""}
      data-tetris-piece-id={predicted?.id ?? ""}
      data-tetris-score={own?.score ?? ""}
      data-tetris-lines={own?.lines ?? ""}
      data-tetris-pending={own?.pendingCount ?? ""}
      data-last-event-id={lastEventID}
    >
      <div className="tetris-hud">
        <div className="tetris-hud-mode">
          <span>{modeNames[d.options.mode]}</span>
          <strong>
            {d.options.mode === "sprint" ? "先清 40 行" : "最后存活"}
          </strong>
        </div>
        <div>
          <span>剩余</span>
          <strong>{timeText(remaining)}</strong>
        </div>
        <div>
          <span>行数</span>
          <strong>{own ? progressText(d.options.mode, own.lines) : "—"}</strong>
        </div>
        <div>
          <span>分数</span>
          <strong>{own?.score ?? "—"}</strong>
        </div>
        <div>
          <span>等级</span>
          <strong>{own?.level ?? "—"}</strong>
        </div>
        <div>
          <span>待落</span>
          <strong>{own?.pendingCount ?? "—"}</strong>
        </div>
      </div>
      <div
        className={`tetris-status${suspended ? " is-suspended" : ""}`}
        role="status"
      >
        <span>{status}</span>
        {suspended && (
          <button type="button" onClick={wake}>
            继续操作
          </button>
        )}
      </div>
      {spectator ? (
        <div className="tetris-spectator-grid">
          {d.players.map((player) => (
            <Rival
              key={player.seat}
              player={player}
              roomName={
                room.seats[player.seat]?.name ?? `玩家 ${player.seat + 1}`
              }
              mode={d.options.mode}
              fx={fx[player.seat]}
              you={-1}
            />
          ))}
        </div>
      ) : (
        <div className="tetris-stage">
          <div className="tetris-self">
            <div className="tetris-self-head">
              <strong>{room.seats[you]?.name ?? "你"}</strong>
              <span>
                {own?.alive ? "你的棋盘" : "已出局"} ·{" "}
                {progressText(d.options.mode, own?.lines ?? 0)}
                {d.options.mode === "sprint" &&
                  ` · ${Math.min(own?.lines ?? 0, SPRINT_TARGET)}/${SPRINT_TARGET}`}
              </span>
            </div>
            <BoardGrid
              board={own?.board ?? []}
              active={predicted}
              ghost={ghost}
              fx={fx[you]}
            />
            <div className="tetris-side-panels">
              <Preview
                label="暂存"
                kinds={own?.hold ? [own.hold] : []}
                empty="—"
              />
              <Preview label="下一个" kinds={own?.queue ?? []} empty="—" />
              <div className="tetris-target">
                <span>攻击目标</span>
                <strong>
                  {own && own.targetSeat >= 0
                    ? `${own.targetSeat + 1} 号`
                    : "—"}
                </strong>
              </div>
            </div>
          </div>
          <Controls
            disabled={!enabled}
            autoPlay={autoPlay}
            onAction={push}
            onHoldStart={touchStart}
            onHoldEnd={touchEnd}
          />
          {rivals.length > 0 && (
            <div className="tetris-rivals">
              {rivals.map((player) => (
                <Rival
                  key={player.seat}
                  player={player}
                  roomName={
                    room.seats[player.seat]?.name ?? `玩家 ${player.seat + 1}`
                  }
                  mode={d.options.mode}
                  fx={fx[player.seat]}
                  you={you}
                />
              ))}
            </div>
          )}
        </div>
      )}
      {d.phase === "countdown" && (
        <div className="tetris-countdown" aria-label={`倒计时 ${countdown}`}>
          <strong>{countdown || "开始"}</strong>
          <span>准备落块</span>
        </div>
      )}
      {error && (
        <p className="tetris-error" role="alert">
          {error}
        </p>
      )}
      {!spectator && eliminated && (
        <p className="tetris-eliminated-note">你已出局，可以继续观战或离桌。</p>
      )}
    </section>
  );
}
