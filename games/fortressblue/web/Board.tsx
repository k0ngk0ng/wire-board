import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from "react";
import {
  ChevronLeft,
  ChevronRight,
  Crosshair,
  Minus,
  Plus,
  RotateCcw,
} from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import {
  clamp,
  explosion,
  itemById,
  mapById,
  maps,
  sourceList,
  tankInfo,
  tankName,
  tankPortraits,
  tanks as tankCatalog,
  sounds,
  type View,
} from "./types";
import { aimFromPoint, pointerWorld, type Camera } from "./aim";
import { centeredCamera, panCamera, zoomAt } from "./camera";
import {
  beginCharge,
  cancelCharge,
  chargePower,
  releaseCharge,
  finishFullCharge,
  CHARGE_HZ,
  acceptsGameKeys,
  type Charge,
} from "./controls";
import { arcAt, renderableShot } from "./timeline";
import { shouldRevealBoard } from "./reveal";
import {
  animations,
  drawScene,
  miniRect,
  paintTerrain,
  type Images,
  type Motion,
} from "./render";

const cache = new Map<string, Promise<HTMLImageElement | null>>();
function loadImage(base: string, path: string) {
  if (!path) return Promise.resolve(null);
  const sources = sourceList(base, path),
    key = sources.join("|");
  if (cache.has(key)) return cache.get(key)!;
  const promise = new Promise<HTMLImageElement | null>((resolve) => {
    const img = new Image();
    img.decoding = "async";
    let i = 0;
    img.onload = () => resolve(img);
    img.onerror = () => {
      if (++i < sources.length) img.src = sources[i];
      else resolve(null);
    };
    img.src = sources[0];
  });
  cache.set(key, promise);
  return promise;
}
function useAssets(base: string, paths: string[]) {
  const [images, setImages] = useState<Images>({});
  const key = paths.join("|");
  useEffect(() => {
    let alive = true;
    setImages({});
    for (const path of paths)
      void loadImage(base, path).then((img) => {
        if (alive) setImages((current) => ({ ...current, [path]: img }));
      });
    return () => {
      alive = false;
    };
  }, [base, key]);
  return images;
}
function FallbackImage({
  base,
  path,
  ...props
}: {
  base: string;
  path: string;
  alt: string;
  className?: string;
}) {
  const sources = sourceList(base, path);
  return (
    <img
      {...props}
      src={sources[0]}
      onError={(event) => {
        const el = event.currentTarget;
        const next = Number(el.dataset.fallback ?? 0) + 1;
        if (next < sources.length) {
          el.dataset.fallback = String(next);
          el.src = sources[next];
        }
      }}
    />
  );
}
function nowAt(clock: BoardProps<View>["clock"]) {
  return clock.serverNow + Math.max(0, performance.now() - clock.receivedAt);
}
const seconds = (ms: number) => Math.max(0, Math.ceil(ms / 1000));
const newCharge = (): Charge => ({ start: null, token: "", source: "" });
type Pointer = {
  x: number;
  y: number;
  startX: number;
  startY: number;
  dragged: boolean;
  mini: boolean;
};

export function Board(props: BoardProps<View>) {
  const { data, room, act, busy, assets, sound, audio, clock } = props;
  const map = mapById.get(data.mapId) ?? maps[0];
  const own = data.tanks.find((t) => t.seat === data.you);
  const active = data.tanks.find((t) => t.seat === data.current);
  const info = tankInfo(own?.tankId ?? 0);
  const minAngle = info?.minAngle ?? 0,
    maxAngle = info?.maxAngle ?? info?.angle ?? 80;
  const [angle, setAngle] = useState(45),
    [facing, setFacing] = useState(1),
    [weapon, setWeapon] = useState<"primary" | "special">("primary");
  const [power, setPower] = useState(0),
    [lastPower, setLastPower] = useState<number | null>(null),
    [charging, setCharging] = useState(false);
  const [tick, setTick] = useState(0),
    [size, setSize] = useState({ w: 800, h: 450 });
  const [reduced, setReduced] = useState(false);
  const boardRef = useRef<HTMLDivElement>(null);
  const revealedGames = useRef(new Set<string>());
  const canvasRef = useRef<HTMLCanvasElement>(null),
    terrainRef = useRef<HTMLCanvasElement>(null);
  const cameraRef = useRef<Camera>({
    x: 0,
    y: 0,
    worldViewW: 800,
    worldViewH: 450,
  });
  const initialized = useRef(""),
    manualCamera = useRef(false),
    lastFrame = useRef(0);
  const charge = useRef(newCharge()),
    moveDir = useRef(0),
    requestInFlight = useRef(false),
    firedTurn = useRef("");
  const pointers = useRef(new Map<number, Pointer>()),
    gestureWasPinch = useRef(false);
  const motions = useRef(new Map<number, Motion>()),
    previous = useRef<View | null>(null);
  const audioCursor = useRef({ epoch: "", id: 0 }),
    music = useRef("");
  const turnToken = `${data.epoch}:${data.turn}:${data.current}`;
  const permitted =
    data.phase === "playing" &&
    data.you === data.current &&
    !data.resolution &&
    !room.seats[data.you]?.autoPlay &&
    !!own?.alive;
  const canFire =
    permitted && data.canFire && !busy && firedTurn.current !== turnToken;
  const canMove = permitted && data.canMove && !busy;
  const latest = useRef({
    props,
    angle,
    facing,
    weapon,
    minAngle,
    maxAngle,
    turnToken,
    permitted,
    canFire,
    canMove,
  });
  latest.current = {
    props,
    angle,
    facing,
    weapon,
    minAngle,
    maxAngle,
    turnToken,
    permitted,
    canFire,
    canMove,
  };
  const assetPaths = useMemo(
    () =>
      [
        map.bg ?? "",
        map.fg,
        map.fg2 ?? "",
        explosion.url,
        ...data.tanks.flatMap((t) => [
          ...(animations[String(t.tankId)]?.atlases ?? []),
          tankInfo(t.tankId)?.bullet?.url ?? "",
        ]),
      ].filter((p, i, a) => p && a.indexOf(p) === i),
    [map, data.tanks.map((t) => t.tankId).join(",")],
  );
  const images = useAssets(assets, assetPaths);
  const fg = images[map.fg] ?? null,
    fg2 = map.fg2 ? (images[map.fg2] ?? null) : null,
    bg = map.bg ? (images[map.bg] ?? null) : null;
  const craterKey = JSON.stringify(data.craters ?? []);
  useEffect(() => {
    if (terrainRef.current)
      paintTerrain(terrainRef.current, map, fg, fg2, data.craters ?? []);
  }, [map, fg, fg2, craterKey]);

  function cancelInputs() {
    cancelCharge(charge.current);
    moveDir.current = 0;
    setCharging(false);
  }
  function startCharge(source: string) {
    if (
      !latest.current.canFire ||
      requestInFlight.current ||
      firedTurn.current === latest.current.turnToken
    )
      return;
    if (
      beginCharge(
        charge.current,
        performance.now(),
        latest.current.turnToken,
        source,
        tankInfo(
          latest.current.props.data.tanks.find(
            (t) => t.seat === latest.current.props.data.you,
          )?.tankId ?? 0,
        )?.range ?? 45,
      )
    ) {
      moveDir.current = 0;
      setPower(0);
      setCharging(true);
    }
  }
  function endCharge(source: string) {
    const current = latest.current;
    const value = releaseCharge(
      charge.current,
      performance.now(),
      current.turnToken,
      source,
    );
    if (value !== null) firePower(value);
  }
  function firePower(value: number) {
    const current = latest.current;
    setCharging(false);
    if (
      !current.canFire ||
      requestInFlight.current ||
      document.hidden ||
      firedTurn.current === current.turnToken
    )
      return;
    firedTurn.current = current.turnToken;
    setPower(value);
    setLastPower(value);
    requestInFlight.current = true;
    void current.props
      .act({
        type: "fire",
        epoch: current.props.data.epoch,
        turn: current.props.data.turn,
        angle: clamp(current.angle, current.minAngle, current.maxAngle),
        power: value,
        facing: current.facing,
        weapon: current.weapon,
      })
      .catch(() => {
        if (firedTurn.current === current.turnToken) firedTurn.current = "";
      })
      .finally(() => {
        requestInFlight.current = false;
        setTick((n) => n + 1);
      });
  }
  function changeAngle(value: number) {
    const n = clamp(value, latest.current.minAngle, latest.current.maxAngle);
    latest.current.angle = n;
    setAngle(n);
  }
  function changeFacing(value: number) {
    latest.current.facing = value;
    setFacing(value);
  }
  function setMove(dir: number) {
    if (latest.current.permitted && charge.current.start === null) {
      moveDir.current = dir;
      changeFacing(dir);
    }
  }
  function resetCamera() {
    manualCamera.current = false;
    initialized.current = "";
  }
  function zoom(factor: number, anchor = { x: 0.5, y: 0.5 }) {
    manualCamera.current = true;
    cameraRef.current = zoomAt(
      cameraRef.current,
      factor,
      anchor,
      map.width,
      map.height,
    );
  }

  // Enter the playable board once per room/match, without moving the selection
  // overlay or fighting the player's scrolling on subsequent snapshots.
  useEffect(() => {
    if (data.phase !== "playing") return;
    const key = `${room.id}:${data.epoch}`;
    if (revealedGames.current.has(key)) return;
    const frame = requestAnimationFrame(() => {
      const board = boardRef.current;
      if (!board) return;
      const rect = board.getBoundingClientRect();
      const height = window.visualViewport?.height ?? window.innerHeight;
      const reveal = shouldRevealBoard(
        rect,
        height,
        data.phase,
        revealedGames.current.has(key),
      );
      revealedGames.current.add(key);
      board.dataset.autoReveal = reveal ? "1" : "0";
      if (reveal)
        board.scrollIntoView({
          block: "center",
          inline: "nearest",
          behavior: "auto",
        });
    });
    return () => cancelAnimationFrame(frame);
  }, [room.id, data.epoch, data.phase]);

  // Preserve angle between the player's own turns; clamp only for a changed tank.
  useEffect(() => {
    setFacing(own?.facing ?? 1);
  }, [data.epoch, own?.tankId]);
  useEffect(() => {
    setAngle((a) => clamp(a, minAngle, maxAngle));
  }, [minAngle, maxAngle]);
  useEffect(() => {
    cancelInputs();
  }, [turnToken, data.resolution?.stage, data.canFire, own?.alive, permitted]);
  useEffect(() => {
    if (!own?.specialUnlimited && own?.special === 0) setWeapon("primary");
  }, [own?.special, own?.specialUnlimited]);
  useEffect(() => {
    firedTurn.current = "";
    setLastPower(null);
    setWeapon("primary");
    manualCamera.current = false;
  }, [data.epoch]);
  useEffect(() => {
    const media = matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const resize = () => {
      const r = canvas.getBoundingClientRect();
      const dpr = Math.min(2, devicePixelRatio || 1);
      canvas.width = Math.round(r.width * dpr);
      canvas.height = Math.round(r.height * dpr);
      setSize({ w: r.width, h: r.height });
      initialized.current = "";
    };
    resize();
    const ro = new ResizeObserver(resize);
    ro.observe(canvas);
    return () => ro.disconnect();
  }, []);
  useEffect(() => {
    const old = previous.current;
    const at = nowAt(clock);
    if (old?.epoch === data.epoch) {
      for (const tank of data.tanks) {
        const before = old.tanks.find((t) => t.seat === tank.seat);
        if (
          before &&
          (before.x !== tank.x || before.y !== tank.y) &&
          !data.resolution?.falls?.some((f) => f.seat === tank.seat) &&
          !old.resolution?.falls?.some((f) => f.seat === tank.seat)
        ) {
          const prior = motions.current.get(tank.seat);
          const t = prior ? clamp((at - prior.at) / 180, 0, 1) : 1;
          motions.current.set(tank.seat, {
            fromX: prior ? prior.fromX + (prior.x - prior.fromX) * t : before.x,
            fromY: prior ? prior.fromY + (prior.y - prior.fromY) * t : before.y,
            x: tank.x,
            y: tank.y,
            at,
          });
        }
      }
    } else motions.current.clear();
    if (data.lastShot && data.lastShot.id !== old?.lastShot?.id)
      motions.current.delete(data.lastShot.seat);
    previous.current = data;
  }, [data]);
  useEffect(() => {
    const timer = window.setInterval(() => {
      setTick((n) => n + 1);
      const state = latest.current;
      if (
        !moveDir.current ||
        !state.canMove ||
        requestInFlight.current ||
        document.hidden
      )
        return;
      const dir = moveDir.current;
      requestInFlight.current = true;
      void state.props
        .act({
          type: "move",
          epoch: state.props.data.epoch,
          turn: state.props.data.turn,
          dir,
        })
        .catch(() => {
          moveDir.current = 0;
        })
        .finally(() => {
          requestInFlight.current = false;
        });
    }, 65);
    const chargeTimer = window.setInterval(() => {
      if (charge.current.start === null) return;
      if (!latest.current.canFire || document.hidden) {
        cancelInputs();
        return;
      }
      const now = performance.now();
      setPower(chargePower(charge.current.start, now, charge.current.maxTicks));
      const full = finishFullCharge(
        charge.current,
        now,
        latest.current.turnToken,
      );
      if (full !== null) firePower(full);
    }, 1000 / CHARGE_HZ);
    const cancel = () => cancelInputs();
    const visibility = () => {
      if (document.hidden) {
        cancel();
        audioCursor.current = {
          epoch: latest.current.props.data.epoch,
          id: Math.max(
            0,
            ...(latest.current.props.data.events ?? []).map((e) => e.id),
          ),
        };
      }
    };
    const focus = (event: FocusEvent) => {
      if (!acceptsGameKeys(event.target as HTMLElement)) cancel();
    };
    window.addEventListener("blur", cancel);
    document.addEventListener("visibilitychange", visibility);
    document.addEventListener("focusin", focus);
    return () => {
      clearInterval(timer);
      clearInterval(chargeTimer);
      window.removeEventListener("blur", cancel);
      document.removeEventListener("visibilitychange", visibility);
      document.removeEventListener("focusin", focus);
    };
  }, []);
  useEffect(() => {
    const down = (event: KeyboardEvent) => {
      const state = latest.current;
      if (
        !state.permitted ||
        !acceptsGameKeys(event.target as HTMLElement) ||
        document.querySelector('[role="dialog"][aria-modal="true"]')
      )
        return;
      const k = event.key.toLowerCase();
      if (
        ![
          " ",
          "arrowup",
          "arrowdown",
          "arrowleft",
          "arrowright",
          "w",
          "s",
          "a",
          "d",
          "q",
          "e",
          "escape",
        ].includes(k)
      )
        return;
      event.preventDefault();
      if (k === "escape") {
        cancelInputs();
        return;
      }
      if (k === " ") {
        if (!event.repeat) startCharge("key");
        return;
      }
      if (charge.current.start !== null) return;
      if (k === "arrowup" || k === "w") changeAngle(state.angle + 1);
      if (k === "arrowdown" || k === "s") changeAngle(state.angle - 1);
      if (k === "arrowleft" || k === "a") setMove(-1);
      if (k === "arrowright" || k === "d") setMove(1);
      if ((k === "q" || k === "e") && !event.repeat) {
        const self = state.props.data.tanks.find(
          (t) => t.seat === state.props.data.you,
        );
        setWeapon((w) =>
          w === "primary" &&
          (self?.specialUnlimited || (self?.special ?? 0) !== 0)
            ? "special"
            : "primary",
        );
      }
    };
    const up = (event: KeyboardEvent) => {
      const k = event.key.toLowerCase();
      if (k === " ") {
        if (charge.current.source === "key") {
          event.preventDefault();
          if (acceptsGameKeys(event.target as HTMLElement)) endCharge("key");
          else cancelInputs();
        }
      }
      if (["arrowleft", "arrowright", "a", "d"].includes(k))
        moveDir.current = 0;
    };
    window.addEventListener("keydown", down);
    window.addEventListener("keyup", up);
    return () => {
      window.removeEventListener("keydown", down);
      window.removeEventListener("keyup", up);
    };
  }, []);
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const wheel = (event: WheelEvent) => {
      event.preventDefault();
      const r = canvas.getBoundingClientRect();
      zoom(Math.exp(clamp(event.deltaY, -160, 160) * 0.002), {
        x: (event.clientX - r.left) / r.width,
        y: (event.clientY - r.top) / r.height,
      });
    };
    canvas.addEventListener("wheel", wheel, { passive: false });
    return () => canvas.removeEventListener("wheel", wheel);
  }, [map]);

  useEffect(() => {
    const events = data.events ?? [],
      max = Math.max(0, ...events.map((e) => e.id)),
      cursor = audioCursor.current;
    if (cursor.epoch !== data.epoch) {
      cursor.epoch = data.epoch;
      cursor.id = max;
      return;
    }
    const now = nowAt(clock);
    if (!sound || document.hidden) {
      cursor.id = max;
      return;
    }
    for (const e of events
      .filter((e) => e.id > cursor.id)
      .sort((a, b) => a.id - b.id)) {
      if (e.at > now) break;
      cursor.id = e.id;
      // A missed old event does not start a delayed soundtrack after reconnect.
      if (now - e.at > 800) continue;
      const key =
        e.type === "shot"
          ? "fire"
          : e.type === "explode"
            ? "explode"
            : e.type === "death"
              ? "hit"
              : e.type === "heal"
                ? "select"
                : e.type === "turn"
                  ? "turn"
                  : e.type === "finish"
                    ? "win"
                    : "";
      const path = sounds[key];
      if (path) void audio.playWithFallback(sourceList(assets, path));
    }
  }, [data.events, data.epoch, sound, tick, assets, audio]);
  useEffect(() => {
    if (sound && map.music && music.current !== `${data.epoch}:${map.id}`) {
      music.current = `${data.epoch}:${map.id}`;
      void audio.music(sourceList(assets, map.music));
    }
  }, [data.epoch, map, assets, sound, audio]);

  const drawState = useRef({
    data,
    map,
    images,
    bg,
    size,
    angle,
    facing,
    canFire,
    reduced,
    room,
    clock,
  });
  drawState.current = {
    data,
    map,
    images,
    bg,
    size,
    angle,
    facing,
    canFire,
    reduced,
    room,
    clock,
  };
  useEffect(() => {
    let raf = 0;
    const frame = () => {
      const canvas = canvasRef.current;
      if (!canvas) return;
      const s = drawState.current,
        now = nowAt(s.clock),
        dt = Math.min(50, performance.now() - lastFrame.current);
      lastFrame.current = performance.now();
      const current = s.data.tanks.find((t) => t.seat === s.data.current) ??
        s.data.tanks.find((t) => t.alive) ?? {
          x: s.map.width / 2,
          y: s.map.height / 2,
        };
      let target = { x: current.x, y: current.y };
      const shot = renderableShot(s.data.lastShot);
      if (s.data.resolution?.shotId && shot) {
        const p = shot.arcs
          .map((a) =>
            arcAt(
              { ...a, impactAt: a.impactAt ?? shot.impactAt },
              shot.at,
              now,
              shot.launchAt ?? shot.at,
            ),
          )
          .find(Boolean);
        // Remain with the impact through the server's fall/settle interval.
        // A delayed snapshot cannot pull the camera back before the hit arrives.
        const last = shot.arcs[shot.arcs.length - 1];
        if (p) target = p;
        else if (last && now >= (shot.launchAt ?? shot.at))
          target = last.impact;
      }
      const key = `${s.data.epoch}:${s.map.id}:${s.size.w}:${s.size.h}`;
      if (initialized.current !== key) {
        cameraRef.current = centeredCamera(
          s.size.w,
          s.size.h,
          Math.min(800, s.size.w * 1.25),
          target,
          s.map.width,
          s.map.height,
        );
        initialized.current = key;
      } else if (!manualCamera.current) {
        const c = cameraRef.current,
          dest = centeredCamera(
            s.size.w,
            s.size.h,
            c.worldViewW,
            target,
            s.map.width,
            s.map.height,
          );
        const blend = s.reduced ? 1 : 1 - Math.exp(-dt / 160);
        cameraRef.current = {
          ...c,
          x: c.x + (dest.x - c.x) * blend,
          y: c.y + (dest.y - c.y) * blend,
        };
      }
      drawScene({
        canvas,
        terrain: terrainRef.current,
        map: s.map,
        view: s.data,
        camera: cameraRef.current,
        images: s.images,
        bg: s.bg,
        now,
        angle: s.angle,
        facing: s.facing,
        canAim: s.canFire,
        names: s.room.seats.map((seat) => seat?.name ?? ""),
        motions: motions.current,
        reduced: s.reduced,
      });
      raf = requestAnimationFrame(frame);
    };
    raf = requestAnimationFrame(frame);
    return () => cancelAnimationFrame(raf);
  }, []);

  function moveMini(event: ReactPointerEvent<HTMLCanvasElement>) {
    const r = event.currentTarget.getBoundingClientRect(),
      mm = miniRect(r.width, map),
      c = cameraRef.current;
    const x = clamp((event.clientX - r.left - mm.x) / mm.w, 0, 1) * map.width,
      y = clamp((event.clientY - r.top - mm.y) / mm.h, 0, 1) * map.height;
    cameraRef.current = panCamera(
      c,
      x - c.worldViewW / 2 - c.x,
      y - c.worldViewH / 2 - c.y,
      map.width,
      map.height,
    );
    manualCamera.current = true;
  }
  function pointerDown(event: ReactPointerEvent<HTMLCanvasElement>) {
    if (event.button !== 0) return;
    event.preventDefault();
    event.currentTarget.focus({ preventScroll: true });
    event.currentTarget.setPointerCapture(event.pointerId);
    const r = event.currentTarget.getBoundingClientRect(),
      mm = miniRect(r.width, map),
      x = event.clientX - r.left,
      y = event.clientY - r.top;
    const mini =
      x >= mm.x - 3 &&
      x <= mm.x + mm.w + 3 &&
      y >= mm.y - 3 &&
      y <= mm.y + mm.h + 3;
    pointers.current.set(event.pointerId, {
      x: event.clientX,
      y: event.clientY,
      startX: event.clientX,
      startY: event.clientY,
      dragged: false,
      mini,
    });
    if (pointers.current.size > 1) {
      gestureWasPinch.current = true;
      for (const p of pointers.current.values()) p.dragged = true;
    }
    if (mini) moveMini(event);
  }
  function pointerMove(event: ReactPointerEvent<HTMLCanvasElement>) {
    const pointer = pointers.current.get(event.pointerId);
    if (!pointer) return;
    const r = event.currentTarget.getBoundingClientRect(),
      other = [...pointers.current.entries()].find(
        ([id]) => id !== event.pointerId,
      )?.[1];
    if (other) {
      const before = Math.hypot(pointer.x - other.x, pointer.y - other.y),
        after = Math.hypot(event.clientX - other.x, event.clientY - other.y);
      if (before > 2 && after > 2) {
        zoom(before / after, {
          x: ((pointer.x + other.x) / 2 - r.left) / r.width,
          y: ((pointer.y + other.y) / 2 - r.top) / r.height,
        });
        const c = cameraRef.current;
        cameraRef.current = panCamera(
          c,
          (-(event.clientX - pointer.x) / 2 / r.width) * c.worldViewW,
          (-(event.clientY - pointer.y) / 2 / r.height) * c.worldViewH,
          map.width,
          map.height,
        );
      }
    } else if (pointer.mini) moveMini(event);
    else if (
      pointer.dragged ||
      Math.hypot(
        event.clientX - pointer.startX,
        event.clientY - pointer.startY,
      ) > 5
    ) {
      pointer.dragged = true;
      manualCamera.current = true;
      const c = cameraRef.current;
      cameraRef.current = panCamera(
        c,
        (-(event.clientX - pointer.x) / r.width) * c.worldViewW,
        (-(event.clientY - pointer.y) / r.height) * c.worldViewH,
        map.width,
        map.height,
      );
    }
    pointer.x = event.clientX;
    pointer.y = event.clientY;
  }
  function pointerEnd(
    event: ReactPointerEvent<HTMLCanvasElement>,
    cancelled = false,
  ) {
    const pointer = pointers.current.get(event.pointerId);
    pointers.current.delete(event.pointerId);
    if (
      pointer &&
      !cancelled &&
      !pointer.dragged &&
      !pointer.mini &&
      !gestureWasPinch.current &&
      canFire &&
      own
    ) {
      const p = pointerWorld(
          event.clientX,
          event.clientY,
          event.currentTarget.getBoundingClientRect(),
          cameraRef.current,
        ),
        aim = aimFromPoint(own, p, maxAngle, 26);
      changeAngle(Math.round(aim.angle));
      changeFacing(aim.facing);
    }
    if (!pointers.current.size) gestureWasPinch.current = false;
  }
  const now = nowAt(clock),
    remaining = data.resolution ? null : seconds(data.deadline - now);
  const isSelect = data.phase === "select";
  const currentName = active
    ? (room.seats[active.seat]?.name ?? `P${active.seat + 1}`)
    : "";
  const shown = own ?? active;
  const order = data.order.filter(
    (seat) => data.tanks.find((t) => t.seat === seat)?.alive,
  );
  const currentIndex = order.indexOf(data.current),
    next =
      currentIndex < 0
        ? order
        : [...order.slice(currentIndex + 1), ...order.slice(0, currentIndex)];
  const status = isSelect
    ? "选择坦克"
    : data.phase === "finished"
      ? "对局结束"
      : data.resolution?.stage === "flying"
        ? now < (data.lastShot?.launchAt ?? 0)
          ? "准备发射"
          : "炮弹飞行中"
        : data.resolution
          ? "等待落地"
          : data.you === data.current
            ? "轮到你了"
            : `${currentName} 行动中`;
  return (
    <div
      ref={boardRef}
      className="fortressblue-board"
      data-fortressblue-phase={data.phase}
      data-fortressblue-turn={data.turn}
      data-fortressblue-shot={data.lastShot?.id ?? 0}
      data-fortressblue-animating={data.resolution ? "1" : "0"}
      data-fortressblue-craters={data.craters?.length ?? 0}
      data-fortressblue-epoch={data.epoch}
      data-fortressblue-map={map.id}
      data-fortressblue-my-turn={canFire ? "1" : "0"}
      data-fortressblue-angle={Math.round(angle)}
      data-fortressblue-power={power}
      data-fortressblue-facing={facing}
      data-fortressblue-preview="0"
      data-fortressblue-art-scale="1"
      data-fortressblue-charging={charging ? "1" : "0"}
      data-fortressblue-charge-ticks={info?.range ?? 45}
      data-fortressblue-charge-hz={CHARGE_HZ}
      data-fortressblue-pending-shot={own?.pendingShotEffect ?? ""}
    >
      <div className="fb-battlefield">
        <canvas
          ref={canvasRef}
          className="fb-canvas"
          data-fortressblue-canvas
          tabIndex={0}
          aria-label={`${map.name}战场。拖动平移，滚轮缩放，点击调整瞄准；按住空格蓄力，松开发射。`}
          onPointerDown={pointerDown}
          onPointerMove={pointerMove}
          onPointerUp={(event) => pointerEnd(event)}
          onPointerCancel={(event) => pointerEnd(event, true)}
          onLostPointerCapture={(event) => pointerEnd(event, true)}
        />
        <canvas ref={terrainRef} className="fb-terrain" aria-hidden="true" />
        <div className="fb-map-caption">
          <strong>{map.name}</strong>
          <span>第 {data.turn} 回合</span>
          {data.sudden && <span>决胜阶段</span>}
        </div>
        <div className="fb-camera-tools" aria-label="地图视野">
          <button onClick={() => zoom(0.8)} aria-label="放大地图">
            <Plus size={16} />
          </button>
          <button onClick={() => zoom(1.25)} aria-label="缩小地图">
            <Minus size={16} />
          </button>
          <button onClick={resetCamera} aria-label="跟随当前坦克">
            <Crosshair size={16} />
          </button>
        </div>
      </div>
      <div className="fb-console">
        <div className="fb-dial-area">
          <svg
            className="fb-angle-dial"
            viewBox="0 0 130 114"
            aria-label={`炮角 ${Math.round(angle)} 度，朝${facing > 0 ? "右" : "左"}`}
          >
            <defs>
              <radialGradient id="fb-dial-glass">
                <stop offset="0" stopColor="#172d3c" />
                <stop offset="1" stopColor="#020c14" />
              </radialGradient>
            </defs>
            <circle
              cx="65"
              cy="57"
              r="53"
              fill="#36668c"
              stroke="#8cb4cb"
              strokeWidth="3"
            />
            <circle
              cx="65"
              cy="57"
              r="46"
              fill="url(#fb-dial-glass)"
              stroke="#0a1c2b"
              strokeWidth="3"
            />
            {[0, 15, 30, 45, 60, 75, 90, 105, 120, 135, 150, 165, 180].map(
              (a) => (
                <line
                  key={a}
                  x1={65 + Math.cos((a * Math.PI) / 180) * 39}
                  y1={57 - Math.sin((a * Math.PI) / 180) * 39}
                  x2={65 + Math.cos((a * Math.PI) / 180) * 44}
                  y2={57 - Math.sin((a * Math.PI) / 180) * 44}
                  stroke="#a9c7d4"
                  strokeWidth="1"
                />
              ),
            )}
            <line x1="20" y1="57" x2="110" y2="57" stroke="#9babb3" />
            {[minAngle, maxAngle].map((a, i) => (
              <line
                key={i}
                x1="65"
                y1="57"
                x2={65 + facing * Math.cos((a * Math.PI) / 180) * 42}
                y2={57 - Math.sin((a * Math.PI) / 180) * 42}
                stroke="#ed6e47"
                strokeWidth="1.5"
              />
            ))}
            <line
              x1="65"
              y1="57"
              x2={65 + facing * Math.cos((angle * Math.PI) / 180) * 40}
              y2={57 - Math.sin((angle * Math.PI) / 180) * 40}
              stroke="#ffdf79"
              strokeWidth="3"
            />
            <circle cx="65" cy="57" r="5" fill="#79adc1" />
            <text
              x="65"
              y="84"
              textAnchor="middle"
              fill="#ffdf79"
              fontSize="19"
              fontFamily="monospace"
            >
              {Math.round(angle)}°
            </text>
          </svg>
          <span className="fb-wind">
            风{" "}
            {data.wind === 0
              ? "· 0"
              : `${data.wind > 0 ? "→" : "←"} ${Math.abs(data.wind)}`}
          </span>
        </div>
        <div className="fb-instruments">
          <div className="fb-turn-strip">
            <strong>{status}</strong>
            <span>
              {own?.pendingShotEffect === "heal"
                ? "待发回血弹"
                : shown
                  ? tankName(shown.tankId)
                  : "等待选择"}
            </span>
            <b className="fb-timer" data-fortressblue-timer>
              {isSelect
                ? `${seconds(data.selectUntil - now)} 秒`
                : data.phase === "finished"
                  ? "结束"
                  : remaining === null
                    ? "—"
                    : `${remaining} 秒`}
            </b>
          </div>
          <div className="fb-gauges">
            <div className="fb-gauge">
              <span>生命</span>
              <div>
                <i
                  className="fb-life-fill"
                  style={{
                    width: `${shown ? clamp((shown.hp / Math.max(1, shown.maxHp)) * 100, 0, 100) : 0}%`,
                  }}
                />
              </div>
              <b>{shown ? `${shown.hp}/${shown.maxHp}` : "—"}</b>
            </div>
            <div className="fb-gauge">
              <span>力度</span>
              <div>
                <i className="fb-power-fill" style={{ width: `${power}%` }} />
                {lastPower !== null && (
                  <em
                    style={{ left: `${lastPower}%` }}
                    title={`上一发力度 ${lastPower}`}
                  />
                )}
              </div>
              <b>{power}</b>
            </div>
            <div className="fb-gauge">
              <span>移动力</span>
              <div>
                <i
                  className="fb-fuel-fill"
                  style={{
                    width: `${shown ? clamp((shown.fuel / Math.max(1, tankInfo(shown.tankId)?.speed ?? 100)) * 100, 0, 100) : 0}%`,
                  }}
                />
              </div>
              <b>{shown ? Math.round(shown.fuel) : "—"}</b>
            </div>
          </div>
        </div>
        <div className="fb-control-row" data-game-controls>
          <div className="fb-angle-controls">
            <label htmlFor="fb-angle">角度</label>
            <button
              disabled={!canFire || charging}
              onClick={() => changeAngle(angle - 1)}
              aria-label="降低炮角"
            >
              −
            </button>
            <input
              id="fb-angle"
              aria-label="角度"
              type="range"
              min={minAngle}
              max={maxAngle}
              value={angle}
              disabled={!canFire || charging}
              onChange={(e) => changeAngle(Number(e.target.value))}
            />
            <button
              disabled={!canFire || charging}
              onClick={() => changeAngle(angle + 1)}
              aria-label="抬高炮角"
            >
              ＋
            </button>
          </div>
          <div className="fb-move-controls">
            {[-1, 1].map((dir) => (
              <button
                key={dir}
                disabled={!canMove || charging}
                aria-label={dir < 0 ? "向左移动" : "向右移动"}
                data-fortressblue-move={dir}
                onPointerDown={(e) => {
                  e.preventDefault();
                  e.currentTarget.setPointerCapture(e.pointerId);
                  setMove(dir);
                }}
                onPointerUp={() => (moveDir.current = 0)}
                onPointerCancel={() => (moveDir.current = 0)}
                onLostPointerCapture={() => (moveDir.current = 0)}
                onKeyDown={(e) => {
                  if (e.key === " " || e.key === "Enter") {
                    e.preventDefault();
                    setMove(dir);
                  }
                }}
                onKeyUp={() => (moveDir.current = 0)}
              >
                {dir < 0 ? (
                  <ChevronLeft size={18} />
                ) : (
                  <ChevronRight size={18} />
                )}
              </button>
            ))}
            <button
              aria-label="切换朝向"
              disabled={!canFire || charging}
              onClick={() => changeFacing(-facing)}
            >
              <RotateCcw size={14} />
              <span>{facing < 0 ? "向左" : "向右"}</span>
            </button>
          </div>
          <div className="fb-weapon-controls">
            {(["primary", "special"] as const).map((value, i) => (
              <button
                key={value}
                data-fortressblue-weapon={value}
                aria-pressed={weapon === value}
                disabled={
                  !canFire ||
                  charging ||
                  (value === "special" &&
                    !own?.specialUnlimited &&
                    own?.special === 0)
                }
                onClick={() => setWeapon(value)}
              >
                {i + 1} 号弹
                {value === "special" &&
                !own?.specialUnlimited &&
                (own?.special ?? -1) >= 0
                  ? ` · ${own?.special}`
                  : ""}
              </button>
            ))}
          </div>
          <button
            className={`fb-fire ${charging ? "charging" : ""}`}
            data-fortressblue-fire
            title="按住蓄力，松开或满力自动发射"
            disabled={!canFire}
            onPointerDown={(e) => {
              e.preventDefault();
              e.currentTarget.setPointerCapture(e.pointerId);
              startCharge(`pointer:${e.pointerId}`);
            }}
            onPointerUp={(e) => endCharge(`pointer:${e.pointerId}`)}
            onPointerCancel={cancelInputs}
            onLostPointerCapture={() => {
              if (charge.current.source.startsWith("pointer:")) cancelInputs();
            }}
            onKeyDown={(e) => {
              if (e.key === " " || e.key === "Enter") {
                e.preventDefault();
                if (!e.repeat) startCharge("button-key");
              }
            }}
            onKeyUp={(e) => {
              if (e.key === " " || e.key === "Enter") {
                e.preventDefault();
                endCharge("button-key");
              }
            }}
          >
            {charging
              ? "松开发射"
              : own?.pendingShotEffect === "heal"
                ? "蓄力回血弹"
                : "按住蓄力"}
          </button>
        </div>
        <div className="fb-lower-strip">
          <div className="fb-items" aria-label="道具">
            {Array.from({ length: 4 }, (_, slot) => {
              const id = own?.items?.[slot],
                item = id !== undefined ? itemById.get(id) : null;
              return item ? (
                <button
                  key={slot}
                  disabled={!canFire || charging}
                  aria-label={`使用${item.name}：${item.description}`}
                  title={item.description}
                  data-fortressblue-item={item.id}
                  onClick={() => {
                    if (!latest.current.canFire || requestInFlight.current)
                      return;
                    requestInFlight.current = true;
                    void act({
                      type: "item",
                      epoch: data.epoch,
                      turn: data.turn,
                      slot,
                    }).finally(() => {
                      requestInFlight.current = false;
                    });
                  }}
                >
                  <FallbackImage base={assets} path={item.icon} alt="" />
                  <span>{item.name}</span>
                </button>
              ) : (
                <span
                  key={slot}
                  className="fb-empty-slot"
                  aria-label="空道具槽"
                >
                  —
                </span>
              );
            })}
          </div>
          <div className="fb-order">
            <span>后续顺序</span>
            <b>
              {next.length
                ? next.map((seat) => `P${seat + 1}`).join(" → ")
                : "—"}
            </b>
          </div>
        </div>
      </div>
      <p className="fb-help">
        方向键移动／调角 · Q／E 换弹 · 按住空格蓄力，松开或满力发射 ·
        拖图与滚轮缩放
      </p>
      <div className="fb-live" aria-live="polite">
        {data.log?.at(-1) ?? "观察风向，调整角度和力度。"}
      </div>
      {isSelect && (
        <div className="fb-draft" role="dialog" aria-label="选择坦克">
          <div className="fb-draft-head">
            <div>
              <h2>选择坦克</h2>
              <p>
                全部选定或 {seconds(data.selectUntil - now)}{" "}
                秒后开战；同款坦克可重复选择。
              </p>
            </div>
            <b>
              {data.tanks.filter((t) => t.selected).length}/{data.tanks.length}
            </b>
          </div>
          <div className="fb-tank-grid">
            {tankCatalog.map((t) => {
              const portrait = tankPortraits[String(t.id)] ?? t.sprites.idle;
              return (
                <button
                  key={t.id}
                  className={`fb-tank-card ${data.selectedTank === t.id ? "selected" : ""}`}
                  disabled={!data.canSelect || busy}
                  onClick={() =>
                    void act({ type: "select", epoch: data.epoch, tank: t.id })
                  }
                  data-fortressblue-tank={t.id}
                >
                  <FallbackImage
                    base={assets}
                    path={portrait?.url ?? ""}
                    alt=""
                  />
                  <strong>{t.name}</strong>
                  <span>
                    生命 {t.hp} · 炮角 {t.minAngle ?? 0}–{t.maxAngle ?? t.angle}
                    °
                  </span>
                  <small>
                    {t.skillName ? `${t.skillName}特殊弹` : "双弹种炮击"}
                  </small>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
