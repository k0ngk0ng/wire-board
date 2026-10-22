import { MAX_ROAD_SPEED } from "./dynamics";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type {
  PointerEvent as ReactPointerEvent,
  KeyboardEvent as ReactKeyboardEvent,
  FocusEvent as ReactFocusEvent,
} from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { LatestDrive, neutral, wheelSteer, type Drive } from "./input";
import { TrackCanvas } from "./TrackCanvas";
import { RaceMap } from "./RaceMap";
import { RearViewHold } from "./chase-camera";
import { Modal } from "../../../web/src/ui/board-host";
import { ControlLifecycle } from "./lifecycle";
import { soundDelay, RaceAudioQueue } from "./timeline";
import { sources, skinNames, type View } from "./types";
import "./style.css";
const actionError = (error: unknown) =>
  error instanceof TypeError
    ? "网络连接中断，正在恢复控制"
    : error instanceof Error
      ? error.message
      : String(error);
const readSkin = () => {
  try {
    return Math.max(
      0,
      Math.min(5, Number(localStorage.getItem("racing.skin")) || 0),
    );
  } catch {
    return 0;
  }
};
const isTouch = () =>
  typeof matchMedia !== "undefined" && matchMedia("(pointer: coarse)").matches;
const timeText = (ms: number) => {
  const seconds = Math.max(0, Math.floor(ms / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
};
export function Board(props: BoardProps<View>) {
  const { room, data: d, act, busy, clock, assets, audio, sound } = props;
  const [mobile, setMobile] = useState(isTouch),
    [suspended, setSuspended] = useState(false),
    [drive, setDrive] = useState<Drive>({ ...neutral }),
    [error, setError] = useState(""),
    [now, setNow] = useState(performance.now()),
    [skin, setSkin] = useState(readSkin),
    [watch, setWatch] = useState(0),
    [rear, setRear] = useState(false),
    [mapOpen, setMapOpen] = useState(false),
    [reduced, setReduced] = useState(
      () => matchMedia("(prefers-reduced-motion: reduce)").matches,
    );
  const boardElement = useRef<HTMLElement>(null);
  const mapWasOpen = useRef(false);
  useEffect(() => {
    if (d.phase === "finished") setMapOpen(false);
  }, [d.phase]);
  useEffect(() => {
    if (
      mapWasOpen.current &&
      !mapOpen &&
      !document.querySelector('[role="dialog"]')
    )
      boardElement.current
        ?.querySelector<HTMLCanvasElement>(".racing-chase")
        ?.focus({ preventScroll: true });
    mapWasOpen.current = mapOpen;
  }, [mapOpen]);

  const rearHold = useRef(new RearViewHold());
  const queue = useRef(new LatestDrive()),
    lifecycle = useRef(new ControlLifecycle()),
    input = useRef<Drive>({ ...neutral }),
    keys = useRef(new Set<string>()),
    wheel = useRef<{ id: number; steer: number } | null>(null),
    throttles = useRef(new Set<number>()),
    brakes = useRef(new Set<number>()),
    drifts = useRef(new Set<number>()),
    pedalKeys = useRef({
      throttle: new Set<string>(),
      brake: new Set<string>(),
      drift: new Set<string>(),
    }),
    latest = useRef(props),
    paused = useRef(false),
    activeRef = useRef(false),
    skinSent = useRef("");
  latest.current = props;
  const car = d.cars.find((c) => c.seat === room.you),
    spectating = !!room.spectating || room.you < 0;
  const enabled =
    room.status === "playing" &&
    !spectating &&
    !mapOpen &&
    !room.seats[room.you]?.autoPlay &&
    !busy &&
    d.controls.canDrive &&
    !car?.finishAt;
  const scope = `${room.id}:${d.epoch}:${clock.epoch ?? 0}:${room.you}`;
  activeRef.current = enabled;
  useLayoutEffect(() => {
    queue.current.sync({
      scope,
      identity: `${room.id}:${d.epoch}:${room.you}`,
      epoch: d.epoch,
      seq: car?.seq ?? 0,
      enabled,
    });
    const wasPaused = paused.current;
    paused.current = lifecycle.current.sync({
      race: `${room.id}:${d.epoch}:${room.you}`,
      transport: clock.epoch ?? 0,
      enabled,
      hidden: document.hidden,
      phase: d.phase,
      snapshotAge:
        clock.serverNow + performance.now() - clock.receivedAt - d.serverAt,
    });
    setSuspended(paused.current);
    if (paused.current && !wasPaused) release();
    input.current = queue.current.value;
  }, [scope, d.epoch, car?.seq, enabled]);
  const setInput = (next: Drive) => {
    queue.current.set(next);
    input.current = queue.current.value;
    setDrive({ ...input.current });
  };
  const compute = () => {
    if (!activeRef.current || paused.current) {
      setInput({ ...neutral });
      return;
    }
    const held = keys.current;
    const braking =
      brakes.current.size > 0 ||
      pedalKeys.current.brake.size > 0 ||
      held.has("ArrowDown") ||
      held.has("KeyS");
    setInput({
      steer:
        wheel.current?.steer ??
        (held.has("ArrowRight") || held.has("KeyD") ? 1 : 0) -
          (held.has("ArrowLeft") || held.has("KeyA") ? 1 : 0),
      throttle:
        !braking &&
        (throttles.current.size > 0 ||
          pedalKeys.current.throttle.size > 0 ||
          held.has("ArrowUp") ||
          held.has("KeyW"))
          ? 1
          : 0,
      brake: braking,
      drift:
        drifts.current.size > 0 ||
        pedalKeys.current.drift.size > 0 ||
        held.has("Space"),
    });
  };
  const release = () => {
    keys.current.clear();
    rearHold.current.clear();
    setRear(false);
    throttles.current.clear();
    pedalKeys.current.throttle.clear();
    wheel.current = null;
    brakes.current.clear();
    drifts.current.clear();
    pedalKeys.current.brake.clear();
    pedalKeys.current.drift.clear();
    setInput({ ...neutral });
  };
  const suspendInput = (networkFailure = false) => {
    if (networkFailure) lifecycle.current.networkFailed();
    else lifecycle.current.suspend();
    paused.current = true;
    setSuspended(true);
    release();
    latest.current.audio.stopLoop("racing-engine");
    latest.current.audio.stopLoop("racing-tires");
  };
  const wake = (touch = false) => {
    const p = latest.current;
    paused.current = lifecycle.current.sync({
      race: `${p.room.id}:${p.data.epoch}:${p.room.you}`,
      transport: p.clock.epoch ?? 0,
      enabled: activeRef.current,
      hidden: document.hidden,
      phase: p.data.phase,
      snapshotAge:
        p.clock.serverNow +
        performance.now() -
        p.clock.receivedAt -
        p.data.serverAt,
    });
    if (touch && !mobile) {
      setMobile(true);
    }
    if (paused.current) {
      paused.current = lifecycle.current.wake();
      setSuspended(paused.current);
    }
  };
  useEffect(() => {
    release();
    compute();
  }, [scope, enabled]);
  useEffect(() => {
    const query = matchMedia("(prefers-reduced-motion: reduce)"),
      change = () => setReduced(query.matches);
    query.addEventListener("change", change);
    return () => query.removeEventListener("change", change);
  }, []);
  useEffect(() => {
    const id = setInterval(() => {
      const time = performance.now(),
        p = latest.current;
      setNow(time);
      const snapshotAge =
        p.clock.serverNow + time - p.clock.receivedAt - p.data.serverAt;
      if (activeRef.current && !paused.current && snapshotAge > 1000) {
        suspendInput(true);
        setError("赛况同步中断，操作已暂停；连接恢复后请重新操作。");
      }
      const r = queue.current.next(time, p.inputWindow ?? 1);
      if (!r) return;
      const { token, ...action } = r;
      const sentScope = `${p.room.id}:${p.data.epoch}:${p.clock.epoch ?? 0}:${p.room.you}`;
      void latest.current
        .act(action)
        .then(() => setError(""))
        .catch((e) => {
          const current = latest.current;
          if (
            sentScope !==
            `${current.room.id}:${current.data.epoch}:${current.clock.epoch ?? 0}:${current.room.you}`
          )
            return;
          suspendInput(true);
          setError(actionError(e));
        })
        .finally(() => queue.current.complete(token));
    }, 40);
    return () => clearInterval(id);
  }, []);
  useEffect(() => {
    const keyboard = new Set([
      "KeyW",
      "KeyA",
      "KeyS",
      "KeyD",
      "ArrowUp",
      "ArrowDown",
      "ArrowLeft",
      "ArrowRight",
      "Space",
    ]);
    const ignored = (target: EventTarget | null) =>
      target instanceof HTMLElement &&
      (!!target.closest("input,select,textarea,[role=dialog]") ||
        target.isContentEditable);
    const down = (e: KeyboardEvent) => {
      if (ignored(e.target) || e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.repeat && (paused.current || !keys.current.has(e.code))) {
        if (keyboard.has(e.code) || e.code === "KeyB" || e.code === "KeyR")
          e.preventDefault();
        return;
      }
      if (e.code === "KeyB") {
        if (mapOpen) return;
        e.preventDefault();
        keys.current.add("KeyB");
        rearHold.current.press("KeyB");
        setRear(rearHold.current.active);
        return;
      }
      if (!activeRef.current) return;
      if (e.code === "KeyR" && !e.repeat) {
        e.preventDefault();
        void reset();
        return;
      }
      if (!keyboard.has(e.code)) return;
      if (e.code === "Space" && e.target instanceof HTMLButtonElement) return;
      e.preventDefault();
      if (
        e.target instanceof HTMLButtonElement &&
        !e.target.closest("[data-game-controls]")
      ) {
        boardElement.current
          ?.querySelector<HTMLCanvasElement>(".racing-chase")
          ?.focus({ preventScroll: true });
      }
      wake();
      keys.current.add(e.code);
      compute();
    };
    const up = (e: KeyboardEvent) => {
      if (e.code === "KeyB") {
        keys.current.delete("KeyB");
        rearHold.current.release("KeyB");
        setRear(rearHold.current.active);
        return;
      }
      if (!keyboard.has(e.code)) return;
      keys.current.delete(e.code);
      compute();
    };
    const suspend = () => {
      suspendInput();
    };
    const visibility = () => {
      if (document.hidden) suspend();
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    window.addEventListener("blur", suspend);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
      window.removeEventListener("blur", suspend);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, [scope, enabled, mobile, mapOpen]);
  useEffect(() => {
    if (!enabled || !d.controls.canSkin || skinSent.current === scope) return;
    skinSent.current = scope;
    if (skin !== car?.skin)
      void act({ type: "skin", epoch: d.epoch, skin }).catch((e) =>
        setError(actionError(e)),
      );
  }, [scope, enabled, d.controls.canSkin]);
  async function reset() {
    const p = latest.current;
    if (!p.data.controls.canReset) return;
    try {
      await p.act({ type: "reset", epoch: p.data.epoch });
      setError("");
    } catch (e) {
      setError(actionError(e));
    }
  }
  const seen = useRef({ scope, id: Math.max(0, ...d.events.map((e) => e.id)) }),
    eventTimers = useRef(new RaceAudioQueue());
  useEffect(() => {
    if (seen.current.scope !== scope) {
      seen.current = { scope, id: Math.max(0, ...d.events.map((e) => e.id)) };
      eventTimers.current.clear();
      return;
    }
    for (const event of d.events) {
      if (event.id <= seen.current.id) continue;
      seen.current.id = event.id;
      if (!sound || document.hidden) continue;
      let file = "";
      if (event.type === "start") file = "start";
      else if (event.type === "collision")
        file = event.intensity > 0.45 ? "collision-heavy" : "collision-light";
      else if (event.type === "lap" && (event.seat === room.you || spectating))
        file = "lap";
      else if (
        event.type === "finish" &&
        (event.seat === room.you || spectating)
      )
        file = "finish";
      if (file) {
        const delay = soundDelay(
          event.at,
          clock.serverNow + performance.now() - clock.receivedAt,
          enabled && !suspended,
        );
        if (delay !== null)
          eventTimers.current.enqueue(() => {
            if (!document.hidden)
              void audio.playWithFallback(sources(assets, file + ".mp3"));
          }, delay);
      }
    }
  }, [d.events, scope, sound]);
  useEffect(() => {
    if (!sound || suspended) eventTimers.current.clear();
  }, [sound, suspended]);
  useEffect(
    () => () => {
      eventTimers.current.clear();
      audio.stopLoop("racing-engine");
      audio.stopLoop("racing-tires");
    },
    [audio],
  );
  const serverNow = clock.serverNow + now - clock.receivedAt,
    countdown = Math.max(0, Math.ceil((d.startAt - serverNow) / 1000)),
    lastCount = useRef(countdown);
  useEffect(() => {
    if (
      countdown > 0 &&
      countdown < lastCount.current &&
      sound &&
      !document.hidden
    )
      void audio.playWithFallback(sources(assets, "countdown.mp3"));
    lastCount.current = countdown;
  }, [countdown, sound]);
  useEffect(() => {
    const followed = spectating ? d.cars.find((c) => c.seat === watch) : car;
    const running =
      sound &&
      !document.hidden &&
      !suspended &&
      d.phase !== "finished" &&
      followed &&
      !followed.finishAt;
    if (!running) {
      audio.stopLoop("racing-engine");
      audio.stopLoop("racing-tires");
      return;
    }
    const speed = Math.min(1, followed.speed / MAX_ROAD_SPEED);
    void audio.loop("racing-engine", sources(assets, "engine.mp3"), {
      volume: 0.12 + speed * 0.16,
      rate: 0.72 + speed * 0.7,
    });
    if ((spectating ? followed.drift : drive.drift) && followed.speed > 70)
      void audio.loop("racing-tires", sources(assets, "tires.mp3"), {
        volume: 0.12 + speed * 0.12,
        rate: 0.9 + speed * 0.12,
      });
    else audio.stopLoop("racing-tires");
  }, [now, sound, suspended, d.phase, car?.speed, drive.drift, watch]);
  const wheelStart = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (!enabled || wheel.current || e.button !== 0) return;
    e.preventDefault();
    wake(e.pointerType === "touch");
    e.currentTarget.setPointerCapture(e.pointerId);
    const rect = e.currentTarget.getBoundingClientRect();
    wheel.current = {
      id: e.pointerId,
      steer: wheelSteer(e.clientX, rect.left, rect.width),
    };
    compute();
  };
  const wheelMove = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (wheel.current?.id !== e.pointerId) return;
    const rect = e.currentTarget.getBoundingClientRect();
    wheel.current.steer = wheelSteer(e.clientX, rect.left, rect.width);
    compute();
  };
  const wheelEnd = (e: ReactPointerEvent<HTMLDivElement>) => {
    if (wheel.current?.id !== e.pointerId) return;
    wheel.current = null;
    compute();
  };
  const heldButton = (which: "throttle" | "brake" | "drift") => ({
    onKeyDown: (e: ReactKeyboardEvent<HTMLButtonElement>) => {
      if (!enabled || (e.code !== "Space" && e.code !== "Enter")) return;
      e.preventDefault();
      e.stopPropagation();
      if (e.repeat && !pedalKeys.current[which].has(e.code)) return;
      wake();
      pedalKeys.current[which].add(e.code);
      compute();
    },
    onKeyUp: (e: ReactKeyboardEvent<HTMLButtonElement>) => {
      if (e.code !== "Space" && e.code !== "Enter") return;
      e.preventDefault();
      e.stopPropagation();
      pedalKeys.current[which].delete(e.code);
      compute();
    },
    onBlur: (_e: ReactFocusEvent<HTMLButtonElement>) => {
      pedalKeys.current[which].clear();
      compute();
    },
    onPointerDown: (e: ReactPointerEvent<HTMLButtonElement>) => {
      if (!enabled || e.button !== 0) return;
      e.preventDefault();
      wake(e.pointerType === "touch");
      e.currentTarget.setPointerCapture(e.pointerId);
      (which === "throttle"
        ? throttles
        : which === "brake"
          ? brakes
          : drifts
      ).current.add(e.pointerId);
      compute();
    },
    onPointerUp: (e: ReactPointerEvent<HTMLButtonElement>) => {
      (which === "throttle"
        ? throttles
        : which === "brake"
          ? brakes
          : drifts
      ).current.delete(e.pointerId);
      compute();
    },
    onPointerCancel: (e: ReactPointerEvent<HTMLButtonElement>) => {
      (which === "throttle"
        ? throttles
        : which === "brake"
          ? brakes
          : drifts
      ).current.delete(e.pointerId);
      compute();
    },
    onLostPointerCapture: (e: ReactPointerEvent<HTMLButtonElement>) => {
      (which === "throttle"
        ? throttles
        : which === "brake"
          ? brakes
          : drifts
      ).current.delete(e.pointerId);
      compute();
    },
  });
  const finishMs = d.finishDeadline
    ? Math.max(0, d.finishDeadline - serverNow)
    : Math.max(0, d.maxRaceAt - serverNow);
  const status =
    d.phase === "finished"
      ? "比赛结束"
      : car?.finishAt
        ? "已冲线，等待比赛结束"
        : d.phase === "countdown"
          ? `发车倒计时 ${countdown}`
          : d.phase === "finishing"
            ? `完赛倒计时 ${Math.ceil(finishMs / 1000)} 秒`
            : spectating
              ? "正在观战"
              : room.seats[room.you]?.autoPlay
                ? "电脑正在托管"
                : car && car.resetUntil > serverNow
                  ? `重置中 · ${Math.ceil((car.resetUntil - serverNow) / 1000)} 秒后起步`
                  : suspended
                    ? "操作已释放，点击方向盘或按键继续"
                    : "比赛进行中";
  return (
    <section
      ref={boardElement}
      className="racing-board"
      data-phase={d.phase}
      data-epoch={d.epoch}
      data-tick={d.serverAt}
    >
      <div className="racing-hud">
        <div className="racing-track-title">
          <button
            type="button"
            className="racing-room-link"
            aria-label="车手与房间"
            onClick={() =>
              window.scrollTo({ top: 0, behavior: reduced ? "auto" : "smooth" })
            }
          >
            ↑ 车手／房间
          </button>
          <strong>{d.track.name}</strong>
        </div>
        <div>
          <span>名次</span>
          <strong>{car ? `${car.rank} / ${d.cars.length}` : "观战"}</strong>
        </div>
        <div>
          <span>圈数</span>
          <strong>
            {car ? Math.min(car.lap + 1, d.options.laps) : "—"} /{" "}
            {d.options.laps}
          </strong>
        </div>
        <div>
          <span>用时</span>
          <strong>
            {timeText(
              (car?.finishAt ||
                (d.phase === "finished" ? d.serverAt : serverNow)) - d.startAt,
            )}
          </strong>
        </div>
      </div>
      <div
        className={`racing-status ${d.phase === "finishing" ? "is-urgent" : ""}`}
        role="status"
      >
        <span>{status}</span>
        <span>
          {d.phase === "racing"
            ? `剩余 ${timeText(finishMs)}`
            : car?.offroad
              ? "路外减速"
              : ""}
        </span>
      </div>
      <div className="racing-stage">
        <TrackCanvas
          onReady={props.onReady}
          onLoadError={props.onLoadError}
          data={d}
          clock={clock}
          seat={spectating ? -1 : room.you}
          assets={assets}
          input={input}
          driving={enabled && !suspended}
          reduced={reduced}
          watch={watch}
          rear={rear}
          onOpenMap={() => {
            release();
            setMapOpen(true);
          }}
        />
        {d.phase === "countdown" && (
          <div
            className="racing-start-lights"
            aria-label={`发车倒计时 ${countdown}`}
          >
            <div>
              {[3, 2, 1].map((n) => (
                <i key={n} className={countdown <= n ? "lit" : ""} />
              ))}
            </div>
            <strong>{countdown || "出发"}</strong>
            <small>准备起步</small>
          </div>
        )}
      </div>
      {mapOpen && d.phase !== "finished" && (
        <Modal
          title="赛道地图"
          className="racing-map-modal"
          onClose={() => setMapOpen(false)}
        >
          <p className="racing-map-note">
            比赛继续进行。拖动平移，滚轮或双指缩放。
          </p>
          <RaceMap
            data={d}
            clock={clock}
            seat={spectating ? -1 : room.you}
            assets={assets}
            input={input}
            driving={false}
            reduced={reduced}
            watch={watch}
          />
        </Modal>
      )}
      <div className="racing-view-controls" data-game-controls>
        <button
          type="button"
          aria-label="向后看"
          aria-pressed={rear}
          onPointerDown={(e) => {
            e.preventDefault();
            e.currentTarget.setPointerCapture(e.pointerId);
            rearHold.current.press(`pointer:${e.pointerId}`);
            setRear(rearHold.current.active);
          }}
          onPointerUp={(e) => {
            rearHold.current.release(`pointer:${e.pointerId}`);
            setRear(rearHold.current.active);
          }}
          onPointerCancel={(e) => {
            rearHold.current.release(`pointer:${e.pointerId}`);
            setRear(rearHold.current.active);
          }}
          onLostPointerCapture={(e) => {
            rearHold.current.release(`pointer:${e.pointerId}`);
            setRear(rearHold.current.active);
          }}
          onKeyDown={(e) => {
            if (e.key === " " || e.key === "Enter") {
              e.preventDefault();
              rearHold.current.press("button");
              setRear(true);
            }
          }}
          onKeyUp={(e) => {
            if (e.key === " " || e.key === "Enter") {
              rearHold.current.release("button");
              setRear(rearHold.current.active);
            }
          }}
          onBlur={() => {
            rearHold.current.release("button");
            setRear(rearHold.current.active);
          }}
        >
          向后看 <kbd>B</kbd>
        </button>
        <span>自动挡 · 无需换挡</span>
        <button
          type="button"
          aria-label="重置赛车"
          disabled={!d.controls.canReset || !enabled}
          onClick={() => void reset()}
        >
          {car && car.resetUntil > 0 && car.resetUntil + 3000 > serverNow
            ? `重置 ${Math.ceil((car.resetUntil + 3000 - serverNow) / 1000)}秒`
            : "重置"}{" "}
          <kbd>R</kbd>
        </button>
      </div>
      {spectating ? (
        <label className="racing-watch">
          跟随车辆
          <select
            aria-label="跟随车辆"
            value={watch}
            onChange={(e) => setWatch(+e.target.value)}
          >
            {d.cars.map((c) => (
              <option key={c.seat} value={c.seat}>
                {c.seat + 1} 号 · {room.seats[c.seat]?.name}
              </option>
            ))}
          </select>
        </label>
      ) : (
        <>
          <div
            className={`racing-controls ${mobile ? "is-touch" : ""}`}
            data-game-controls
            data-steer={drive.steer}
            data-throttle={drive.throttle}
            data-brake={drive.brake}
            data-drift={drive.drift}
          >
            <div
              className="racing-wheel"
              role="slider"
              aria-label="方向盘"
              aria-valuemin={-100}
              aria-valuemax={100}
              aria-valuenow={Math.round(drive.steer * 100)}
              aria-valuetext={
                drive.steer < -0.07
                  ? "向左转向"
                  : drive.steer > 0.07
                    ? "向右转向"
                    : "直行"
              }
              aria-disabled={!enabled}
              tabIndex={enabled ? 0 : -1}
              onPointerDown={wheelStart}
              onPointerMove={wheelMove}
              onPointerUp={wheelEnd}
              onPointerCancel={wheelEnd}
              onLostPointerCapture={wheelEnd}
            >
              <svg
                viewBox="0 0 120 120"
                aria-hidden="true"
                style={{ transform: `rotate(${drive.steer * 100}deg)` }}
              >
                <circle
                  cx="60"
                  cy="60"
                  r="47"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="12"
                />
                <path
                  d="M17 52H103M60 59V106"
                  stroke="currentColor"
                  strokeWidth="10"
                />
                <circle
                  cx="60"
                  cy="58"
                  r="16"
                  fill="#d6e3db"
                  stroke="currentColor"
                  strokeWidth="5"
                />
                <path d="M60 9v8" stroke="#c17643" strokeWidth="6" />
              </svg>
              <small>滑动转向</small>
            </div>
            <div className="racing-drive-info">
              <span className="racing-key-hint">
                WASD／方向键
                <br />
                空格手刹 · B 后视 · R 重置
              </span>
            </div>
            <div className="racing-pedals">
              <button
                type="button"
                className={`racing-drift ${drive.drift ? "pressed" : ""}`}
                aria-label="手刹"
                aria-pressed={drive.drift}
                disabled={!enabled}
                {...heldButton("drift")}
              >
                手刹<small>空格</small>
              </button>
              <button
                type="button"
                className={drive.brake ? "pressed" : ""}
                aria-label="刹车"
                aria-pressed={drive.brake}
                disabled={!enabled}
                {...heldButton("brake")}
              >
                刹车<small>S／↓</small>
              </button>
              <button
                type="button"
                aria-label="油门"
                aria-pressed={drive.throttle > 0}
                className={drive.throttle > 0 ? "pressed" : ""}
                disabled={!enabled}
                {...heldButton("throttle")}
              >
                油门<small>W／↑</small>
              </button>
            </div>
          </div>
          <div className="racing-settings">
            <label>
              车辆外观
              <select
                aria-label="车辆外观"
                value={skin}
                disabled={
                  busy ||
                  spectating ||
                  (!d.controls.canSkin && d.phase !== "finished")
                }
                onChange={(e) => {
                  const s = +e.target.value;
                  setSkin(s);
                  try {
                    localStorage.setItem("racing.skin", String(s));
                  } catch {}
                  if (d.controls.canSkin)
                    void act({ type: "skin", epoch: d.epoch, skin: s }).catch(
                      (e) => setError(actionError(e)),
                    );
                }}
              >
                {skinNames.map((name, i) => (
                  <option key={i} value={i}>
                    {name}
                  </option>
                ))}
              </select>
            </label>

            <span>车型性能相同 · 重置前先减速</span>
          </div>
        </>
      )}
      {error && (
        <p className="racing-error" role="alert">
          {error}
        </p>
      )}
      {clock.serverNow + now - clock.receivedAt - d.serverAt > 1000 &&
        d.phase !== "finished" && (
          <p className="racing-network" role="status">
            正在同步赛况，松开操作可安全减速。
          </p>
        )}
    </section>
  );
}
