import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { soundSources, useAtlas, type Atlas } from "./assets";
import { drawBattlefield, drawTankIcon } from "./canvas";
import { TankMotion, type TankWorld, type TankPose } from "./prediction";
import {
  LatestInput,
  NEUTRAL,
  isFireKey,
  keyMove,
  padDirection,
  type Control,
  type Move,
} from "./input";
import { MapPreview } from "./MapPreview";
import {
  MAPS,
  mapById,
  decodeBroken,
  TANK_NAMES,
  TANK_ORDER,
  TANK_STATS,
  TEAM_COLORS,
  TEAM_NAMES,
  eventSound,
  phaseLabel,
  type TankClass,
  type TankView,
  type View,
} from "./types";
import "./style.css";

const TEAM_LABEL = ["上方", "下方"];
const DIR_NAMES = ["up", "right", "down", "left"];

function isEditable(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  const tag = el.tagName?.toLowerCase();
  return (
    tag === "input" ||
    tag === "textarea" ||
    tag === "select" ||
    el.isContentEditable
  );
}

function TankIcon({
  atlas,
  tankClass,
  team,
}: {
  atlas: Atlas | null;
  tankClass: TankClass;
  team: number;
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const ctx = ref.current?.getContext("2d");
    if (ctx) drawTankIcon(ctx, atlas, tankClass, team);
  }, [atlas, tankClass, team]);
  return (
    <canvas
      ref={ref}
      className="bc-tank-icon"
      width={64}
      height={64}
      aria-hidden="true"
    />
  );
}

function hpPips(tank: TankView) {
  const max = Math.max(1, tank.maxHp);
  return Array.from({ length: max }, (_, i) => (
    <span
      key={i}
      className={`bc-hp-pip ${i < tank.hp ? "is-full" : ""}`}
      aria-hidden="true"
    />
  ));
}

function TeamHud({
  room,
  view,
  team,
  serverNow,
  viewer,
}: {
  room: BoardProps<View>["room"];
  view: View;
  team: number;
  serverNow: number;
  viewer: number;
}) {
  const size = view.teamSize;
  const seats = Array.from({ length: size }, (_, i) => team * size + i);
  return (
    <div
      className={`bc-team bc-team-${team === 0 ? "top" : "bottom"} ${
        viewer >= 0 && view.tanks[viewer]?.team === team ? "is-yours" : ""
      }`}
      data-battlecity-team={team}
    >
      <div className="bc-team-title">
        <span
          className="bc-team-dot"
          style={{ background: TEAM_COLORS[team] }}
        />
        {TEAM_LABEL[team]}队伍
        <span className="bc-team-size">{size} 人</span>
      </div>
      <div className="bc-team-players">
        {seats.map((seat) => {
          const tank = view.tanks[seat];
          const seatInfo = room.seats[seat];
          if (!tank) return null;
          const respawn =
            !tank.alive && tank.respawnAt > 0
              ? Math.max(1, Math.ceil((tank.respawnAt - serverNow) / 1000))
              : 0;
          return (
            <div
              key={seat}
              className={`bc-player ${viewer === seat ? "is-you" : ""} ${
                tank.alive ? "" : "is-dead"
              }`}
              data-battlecity-seat={seat}
              data-battlecity-seq={tank.seq}
              data-battlecity-x={tank.x}
              data-battlecity-y={tank.y}
              data-battlecity-alive={tank.alive ? "1" : "0"}
            >
              <div className="bc-player-name">
                <span>{seatInfo?.name ?? `玩家 ${seat + 1}`}</span>
                {seatInfo?.autoPlay && <small>托管</small>}
                {viewer === seat && <small>你</small>}
              </div>
              <div className="bc-player-class">
                {tank.class ? TANK_NAMES[tank.class] : "选择中"}
                <span className="bc-hp">{hpPips(tank)}</span>
              </div>
              <div className="bc-player-stats">
                <span>
                  {tank.alive
                    ? `装甲 ${tank.hp}/${tank.maxHp}`
                    : `重生 ${respawn}s`}
                </span>
                <span>击杀 {tank.kills}</span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function SelectPanel({
  view,
  room,
  atlas,
  canSelect,
  onSelect,
  serverNow,
}: {
  view: View;
  room: BoardProps<View>["room"];
  atlas: Atlas | null;
  canSelect: boolean;
  onSelect: (tank: TankClass) => void;
  serverNow: number;
}) {
  const you = room.you;
  const myTank = you >= 0 ? view.tanks[you] : undefined;
  const myTeam = myTank?.team ?? 0;
  const counts = new Map<TankClass, number>();
  for (const tank of view.tanks) {
    if (tank.class) counts.set(tank.class, (counts.get(tank.class) ?? 0) + 1);
  }
  const left = Math.max(0, Math.ceil((view.selectUntil - serverNow) / 1000));
  return (
    <div className="bc-select">
      <div className="bc-select-map">
        <MapPreview mapId={view.mapId} />
        <div className="bc-select-map-caption">
          <strong>{MAPS.find((m) => m.id === view.mapId)?.name}</strong>
          <span>选择截止 {left}s</span>
        </div>
      </div>
      <div className="bc-select-main">
        <div className="bc-select-head">
          <strong>选择坦克</strong>
          <span>
            {myTank?.selected ? "已选择，等待队友" : "双方可重复选择"}
          </span>
        </div>
        <div className="bc-tank-cards">
          {TANK_ORDER.map((tankClass) => {
            const spec = TANK_STATS[tankClass];
            const chosen = myTank?.class === tankClass;
            const disabled =
              !canSelect ||
              !!myTank?.selected ||
              room.seats[you]?.autoPlay ||
              room.spectating;
            return (
              <button
                key={tankClass}
                type="button"
                className={`bc-tank-card ${chosen ? "is-chosen" : ""}`}
                disabled={disabled}
                onClick={() => onSelect(tankClass)}
                data-battlecity-tank={tankClass}
              >
                <TankIcon atlas={atlas} tankClass={tankClass} team={myTeam} />
                <strong>{TANK_NAMES[tankClass]}</strong>
                <span className="bc-tank-blurb">{spec.blurb}</span>
                <span className="bc-tank-meta">
                  速度 {spec.speed} · 装甲 {spec.hp} · 弹速 {spec.bullet}
                </span>
                <span className="bc-tank-count">
                  已选 {counts.get(tankClass) ?? 0}
                </span>
              </button>
            );
          })}
        </div>
        <div className="bc-choice-lists">
          {[0, 1].map((team) => (
            <div key={team} className={`bc-choice-team team-${team}`}>
              <strong>{TEAM_NAMES[team]}</strong>
              {Array.from({ length: view.teamSize }, (_, i) => {
                const seat = team * view.teamSize + i;
                const tank = view.tanks[seat];
                return (
                  <span key={seat}>
                    {room.seats[seat]?.name ?? `玩家 ${seat + 1}`}：
                    {tank?.class ? TANK_NAMES[tank.class] : "选择中"}
                  </span>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function TouchPad({
  enabled,
  onMove,
  onFire,
}: {
  enabled: boolean;
  onMove: (move: Move) => void;
  onFire: (fire: boolean) => void;
}) {
  const pointer = useRef<number | null>(null);
  const update = (event: ReactPointerEvent<HTMLDivElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    onMove(padDirection(event.clientX, event.clientY, rect));
  };
  const down = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!enabled || event.button !== 0) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    pointer.current = event.pointerId;
    update(event);
  };
  const move = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (pointer.current !== event.pointerId) return;
    event.preventDefault();
    update(event);
  };
  const end = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (pointer.current !== event.pointerId) return;
    pointer.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
    onMove("none");
  };
  const firePointer = useRef<number | null>(null);
  return (
    <div className="bc-touch-controls" aria-label="触屏操作" data-game-controls>
      <div
        className="bc-dpad"
        role="group"
        aria-label="方向控制"
        data-battlecity-dpad="1"
        onPointerDown={down}
        onPointerMove={move}
        onPointerUp={end}
        onPointerCancel={end}
        onPointerLeave={end}
        onContextMenu={(event) => event.preventDefault()}
      >
        <span className="bc-dpad-up">▲</span>
        <span className="bc-dpad-left">◀</span>
        <span className="bc-dpad-center" />
        <span className="bc-dpad-right">▶</span>
        <span className="bc-dpad-down">▼</span>
      </div>
      <button
        type="button"
        className="bc-fire"
        disabled={!enabled}
        aria-label="开火"
        data-battlecity-fire="1"
        onPointerDown={(event) => {
          if (!enabled || event.button !== 0) return;
          event.preventDefault();
          event.currentTarget.setPointerCapture(event.pointerId);
          firePointer.current = event.pointerId;
          onFire(true);
        }}
        onPointerUp={(event) => {
          if (firePointer.current !== event.pointerId) return;
          firePointer.current = null;
          if (event.currentTarget.hasPointerCapture(event.pointerId))
            event.currentTarget.releasePointerCapture(event.pointerId);
          onFire(false);
        }}
        onPointerCancel={() => {
          firePointer.current = null;
          onFire(false);
        }}
        onPointerLeave={() => {
          firePointer.current = null;
          onFire(false);
        }}
        onContextMenu={(event) => event.preventDefault()}
      >
        开火
      </button>
    </div>
  );
}

export function Board(props: BoardProps<View>) {
  const { room, data, act, busy, assets, sound, audio, clock } = props;
  const view = data as View | undefined;
  const seat = room.you;
  const spectator = seat < 0 || !!room.spectating;
  const myTank = seat >= 0 ? view?.tanks?.[seat] : undefined;
  const mySeat = seat >= 0 ? room.seats[seat] : undefined;
  const [error, setError] = useState("");
  const [suspended, setSuspended] = useState(false);
  const [, setTick] = useState(0);
  const atlas = useAtlas(assets);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const inputRef = useRef(new LatestInput());
  const motion = useRef(new TankMotion());
  const world = useRef<TankWorld | null>(null);
  const sending = useRef(false);
  const latest = useRef(props);
  latest.current = props;
  const prevView = useRef<View | null>(null);
  const curView = useRef<View | null>(null);
  const receivedAt = useRef(0);
  const seen = useRef({ scope: "", id: 0 });
  const resultSeen = useRef("");
  const heldMoves = useRef<Move[]>([]);
  const keysFire = useRef(new Set<string>());
  const touchMove = useRef<Move>("none");
  const touchFire = useRef(false);
  const touchTimers = useRef<number[]>([]);
  const suspendedRef = useRef(false);
  const scope = view
    ? `${room.id}:${view.epoch}:${clock.epoch ?? 0}:${seat}`
    : "";
  const scopeRef = useRef(scope);
  scopeRef.current = scope;
  const enabled =
    !spectator &&
    !mySeat?.autoPlay &&
    !busy &&
    room.status === "playing" &&
    view?.phase === "playing" &&
    !!myTank?.alive &&
    !suspended;
  const enabledRef = useRef(enabled);
  enabledRef.current = enabled;
  suspendedRef.current = suspended;

  useEffect(() => {
    const id = window.setInterval(() => setTick((value) => value + 1), 100);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    if (!view) return;
    prevView.current = curView.current;
    curView.current = view;
    receivedAt.current = performance.now();
    world.current = {
      cells: mapById(view.mapId)?.cells ?? [],
      broken: decodeBroken(view.broken),
      shovelUntil: view.shovelUntil,
      tanks: view.tanks,
      speed: TANK_STATS[view.tanks[seat]?.class ?? "balanced"].speed,
    };
  }, [view]);

  useEffect(() => {
    if (!view) return;
    inputRef.current.reset(myTank?.seq ?? 0);
    heldMoves.current = [];
    touchTimers.current.forEach(clearTimeout);
    touchTimers.current = [];
    keysFire.current.clear();
    touchMove.current = "none";
    touchFire.current = false;
    setSuspended(false);
    setError("");
    const max = (view.events ?? []).reduce(
      (value, event) => Math.max(value, event.id),
      0,
    );
    seen.current = { scope, id: max };
  }, [scope]);

  useEffect(() => {
    if (!view) return;
    inputRef.current.sync({
      scope,
      epoch: view.epoch,
      seq: myTank?.seq ?? 0,
      enabled,
    });
    if (!enabled) inputRef.current.release();
  }, [scope, view?.epoch, myTank?.seq, enabled]);

  const serverNow =
    (view?.serverAt ?? clock.serverNow) +
    Math.max(0, performance.now() - (clock.receivedAt || performance.now()));

  const sendOne = useCallback((force: boolean) => {
    if (
      !enabledRef.current ||
      suspendedRef.current ||
      (sending.current && (latest.current.inputWindow ?? 1) <= 1) ||
      document.hidden
    )
      return;
    const request = inputRef.current.next(
      performance.now(),
      force,
      latest.current.inputWindow ?? 1,
    );
    if (!request) return;
    sending.current = true;
    const sentScope = scopeRef.current;
    const p = latest.current;
    const { token, ...action } = request;
    void p
      .act(action)
      .then(() => inputRef.current.complete(token))
      .catch(() => {
        if (scopeRef.current !== sentScope) return;
        const current = p.data as View | undefined;
        const seq =
          p.room.you >= 0 ? (current?.tanks?.[p.room.you]?.seq ?? 0) : 0;
        inputRef.current.reset(seq);
        setSuspended(true);
        setError("操作同步失败，请点击重新同步");
      })
      .finally(() => {
        sending.current = false;
      });
  }, []);

  const applyControls = useCallback(() => {
    const move =
      touchMove.current !== "none"
        ? touchMove.current
        : (heldMoves.current[heldMoves.current.length - 1] ?? "none");
    const fire = touchFire.current || keysFire.current.size > 0;
    inputRef.current.set({ move, fire } as Control);
    // A quick keyboard or touch tap is sent immediately; the interval below
    // only handles throttled changes, heartbeats and post-flight releases.
    sendOne(true);
  }, [sendOne]);

  const clearTouchTimers = useCallback(() => {
    for (const id of touchTimers.current) window.clearTimeout(id);
    touchTimers.current = [];
  }, []);

  const setMove = useCallback(
    (move: Move) => {
      clearTouchTimers();
      touchMove.current = move;
      applyControls();
      if (move === "none") {
        // A quick tap must survive the 40 ms sender interval; holding a
        // direction still updates immediately and releases after this latch.
        touchTimers.current.push(
          window.setTimeout(() => {
            touchMove.current = "none";
            applyControls();
          }, 120),
        );
      }
    },
    [applyControls, clearTouchTimers],
  );
  const setFire = useCallback(
    (fire: boolean) => {
      clearTouchTimers();
      touchFire.current = fire;
      applyControls();
      if (!fire) {
        touchTimers.current.push(
          window.setTimeout(() => {
            touchFire.current = false;
            applyControls();
          }, 120),
        );
      }
    },
    [applyControls, clearTouchTimers],
  );

  useEffect(() => {
    if (enabled) applyControls();
    else inputRef.current.release();
  }, [enabled, applyControls]);

  useEffect(() => () => clearTouchTimers(), [clearTouchTimers]);

  useEffect(() => {
    const isEnabled = () => enabledRef.current;
    const clear = () => {
      heldMoves.current = [];
      keysFire.current.clear();
      touchMove.current = "none";
      touchFire.current = false;
      clearTouchTimers();
      inputRef.current.release();
    };
    const down = (event: KeyboardEvent) => {
      if (!isEnabled() || isEditable(event.target)) return;
      const move = keyMove(event.key);
      if (move) {
        event.preventDefault();
        if (!heldMoves.current.includes(move)) heldMoves.current.push(move);
        applyControls();
      }
      if (isFireKey(event.key)) {
        event.preventDefault();
        keysFire.current.add(event.key);
        applyControls();
      }
    };
    const up = (event: KeyboardEvent) => {
      const move = keyMove(event.key);
      if (move) {
        heldMoves.current = heldMoves.current.filter((value) => value !== move);
        applyControls();
      }
      if (isFireKey(event.key)) {
        keysFire.current.delete(event.key);
        applyControls();
      }
    };
    const visibility = () => {
      if (document.hidden) clear();
    };
    window.addEventListener("keydown", down, { passive: false });
    window.addEventListener("keyup", up);
    window.addEventListener("blur", clear);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", clear);
      document.removeEventListener("visibilitychange", visibility);
      clear();
    };
  }, [applyControls, clearTouchTimers]);

  useEffect(() => {
    const id = window.setInterval(() => sendOne(false), 40);
    return () => window.clearInterval(id);
  }, [sendOne]);

  useEffect(() => {
    let raf = 0;
    const reduced = window.matchMedia?.(
      "(prefers-reduced-motion: reduce)",
    ).matches;
    const frame = () => {
      const current = curView.current;
      const ctx = canvasRef.current?.getContext("2d");
      if (current && ctx) {
        const alpha = reduced
          ? 1
          : Math.min(1, (performance.now() - receivedAt.current) / 110);
        const now =
          (latest.current.data as View | undefined)?.serverAt ??
          latest.current.clock.serverNow;
        const props = latest.current,
          own = current.tanks[props.room.you];
        let local: TankPose | undefined;
        if (own && world.current && !props.room.spectating) {
          local = motion.current.frame(
            current,
            props.room.you,
            inputRef.current.value.move,
            world.current,
            performance.now(),
            `${props.room.id}:${current.epoch}:${props.clock.epoch ?? 0}:${props.room.you}`,
            enabledRef.current && !document.hidden,
          );
          if (canvasRef.current) {
            canvasRef.current.dataset.renderX = String(local.x);
            canvasRef.current.dataset.renderY = String(local.y);
            canvasRef.current.dataset.serverX = String(own.x);
            canvasRef.current.dataset.serverY = String(own.y);
          }
        }
        drawBattlefield(
          ctx,
          current,
          prevView.current,
          alpha,
          atlas,
          latest.current.room.you,
          now +
            Math.max(0, performance.now() - latest.current.clock.receivedAt),
          local,
        );
      }
      raf = requestAnimationFrame(frame);
    };
    raf = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(raf);
  }, [atlas]);

  useEffect(() => {
    if (!sound || !view) {
      if (!sound) audio.stop();
      return;
    }
    if (view.phase === "countdown") {
      void audio.music(soundSources(assets, "bgm-start"));
    } else if (view.phase === "playing") {
      void audio.music(soundSources(assets, "bgm-battle"));
    } else {
      audio.stopMusic();
    }
    return () => audio.stopMusic();
  }, [view?.phase, sound, assets, audio]);

  useEffect(() => () => audio.stop(), [audio]);

  useEffect(() => {
    if (!sound || !view) return;
    const max = (view.events ?? []).reduce(
      (value, event) => Math.max(value, event.id),
      0,
    );
    if (seen.current.scope !== scope) {
      seen.current = { scope, id: max };
      return;
    }
    const fresh = (view.events ?? [])
      .filter((event) => event.id > seen.current.id)
      .sort((a, b) => a.id - b.id);
    if (!fresh.length) return;
    seen.current.id = Math.max(...fresh.map((event) => event.id));
    for (const event of fresh) {
      const key = eventSound(event.type);
      if (key) void audio.playWithFallback(soundSources(assets, key));
    }
  }, [view?.events, scope, sound, assets, audio]);

  useEffect(() => {
    const game = room.game;
    if (!sound || !game?.finished || spectator || !view) return;
    const key = `${room.id}:${view.epoch}`;
    if (resultSeen.current === key) return;
    resultSeen.current = key;
    if (game.winners.length === 0) return;
    void audio.playWithFallback(
      soundSources(
        assets,
        game.winners.includes(seat) ? "sfx-win" : "sfx-lose",
      ),
    );
  }, [room.game, room.id, sound, spectator, seat, assets, audio, view]);

  const selectTank = (tankClass: TankClass) => {
    if (!view || view.phase !== "select" || spectator || busy) return;
    void act({ type: "select", epoch: view.epoch, tank: tankClass }).catch(() =>
      setError("选择失败，请重试"),
    );
  };

  const canSelect =
    !!view &&
    view.phase === "select" &&
    !spectator &&
    !mySeat?.autoPlay &&
    !busy &&
    !!myTank &&
    !myTank.selected;

  const countdown = view
    ? Math.max(
        0,
        Math.ceil(
          ((view.phase === "select" ? view.selectUntil : view.startAt) -
            serverNow) /
            1000,
        ),
      )
    : 0;
  const remaining = view
    ? Math.max(0, Math.floor((view.matchEndsAt - serverNow) / 1000))
    : 0;
  const matchClock = `${Math.floor(remaining / 60)}:${String(
    remaining % 60,
  ).padStart(2, "0")}`;

  const statusText = useMemo(() => {
    if (!view) return "加载中";
    if (spectator) return "观战中";
    if (mySeat?.autoPlay) return "托管中";
    return phaseLabel(view.phase);
  }, [view, spectator, mySeat?.autoPlay]);

  return (
    <div
      className="battlecity-board"
      data-battlecity-phase={view?.phase ?? "loading"}
      data-battlecity-team-size={view?.teamSize ?? 0}
      data-battlecity-epoch={view?.epoch ?? ""}
      data-battlecity-atlas={atlas ? "1" : "0"}
    >
      <div className="bc-status">
        <strong>{statusText}</strong>
        {view && view.phase === "select" && <span>选择 {countdown}s</span>}
        {view && view.phase === "countdown" && <span>开战 {countdown}s</span>}
        {view && view.phase === "playing" && <span>剩余 {matchClock}</span>}
        <span className="bc-map-name">
          {MAPS.find((m) => m.id === view?.mapId)?.name ?? ""}
        </span>
      </div>
      {!view ? (
        <p className="bc-loading">正在连接坦克大战…</p>
      ) : (
        <>
          <TeamHud
            room={room}
            view={view}
            team={0}
            serverNow={serverNow}
            viewer={seat}
          />
          {view.phase === "select" ? (
            <SelectPanel
              view={view}
              room={room}
              atlas={atlas}
              canSelect={canSelect}
              onSelect={selectTank}
              serverNow={serverNow}
            />
          ) : (
            <div className="bc-field-wrap">
              <canvas
                ref={canvasRef}
                className="bc-canvas"
                width={208}
                height={208}
                role="img"
                aria-label="坦克大战战场"
                data-battlecity-canvas="1"
              />
              {view.phase === "countdown" && (
                <div
                  className="bc-overlay"
                  data-battlecity-countdown={countdown}
                >
                  <strong>{countdown > 0 ? `${countdown}` : "开战"}</strong>
                  <span>准备战斗</span>
                </div>
              )}
              {view.phase === "finished" && (
                <div className="bc-overlay is-result">
                  <strong>对局结束</strong>
                  <span>
                    {view.bases[0].alive && !view.bases[1].alive
                      ? "上方队伍摧毁了下方大本营"
                      : !view.bases[0].alive && view.bases[1].alive
                        ? "下方队伍摧毁了上方大本营"
                        : "双方大本营均未被摧毁"}
                  </span>
                </div>
              )}
            </div>
          )}
          <TeamHud
            room={room}
            view={view}
            team={1}
            serverNow={serverNow}
            viewer={seat}
          />
          {view.phase === "playing" && !spectator && (
            <div className="bc-controls">
              <div className="bc-keyboard-hint">
                键盘：WASD／方向键移动 · 空格／J 开火
                {mySeat?.autoPlay ? " · 当前由电脑托管" : ""}
              </div>
              <TouchPad enabled={enabled} onMove={setMove} onFire={setFire} />
            </div>
          )}
          {spectator && (
            <p className="bc-spectator-note">
              你正在观战，操作只对参赛席开放。
            </p>
          )}
        </>
      )}
      {error && (
        <div className="bc-error" role="alert">
          <span>{error}</span>
          <button
            type="button"
            onClick={() => {
              const current = latest.current.data as View | undefined;
              const seq =
                latest.current.room.you >= 0
                  ? (current?.tanks?.[latest.current.room.you]?.seq ?? 0)
                  : 0;
              inputRef.current.reset(seq);
              setSuspended(false);
              setError("");
            }}
          >
            重新同步
          </button>
        </div>
      )}
    </div>
  );
}
