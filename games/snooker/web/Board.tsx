import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import { Table } from "./Table";
import { BAULK, H, sampleFlight, recentImpacts } from "./presentation";
import {
  ballName,
  colourNames,
  valueOf,
  type Ball,
  type View,
  type Vec,
} from "./types";
import { soundForImpact } from "./sounds";
import "./style.css";
const phases: Record<string, string> = {
  start_choice: "抽签胜方选择开球队伍",
  aim: "瞄准并击球",
  rolling: "球正在运动",
  foul: "处理犯规判罚",
  concede: "对方提出认输本局",
  black_choice: "黑球决胜 · 选择先打队伍",
  black_player: "选择本队击球者",
  frame_end: "本局结束",
  finished: "本场结束",
};
const teamName = (team: number) => (team === 0 ? "A 队" : "B 队");
export function Board({
  room,
  data: d,
  act,
  busy,
  assets,
  audio,
  sound,
  clock,
}: BoardProps<View>) {
  const [angle, setAngle] = useState(0),
    [power, setPower] = useState(35),
    [top, setTop] = useState(0),
    [side, setSide] = useState(0),
    [nominate, setNominate] = useState(7),
    [freeBall, setFreeBall] = useState(0),
    [placing, setPlacing] = useState(false),
    [error, setError] = useState(""),
    [confirm, setConfirm] = useState<
      "resign" | "rerack" | "concede_frame" | null
    >(null),
    [drawBalls, setDrawBalls] = useState<Ball[]>(d.balls),
    [respotted, setRespotted] = useState({ id: 0, ids: [] as number[], at: 0 }),
    [spotStrength, setSpotStrength] = useState(0),
    [reduced, setReduced] = useState(
      () => matchMedia("(prefers-reduced-motion: reduce)").matches,
    );
  const played = useRef({
      id: d.flight?.id ?? 0,
      time: d.flight
        ? Math.max(0, (clock.serverNow - d.flight.start) / 1000)
        : 0,
    }),
    lastPhase = useRef(d.phase);
  useEffect(() => {
    setPlacing(
      d.phase === "aim" && d.inHand && !!d.balls.find((b) => b.id === 0)?.down,
    );
    setFreeBall(0);
    setError("");
  }, [d.window, room.id]);
  async function send(type: string, extra: Record<string, unknown> = {}) {
    setError("");
    try {
      await act({ type, window: d.window, ...extra });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }
  const play = (name: string) => {
    const local = "/snooker/v1/audio/" + name + ".mp3";
    void audio.playWithFallback(
      assets ? [assets.replace(/\/$/, "") + local, local] : [local],
    );
  };
  useEffect(() => {
    const flight = d.flight;
    if (!flight || d.phase !== "rolling") {
      setDrawBalls(d.balls);
      return;
    }
    let request = 0;
    if (played.current.id !== flight.id)
      played.current = { id: flight.id, time: -0.001 };
    const animate = () => {
      const elapsed = Math.max(
        0,
        (clock.serverNow +
          performance.now() -
          clock.receivedAt -
          flight.start) /
          1000,
      );
      setDrawBalls(sampleFlight(flight, elapsed));
      const events = [
        { time: 0, kind: "cue", ball: 0, speed: 0 },
        ...flight.motion.events,
      ];
      if (!document.hidden) {
        const impacts = recentImpacts(events, played.current.time, elapsed);
        const event = impacts.at(-1);
        if (event) {
          const name = soundForImpact(event, flight.id);
          if (name) play(name);
        }
      }
      played.current.time = elapsed;
      if (elapsed < flight.motion.duration)
        request = requestAnimationFrame(animate);
    };
    request = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(request);
  }, [d.flight?.id, d.phase, d.balls, clock.serverNow, clock.receivedAt]);
  const skipElapsedAudio = () => {
    const flight = d.flight;
    if (flight)
      played.current.time = Math.max(
        played.current.time,
        (clock.serverNow +
          performance.now() -
          clock.receivedAt -
          flight.start) /
          1000,
      );
  };
  useEffect(() => {
    const changed = () => {
      skipElapsedAudio();
      if (document.hidden) audio.stop();
    };
    document.addEventListener("visibilitychange", changed);
    return () => document.removeEventListener("visibilitychange", changed);
  }, [d.flight?.id, clock.serverNow, clock.receivedAt, audio]);
  useEffect(() => {
    skipElapsedAudio();
  }, [sound]);
  useEffect(() => {
    const media = matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    if (lastPhase.current === "rolling" && d.phase !== "rolling" && d.flight) {
      const ids = d.balls
        .filter(
          (b) =>
            b.id >= 16 &&
            !b.down &&
            d.flight!.motion.balls.some((old) => old.id === b.id && old.down),
        )
        .map((b) => b.id);
      if (ids.length)
        setRespotted({ id: d.flight.id, ids, at: performance.now() });
    }
    if (lastPhase.current === "rolling" && d.phase === "foul") play("foul");
    else if (
      lastPhase.current !== d.phase &&
      ["frame_end", "finished"].includes(d.phase)
    )
      play("win");
    lastPhase.current = d.phase;
  }, [d.phase]);
  useEffect(() => {
    if (!respotted.ids.length) return;
    let request = 0;
    const tick = () => {
      const age = (performance.now() - respotted.at) / 850;
      if (age >= 1) {
        setSpotStrength(0);
        return;
      }
      setSpotStrength(reduced ? 1 : 1 - age);
      request = requestAnimationFrame(tick);
    };
    tick();
    return () => cancelAnimationFrame(request);
  }, [respotted.id, reduced]);
  const flightAge = d.flight
    ? Math.max(
        0,
        (clock.serverNow +
          performance.now() -
          clock.receivedAt -
          d.flight.start) /
          1000,
      )
    : 1;
  const strikeCue =
    d.phase === "rolling" && d.flight && flightAge < 0.18 && !reduced
      ? {
          pos: d.flight.motion.frames[0].balls.find((b) => b.id === 0)!.pos,
          angle: d.flight.strike.angle,
          power: d.flight.strike.power,
          progress: flightAge / 0.18,
        }
      : undefined;
  const activeTeam =
    d.phase === "finished"
      ? d.winner
      : d.phase === "frame_end"
        ? (d.results.at(-1)?.winner ?? d.teams[d.turn])
        : d.teams[d.turn];
  const player = (p: number) => room.seats[p]?.name ?? `玩家 ${p + 1}`,
    mine = !room.spectating && room.you >= 0,
    can = d.controls.canAct && !busy,
    target =
      d.target === "red"
        ? "红球"
        : d.target === "colour"
          ? "自选彩球"
          : (colourNames[d.clear] ?? "清台");
  const tip = (which: "top" | "side", n: number) => {
    const other = which === "top" ? side : top,
      limit = Math.sqrt(Math.max(0, 0.81 - n * n));
    if (which === "top") {
      setTop(n);
      setSide(Math.max(-limit, Math.min(limit, other)));
    } else {
      setSide(n);
      setTop(Math.max(-limit, Math.min(limit, other)));
    }
  };
  return (
    <section className="snooker-board" data-phase={d.phase}>
      <div className="snooker-scoreboard">
        {[0, 1].map((team) => (
          <div key={team} className={activeTeam === team ? "active" : ""}>
            <span>
              {teamName(team)} · 已赢 {d.framesWon[team]} 局
            </span>
            <strong data-snooker-score={team}>{d.scores[team]}</strong>
            <small>
              {room.seats
                .filter((_, p) => d.teams[p] === team)
                .map((p) => p.name)
                .join(" ／ ")}
            </small>
          </div>
        ))}
      </div>
      <div className="snooker-status">
        <strong>
          第 {d.frame} 局 · {phases[d.phase]}
        </strong>
        <span>
          {player(d.turn)}
          {d.phase === "aim" ? ` · 打${target}` : ""}
        </span>
        <span>
          单杆 {d.break} · 台面余分 {d.remaining}
        </span>
      </div>
      {error && (
        <p className="snooker-error" role="alert">
          {error}
        </p>
      )}
      <div className="snooker-game-layout">
        <Table
          balls={d.phase === "rolling" ? drawBalls : d.balls}
          angle={(angle * Math.PI) / 180}
          power={power / 100}
          canAim={d.controls.canShoot && !busy}
          assist={d.options.aimAssist === "line"}
          placing={placing}
          assets={assets}
          flash={
            spotStrength > 0
              ? { ids: respotted.ids, intensity: reduced ? 0 : spotStrength }
              : undefined
          }
          strikeCue={strikeCue}
          onAim={(r) => setAngle(Math.round(((r * 180) / Math.PI) * 100) / 100)}
          onPlace={(p: Vec) => void send("place", { x: p.x, y: p.y })}
        />
        <div className="snooker-controls">
          {["start_choice", "black_choice"].includes(d.phase) && (
            <div className="snooker-decision">
              <p>{phases[d.phase]}</p>
              {[0, 1].map((team) => (
                <button
                  key={team}
                  disabled={!can}
                  onClick={() =>
                    void send(
                      d.phase === "start_choice"
                        ? "break_choice"
                        : "black_choice",
                      { team },
                    )
                  }
                >
                  {teamName(team)}先开球
                </button>
              ))}
            </div>
          )}
          {d.phase === "black_player" && (
            <div className="snooker-decision">
              <p>由 {teamName(d.teams[d.turn])} 选择黑球先打者</p>
              {room.seats.map(
                (seat, p) =>
                  d.teams[p] === d.teams[d.turn] && (
                    <button
                      key={p}
                      disabled={!can}
                      onClick={() => void send("black_player", { seat: p })}
                    >
                      {seat.name} 先打
                    </button>
                  ),
              )}
            </div>
          )}
          {d.phase === "foul" && d.foul && (
            <div className="snooker-decision">
              <strong>对方获得 {d.foul.ruling.foul} 分</strong>
              <p>{d.foul.ruling.reasons.join("；")}</p>
              {d.foul.ruling.miss && <p>解球未中，可要求原位重打。</p>}
              {d.missRun?.warned && (
                <p className="snooker-error">
                  已发出警告：再次原位重打仍未击中目标球，将判负本局。
                </p>
              )}
              <button
                disabled={!can}
                onClick={() => void send("foul_choice", { choice: "accept" })}
              >
                我方接着打
              </button>
              <button
                disabled={!can}
                onClick={() => void send("foul_choice", { choice: "replay" })}
              >
                让原犯规者继续
              </button>
              {d.foul.ruling.miss && (
                <button
                  disabled={!can}
                  onClick={() =>
                    void send("foul_choice", { choice: "restore" })
                  }
                >
                  原位重打
                </button>
              )}
            </div>
          )}
          {d.phase === "concede" && (
            <div className="snooker-decision">
              <strong>{teamName(d.offerTeam)}提出认输本局</strong>
              <p>可以接受并结束本局，也可以继续打。</p>
              <button
                disabled={!can}
                onClick={() => void send("concede_reply", { choice: "accept" })}
              >
                接受认输本局
              </button>
              <button
                disabled={!can}
                onClick={() =>
                  void send("concede_reply", { choice: "continue" })
                }
              >
                继续本局
              </button>
            </div>
          )}
          {d.phase === "aim" && (
            <>
              <p className="snooker-turn">
                {d.controls.canShoot ? "轮到你击球" : `等待 ${player(d.turn)}`}{" "}
                · 下一位 {player(d.nextSeat)}
              </p>
              {d.inHand && (
                <div className="snooker-placement">
                  <button
                    disabled={!d.controls.canPlace || busy}
                    onClick={() => setPlacing((v) => !v)}
                  >
                    {placing ? "取消摆球" : "摆放白球"}
                  </button>
                  {placing && (
                    <>
                      <p>点击 D 区摆放白球，再取消摆球以瞄准。</p>
                      <button
                        disabled={busy}
                        onClick={() =>
                          void send("place", { x: BAULK - 0.15, y: H / 2 })
                        }
                      >
                        白球置于 D 区中点
                      </button>
                    </>
                  )}
                </div>
              )}
              {d.target === "colour" && (
                <label>
                  本杆彩球
                  <select
                    aria-label="本杆彩球"
                    value={nominate}
                    disabled={!d.controls.canShoot}
                    onChange={(e) => setNominate(+e.target.value)}
                  >
                    {[2, 3, 4, 5, 6, 7].map((v) => (
                      <option key={v} value={v}>
                        {colourNames[v]} · {v} 分
                      </option>
                    ))}
                  </select>
                </label>
              )}
              {d.freeBall && (
                <label>
                  自由球
                  <select
                    aria-label="自由球"
                    value={freeBall}
                    disabled={!d.controls.canShoot}
                    onChange={(e) => setFreeBall(+e.target.value)}
                  >
                    <option value={0}>不使用自由球</option>
                    {d.balls
                      .filter(
                        (b) =>
                          !b.down &&
                          b.id > 0 &&
                          (d.target === "red"
                            ? valueOf(b.id) !== 1
                            : valueOf(b.id) !== d.clear),
                      )
                      .map((b) => (
                        <option key={b.id} value={b.id}>
                          {ballName(b.id)}
                          {b.id <= 15 ? ` ${b.id}` : ""}
                        </option>
                      ))}
                  </select>
                </label>
              )}
              <label>
                方向（度）
                <input
                  type="number"
                  aria-label="方向（度）"
                  min={-180}
                  max={180}
                  step={0.05}
                  value={angle}
                  disabled={!d.controls.canShoot}
                  onChange={(e) => setAngle(Number(e.target.value))}
                />
              </label>
              <label>
                力度 <output>{power}%</output>
                <input
                  type="range"
                  aria-label="力度"
                  min={2}
                  max={100}
                  step={1}
                  value={power}
                  disabled={!d.controls.canShoot}
                  onChange={(e) => setPower(+e.target.value)}
                />
              </label>
              <details className="snooker-spin">
                <summary>击点 · 高低杆与侧旋</summary>
                <label>
                  高低杆{" "}
                  <output>
                    {top > 0 ? "高杆" : top < 0 ? "低杆" : "中杆"}{" "}
                    {Math.abs(top).toFixed(2)}
                  </output>
                  <input
                    type="range"
                    aria-label="高低杆"
                    min={-0.9}
                    max={0.9}
                    step={0.05}
                    value={top}
                    disabled={!d.controls.canShoot}
                    onChange={(e) => tip("top", +e.target.value)}
                  />
                </label>
                <label>
                  侧旋{" "}
                  <output>
                    {side > 0 ? "右塞" : side < 0 ? "左塞" : "无侧旋"}{" "}
                    {Math.abs(side).toFixed(2)}
                  </output>
                  <input
                    type="range"
                    aria-label="侧旋"
                    min={-0.9}
                    max={0.9}
                    step={0.05}
                    value={side}
                    disabled={!d.controls.canShoot}
                    onChange={(e) => tip("side", +e.target.value)}
                  />
                </label>
                <button
                  onClick={() => {
                    setTop(0);
                    setSide(0);
                  }}
                >
                  恢复中杆
                </button>
              </details>
              <button
                className="snooker-strike"
                disabled={
                  !d.controls.canShoot ||
                  busy ||
                  placing ||
                  !!d.balls.find((b) => b.id === 0)?.down
                }
                onClick={() =>
                  void send("shoot", {
                    angle: (angle * Math.PI) / 180,
                    power: power / 100,
                    top,
                    side,
                    nominate,
                    freeBall,
                  })
                }
              >
                击球
              </button>
            </>
          )}
          {d.phase === "rolling" && (
            <p className="snooker-motion-notice">
              球正在运动，停稳并复位后继续。
            </p>
          )}
          {d.phase === "frame_end" && (
            <div className="snooker-decision">
              <strong>
                {teamName(d.results.at(-1)?.winner ?? activeTeam)}获胜 ·{" "}
                {d.scores[0]} : {d.scores[1]}
              </strong>
              {mine && room.capacity === 4 && (
                <label>
                  本队下一局顺序
                  <select
                    aria-label="本队下一局先打者"
                    value={d.nextFirst[d.teams[room.you]]}
                    onChange={(e) =>
                      void send("lead", { seat: +e.target.value })
                    }
                  >
                    {room.seats.map(
                      (p, i) =>
                        d.teams[i] === d.teams[room.you] && (
                          <option key={i} value={i}>
                            {p.name} 先上手
                          </option>
                        ),
                    )}
                  </select>
                </label>
              )}
              <button
                disabled={!mine || busy || d.ready[room.you]}
                onClick={() => void send("ready")}
              >
                {d.ready[room.you] ? "已准备，等待其他玩家" : "准备下一局"}
              </button>
            </div>
          )}
          {d.phase === "finished" && (
            <p>
              {teamName(d.winner)}获胜
              {d.finishReason === "resigned"
                ? " · 对方认输本场"
                : ` · ${d.framesWon[0]} : ${d.framesWon[1]}`}
            </p>
          )}
        </div>
      </div>
      {room.capacity === 4 && (
        <p className="snooker-order">
          上手顺序：{d.order.map(player).join(" → ")}。连续进球由同一人继续。
        </p>
      )}
      {mine && ["aim", "frame_end"].includes(d.phase) && (
        <div className="snooker-secondary">
          {d.phase === "concede" && (
            <div className="snooker-decision">
              <strong>{teamName(d.offerTeam)}提出认输本局</strong>
              <p>可以接受并结束本局，也可以继续打。</p>
              <button
                disabled={!can}
                onClick={() => void send("concede_reply", { choice: "accept" })}
              >
                接受认输本局
              </button>
              <button
                disabled={!can}
                onClick={() =>
                  void send("concede_reply", { choice: "continue" })
                }
              >
                继续本局
              </button>
            </div>
          )}
          {d.phase === "aim" && (
            <button onClick={() => setConfirm("rerack")}>提议重新摆球</button>
          )}
          <button onClick={() => setConfirm("resign")}>弃赛本场</button>
          {d.controls.canConcede && (
            <button onClick={() => setConfirm("concede_frame")}>
              认输本局
            </button>
          )}
          {d.controls.canClaim && (
            <button onClick={() => void send("claim_frame")}>
              宣告本局胜利
            </button>
          )}
          {(d.rerack.some(Boolean) ||
            d.resign.some(Boolean) ||
            d.concede.some(Boolean)) && (
            <>
              <span>
                {d.rerack.filter(Boolean).length} 人同意重新摆球 ·{" "}
                {d.resign.filter(Boolean).length} 人确认弃赛 ·{" "}
                {d.concede.filter(Boolean).length} 人同意认输本局
              </span>
              <button onClick={() => void send("cancel_vote")}>
                撤回我的确认
              </button>
            </>
          )}
        </div>
      )}
      {confirm && (
        <Modal
          title={
            confirm === "resign"
              ? "确认弃赛本场"
              : confirm === "concede_frame"
                ? "提出认输本局"
                : "提议重新摆球"
          }
          onClose={() => setConfirm(null)}
        >
          <p>
            {confirm === "resign"
              ? "确认后退出并判负整场比赛。双打需要两位搭档都确认。"
              : confirm === "concede_frame"
                ? "提出认输本局。双打需要搭档共同确认，对方仍可选择继续打。"
                : "双方所有玩家同意后重新摆球。普通局清零本局比分并保留开球顺序；黑球决胜只重置黑球。"}
          </p>
          <button
            onClick={() => {
              void send(confirm);
              setConfirm(null);
            }}
          >
            确认
          </button>
        </Modal>
      )}
    </section>
  );
}
