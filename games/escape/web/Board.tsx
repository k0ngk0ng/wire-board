import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent,
} from "react";
import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  ArrowUpRight,
  Battery,
  BookOpen,
  Check,
  ChevronRight,
  Clock,
  Compass,
  FileText,
  Heart,
  Map as MapIcon,
  Package,
  Pause,
  Play,
  Radio,
  ShieldCheck,
  Sparkles,
  X,
  Zap,
} from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { api } from "../../../web/src/ui/board-host";
import { useGameController } from "../../../web/src/controllers/use-game-controller";
import { loadArt, type Art } from "./art";
import { draw } from "./canvas";
import { Motion } from "./prediction";
import { spreadCharacters } from "./spacing";
import {
  directionNames,
  mergeView,
  playerColors,
  roleNames,
  scenarioNames,
  type Device,
  type Player,
  type Point,
  type View,
} from "./types";

const clamp = (n: number, a: number, b: number) => Math.max(a, Math.min(b, n));
const near = (a: Point, b: Point) => Math.hypot(a.x - b.x, a.y - b.y);
const clock = (ms: number) =>
  `${Math.floor(Math.max(0, ms) / 60000)
    .toString()
    .padStart(
      2,
      "0",
    )}:${Math.floor(Math.max(0, ms) / 1000) % 60 < 10 ? "0" : ""}${Math.floor(Math.max(0, ms) / 1000) % 60}`;
type Screen = HTMLCanvasElement & {
  __escapeFrame?: {
    at: number;
    x: number;
    y: number;
    room: string;
    frame: number;
    gaps: number[];
    serverX: number;
    serverY: number;
    camera: number;
    width: number;
    authorityAt: number;
    tick: number;
  };
};

export function Board(props: BoardProps<View>) {
  const merged = useRef<View | undefined>(undefined);
  merged.current = mergeView(merged.current, props.data);
  const view = merged.current;
  const observed = useRef({ key: "", at: performance.now() });
  const observationKey = `${view.epoch}/${view.tick}/${view.phase}/${view.paused}/${!!props.room.loading}/${props.clock.epoch ?? 0}`;
  if (observed.current.key !== observationKey)
    observed.current = { key: observationKey, at: performance.now() };
  const { room } = props;
  const me = view.players[room.you];
  const focusPlayer = me ?? view.players[0];
  const roomState =
    view.rooms?.find((r) => r.id === focusPlayer?.room) ??
    view.rooms?.find((r) => r.known);
  const current = roomState
    ? {
        ...roomState,
        flood: view.water?.[Number(roomState.id.slice(1))] ?? roomState.flood,
      }
    : undefined;
  const [art, setArt] = useState<Art>();
  const [selected, setSelected] = useState("");
  const [hovered, setHovered] = useState("");
  const [modal, setModal] = useState<"map" | "journal" | "inventory" | null>(
    null,
  );
  const [error, setError] = useState("");
  const [worldWidth, setWorldWidth] = useState(1000);
  const [pressed, setPressed] = useState<string[]>([]);
  const [, recovered] = useState(0);
  const root = useRef<HTMLElement>(null);
  const canvas = useRef<Screen>(null);
  const scene = useRef<HTMLDivElement>(null);
  const hotspots = useRef<HTMLDivElement>(null);
  const latest = useRef(props);
  latest.current = props;
  const state = useRef({ view, selected, hovered, current, modal });
  state.current = { view, selected, hovered, current, modal };
  const motion = useRef(new Motion());
  const remoteMotion = useRef(new Map<number, Motion>());
  const crowdOffsets = useRef(new Map<number, Point>());
  const artRef = useRef<Art | undefined>(undefined);
  const camera = useRef(0);
  const worldWidthRef = useRef(1000);
  worldWidthRef.current = worldWidth;
  const nextSeq = useRef(me?.seq ?? 0);
  const pending = useRef<Record<string, unknown>[]>([]);
  const inFlight = useRef(0);
  const pump = useRef<() => void>(() => {});
  const scope = `${room.id}/${view.epoch}/${room.you}/${props.clock.epoch ?? 0}`;
  const identity = useRef(scope);
  identity.current = scope;
  const canControl =
    !!me &&
    me.hp > 0 &&
    !room.spectating &&
    !room.seats[room.you]?.autoPlay &&
    view.phase === "playing" &&
    !view.paused &&
    !room.loading;
  const readyReported = useRef("");
  const seenEvent = useRef({ epoch: "", id: 0 });
  const lastAction = useRef(new Set<string>());
  const input = useRef({ dx: 0, dy: 0, sent: 0 });
  const touch = useRef(new Map<number, string>());
  const readControls = useRef<(at: number) => void>(() => {});
  const send = useCallback((action: Record<string, unknown>) => {
    const last = pending.current.at(-1);
    if (
      action.type === "input" &&
      last?.type === "input" &&
      last.dx === action.dx &&
      last.dy === action.dy
    )
      return;
    if (
      last?.type === "input" &&
      action.type === "input" &&
      (Number(last.dx) || Number(last.dy)) &&
      (Number(action.dx) || Number(action.dy)) &&
      Math.sign(Number(last.dx)) === Math.sign(Number(action.dx)) &&
      Math.sign(Number(last.dy)) === Math.sign(Number(action.dy))
    ) {
      pending.current[pending.current.length - 1] = action;
      pump.current();
      return;
    }
    if (pending.current.length >= 32) {
      setError("操作过快，请稍候");
      return;
    }
    pending.current.push(action);
    pump.current();
  }, []);
  pump.current = () => {
    const p = latest.current;
    const limit = Math.max(1, Math.min(4, p.inputWindow ?? 1));
    while (inFlight.current < limit && pending.current.length) {
      const action = pending.current.shift()!;
      nextSeq.current = Math.max(
        nextSeq.current,
        p.data.players[p.room.you]?.seq ?? 0,
      );
      const epoch = state.current.view.epoch;
      const currentScope = identity.current;
      const seq = ++nextSeq.current;
      inFlight.current++;
      void p
        .act({ ...action, epoch, seq })
        .catch((e) => {
          if (identity.current === currentScope)
            setError(e instanceof Error ? e.message : "操作未生效");
        })
        .finally(() => {
          inFlight.current--;
          pump.current();
        });
    }
  };
  const select = useCallback(
    (d: Device) => {
      setError("");
      setSelected(d.id);
      send({ type: "inspect", target: d.id });
    },
    [send],
  );
  const close = () => {
    setSelected("");
    setModal(null);
    setError("");
    canvas.current?.focus({ preventScroll: true });
  };
  const controller = useGameController({
    gameId: "escape",
    userId: room.seats[room.you]?.id ?? "",
    scope,
    root,
    enabled: () => canControl,
    menu: () =>
      root.current?.querySelector<HTMLElement>(
        ".escape-modal[data-escape-dialog]",
      ) ??
      root.current?.querySelector<HTMLElement>(".escape-paused") ??
      root.current?.querySelector<HTMLElement>("[data-escape-dialog]") ??
      null,
    menuActions: {
      up: "navUp",
      down: "navDown",
      left: "navLeft",
      right: "navRight",
      interact: "confirm",
      cancel: "cancel",
    },
    changed: () => readControls.current(performance.now()),
    error: setError,
  });
  useEffect(() => {
    pending.current = [];
    nextSeq.current =
      latest.current.data.players[latest.current.room.you]?.seq ?? 0;
    motion.current = new Motion();
    input.current = { dx: 0, dy: 0, sent: 0 };
    lastAction.current.clear();
    touch.current.clear();
    setSelected("");
    setModal(null);
  }, [scope]);
  useEffect(() => {
    if (view.rooms?.length) return;
    let live = true;
    // The module may finish loading after the initial stream checkpoint has
    // already been replaced by compact updates. Recover once, using the same
    // authenticated per-seat checkpoint route as an ordinary reconnect.
    void api(`/rooms/${room.id}/sync`)
      .then((response: { roomId: string; data: View }) => {
        if (
          !live ||
          response.roomId !== room.id ||
          response.data.epoch !== latest.current.data.epoch
        )
          return;
        const current = merged.current;
        merged.current =
          current && current.tick >= response.data.tick
            ? mergeView(response.data, current)
            : response.data;
        recovered((n) => n + 1);
      })
      .catch(() => {
        if (live) latest.current.onLoadError?.("密室状态恢复失败，请重试。");
      });
    return () => {
      live = false;
    };
  }, [scope, !!view.rooms?.length]);
  useEffect(() => {
    let live = true;
    setArt(undefined);
    artRef.current = undefined;
    loadArt(view.options.scenario, room.seats.length, (n) =>
      latest.current.onLoadProgress?.(n, "准备密室画面"),
    )
      .then((a) => {
        if (live) {
          artRef.current = a;
          setArt(a);
        }
      })
      .catch(() => {
        if (live)
          latest.current.onLoadError?.("密室场景或角色素材加载失败，请重试。");
      });
    return () => {
      live = false;
    };
  }, [view.options.scenario, room.seats.length]);
  useEffect(() => {
    if (!scene.current) return;
    const measure = () => {
      const width = scene.current?.getBoundingClientRect().width ?? 1000;
      const fullscreen = !!root.current?.closest("[data-room-fullscreen]");
      setWorldWidth(
        width < 600
          ? fullscreen && innerHeight > innerWidth
            ? 500
            : 800
          : 1000,
      );
    };
    const observer = new ResizeObserver(measure);
    observer.observe(scene.current);
    const surface = root.current?.closest(".game-fullscreen-surface");
    const changes = new MutationObserver(measure);
    if (surface)
      changes.observe(surface, {
        attributes: true,
        attributeFilter: ["data-room-fullscreen"],
      });
    return () => {
      observer.disconnect();
      changes.disconnect();
    };
  }, []);
  useEffect(() => {
    if (!error) return;
    const id = window.setTimeout(() => setError(""), 6000);
    return () => clearTimeout(id);
  }, [error]);
  useEffect(() => {
    const target = root.current;
    const cancel = (event: Event) => {
      event.preventDefault();
      close();
    };
    target?.addEventListener("controller-cancel", cancel);
    return () => target?.removeEventListener("controller-cancel", cancel);
  }, []);
  useEffect(() => {
    const release = () => {
      touch.current.clear();
      setPressed([]);
      if (input.current.dx || input.current.dy) {
        input.current = { dx: 0, dy: 0, sent: performance.now() };
        motion.current.input(0, 0, performance.now());
        send({ type: "input", dx: 0, dy: 0 });
      }
    };
    const hidden = () => {
      if (document.hidden) release();
    };
    window.addEventListener("blur", release);
    document.addEventListener("visibilitychange", hidden);
    return () => {
      release();
      window.removeEventListener("blur", release);
      document.removeEventListener("visibilitychange", hidden);
    };
  }, [send]);
  useEffect(() => {
    const dialog = root.current?.querySelector<HTMLElement>(
      "[data-escape-dialog]",
    );
    if (!dialog) return;
    if (modal === "inventory")
      dialog.scrollIntoView({ block: "nearest", behavior: "instant" });
    dialog
      .querySelector<HTMLElement>("button:not(:disabled)")
      ?.focus({ preventScroll: true });
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        close();
        return;
      }
      if (event.key !== "Tab") return;
      const targets = [
        ...dialog.querySelectorAll<HTMLElement>(
          'button:not(:disabled),input:not(:disabled),select:not(:disabled),[tabindex="0"]',
        ),
      ].filter((e) => e.getClientRects().length);
      if (!targets.length) return;
      const first = targets[0],
        last = targets.at(-1)!;
      if (
        !dialog.contains(document.activeElement) ||
        (event.shiftKey && document.activeElement === first) ||
        (!event.shiftKey && document.activeElement === last)
      ) {
        event.preventDefault();
        (event.shiftKey ? last : first).focus({ preventScroll: true });
      }
    };
    dialog.addEventListener("keydown", keyboard);
    return () => dialog.removeEventListener("keydown", keyboard);
  }, [modal, selected]);
  useEffect(() => {
    const previous = seenEvent.current;
    if (previous.epoch !== view.epoch) {
      seenEvent.current = {
        epoch: view.epoch,
        id: view.events?.at(-1)?.id ?? 0,
      };
      return;
    }
    for (const e of view.events ?? []) {
      if (e.id <= seenEvent.current.id) continue;
      seenEvent.current.id = e.id;
      if (!props.sound) continue;
      const sound =
        e.kind === "solved"
          ? "solved"
          : e.kind === "warning"
            ? "warning"
            : e.kind === "clue"
              ? "clue"
              : e.kind === "supply" || e.kind === "deposit"
                ? "pickup"
                : e.kind === "victory"
                  ? "victory"
                  : e.kind === "mistake"
                    ? "error"
                    : null;
      if (sound) void props.audio.play(["/escape/v1/audio/" + sound + ".flac"]);
    }
  }, [view.revision, view.epoch, props.sound, props.audio, props.assets]);
  useEffect(
    () => () => {
      latest.current.audio.stop();
    },
    [],
  );
  useEffect(() => {
    setSelected("");
    setHovered("");
  }, [current?.id]);
  useEffect(() => {
    if (!art || !props.sound || view.phase !== "playing") {
      props.audio.stopMusic();
      return;
    }
    if (view.paused) {
      props.audio.pause();
      return;
    }
    void props.audio.music(["/escape/v1/audio/ambient.flac"], {
      loop: true,
      startOffset: view.tick / 1000,
      playbackID: view.epoch,
    });
    void props.audio.resume();
  }, [
    art,
    props.sound,
    view.paused,
    view.phase,
    view.epoch,
    props.assets,
    props.audio,
  ]);
  readControls.current = (at) => {
    const actions = controller.current?.sample(at) ?? new Set<string>();
    const st = state.current;
    const p = latest.current;
    const player = st.view.players[p.room.you];
    const enabled =
      !!player &&
      player.hp > 0 &&
      !p.room.spectating &&
      !p.room.seats[p.room.you]?.autoPlay &&
      !st.view.paused &&
      st.view.phase === "playing" &&
      !p.room.loading;
    const held = new Set([...actions, ...touch.current.values()]);
    const touchHeld = new Set(touch.current.values());
    const level = (direction: string) =>
      touchHeld.has(direction)
        ? 1
        : clamp(
            controller.current?.values.get(direction) ??
              Number(actions.has(direction)),
            0,
            1,
          );
    let dx =
        held.has("right") && held.has("left")
          ? 0
          : level("right") - level("left"),
      dy = held.has("down") && held.has("up") ? 0 : level("down") - level("up");
    if (!enabled || st.modal || st.selected) {
      dx = 0;
      dy = 0;
    }
    if (dx !== input.current.dx || dy !== input.current.dy) {
      motion.current.input(dx, dy, at);
      input.current = { dx, dy, sent: at };
      if (enabled) send({ type: "input", dx, dy });
    } else if (enabled && (dx || dy) && at - input.current.sent > 180) {
      input.current.sent = at;
      send({ type: "input", dx, dy });
    }
    for (const a of actions) {
      if (lastAction.current.has(a)) continue;
      if (a === "map") setModal((m) => (m === "map" ? null : "map"));
      if (a === "journal")
        setModal((m) => (m === "journal" ? null : "journal"));
      if (a === "cancel") close();
      if (a === "ping" && enabled) send({ type: "ping" });
      if (a === "interact" && enabled && !st.modal && !st.selected) {
        const nearest = [...(st.current?.objects ?? [])].sort(
          (a, b) => near(player, a) - near(player, b),
        )[0];
        if (nearest) select(nearest);
      }
    }
    lastAction.current = new Set(actions);
  };
  useEffect(() => {
    let frame = 0,
      last = 0,
      stopped = false;
    const gaps: number[] = [];
    const run = (now: number) => {
      if (stopped) return;
      readControls.current(now);
      const cv = canvas.current,
        st = state.current,
        a = artRef.current;
      if (cv && st.current && a) {
        const rect = cv.getBoundingClientRect(),
          width = worldWidthRef.current,
          ratio = Math.min(devicePixelRatio || 1, 2);
        const h = (rect.width / width) * 666.667;
        if (
          cv.width !== Math.round(rect.width * ratio) ||
          cv.height !== Math.round(h * ratio)
        ) {
          cv.width = Math.round(rect.width * ratio);
          cv.height = Math.round(h * ratio);
        }
        const who = latest.current.room.you;
        const local = st.view.players[who];
        const points = new Map<number, Point>();
        if (local) {
          motion.current.update(
            local,
            st.view.tick,
            st.view.epoch,
            observed.current.at,
          );
          points.set(who, motion.current.sample(now));
        }
        for (const p of st.view.players) {
          if (p.seat !== who) {
            let prediction = remoteMotion.current.get(p.seat);
            if (!prediction) {
              prediction = new Motion();
              remoteMotion.current.set(p.seat, prediction);
            }
            prediction.update(
              p,
              st.view.tick,
              st.view.epoch,
              observed.current.at,
            );
            points.set(p.seat, prediction.sample(now));
          }
        }
        const pos = local ? (points.get(who) ?? local) : st.view.players[0];
        camera.current =
          width < 1000 ? clamp(pos.x - width / 2, 0, 1000 - width) : 0;
        if (hotspots.current)
          hotspots.current.style.transform = `translateX(${(-camera.current / 1000) * 100}%)`;
        const context = cv.getContext("2d");
        if (context) {
          const arranged = spreadCharacters(
            st.view.players.filter((p) => p.room === st.current!.id),
            points,
            who,
          );
          const shown = new Map(points);
          for (const p of st.view.players) {
            const point = points.get(p.seat),
              target = arranged.get(p.seat);
            if (!point || !target) continue;
            const wanted = { x: target.x - point.x, y: target.y - point.y };
            const previous = crowdOffsets.current.get(p.seat) ?? wanted;
            const easing = matchMedia("(prefers-reduced-motion: reduce)")
              .matches
              ? 1
              : 1 - Math.exp(-Math.min(100, Math.max(0, now - last)) / 70);
            const offset = {
              x: previous.x + (wanted.x - previous.x) * easing,
              y: previous.y + (wanted.y - previous.y) * easing,
            };
            crowdOffsets.current.set(p.seat, offset);
            shown.set(p.seat, { x: point.x + offset.x, y: point.y + offset.y });
          }
          draw(context, cv, {
            view: st.view,
            room: st.current,
            art: a,
            positions: shown,
            you: who,
            names: latest.current.room.seats.map((p) => p.name),
            now,
            selected: st.selected,
            hovered: st.hovered,
            reduced: matchMedia("(prefers-reduced-motion: reduce)").matches,
            camera: camera.current,
            width,
            input: input.current,
          });
          if (last && now - last < 1000) {
            gaps.push(now - last);
            if (gaps.length > 240) gaps.shift();
          }
          cv.__escapeFrame = {
            at: performance.now(),
            x: pos.x,
            y: pos.y,
            room: st.current.id,
            frame: ++frame,
            gaps: [...gaps],
            serverX: local?.x ?? pos.x,
            serverY: local?.y ?? pos.y,
            camera: camera.current,
            width,
            authorityAt: observed.current.at,
            tick: st.view.tick,
          };
          last = now;
          if (readyReported.current !== identity.current) {
            readyReported.current = identity.current;
            latest.current.onReady?.();
          }
        }
      }
      requestAnimationFrame(run);
    };
    const id = requestAnimationFrame(run);
    return () => {
      stopped = true;
      cancelAnimationFrame(id);
    };
  }, []);
  const travel = (target: string) => {
    setSelected("");
    setModal(null);
    setError("");
    send({ type: "travel", target });
  };
  const clickFloor = (e: ReactPointerEvent<HTMLCanvasElement>) => {
    if (!canControl || modal || selected) return;
    const r = e.currentTarget.getBoundingClientRect(),
      x = clamp(
        ((e.clientX - r.left) / r.width) * worldWidth + camera.current,
        145,
        855,
      ),
      y = clamp(((e.clientY - r.top) / r.height) * 666.667, 270, 565);
    motion.current.input(0, 0, performance.now());
    send({ type: "walk", x, y });
    e.currentTarget.focus({ preventScroll: true });
  };
  const touchDown = (
    e: ReactPointerEvent<HTMLButtonElement>,
    direction: string,
  ) => {
    if (!canControl) return;
    e.preventDefault();
    e.currentTarget.setPointerCapture(e.pointerId);
    touch.current.set(e.pointerId, direction);
    setPressed([...touch.current.values()]);
    readControls.current(performance.now());
  };
  const touchUp = (e: ReactPointerEvent<HTMLButtonElement>) => {
    touch.current.delete(e.pointerId);
    setPressed([...touch.current.values()]);
    readControls.current(performance.now());
  };
  const device = current?.objects?.find((d) => d.id === selected);
  const closeBtn = (
    <button
      className="escape-close"
      aria-label="关闭调查面板"
      data-controller-cancel
      onClick={close}
    >
      <X size={18} />
    </button>
  );
  const lastEvent = view.events?.at(-1);
  const warning = view.incident;
  const manualSeats = room.seats.filter(
    (p) => !p.bot && !p.autoPlay && !p.left,
  ).length;
  const votes = view.votes.filter(
    (v, i) => v && !room.seats[i]?.bot && !room.seats[i]?.autoPlay,
  ).length;
  const imageBase = "/escape/v1/";
  const modalTitle =
    modal === "map"
      ? "设施地图"
      : modal === "inventory"
        ? "物品栏"
        : "调查笔记";
  return (
    <section
      ref={root}
      className={`escape-board ${view.options.scenario}`}
      data-escape-board
      data-ready={!!art}
      aria-label={`${scenarioNames[view.options.scenario]}游戏区`}
    >
      <div className="escape-topbar">
        <div className="escape-title">
          <span className="escape-emblem">
            <Compass size={23} />
          </span>
          <div>
            <small>密室逃脱</small>
            <strong>{scenarioNames[view.options.scenario]}</strong>
          </div>
        </div>
        <div className="escape-team-power">
          <Zap size={16} />
          <span>
            电力 <b>{Math.ceil(view.power)}%</b>
          </span>
          <i>
            <span style={{ width: `${view.power}%` }} />
          </i>
        </div>
        <div
          className={`escape-clock ${view.remaining < 120000 ? "urgent" : ""}`}
        >
          <Clock size={16} />
          <b>{clock(view.remaining)}</b>
        </div>
        <button
          className="escape-icon-button"
          disabled={
            room.spectating ||
            !!room.seats[room.you]?.autoPlay ||
            view.phase !== "playing"
          }
          aria-label={view.paused ? "投票恢复" : "投票暂停"}
          aria-pressed={!!view.votes[room.you]}
          title={view.paused ? "投票恢复" : "投票暂停"}
          onClick={() => send({ type: view.paused ? "resume" : "pause" })}
        >
          {view.paused ? <Play size={17} /> : <Pause size={17} />}
        </button>
      </div>
      <div
        className="escape-field-team"
        style={{ "--escape-players": view.players.length } as CSSProperties}
      >
        {view.players.map((p) => (
          <span
            key={p.seat}
            data-escape-field-health={p.seat}
            style={{ borderColor: playerColors[p.seat] }}
          >
            <b>{room.seats[p.seat]?.name}</b>
            <small>
              <Heart size={10} />
              {Math.ceil(p.hp)}
            </small>
          </span>
        ))}
      </div>
      <div className="escape-layout">
        <div className="escape-play-area">
          <div
            className="escape-scene"
            ref={scene}
            style={
              {
                aspectRatio: worldWidth / 666.667,
                "--escape-aspect": worldWidth / 666.667,
              } as CSSProperties
            }
          >
            <canvas
              ref={canvas}
              data-escape-canvas
              tabIndex={0}
              aria-label={`${current?.title ?? "密室"}，点击地面移动，点击物件调查`}
              onPointerDown={clickFloor}
            />
            <div className="escape-room-heading">
              <span>{current?.title ?? "正在进入密室"}</span>
              <small>
                <i />{" "}
                {room.spectating
                  ? "共同视野"
                  : roleNames[Math.max(0, room.you)]}
              </small>
            </div>
            <div
              className="escape-hotspots"
              ref={hotspots}
              style={{ width: `${(1000 / worldWidth) * 100}%` }}
            >
              {current?.doors?.map((d) => (
                <button
                  className="escape-door-hit"
                  key={d.room}
                  data-escape-door={d.room}
                  style={{
                    left: `${d.x / 10}%`,
                    top: `${((d.y - 45) / 666.667) * 100}%`,
                  }}
                  aria-label={`前往${view.rooms?.find((r) => r.id === d.room)?.title ?? directionNames[d.direction] + "侧房间"}`}
                  disabled={!canControl}
                  onClick={() => travel(d.room)}
                />
              ))}
              {current?.objects?.map((d) => (
                <button
                  key={d.id}
                  className="escape-object-hit"
                  data-escape-object={d.id}
                  style={{
                    left: `${d.x / 10}%`,
                    top: `${((d.y - 55) / 666.667) * 100}%`,
                  }}
                  aria-label={`调查${d.label}`}
                  disabled={!canControl}
                  onPointerEnter={() => setHovered(d.id)}
                  onPointerLeave={() => setHovered("")}
                  onFocus={() => setHovered(d.id)}
                  onBlur={() => setHovered("")}
                  onClick={() => select(d)}
                />
              ))}
              {view.players
                .filter(
                  (p) =>
                    p.hp <= 0 && p.room === current?.id && p.seat !== room.you,
                )
                .map((p) => (
                  <button
                    key={p.seat}
                    className="escape-rescue-hit"
                    style={{
                      left: `${p.x / 10}%`,
                      top: `${(p.y / 666.667) * 100}%`,
                    }}
                    disabled={!canControl}
                    aria-label={`救援${room.seats[p.seat]?.name}`}
                    onClick={() =>
                      send({ type: "rescue", target: `player-${p.seat}` })
                    }
                  >
                    <Heart size={16} />
                    救援
                  </button>
                ))}
            </div>
            {!art && (
              <div className="escape-art-loading">
                <Compass size={32} />
                <span>正在点亮密室…</span>
              </div>
            )}
            {view.paused && (
              <div className="escape-paused">
                <Pause size={30} />
                <strong>探索已暂停</strong>
                <button
                  onClick={() => send({ type: "resume" })}
                  disabled={room.spectating || !!room.seats[room.you]?.autoPlay}
                >
                  投票恢复 · {view.votes.filter(Boolean).length}/{manualSeats}
                </button>
              </div>
            )}
            {warning && (
              <div
                className={`escape-incident ${warning.room === current?.id ? "here" : ""}`}
              >
                <Radio size={14} />
                <span>
                  {view.rooms?.find((r) => r.id === warning.room)?.title ??
                    "未知区域"}{" "}
                  ·{" "}
                  {view.tick < warning.begins
                    ? `${Math.ceil((warning.begins - view.tick) / 1000)} 秒后${warning.kind === "steam" ? "蒸汽泄漏" : "潮水涌入"}`
                    : warning.kind === "steam"
                      ? "蒸汽泄漏中"
                      : "潮水涌入中"}
                </span>
              </div>
            )}
            {device && (
              <div
                className="escape-device-panel"
                data-escape-dialog
                role="dialog"
                aria-label={device.label}
              >
                <div className="escape-panel-header">
                  <span className="escape-panel-icon">
                    <Sparkles size={19} />
                  </span>
                  <div>
                    <small>{current?.title}</small>
                    <h3>{device.label}</h3>
                  </div>
                  {closeBtn}
                </div>
                <p>{device.description}</p>
                {me && near(me, device) > 82 ? (
                  <div className="escape-approaching">
                    <i />
                    正在走近装置…
                  </div>
                ) : (
                  <>
                    {device.dials && (
                      <>
                        <div className="escape-dials">
                          {device.dials.map((n, index) => (
                            <button
                              key={index}
                              aria-label={`旋钮 ${index + 1}，当前 ${n + 1}`}
                              disabled={!canControl || device.done}
                              onClick={() =>
                                send({ type: "turn", target: device.id, index })
                              }
                            >
                              <i
                                style={
                                  {
                                    "--dial-angle": `${-135 + n * 90}deg`,
                                  } as CSSProperties
                                }
                              />
                              <b>{n + 1}</b>
                              <small>
                                {device.kind === "pump"
                                  ? ["入水", "旁路", "排放"][index]
                                  : view.options.scenario === "archive" &&
                                      device.kind === "power"
                                    ? ["内环", "中环", "外环"][index]
                                    : ["左路", "中路", "右路"][index]}
                              </small>
                            </button>
                          ))}
                        </div>
                        <div className="escape-clue-slip">
                          <BookOpen size={15} />
                          <span>
                            {view.journal?.find((c) => c.key === device.kind)
                              ?.text ?? "尚未找到这台装置的校准记录"}
                          </span>
                        </div>
                      </>
                    )}
                    {device.kind === "power" && (
                      <div className="escape-circuits">
                        <span>配电 · 最多接通两路</span>
                        <div>
                          {[
                            ["lights", "照明"],
                            ["pump", "排水"],
                            ["lift", "升降"],
                          ].map(([c, label]) => (
                            <button
                              key={c}
                              aria-label={label + "供电"}
                              aria-pressed={view.circuits?.includes(c) ?? false}
                              disabled={
                                !canControl ||
                                (!view.circuits?.includes(c) &&
                                  (view.circuits?.length ?? 0) >= 2)
                              }
                              onClick={() =>
                                send({
                                  type: "circuit",
                                  target: device.id,
                                  circuit: c,
                                })
                              }
                            >
                              <i />
                              {label}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}
                    {device.kind === "extract" ? (
                      <div className="escape-exit-choices">
                        {me?.carry ? (
                          <button
                            className="escape-primary"
                            disabled={!canControl}
                            onClick={() =>
                              send({ type: "interact", target: device.id })
                            }
                          >
                            交付密封档案 <Package size={17} />
                          </button>
                        ) : (
                          view.routes?.map((r) => (
                            <button
                              key={r.id}
                              disabled={!canControl || !r.available}
                              onClick={() => {
                                send({ type: "plan", route: r.id });
                                send({
                                  type: "interact",
                                  target: device.id,
                                  route: r.id,
                                });
                              }}
                            >
                              <span>
                                <strong>{r.label}</strong>
                                <small>{r.reason}</small>
                              </span>
                              {r.available ? (
                                <ArrowUpRight size={18} />
                              ) : (
                                <ShieldCheck size={18} />
                              )}
                            </button>
                          ))
                        )}
                        {me?.ready && <p>你已就绪，等待队友集合。</p>}
                      </div>
                    ) : device.kind === "relay" ? (
                      <>
                        <p className="escape-clue-slip">
                          <Radio size={16} />
                          {device.heldUntil && device.heldUntil > view.tick
                            ? "本端已接通，请留在附近。"
                            : "另一位队友需要同时操作另一端。"}
                        </p>
                        <button
                          className="escape-primary"
                          disabled={
                            !canControl ||
                            !view.objectives?.find((o) => o.id === "power")
                              ?.done ||
                            !!view.objectives?.find((o) => o.id === "relay")
                              ?.done
                          }
                          onClick={() =>
                            send({ type: "interact", target: device.id })
                          }
                        >
                          {view.objectives?.find((o) => o.id === "relay")?.done
                            ? "联锁已完成"
                            : view.objectives?.find((o) => o.id === "power")
                                  ?.done
                              ? "接通联锁"
                              : "等待主设备恢复"}{" "}
                          <Zap size={16} />
                        </button>
                      </>
                    ) : device.dials ? (
                      <button
                        className="escape-primary"
                        disabled={
                          !canControl ||
                          device.done ||
                          (device.kind === "power" &&
                            view.options.scenario === "station" &&
                            view.supplies.fuses < 2)
                        }
                        onClick={() =>
                          send({ type: "interact", target: device.id })
                        }
                      >
                        {device.done
                          ? "设备已恢复"
                          : device.kind === "power" &&
                              view.options.scenario === "station" &&
                              view.supplies.fuses < 2
                            ? "需两枚保险丝"
                            : "确认校准"}{" "}
                        {device.done ? (
                          <Check size={17} />
                        ) : (
                          <ChevronRight size={17} />
                        )}
                      </button>
                    ) : (
                      <div className="escape-clue-content">
                        {view.journal?.find(
                          (c) =>
                            (c.room === current?.id &&
                              c.title === device.label) ||
                            (device.kind === "briefing" &&
                              c.key === "briefing"),
                        )?.text ??
                          (device.done ? "物资已由队伍收集。" : "正在调查…")}
                      </div>
                    )}
                  </>
                )}
              </div>
            )}
          </div>
          <div className="escape-under-scene">
            <div className="escape-dispatch">
              <span className="escape-radio-dot" />
              <span>
                {warning && warning.room === current?.id
                  ? view.tick < warning.begins
                    ? `注意：${Math.ceil((warning.begins - view.tick) / 1000)} 秒后${warning.kind === "steam" ? "蒸汽泄漏" : "潮水涌入"}，请准备撤离。`
                    : "当前房间发生险情，请撤离或救援队友。"
                  : !view.paused && votes > 0
                    ? `暂停提议 ${votes}/${manualSeats}，点击暂停按钮加入。`
                    : (lastEvent?.text ?? "一起找到离开的路。")}
              </span>
            </div>
            <div className="escape-toolstrip" data-game-controls>
              <button
                aria-label="设施地图"
                onClick={() => {
                  setSelected("");
                  setModal("map");
                }}
              >
                <MapIcon size={17} />
                <span>地图</span>
                <kbd>M</kbd>
              </button>
              <button
                aria-label="调查笔记"
                onClick={() => {
                  setSelected("");
                  setModal("journal");
                }}
              >
                <BookOpen size={17} />
                <span>笔记</span>
                <b>{view.journal?.length ?? 0}</b>
              </button>
              <button
                aria-label="物品栏"
                onClick={() => {
                  setSelected("");
                  setModal("inventory");
                }}
              >
                <Package size={17} />
                <span>物品栏</span>
              </button>
              <button
                aria-label="呼叫队友"
                disabled={!me || room.spectating || view.phase !== "playing"}
                onClick={() => send({ type: "ping" })}
              >
                <Radio size={17} />
                <span>呼叫</span>
              </button>
            </div>
          </div>
          <div className="escape-touch-controls" data-game-controls>
            <div className="escape-direction-pad">
              {[
                { d: "up", Icon: ArrowUp },
                { d: "left", Icon: ArrowLeft },
                { d: "down", Icon: ArrowDown },
                { d: "right", Icon: ArrowRight },
              ].map(({ d, Icon }) => (
                <button
                  key={d}
                  className={d + (pressed.includes(d) ? " held" : "")}
                  aria-label={`向${{ up: "上", down: "下", left: "左", right: "右" }[d]}移动`}
                  disabled={!canControl}
                  onPointerDown={(e) => touchDown(e, d)}
                  onPointerUp={touchUp}
                  onPointerCancel={touchUp}
                  onLostPointerCapture={touchUp}
                >
                  <Icon size={19} />
                </button>
              ))}
            </div>
            <span className="escape-touch-location">
              {me?.task ?? (me?.carry ? "携带密封档案" : "点击地面移动")}
              <small>{view.stats.explored} / 9 个房间已探索</small>
            </span>
            <button
              className="escape-interact-button"
              disabled={!canControl}
              onClick={() => {
                if (!me || !current) return;
                const d = [...(current.objects ?? [])].sort(
                  (a, b) => near(me, a) - near(me, b),
                )[0];
                if (d) select(d);
              }}
            >
              <Sparkles size={21} />
              交互
            </button>
          </div>
        </div>
        <aside className="escape-sidebar">
          <div className="escape-plan-heading">
            <span className="escape-section-rule" />
            <h3>撤离计划</h3>
            <small>任选一条</small>
          </div>
          <div className="escape-route-cards">
            {view.routes?.map((r, i) => (
              <button
                className={`escape-route-card ${r.available ? "available" : ""}`}
                key={r.id}
                aria-label={`计划从${r.label}撤离`}
                aria-pressed={view.stats.route === r.id}
                disabled={!canControl}
                onClick={() => send({ type: "plan", route: r.id })}
              >
                <span className="escape-route-icon">
                  {r.available ? (
                    <Check size={19} />
                  ) : i === 0 ? (
                    <Compass size={19} />
                  ) : (
                    <ShieldCheck size={19} />
                  )}
                </span>
                <strong>{r.label}</strong>
                <small>{r.reason}</small>
                {(r.available || view.stats.route === r.id) && (
                  <span className="escape-route-ready">
                    {view.stats.route === r.id ? "队伍计划" : "路线就绪"}
                  </span>
                )}
              </button>
            ))}
          </div>
          <div className="escape-objective-list">
            {view.objectives?.map((o) => (
              <div key={o.id} className={o.done ? "done" : ""}>
                <span>{o.done ? <Check size={11} /> : null}</span>
                <small>{o.label}</small>
                {o.progress && <b>{o.progress}</b>}
              </div>
            ))}
          </div>
          <button
            className="escape-map-preview"
            aria-label="展开设施地图"
            onClick={() => {
              setSelected("");
              setModal("map");
            }}
          >
            <MiniMap view={view} current={current?.id ?? ""} />
            <span>
              设施地图 <ArrowUpRight size={14} />
            </span>
          </button>
          <div className="escape-crew-heading">
            <span>同行者</span>
            <small>{room.seats.length} 人</small>
          </div>
          <div className="escape-crew">
            {view.players.map((p) => (
              <div key={p.seat} className={p.seat === room.you ? "you" : ""}>
                <i
                  className="escape-portrait"
                  style={{
                    backgroundImage: `url(${imageBase}characters.webp)`,
                    backgroundPosition: `0 -${p.seat * 54}px`,
                  }}
                />
                <span>
                  <strong style={{ color: playerColors[p.seat] }}>
                    {room.seats[p.seat]?.name}
                  </strong>
                  <small>
                    {view.rooms?.find((r) => r.id === p.room)?.title ??
                      "未知房间"}
                    {p.carry ? " · 携带档案" : ""}
                  </small>
                </span>
                <span className="escape-crew-health">
                  <Heart size={10} />
                  {Math.ceil(p.hp)}
                </span>
              </div>
            ))}
          </div>
        </aside>
      </div>
      {error && (
        <div className="escape-error" role="alert">
          {error}
          <button aria-label="关闭操作提示" onClick={() => setError("")}>
            <X size={14} />
          </button>
        </div>
      )}
      {modal && (
        <div className="escape-modal-shade" onClick={close}>
          <div
            className={`escape-modal ${modal}`}
            data-escape-dialog
            role="dialog"
            aria-modal="true"
            aria-label={modalTitle}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="escape-modal-heading">
              <span>
                {modal === "map" ? (
                  <MapIcon size={21} />
                ) : modal === "inventory" ? (
                  <Package size={21} />
                ) : (
                  <BookOpen size={21} />
                )}
              </span>
              <div>
                <small>{scenarioNames[view.options.scenario]}</small>
                <h3>{modalTitle}</h3>
              </div>
              <button
                aria-label={`关闭${modalTitle}`}
                data-controller-cancel
                className="escape-close"
                onClick={close}
              >
                <X size={20} />
              </button>
            </div>
            {warning && warning.room === current?.id && (
              <div className="escape-modal-warning">
                当前房间出现险情，请留意预警并及时撤离。
              </div>
            )}
            {modal === "map" ? (
              <>
                <div data-escape-map className="escape-large-map">
                  <svg viewBox="0 0 600 400" aria-hidden="true">
                    {view.rooms
                      ?.filter((r) => r.known)
                      .flatMap((r) =>
                        (r.doors ?? [])
                          .filter((d) => r.id < d.room)
                          .map((d) => {
                            const other = view.rooms?.find(
                              (t) => t.id === d.room,
                            );
                            return other ? (
                              <path
                                key={r.id + d.room}
                                d={`M${r.x * 180 + 120} ${r.y * 125 + 70}L${other.x * 180 + 120} ${other.y * 125 + 70}`}
                              />
                            ) : null;
                          }),
                      )}
                  </svg>
                  {view.rooms?.map((r) => (
                    <button
                      key={r.id}
                      className={`${r.known ? "known" : ""} ${r.id === current?.id ? "current" : ""}`}
                      style={{
                        left: `${r.x * 30 + 20}%`,
                        top: `${r.y * 31.25 + 17.5}%`,
                      }}
                      disabled={
                        !canControl ||
                        !current?.doors?.some((d) => d.room === r.id)
                      }
                      onClick={() => travel(r.id)}
                    >
                      <span>
                        {r.known ? (
                          r.kind === "exit" ? (
                            <ArrowUpRight size={19} />
                          ) : r.id === current?.id ? (
                            <Compass size={19} />
                          ) : (
                            <span className="escape-map-room-mark" />
                          )
                        ) : (
                          "?"
                        )}
                      </span>
                      <strong>{r.title ?? "未探索"}</strong>
                      <small>
                        {view.players
                          .filter((p) => p.room === r.id)
                          .map((p) => room.seats[p.seat]?.name)
                          .join("、")}
                      </small>
                    </button>
                  ))}
                </div>
                <p className="escape-map-caption">
                  点击相邻房间，沿通道前往。队员共享调查结果。
                </p>
                <div className="escape-map-plans">
                  {view.routes?.map((r) => (
                    <button
                      key={r.id}
                      aria-label={`计划从${r.label}撤离`}
                      aria-pressed={view.stats.route === r.id}
                      disabled={!canControl}
                      onClick={() => send({ type: "plan", route: r.id })}
                    >
                      <Compass size={15} />
                      <span>
                        <strong>{r.label}</strong>
                        <small>{r.reason}</small>
                      </span>
                      {view.stats.route === r.id && <Check size={15} />}
                    </button>
                  ))}
                </div>
              </>
            ) : modal === "inventory" ? (
              <Inventory
                view={view}
                player={me}
                canControl={canControl}
                useItem={(type) => send({ type })}
              />
            ) : (
              <div className="escape-journal-pages">
                {view.journal?.length ? (
                  view.journal.map((c) => (
                    <article key={c.key}>
                      <BookOpen size={18} />
                      <small>
                        {view.rooms?.find((r) => r.id === c.room)?.title}
                      </small>
                      <h4>{c.title}</h4>
                      <p>{c.text}</p>
                      {c.values && (
                        <div className="escape-note-values">
                          {c.values.map((v, i) => (
                            <b key={i}>{v + 1}</b>
                          ))}
                        </div>
                      )}
                    </article>
                  ))
                ) : (
                  <div className="escape-empty-notes">
                    <BookOpen size={36} />
                    <p>还没有调查记录。先查看大厅里的值守记录。</p>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </section>
  );
}
function Inventory({
  view,
  player,
  canControl,
  useItem,
}: {
  view: View;
  player?: Player;
  canControl: boolean;
  useItem: (type: "cell" | "med") => void;
}) {
  const items = [
    {
      key: "cells",
      name: "备用电池",
      count: view.supplies.cells,
      Icon: Battery,
      description: "为全队恢复 30% 电力。",
      action: "cell" as const,
      label: "使用共享电池",
      unavailable: view.power >= 99 ? "电力充足" : "使用电池",
      disabled: view.power >= 99,
    },
    {
      key: "meds",
      name: "急救包",
      count: view.supplies.meds,
      Icon: Heart,
      description: "为自己恢复 50 点生命。",
      action: "med" as const,
      label: "使用急救包",
      unavailable: player?.hp === 100 ? "无需治疗" : "使用急救包",
      disabled: player?.hp === 100,
    },
    view.options.scenario === "station"
      ? {
          key: "fuses",
          name: "保险丝",
          count: view.supplies.fuses,
          Icon: Package,
          description: "修复备用发电机时消耗两枚。",
        }
      : {
          key: "documents",
          name: "已运回档案",
          count: view.supplies.documents,
          Icon: FileText,
          description: "已送达撤离装置，救生艇需要两份。",
        },
  ];
  return (
    <div className="escape-inventory" data-escape-inventory>
      <h4>队伍物资</h4>
      <div className="escape-inventory-grid">
        {items.map((item) => (
          <article key={item.key} data-escape-item={item.key}>
            <div className="escape-item-heading">
              <item.Icon size={21} />
              <h4>{item.name}</h4>
              <b data-escape-item-count>{item.count}</b>
            </div>
            <p>{item.description}</p>
            {item.action && (
              <button
                aria-label={item.label}
                disabled={!canControl || item.count < 1 || item.disabled}
                onClick={() => useItem(item.action!)}
              >
                {item.count < 1 ? "暂无库存" : item.unavailable}
              </button>
            )}
          </article>
        ))}
      </div>
      <section className="escape-carried-item">
        <h4>携带物</h4>
        <p>{!player ? "观战中" : player.carry ? "密封档案" : "未携带物品"}</p>
        {player?.carry && <small>送至撤离装置后交付。</small>}
      </section>
    </div>
  );
}
function MiniMap({ view, current }: { view: View; current: string }) {
  return (
    <svg viewBox="0 0 220 120" aria-hidden="true">
      {view.rooms
        ?.filter((r) => r.known)
        .flatMap((r) =>
          (r.doors ?? [])
            .filter((d) => r.id < d.room)
            .map((d) => {
              const other = view.rooms?.find((t) => t.id === d.room);
              return other ? (
                <path
                  key={r.id + d.room}
                  d={`M${r.x * 63 + 47} ${r.y * 35 + 24}L${other.x * 63 + 47} ${other.y * 35 + 24}`}
                  stroke="#586a5d"
                  strokeWidth="2"
                />
              ) : null;
            }),
        )}
      {view.rooms?.map((r) => (
        <g key={r.id}>
          <rect
            x={r.x * 63 + 25}
            y={r.y * 35 + 12}
            width="44"
            height="25"
            rx="5"
            fill={
              r.id === current ? "#bca26c" : r.known ? "#3c554b" : "#1a302c"
            }
            stroke={r.known ? "#b6ab83" : "#536255"}
            strokeDasharray={r.known ? "" : "3 3"}
          />
          {r.id === current && (
            <circle
              cx={r.x * 63 + 47}
              cy={r.y * 35 + 24}
              r="4"
              fill="#122f2e"
            />
          )}
          {r.known && r.kind === "exit" && (
            <path
              d={`m${r.x * 63 + 42} ${r.y * 35 + 29} 9-9m-7 0h7v7`}
              fill="none"
              stroke="#efdab1"
              strokeWidth="2"
            />
          )}
        </g>
      ))}
    </svg>
  );
}
