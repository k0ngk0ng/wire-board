import { useGameController } from "../../../web/src/controllers/use-game-controller";
import { MAX_ROAD_SPEED } from "./dynamics";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type {
  PointerEvent as ReactPointerEvent,
  KeyboardEvent as ReactKeyboardEvent,
  FocusEvent as ReactFocusEvent,
} from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { LatestDrive, neutral, type Drive } from "./input";
import {
  RacingTouchContacts,
  type RacingTouchAction,
} from "./racing-touch-contacts";
import { TrackCanvas } from "./TrackCanvas";
import { RaceMap } from "./RaceMap";
import { RearViewHold } from "./chase-camera";
import { api, Modal } from "../../../web/src/ui/board-host";
import { ControlLifecycle } from "./lifecycle";
import { soundDelay, RaceAudioQueue } from "./timeline";
import { sources, skinNames, type View } from "./types";
import { loadClassicScene, type ClassicScenePack } from "./classic-loading";
import {
  CLASSIC_CAMERA_MODES,
  CLASSIC_DRAW_DISTANCES,
} from "./classic-camera-modes";
import { useClassicEngine } from "./use-classic-engine";
import { useClassicHorn } from "./use-classic-horn";
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
  return props.data.physics === 3 ? (
    <ClassicBoard {...props} />
  ) : (
    <RaceBoard {...props} />
  );
}
function ClassicBoard(props: BoardProps<View>) {
  const [pack, setPack] = useState<ClassicScenePack | null>(null);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const latest = useRef(props);
  latest.current = props;
  const key = `${props.data.epoch}:${props.data.track.geometry}:${props.data.options.timeOfDay ?? "day"}`;
  useEffect(() => {
    const controller = new AbortController();
    const p = latest.current;
    setPack(null);
    setError("");
    const followed = p.data.cars.find(
      (car) => car.seat === (p.room.you < 0 ? 0 : p.room.you),
    );
    void loadClassicScene(
      p.data,
      p.assets,
      controller.signal,
      (value) => {
        latest.current.onLoadProgress?.(
          value.total ? Math.min(96, (value.loaded / value.total) * 96) : 0,
          value.label,
          { loaded: value.loaded, total: value.total },
        );
      },
      p.sound ? followed?.vehicle : undefined,
    )
      .then((value) => {
        if (!controller.signal.aborted) setPack(value);
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        const message =
          cause instanceof Error ? cause.message : "赛车资源加载失败";
        setError(message);
        latest.current.onLoadError?.(message);
      });
    return () => controller.abort();
  }, [key, props.assets, attempt]);
  if (!pack)
    return (
      <div
        className="racing-board racing-resource-state"
        role={error ? "alert" : "status"}
      >
        <span>{error || "正在读取赛道与车型…"}</span>
        {error && (
          <button
            type="button"
            onClick={() => setAttempt((value) => value + 1)}
          >
            重新加载
          </button>
        )}
      </div>
    );
  return (
    <RaceBoard
      {...props}
      data={{ ...props.data, track: pack.track }}
      classicAssets={pack}
    />
  );
}
function RaceBoard(
  props: BoardProps<View> & { classicAssets?: ClassicScenePack },
) {
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
    [cameraMode, setCameraMode] = useState(0),
    [drawDistance, setDrawDistance] = useState(2),
    [hudVisible, setHudVisible] = useState(true),
    [mirrorVisible, setMirrorVisible] = useState(false),
    [viewSize, setViewSize] = useState(2),
    [focused, setFocused] = useState(() => document.hasFocus()),
    [viewerID, setViewerID] = useState(""),
    [reduced, setReduced] = useState(
      () => matchMedia("(prefers-reduced-motion: reduce)").matches,
    );
  const boardElement = useRef<HTMLElement>(null);
  useEffect(() => {
    const coarse = matchMedia("(pointer: coarse)");
    const update = () => setMobile(isTouch());
    update();
    coarse.addEventListener("change", update);
    window.addEventListener("resize", update);
    return () => {
      coarse.removeEventListener("change", update);
      window.removeEventListener("resize", update);
    };
  }, []);
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
    mapped = useRef(new Map<string, number>()),
    mappedPressed = useRef(new Set<string>()),
    pumpControls = useRef<() => void>(() => {}),
    steerPointers = useRef(new Map<number, -1 | 0 | 1>()),
    touchContacts = useRef(new RacingTouchContacts()),
    steerKeys = useRef({ left: new Set<string>(), right: new Set<string>() }),
    throttles = useRef(new Set<number>()),
    brakes = useRef(new Set<number>()),
    drifts = useRef(new Set<number>()),
    horns = useRef(new Set<number>()),
    pedalKeys = useRef({
      throttle: new Set<string>(),
      brake: new Set<string>(),
      drift: new Set<string>(),
      horn: new Set<string>(),
    }),
    latest = useRef(props),
    paused = useRef(false),
    activeRef = useRef(false),
    skinSent = useRef(""),
    audioFailure = useRef<{ epoch: number; serverAt: number } | null>(null);
  latest.current = props;
  const car = d.cars.find((c) => c.seat === room.you),
    spectating = !!room.spectating || room.you < 0;
  const followedAudio = spectating ? d.cars.find((c) => c.seat === watch) : car;
  useEffect(() => {
    if (!spectating) return;
    let alive = true;
    void api("/state")
      .then((state) => {
        if (alive) setViewerID(state.user?.id ?? "");
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [spectating, room.id]);
  const audioRunning =
    sound &&
    (!audioFailure.current ||
      audioFailure.current.epoch !== (clock.epoch ?? 0) ||
      d.serverAt > audioFailure.current.serverAt) &&
    focused &&
    !document.hidden &&
    clock.serverNow + now - clock.receivedAt - d.serverAt <= 1000 &&
    d.phase !== "finished";
  const stopClassicEngine = useClassicEngine(
    props.classicAssets,
    followedAudio,
    assets,
    audio,
    !!props.classicAssets && audioRunning && !followedAudio?.finishAt,
  );
  const stopClassicHorn = useClassicHorn(
    props.classicAssets,
    d.cars,
    spectating || room.seats[room.you]?.autoPlay ? null : room.you,
    !!drive.horn && !suspended,
    audio,
    !!props.classicAssets && audioRunning,
  );
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
  const controller = useGameController({
    gameId: "racing",
    userId: spectating ? viewerID : (room.seats[room.you]?.id ?? ""),
    scope,
    root: boardElement,
    enabled: () => latest.current.room.status === "playing" && !mapOpen,
    menu: () =>
      mapOpen
        ? (boardElement.current?.querySelector<HTMLElement>(
            ".racing-map-modal",
          ) ?? null)
        : null,
    menuActions: {
      left: "navLeft",
      right: "navRight",
      throttle: "navUp",
      brake: "navDown",
      drift: "confirm",
      map: "cancel",
      cancel: "cancel",
    },
    keyboardTarget: (target) =>
      !(target instanceof Element && target.closest("button")),
    changed: () => pumpControls.current(),
    error: setError,
  });
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
    const value = input.current;
    setDrive((old) =>
      old.steer === value.steer &&
      old.throttle === value.throttle &&
      old.brake === value.brake &&
      old.brakeAmount === value.brakeAmount &&
      old.drift === value.drift &&
      !!old.horn === !!value.horn
        ? old
        : { ...value },
    );
  };
  const compute = () => {
    if (!activeRef.current || paused.current) {
      setInput({ ...neutral });
      return;
    }
    const values = mapped.current;
    const touches = touchContacts.current.actions;
    const pressure =
      brakes.current.size > 0 ||
      pedalKeys.current.brake.size > 0 ||
      touches.has("brake")
        ? 1
        : (values.get("brake") ?? 0);
    setInput({
      steer:
        Math.max(
          values.get("right") ?? 0,
          touches.has("right") ||
            steerKeys.current.right.size ||
            [...steerPointers.current.values()].includes(1)
            ? 1
            : 0,
        ) -
        Math.max(
          values.get("left") ?? 0,
          touches.has("left") ||
            steerKeys.current.left.size ||
            [...steerPointers.current.values()].includes(-1)
            ? 1
            : 0,
        ),
      throttle:
        pressure > 0
          ? 0
          : Math.max(
              throttles.current.size > 0 ||
                pedalKeys.current.throttle.size > 0 ||
                touches.has("throttle")
                ? 1
                : 0,
              values.get("throttle") ?? 0,
            ),
      brake: pressure > 0,
      brakeAmount: pressure,
      drift:
        drifts.current.size > 0 ||
        pedalKeys.current.drift.size > 0 ||
        !!values.get("drift") ||
        touches.has("drift"),
      ...(d.physics === 3 &&
      (touches.has("horn") ||
        horns.current.size > 0 ||
        pedalKeys.current.horn.size > 0 ||
        values.get("horn"))
        ? { horn: true }
        : {}),
    });
  };
  const release = () => {
    controller.current?.clear();
    mapped.current.clear();
    mappedPressed.current.clear();
    rearHold.current.clear();
    setRear(false);
    throttles.current.clear();
    pedalKeys.current.throttle.clear();
    steerPointers.current.clear();
    touchContacts.current.clear();
    steerKeys.current.left.clear();
    steerKeys.current.right.clear();
    brakes.current.clear();
    drifts.current.clear();
    horns.current.clear();
    pedalKeys.current.horn.clear();
    pedalKeys.current.brake.clear();
    pedalKeys.current.drift.clear();
    setInput({ ...neutral });
  };
  const suspendInput = (networkFailure = false) => {
    if (networkFailure) {
      lifecycle.current.networkFailed();
      audioFailure.current = {
        epoch: latest.current.clock.epoch ?? 0,
        serverAt: latest.current.data.serverAt,
      };
    } else lifecycle.current.suspend();
    paused.current = true;
    setSuspended(true);
    release();
    latest.current.audio.stopLoop("racing-engine");
    latest.current.audio.stopLoop("racing-tires");
    stopClassicEngine();
    stopClassicHorn();
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
  const touchChanged = useRef<(event: TouchEvent) => void>(() => {});
  const pointerEnded = useRef<(event: PointerEvent) => void>(() => {});
  const touchActions = new Set<RacingTouchAction>([
    "left",
    "right",
    "throttle",
    "brake",
    "drift",
    "horn",
    "rear",
  ]);
  const touchAction = (
    element: EventTarget | Element | null,
  ): RacingTouchAction | null => {
    const button =
      element instanceof Element
        ? element.closest<HTMLButtonElement>("[data-racing-touch-action]")
        : null;
    const value = button?.dataset.racingTouchAction as
      RacingTouchAction | undefined;
    return button &&
      boardElement.current?.contains(button) &&
      !button.disabled &&
      value &&
      touchActions.has(value)
      ? value
      : null;
  };
  touchChanged.current = (event) => {
    const contacts = touchContacts.current;
    const wasOwned = contacts.size > 0;
    if (event.type === "touchstart") {
      for (const contact of Array.from(event.changedTouches)) {
        const action = touchAction(contact.target);
        if (action && (action === "rear" || activeRef.current)) {
          if (action !== "rear") wake(true);
          contacts.press(contact.identifier, action);
        }
      }
    }
    contacts.reconcile(
      Array.from(event.touches, (contact) => contact.identifier),
    );
    for (const contact of Array.from(event.touches)) {
      if (contacts.has(contact.identifier))
        contacts.move(
          contact.identifier,
          touchAction(
            document.elementFromPoint(contact.clientX, contact.clientY),
          ),
        );
    }
    if (!wasOwned && contacts.size === 0) return;
    if (event.cancelable) event.preventDefault();
    if (contacts.actions.has("rear")) rearHold.current.press("touch");
    else rearHold.current.release("touch");
    setRear(rearHold.current.active);
    compute();
  };
  pointerEnded.current = (event) => {
    steerPointers.current.delete(event.pointerId);
    for (const held of [throttles, brakes, drifts, horns])
      held.current.delete(event.pointerId);
    rearHold.current.release(`pointer:${event.pointerId}`);
    setRear(rearHold.current.active);
    compute();
  };
  useEffect(() => {
    const touch = (event: TouchEvent) => touchChanged.current(event);
    const pointer = (event: PointerEvent) => pointerEnded.current(event);
    const pointerMove = (event: PointerEvent) => {
      if (event.pointerType !== "touch" && event.buttons === 0) pointer(event);
    };
    const editors =
      'input,textarea,select,[contenteditable]:not([contenteditable="false"])';
    const prevent = (event: Event) => {
      if (
        !(event.target instanceof Element) ||
        !boardElement.current?.contains(event.target) ||
        event.target.closest(editors)
      )
        return;
      event.preventDefault();
      touchContacts.current.clear();
      steerPointers.current.clear();
      for (const held of [throttles, brakes, drifts, horns])
        held.current.clear();
      rearHold.current.release("touch");
      setRear(rearHold.current.active);
      compute();
    };
    const selection = () => {
      if (document.activeElement?.closest(editors)) return;
      const value = document.getSelection();
      const root = boardElement.current;
      if (!value?.rangeCount || value.isCollapsed || !root) return;
      for (let i = 0; i < value.rangeCount; i++) {
        if (value.getRangeAt(i).intersectsNode(root)) {
          value.removeAllRanges();
          break;
        }
      }
    };
    for (const type of ["touchstart", "touchmove", "touchend", "touchcancel"])
      document.addEventListener(type, touch as EventListener, {
        capture: true,
        passive: false,
      });
    for (const type of ["pointerup", "pointercancel", "lostpointercapture"])
      window.addEventListener(type, pointer as EventListener, true);
    window.addEventListener("pointermove", pointerMove, true);
    document.addEventListener("contextmenu", prevent, true);
    document.addEventListener("selectstart", prevent, true);
    document.addEventListener("selectionchange", selection);
    return () => {
      for (const type of ["touchstart", "touchmove", "touchend", "touchcancel"])
        document.removeEventListener(type, touch as EventListener, true);
      for (const type of ["pointerup", "pointercancel", "lostpointercapture"])
        window.removeEventListener(type, pointer as EventListener, true);
      window.removeEventListener("pointermove", pointerMove, true);
      document.removeEventListener("contextmenu", prevent, true);
      document.removeEventListener("selectstart", prevent, true);
      document.removeEventListener("selectionchange", selection);
      touchContacts.current.clear();
    };
  }, [scope]);
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
  pumpControls.current = () => {
    const actions =
      controller.current?.sample(performance.now()) ?? new Set<string>();
    const fresh = (name: string) =>
      actions.has(name) && !mappedPressed.current.has(name);
    const wasRear = rearHold.current.active;
    if (actions.has("rear")) rearHold.current.press("mapped");
    else rearHold.current.release("mapped");
    if (wasRear !== rearHold.current.active) setRear(rearHold.current.active);
    mapped.current = new Map(controller.current?.values ?? []);
    if (actions.size && paused.current) wake();
    if (fresh("reset") && activeRef.current) void reset();
    if (d.physics === 3) {
      if (fresh("camera"))
        setCameraMode((value) => (value + 1) % CLASSIC_CAMERA_MODES.length);
      if (fresh("drawDistance"))
        setDrawDistance((value) => (value + 1) % CLASSIC_DRAW_DISTANCES.length);
      if (fresh("hud")) setHudVisible((value) => !value);
      if (fresh("mirror")) setMirrorVisible((value) => !value);
      if (fresh("viewSmaller")) setViewSize((value) => Math.max(0, value - 1));
      if (fresh("viewLarger")) setViewSize((value) => Math.min(2, value + 1));
    }
    if (fresh("map")) {
      release();
      setMapOpen(true);
    }
    if (fresh("nextSkin")) chooseSkin((skin + 1) % skinNames.length);
    if (fresh("previousSkin"))
      chooseSkin((skin + skinNames.length - 1) % skinNames.length);
    if (document.querySelector('[aria-modal="true"]') && !mapOpen) {
      release();
      return;
    }
    mappedPressed.current = actions;
    compute();
  };
  useEffect(() => {
    const suspend = () => {
      setFocused(false);
      suspendInput();
    };
    const focus = () => setFocused(true);
    const visibility = () => {
      if (document.hidden) suspend();
    };
    const orientation = matchMedia("(orientation: landscape)");
    const rotate = () => {
      if (!isTouch()) return;
      lifecycle.current.suspend();
      paused.current = true;
      setSuspended(true);
      release();
    };
    orientation.addEventListener("change", rotate);
    window.addEventListener("blur", suspend);
    window.addEventListener("focus", focus);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      orientation.removeEventListener("change", rotate);
      window.removeEventListener("blur", suspend);
      window.removeEventListener("focus", focus);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, [scope]);

  useEffect(() => {
    if (
      d.physics === 3 ||
      !enabled ||
      !d.controls.canSkin ||
      skinSent.current === scope
    )
      return;
    skinSent.current = scope;
    if (skin !== car?.skin)
      void act({ type: "skin", epoch: d.epoch, skin }).catch((e) =>
        setError(actionError(e)),
      );
  }, [scope, enabled, d.controls.canSkin]);
  function chooseSkin(value: number) {
    if (latest.current.data.physics === 3) return;
    if (!latest.current.data.controls.canSkin) return;
    setSkin(value);
    try {
      localStorage.setItem("racing.skin", String(value));
    } catch {}
    void latest.current
      .act({ type: "skin", epoch: latest.current.data.epoch, skin: value })
      .catch((e) => setError(actionError(e)));
  }
  async function reset() {
    const p = latest.current;
    if (!activeRef.current || !p.data.controls.canReset) return;
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
      if (!audioRunning) continue;
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
            if (!document.hidden && document.hasFocus())
              void audio.playWithFallback(sources(assets, file + ".mp3"));
          }, delay);
      }
    }
  }, [d.events, scope, sound]);
  useEffect(() => {
    if (!audioRunning) eventTimers.current.clear();
  }, [audioRunning]);
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
    if (countdown > 0 && countdown < lastCount.current && audioRunning)
      void audio.playWithFallback(sources(assets, "countdown.mp3"));
    lastCount.current = countdown;
  }, [countdown, audioRunning]);
  useEffect(() => {
    const followed = spectating ? d.cars.find((c) => c.seat === watch) : car;
    const running = audioRunning && followed && !followed.finishAt;
    if (!running) {
      audio.stopLoop("racing-engine");
      audio.stopLoop("racing-tires");
      return;
    }
    const speed = Math.min(1, followed.speed / MAX_ROAD_SPEED);
    if (d.physics === 3) audio.stopLoop("racing-engine");
    else
      void audio.loop("racing-engine", sources(assets, "engine.mp3"), {
        volume: 0.12 + speed * 0.16,
        rate: 0.72 + speed * 0.7,
      });
    if (
      (enabled && !suspended ? drive.drift : followed.drift) &&
      followed.speed > 70
    )
      void audio.loop("racing-tires", sources(assets, "tires.mp3"), {
        volume: 0.12 + speed * 0.12,
        rate: 0.9 + speed * 0.12,
      });
    else audio.stopLoop("racing-tires");
  }, [now, audioRunning, d.phase, car?.speed, drive.drift, watch]);
  const steerButton = (side: "left" | "right") => ({
    "data-game-hold-control": true,
    "data-racing-touch-action": side,
    onKeyDown: (e: ReactKeyboardEvent<HTMLButtonElement>) => {
      if (!enabled || (e.code !== "Space" && e.code !== "Enter")) return;
      e.preventDefault();
      e.stopPropagation();
      if (e.repeat && !steerKeys.current[side].has(e.code)) return;
      wake();
      steerKeys.current[side].add(e.code);
      compute();
    },
    onKeyUp: (e: ReactKeyboardEvent<HTMLButtonElement>) => {
      steerKeys.current[side].delete(e.code);
      compute();
    },
    onBlur: () => {
      steerKeys.current[side].clear();
      compute();
    },
    onPointerDown: (e: ReactPointerEvent<HTMLButtonElement>) => {
      if (e.pointerType === "touch" || !enabled || e.button !== 0) return;
      e.preventDefault();
      wake();
      e.currentTarget.setPointerCapture(e.pointerId);
      steerPointers.current.set(e.pointerId, side === "left" ? -1 : 1);
      compute();
    },
    onPointerMove: (e: ReactPointerEvent<HTMLButtonElement>) => {
      if (!steerPointers.current.has(e.pointerId)) return;
      const hit = document
        .elementFromPoint(e.clientX, e.clientY)
        ?.closest<HTMLButtonElement>(".racing-steering button");
      const direction = hit?.dataset.steerDirection;
      steerPointers.current.set(
        e.pointerId,
        direction === "left" ? -1 : direction === "right" ? 1 : 0,
      );
      compute();
    },
    onPointerUp: (e: ReactPointerEvent<HTMLButtonElement>) => {
      steerPointers.current.delete(e.pointerId);
      compute();
    },
    onPointerCancel: (e: ReactPointerEvent<HTMLButtonElement>) => {
      steerPointers.current.delete(e.pointerId);
      compute();
    },
    onLostPointerCapture: (e: ReactPointerEvent<HTMLButtonElement>) => {
      steerPointers.current.delete(e.pointerId);
      compute();
    },
  });
  const heldButton = (which: "throttle" | "brake" | "drift" | "horn") => ({
    "data-game-hold-control": true,
    "data-racing-touch-action": which,
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
      if (e.pointerType === "touch" || !enabled || e.button !== 0) return;
      e.preventDefault();
      wake();
      e.currentTarget.setPointerCapture(e.pointerId);
      (which === "throttle"
        ? throttles
        : which === "brake"
          ? brakes
          : which === "horn"
            ? horns
            : drifts
      ).current.add(e.pointerId);
      compute();
    },
    onPointerUp: (e: ReactPointerEvent<HTMLButtonElement>) => {
      (which === "throttle"
        ? throttles
        : which === "brake"
          ? brakes
          : which === "horn"
            ? horns
            : drifts
      ).current.delete(e.pointerId);
      compute();
    },
    onPointerCancel: (e: ReactPointerEvent<HTMLButtonElement>) => {
      (which === "throttle"
        ? throttles
        : which === "brake"
          ? brakes
          : which === "horn"
            ? horns
            : drifts
      ).current.delete(e.pointerId);
      compute();
    },
    onLostPointerCapture: (e: ReactPointerEvent<HTMLButtonElement>) => {
      (which === "throttle"
        ? throttles
        : which === "brake"
          ? brakes
          : which === "horn"
            ? horns
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
                    ? "操作已释放，按下方向或踏板继续"
                    : "比赛进行中";
  const drivingControls = (
    <div
      className={`racing-controls ${mobile ? "is-touch" : ""}`}
      hidden={d.physics === 3 && (!mobile || spectating)}
      data-game-controls
      data-steer={drive.steer}
      data-throttle={drive.throttle}
      data-brake={drive.brake}
      data-brake-amount={drive.brakeAmount ?? (drive.brake ? 1 : 0)}
      data-drift={drive.drift}
      data-horn={!!drive.horn}
    >
      <div className="racing-steering" role="group" aria-label="转向">
        <button
          type="button"
          aria-label="向左转向"
          data-steer-direction="left"
          aria-pressed={drive.steer < 0}
          className={drive.steer < 0 ? "pressed" : ""}
          disabled={!enabled}
          {...steerButton("left")}
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="m15 5-7 7 7 7" />
          </svg>
        </button>
        <button
          type="button"
          aria-label="向右转向"
          data-steer-direction="right"
          aria-pressed={drive.steer > 0}
          className={drive.steer > 0 ? "pressed" : ""}
          disabled={!enabled}
          {...steerButton("right")}
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="m9 5 7 7-7 7" />
          </svg>
        </button>
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
          手刹
        </button>
        <button
          type="button"
          className={drive.brake ? "pressed" : ""}
          style={{
            background: `linear-gradient(to top, #386d72 ${(drive.brakeAmount ?? 0) * 100}%, #2a4352 ${(drive.brakeAmount ?? 0) * 100}%)`,
          }}
          aria-label="刹车"
          aria-pressed={drive.brake}
          disabled={!enabled}
          {...heldButton("brake")}
        >
          刹车
        </button>
        <button
          type="button"
          style={{
            background: `linear-gradient(to top, #386d72 ${drive.throttle * 100}%, #2a4352 ${drive.throttle * 100}%)`,
          }}
          aria-label="油门"
          aria-pressed={drive.throttle > 0}
          className={drive.throttle > 0 ? "pressed" : ""}
          disabled={!enabled}
          {...heldButton("throttle")}
        >
          油门
        </button>
      </div>
    </div>
  );
  const viewControls = (
    <div className="racing-view-controls" data-game-controls>
      {d.physics === 3 && (
        <button
          type="button"
          aria-label="切换视角"
          onClick={() =>
            setCameraMode((value) => (value + 1) % CLASSIC_CAMERA_MODES.length)
          }
        >
          视角
        </button>
      )}
      <button
        type="button"
        aria-label="向后看"
        data-game-hold-control
        data-racing-touch-action="rear"
        aria-pressed={rear}
        onPointerDown={(e) => {
          if (e.pointerType === "touch") return;
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
            if (e.repeat) return;
            rearHold.current.press(`button:${e.code}`);
            setRear(true);
          }
        }}
        onKeyUp={(e) => {
          if (e.key === " " || e.key === "Enter") {
            rearHold.current.release(`button:${e.code}`);
            setRear(rearHold.current.active);
          }
        }}
        onBlur={() => {
          rearHold.current.release("button:Space");
          rearHold.current.release("button:Enter");
          setRear(rearHold.current.active);
        }}
      >
        后视
      </button>
      {d.physics !== 3 && <span>自动挡 · 无需换挡</span>}
      {d.physics === 3 && (
        <button
          type="button"
          aria-label="喇叭"
          aria-pressed={!!drive.horn}
          disabled={!enabled}
          {...heldButton("horn")}
        >
          喇叭
        </button>
      )}
      <button
        type="button"
        aria-label="重置赛车"
        disabled={!d.controls.canReset || !enabled}
        onClick={() => void reset()}
      >
        {car && car.resetUntil > 0 && car.resetUntil + 3000 > serverNow
          ? `${Math.ceil((car.resetUntil + 3000 - serverNow) / 1000)}s`
          : "复位"}{" "}
      </button>
    </div>
  );
  return (
    <section
      ref={boardElement}
      className={`racing-board ${d.physics === 3 ? "is-classic" : ""} ${mobile ? "has-touch" : ""}`}
      data-hud-visible={hudVisible}
      data-mirror-visible={mirrorVisible}
      data-phase={d.phase}
      data-epoch={d.epoch}
      data-tick={d.serverAt}
      data-physics={d.physics ?? 0}
    >
      <div className="racing-hud" hidden={d.physics === 3 && !hudVisible}>
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
        {d.physics === 3 && drivingControls}
        {d.physics === 3 && mobile && viewControls}
        <TrackCanvas
          classicAssets={props.classicAssets}
          onInputFrame={() => pumpControls.current()}
          onReady={props.onReady}
          onLoadError={(message) => {
            suspendInput();
            setError(message);
            props.onLoadError?.(message);
          }}
          data={d}
          clock={clock}
          seat={spectating ? -1 : room.you}
          assets={assets}
          input={input}
          driving={enabled && !suspended}
          reduced={reduced}
          watch={watch}
          rear={rear}
          cameraMode={cameraMode}
          drawDistance={drawDistance}
          mirrorVisible={mirrorVisible}
          hudVisible={hudVisible}
          viewScale={d.physics === 3 ? [0.65, 0.82, 1][viewSize] : 1}
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
      {d.physics !== 3 && viewControls}
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
          {d.physics !== 3 && drivingControls}
          {d.physics !== 3 && (
            <>
              <div className="racing-settings">
                <>
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
                          void act({
                            type: "skin",
                            epoch: d.epoch,
                            skin: s,
                          }).catch((e) => setError(actionError(e)));
                      }}
                    >
                      {skinNames.map((name, i) => (
                        <option key={i} value={i}>
                          {name}
                        </option>
                      ))}
                    </select>
                  </label>
                </>

                {d.physics !== 3 && <span>车型性能相同 · 重置前先减速</span>}
              </div>
            </>
          )}
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
