import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import {
  classInfo,
  classOrder,
  clamp,
  mapNames,
  sources,
  teamColors,
  teamNames,
  type Shot,
  type Tank,
  type View,
} from "./types";

type FrozenTank = { hp: number; alive: boolean };
type Animation = {
  shot: Shot;
  start: number;
  duration: number;
  impactAt: number;
  frozen: FrozenTank[];
  oldHeights: number[];
  reduced: boolean;
};

const mapStyle: Record<
  string,
  { sky: [string, string]; ground: [string, string]; water: string; sun: string }
> = {
  hills: {
    sky: ["#dbe7e2", "#f0e8d6"],
    ground: ["#7e8b70", "#53634f"],
    water: "#6d94a0",
    sun: "#e9d493",
  },
  canyon: {
    sky: ["#e6d8c4", "#f2e8d4"],
    ground: ["#b08360", "#76513e"],
    water: "#6d8f99",
    sun: "#e3bd7e",
  },
  islands: {
    sky: ["#cfe3e4", "#e6ede0"],
    ground: ["#8a9b74", "#536b58"],
    water: "#5e95a2",
    sun: "#f0dca0",
  },
  snow: {
    sky: ["#dce5e8", "#eef0ea"],
    ground: ["#d6dedb", "#94a8a4"],
    water: "#7899a7",
    sun: "#f4e5b8",
  },
  volcano: {
    sky: ["#d7c8c0", "#e7d9c6"],
    ground: ["#7d6b5a", "#4d443d"],
    water: "#8a604f",
    sun: "#e2a878",
  },
};

function tankName(room: BoardProps<View>["room"], seat: number) {
  return room.seats[seat]?.name || `${seat + 1} 号位`;
}

function drawTank(
  ctx: CanvasRenderingContext2D,
  t: Tank,
  hp: number,
  alive: boolean,
  current: boolean,
  aim: number,
) {
  const x = t.x;
  const y = t.y;
  const color = teamColors[t.team];
  const dark = t.team === 0 ? "#65382f" : "#314c66";
  ctx.save();
  ctx.translate(x, y);
  if (!alive) {
    ctx.rotate(t.facing < 0 ? -0.08 : 0.08);
    ctx.fillStyle = "#3f463e";
    ctx.fillRect(-12, -8, 24, 7);
    ctx.fillStyle = "#252b26";
    ctx.fillRect(-8, -13, 12, 6);
    ctx.restore();
    return;
  }
  const wide = t.class === "heavy" ? 15 : t.class === "scout" ? 11 : 13;
  ctx.fillStyle = "#303a31";
  ctx.fillRect(-wide, -1, wide * 2, 7);
  ctx.fillStyle = color;
  ctx.strokeStyle = dark;
  ctx.lineWidth = 1.2;
  if (t.class === "heavy") {
    ctx.fillRect(-wide, -9, wide * 2, 9);
    ctx.strokeRect(-wide, -9, wide * 2, 9);
  } else if (t.class === "support") {
    ctx.fillRect(-wide, -9, wide * 2, 9);
    ctx.strokeRect(-wide, -9, wide * 2, 9);
    ctx.fillStyle = "#c7b98c";
    ctx.fillRect(-5, -13, 10, 5);
  } else if (t.class === "scout") {
    ctx.beginPath();
    ctx.moveTo(-wide, -3);
    ctx.lineTo(-wide + 3, -10);
    ctx.lineTo(wide - 3, -10);
    ctx.lineTo(wide, -3);
    ctx.closePath();
    ctx.fill();
    ctx.stroke();
  } else {
    ctx.fillRect(-wide, -8, wide * 2, 8);
    ctx.strokeRect(-wide, -8, wide * 2, 8);
  }
  ctx.fillStyle = dark;
  ctx.beginPath();
  ctx.arc(0, -9, t.class === "heavy" ? 6 : 5, Math.PI, 0);
  ctx.fill();
  const length = t.class === "sniper" ? 20 : t.class === "heavy" ? 13 : 16;
  const rad = ((current ? aim : 38) * Math.PI) / 180;
  ctx.strokeStyle = "#3b443b";
  ctx.lineWidth = t.class === "sniper" ? 2 : 3;
  ctx.beginPath();
  ctx.moveTo(0, -10);
  ctx.lineTo(
    Math.cos(rad) * length * t.facing,
    -10 - Math.sin(rad) * length,
  );
  ctx.stroke();
  ctx.beginPath();
  ctx.arc(0, 0, 1.8, 0, Math.PI * 2);
  ctx.fillStyle = "#d9c79e";
  ctx.fill();
  if (current) {
    ctx.fillStyle = "#f0d27c";
    ctx.beginPath();
    ctx.moveTo(0, -25);
    ctx.lineTo(-4, -20);
    ctx.lineTo(4, -20);
    ctx.closePath();
    ctx.fill();
  }
  const ratio = clamp(hp / Math.max(1, t.maxHp), 0, 1);
  ctx.fillStyle = "#2f3932";
  ctx.fillRect(-wide, -19, wide * 2, 3);
  ctx.fillStyle = ratio > 0.55 ? "#8fbf86" : ratio > 0.25 ? "#d4b05e" : "#c6725d";
  ctx.fillRect(-wide, -19, wide * 2 * ratio, 3);
  ctx.restore();
}

function drawScene(
  canvas: HTMLCanvasElement,
  data: View,
  angle: number,
  power: number,
  anim: Animation | undefined,
  progress: number,
) {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  const style = mapStyle[data.mapId] ?? mapStyle.hills;
  ctx.setTransform(2, 0, 0, 2, 0, 0);
  ctx.clearRect(0, 0, 320, 180);
  const sky = ctx.createLinearGradient(0, 0, 0, 180);
  sky.addColorStop(0, style.sky[0]);
  sky.addColorStop(1, style.sky[1]);
  ctx.fillStyle = sky;
  ctx.fillRect(0, 0, 320, 180);
  ctx.fillStyle = style.sun;
  ctx.globalAlpha = 0.5;
  ctx.beginPath();
  ctx.arc(266, 34, 14, 0, Math.PI * 2);
  ctx.fill();
  ctx.globalAlpha = 1;
  ctx.fillStyle = "#ffffff";
  ctx.globalAlpha = 0.24;
  for (const [cx, cy, rx] of [
    [48, 38, 24],
    [92, 46, 17],
    [220, 58, 21],
    [248, 50, 13],
  ]) {
    ctx.beginPath();
    ctx.ellipse(cx, cy, rx, 6, 0, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.globalAlpha = 1;
  // Wind streaks communicate direction without influencing any client result.
  ctx.strokeStyle = "#6c7b70";
  ctx.globalAlpha = 0.35;
  ctx.lineWidth = 0.7;
  for (let i = 0; i < 7; i++) {
    const y = 20 + i * 13;
    const x = (i * 47 + 15) % 300;
    const dir = data.wind >= 0 ? 1 : -1;
    ctx.beginPath();
    ctx.moveTo(x, y);
    ctx.lineTo(x + dir * (6 + Math.abs(data.wind)), y);
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
  const heights =
    anim && progress < anim.impactAt ? anim.oldHeights : data.heights;
  const ground = ctx.createLinearGradient(0, 70, 0, 180);
  ground.addColorStop(0, style.ground[0]);
  ground.addColorStop(1, style.ground[1]);
  ctx.beginPath();
  ctx.moveTo(0, 180);
  for (let i = 0; i < heights.length; i++) {
    ctx.lineTo(i * 2, 180 - heights[i]);
  }
  ctx.lineTo(320, 180);
  ctx.closePath();
  ctx.fillStyle = ground;
  ctx.fill();
  ctx.strokeStyle = "#495544";
  ctx.globalAlpha = 0.45;
  ctx.lineWidth = 1;
  ctx.stroke();
  ctx.globalAlpha = 1;
  // Water sits at the bottom of deep craters.
  const waterLine = 180 - (data.mapId === "islands" ? 12 : 7);
  ctx.fillStyle = style.water;
  ctx.globalAlpha = 0.7;
  ctx.fillRect(0, waterLine, 320, 180 - waterLine);
  ctx.globalAlpha = 1;
  ctx.strokeStyle = "#d8c9a5";
  ctx.globalAlpha = 0.14;
  for (let x = 0; x < 320; x += 8) {
    const h = heights[Math.min(heights.length - 1, Math.floor(x / 2))];
    ctx.beginPath();
    ctx.moveTo(x, 180 - h + 3);
    ctx.lineTo(x + 4, 180 - h + 3);
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
  const frozen = anim?.frozen;
  for (const t of data.tanks) {
    const hp = frozen?.[t.seat]?.hp ?? t.hp;
    const alive = frozen?.[t.seat]?.alive ?? t.alive;
    drawTank(ctx, t, hp, alive, t.seat === data.current, angle);
  }
  if (data.phase === "playing" && data.current >= 0 && !anim) {
    const t = data.tanks[data.current];
    if (t?.alive) {
      const rad = (angle * Math.PI) / 180;
      const len = 26 + power * 0.16;
      ctx.strokeStyle = "#e6c66e";
      ctx.setLineDash([3, 4]);
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(
        t.x + Math.cos(rad) * 18 * t.facing,
        t.y - 10 - Math.sin(rad) * 18,
      );
      ctx.lineTo(
        t.x + Math.cos(rad) * len * t.facing,
        t.y - 10 - Math.sin(rad) * len,
      );
      ctx.stroke();
      ctx.setLineDash([]);
    }
  }
  if (anim && progress < 1) {
    for (const arc of anim.shot.arcs) {
      if (!arc.path.length) continue;
      const at = clamp(
        (progress - 0.04) / Math.max(0.01, anim.impactAt - 0.04),
        0,
        1,
      );
      const index = Math.min(
        arc.path.length - 1,
        Math.floor(at * (arc.path.length - 1)),
      );
      const p = arc.path[index];
      ctx.fillStyle = "#f2d27a";
      ctx.beginPath();
      ctx.arc(p.x, p.y, 2.3, 0, Math.PI * 2);
      ctx.fill();
      ctx.strokeStyle = "#c8944c";
      ctx.lineWidth = 1.2;
      ctx.beginPath();
      ctx.moveTo(p.x, p.y);
      ctx.lineTo(p.x - (p.x - arc.path[Math.max(0, index - 4)].x), p.y - 1);
      ctx.stroke();
    }
  }
  if (anim && progress >= anim.impactAt) {
    const boom = (progress - anim.impactAt) / Math.max(0.01, 1 - anim.impactAt);
    for (const blast of anim.shot.blasts) {
      const radius = blast.r * (0.35 + Math.min(1, boom * 3) * 0.9);
      ctx.globalAlpha = Math.max(0, 0.8 - boom * 0.4);
      ctx.fillStyle = blast.heal ? "#8fc7a1" : "#e5a35d";
      ctx.beginPath();
      ctx.arc(blast.x, blast.y, radius, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = "#f3dfaa";
      ctx.beginPath();
      ctx.arc(blast.x, blast.y, Math.max(1.5, radius * 0.35), 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.globalAlpha = 1;
  }
}

export function Board({
  room,
  data,
  act,
  busy,
  assets,
  sound,
  audio,
  clock,
}: BoardProps<View>) {
  const [angle, setAngle] = useState(45);
  const [power, setPower] = useState(65);
  const [weapon, setWeapon] = useState<"primary" | "special">("primary");
  const [picked, setPicked] = useState("");
  const [anim, setAnim] = useState<Animation | undefined>(undefined);
  const [now, setNow] = useState(() => performance.now());
  const canvas = useRef<HTMLCanvasElement>(null);
  const seenShot = useRef(data.lastShot?.id ?? 0);
  const shotInitialized = useRef(false);
  const prevTanks = useRef(data.tanks);
  const prevHeights = useRef(data.heights);
  const prevTurn = useRef(data.turn);
  const prevPhase = useRef(data.phase);
  const animRef = useRef<Animation | undefined>(undefined);
  const timers = useRef<number[]>([]);
  const angleRef = useRef(angle);
  const powerRef = useRef(power);
  angleRef.current = angle;
  powerRef.current = power;

  const tank = data.tanks[room.you];
  const myTurn =
    data.phase === "playing" &&
    data.current === room.you &&
    !!tank?.alive &&
    !room.spectating;
  const canControl =
    myTurn &&
    !busy &&
    !room.seats[room.you]?.autoPlay &&
    !anim &&
    data.canFire;
  const countdown =
    data.phase === "select"
      ? data.selectUntil
      : data.phase === "playing"
        ? data.deadline
        : 0;
  const serverNow = clock.serverNow + Math.max(0, now - clock.receivedAt);
  const remaining = countdown
    ? Math.max(0, Math.ceil((countdown - serverNow) / 1000))
    : 0;
  const timerText =
    data.phase === "finished"
      ? "已结束"
      : `${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, "0")}`;

  useEffect(() => {
    const id = window.setInterval(() => setNow(performance.now()), 250);
    return () => window.clearInterval(id);
  }, []);
  useEffect(() => {
    if (data.selected) setPicked(data.selected);
  }, [data.selected]);
  useEffect(() => {
    const shot = data.lastShot;
    if (!shotInitialized.current) {
      shotInitialized.current = true;
      seenShot.current = shot?.id ?? 0;
      prevTanks.current = data.tanks;
      prevHeights.current = data.heights;
      return;
    }
    if (!shot) {
      prevTanks.current = data.tanks;
      prevHeights.current = data.heights;
      return;
    }
    if (shot.id > seenShot.current) {
      seenShot.current = shot.id;
      const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
      const longest = Math.max(
        1,
        ...shot.arcs.map((arc) => arc.path.length),
      );
      const duration = reduced
        ? 280
        : clamp(longest / 170, 0.55, 2.2) * 1000;
      const next: Animation = {
        shot,
        start: performance.now(),
        duration,
        impactAt: 0.68,
        frozen: prevTanks.current.map((t) => ({ hp: t.hp, alive: t.alive })),
        oldHeights: prevHeights.current.slice(),
        reduced,
      };
      animRef.current = next;
      setAnim(next);
      if (sound) {
        void audio.playWithFallback(sources(assets, "shot.wav"));
        const explode = window.setTimeout(() => {
          void audio.playWithFallback(sources(assets, "explode.wav"));
          if (shot.blasts.some((b) => b.damage > 0 || b.heal))
            void audio.playWithFallback(sources(assets, "hit.wav"));
          if (shot.blasts.some((b) => b.damage > 0 && !b.heal))
            void audio.playWithFallback(sources(assets, "death.wav"));
        }, duration * next.impactAt);
        timers.current.push(explode);
      }
      const done = window.setTimeout(() => {
        animRef.current = undefined;
        setAnim(undefined);
        prevTanks.current = data.tanks;
        prevHeights.current = data.heights;
      }, duration + 480);
      timers.current.push(done);
    }
    prevTanks.current = data.tanks;
    prevHeights.current = data.heights;
  }, [data.lastShot?.id, data.tanks, data.heights, assets, sound, audio]);

  useEffect(() => {
    if (
      data.phase === "playing" &&
      prevTurn.current !== data.turn &&
      sound
    ) {
      void audio.playWithFallback(sources(assets, "turn.wav"));
    }
    prevTurn.current = data.turn;
  }, [data.turn, data.phase, assets, sound, audio]);

  useEffect(() => {
    if (data.sudden && sound) {
      void audio.playWithFallback(sources(assets, "sudden.wav"));
    }
  }, [data.sudden, assets, sound, audio]);

  useEffect(() => {
    if (data.phase === "playing" && sound) {
      void audio.music(sources(assets, "battle.wav"));
    } else {
      audio.stopMusic();
    }
    return () => audio.stopMusic();
  }, [data.phase, assets, sound, audio]);

  useEffect(() => {
    if (data.phase !== "finished" || prevPhase.current === "finished") return;
    const winners = room.game?.winners ?? [];
    const cue =
      room.you >= 0 && winners.includes(room.you) ? "win" : "lose";
    if (sound) void audio.playWithFallback(sources(assets, cue + ".wav"));
    prevPhase.current = "finished";
  }, [data.phase, room.game?.winners, room.you, assets, sound, audio]);

  useEffect(() => () => {
    timers.current.forEach((id) => window.clearTimeout(id));
    timers.current = [];
    audio.stopMusic();
    audio.stop();
  }, [audio]);

  // Static frame (including a live aim indicator).
  useEffect(() => {
    if (!canvas.current || anim) return;
    drawScene(canvas.current, data, angle, power, undefined, 1);
  }, [data, angle, power, anim]);

  // Shot animation frame loop.
  useEffect(() => {
    if (!anim || !canvas.current) return;
    let frame = 0;
    const loop = () => {
      const current = animRef.current;
      if (!current || !canvas.current) return;
      const progress = current.reduced
        ? 1
        : clamp((performance.now() - current.start) / current.duration, 0, 1);
      drawScene(
        canvas.current,
        data,
        angleRef.current,
        powerRef.current,
        current,
        progress,
      );
      if (progress < 1) frame = requestAnimationFrame(loop);
    };
    frame = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(frame);
  }, [anim, data]);

  useEffect(() => {
    if (busy || !canControl) return;
    const key = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target && /INPUT|TEXTAREA|SELECT/.test(target.tagName)) return;
      switch (event.code) {
        case "KeyW":
        case "ArrowUp":
          event.preventDefault();
          setAngle((v) => clamp(v + 2, 5, 85));
          break;
        case "KeyS":
        case "ArrowDown":
          event.preventDefault();
          setAngle((v) => clamp(v - 2, 5, 85));
          break;
        case "ArrowLeft":
          event.preventDefault();
          setPower((v) => clamp(v - 2, 20, 100));
          break;
        case "ArrowRight":
          event.preventDefault();
          setPower((v) => clamp(v + 2, 20, 100));
          break;
        case "KeyA":
          event.preventDefault();
          void act({
            type: "move",
            epoch: data.epoch,
            turn: data.turn,
            dir: -1,
          });
          break;
        case "KeyD":
          event.preventDefault();
          void act({
            type: "move",
            epoch: data.epoch,
            turn: data.turn,
            dir: 1,
          });
          break;
        case "KeyQ":
          event.preventDefault();
          setWeapon("primary");
          break;
        case "KeyE":
          event.preventDefault();
          if ((tank?.special ?? 0) > 0) setWeapon("special");
          break;
        case "Space":
          event.preventDefault();
          fire();
          break;
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  });

  function fire() {
    if (!canControl || !tank) return;
    void act({
      type: "fire",
      epoch: data.epoch,
      turn: data.turn,
      angle,
      power,
      facing: tank.facing,
      weapon,
    });
  }

  function chooseClass(id: string) {
    if (!data.canSelect || busy) return;
    setPicked(id);
    if (sound) void audio.playWithFallback(sources(assets, "select.wav"));
    void act({ type: "select", epoch: data.epoch, class: id });
  }

  const currentName =
    data.current >= 0 ? tankName(room, data.current) : "无人";
  const windText =
    data.wind === 0
      ? "无风"
      : `${data.wind > 0 ? "→" : "←"} ${Math.abs(data.wind)}`;
  const classesChosen = new Map<string, number>();
  for (const t of data.tanks) {
    if (t.selected) classesChosen.set(t.class, (classesChosen.get(t.class) ?? 0) + 1);
  }

  return (
    <section
      className="fortressblue-board"
      data-fortressblue-phase={data.phase}
      data-fortressblue-current={data.current}
      data-fortressblue-epoch={data.epoch}
      data-fortressblue-wind={data.wind}
      data-fortressblue-map={data.mapId}
      data-fortressblue-shot={data.lastShot?.id ?? 0}
      data-fortressblue-animating={anim ? "1" : "0"}
    >
      <header className="fb-head">
        <div>
          <span className="eyebrow">回合制炮战 · {teamNames[0]} vs {teamNames[1]}</span>
          <h2>{mapNames[data.mapId] ?? data.mapId}</h2>
        </div>
        <div className="fb-head-status">
          <span className="fb-wind" aria-label={`风向 ${windText}`}>
            风力 <strong>{windText}</strong>
          </span>
          <span
            className={`fb-timer ${remaining <= 5 && data.phase === "playing" ? "urgent" : ""}`}
            aria-live="polite"
            data-fortressblue-timer={timerText}
          >
            {data.phase === "select" ? "选择坦克" : data.phase === "playing" ? (data.sudden ? "突然死亡" : "行动倒计时") : "对局结束"}{" "}
            <b>{timerText}</b>
          </span>
        </div>
      </header>

      <div className="fb-teams" aria-label="双方坦克状态">
        {[0, 1].map((team) => (
          <div className={`fb-team fb-team-${team}`} key={team}>
            <h3>
              <i style={{ background: teamColors[team] }} />
              {teamNames[team]}
              <small>{data.tanks.filter((t) => t.team === team && t.alive).length} 辆存活</small>
            </h3>
            {data.tanks
              .filter((t) => t.team === team)
              .map((t) => {
                const info = classInfo[t.class];
                const selectedName = t.selected
                  ? info?.name ?? t.class
                  : "选坦克中";
                const hp = anim?.frozen[t.seat]?.hp ?? t.hp;
                const alive = anim?.frozen[t.seat]?.alive ?? t.alive;
                return (
                  <div
                    className={`fb-tank-row ${data.current === t.seat && data.phase === "playing" ? "current" : ""} ${alive ? "" : "dead"}`}
                    key={t.seat}
                    data-fortressblue-seat={t.seat}
                    data-hp={hp}
                    data-alive={alive ? "1" : "0"}
                    data-x={t.x}
                    data-y={t.y}
                    data-fuel={t.fuel}
                  >
                    <span className="fb-tank-name">
                      {t.seat + 1}. {tankName(room, t.seat)}
                    </span>
                    <span className="fb-tank-class">{selectedName}</span>
                    <span className="fb-hp-track" aria-hidden="true">
                      <i
                        style={{
                          width: `${alive ? clamp((hp / Math.max(1, t.maxHp)) * 100, 0, 100) : 0}%`,
                          background: teamColors[t.team],
                        }}
                      />
                    </span>
                    <strong>
                      {alive ? hp : "击毁"}
                    </strong>
                  </div>
                );
              })}
          </div>
        ))}
      </div>

      <div className="fb-field-wrap">
        <canvas
          ref={canvas}
          className="fb-canvas"
          width={640}
          height={360}
          data-fortressblue-canvas
          aria-label="疯狂坦克战场"
        />
        {data.phase === "select" && (
          <div className="fb-select-overlay">
            <div className="fb-select-head">
              <h3>选择你的坦克</h3>
              <span>已选 {data.tanks.filter((t) => t.selected).length}/{data.tanks.length}</span>
            </div>
            <div className="fb-classes">
              {classOrder.map((id) => {
                const info = classInfo[id];
                const chosen = picked === id || data.selected === id;
                return (
                  <button
                    key={id}
                    type="button"
                    className={`fb-class-card ${chosen ? "chosen" : ""}`}
                    disabled={busy || !data.canSelect}
                    aria-pressed={chosen}
                    onClick={() => chooseClass(id)}
                  >
                    <i style={{ background: info.color }} aria-hidden="true" />
                    <strong>{info.name}</strong>
                    <span>HP {info.maxHp} · 移动 {info.move}</span>
                    <small>{info.special}</small>
                    <em>{classesChosen.get(id) ?? 0} 人选择</em>
                  </button>
                );
              })}
            </div>
          </div>
        )}
      </div>

      {data.phase === "playing" && (
        <div className="fb-controls">
          <div className="fb-turn-note" role="status">
            {room.spectating
              ? "观战中"
              : myTurn
                ? "轮到你行动"
                : `等待 ${currentName} 行动`}
            {data.sudden ? " · 突然死亡：伤害翻倍" : ""}
          </div>
          <div className="fb-weapons" role="group" aria-label="弹种">
            <button
              type="button"
              className={weapon === "primary" ? "active" : ""}
              disabled={!canControl}
              aria-pressed={weapon === "primary"}
              onClick={() => setWeapon("primary")}
            >
              主炮
            </button>
            <button
              type="button"
              className={weapon === "special" ? "active" : ""}
              disabled={!canControl || (tank?.special ?? 0) <= 0}
              aria-pressed={weapon === "special"}
              onClick={() => setWeapon("special")}
            >
              特殊弹 · {tank?.special ?? 0}
            </button>
          </div>
          <div className="fb-sliders">
            <label>
              <span>
                角度 <b>{angle}°</b>
              </span>
              <input
                type="range"
                min={5}
                max={85}
                value={angle}
                disabled={!canControl}
                aria-label="射角"
                onChange={(event) => setAngle(Number(event.target.value))}
              />
            </label>
            <label>
              <span>
                力度 <b>{power}</b>
              </span>
              <input
                type="range"
                min={20}
                max={100}
                value={power}
                disabled={!canControl}
                aria-label="力度"
                onChange={(event) => setPower(Number(event.target.value))}
              />
            </label>
          </div>
          <div className="fb-actions">
            <button
              type="button"
              className="outline"
              disabled={!canControl || (tank?.fuel ?? 0) <= 0}
              aria-label="向左移动"
              onClick={() =>
                void act({
                  type: "move",
                  epoch: data.epoch,
                  turn: data.turn,
                  dir: -1,
                })
              }
            >
              ← 移动
            </button>
            <button
              type="button"
              className="outline"
              disabled={!canControl || (tank?.fuel ?? 0) <= 0}
              aria-label="向右移动"
              onClick={() =>
                void act({
                  type: "move",
                  epoch: data.epoch,
                  turn: data.turn,
                  dir: 1,
                })
              }
            >
              移动 →
            </button>
            <button
              type="button"
              className="primary fb-fire"
              disabled={!canControl}
              onClick={fire}
            >
              开火
            </button>
          </div>
          <div className="fb-meta">
            <span>移动力 {Math.round(tank?.fuel ?? 0)}</span>
            <span>Q／E 切换弹种 · 空格开火</span>
            <span>W／S 角度 · ←／→ 力度 · A／D 移动</span>
          </div>
        </div>
      )}

      {data.phase === "finished" && (
        <p className="fb-finished" role="status">
          {data.log.at(-1) ?? "对局已结束，可以查看最终战场"}
        </p>
      )}
    </section>
  );
}
