import { CueTip } from "./CueTip";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import { Table } from "./Table";
import { BAULK, H, sampleFlight, recentImpacts } from "./presentation";
import { ballName, colourNames, valueOf, type View, type Vec } from "./types";
import { cueLeadMs } from "./playback";
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
    [power, setPower] = useState(2),
    [top, setTop] = useState(0),
    [side, setSide] = useState(0),
    [freeBall, setFreeBall] = useState(0),
    [placing, setPlacing] = useState(false),
    [error, setError] = useState(""),
    [confirm, setConfirm] = useState<
      "resign" | "rerack" | "concede_frame" | null
    >(null),
    [frame, setFrame] = useState({
      id: d.flight?.id ?? 0,
      elapsed: d.flight ? (clock.serverNow - d.flight.start) / 1000 : 0,
      balls:
        d.flight && d.phase === "rolling"
          ? sampleFlight(
              d.flight,
              Math.max(0, (clock.serverNow - d.flight.start) / 1000),
            )
          : d.balls,
    }),
    [pendingShot, setPendingShot] = useState<
      { serial: number; angle: number; power: number; at: number } | undefined
    >(),
    [pendingAge, setPendingAge] = useState(0),
    [respotted, setRespotted] = useState({ id: 0, ids: [] as number[], at: 0 }),
    [spotStrength, setSpotStrength] = useState(0),
    [reduced, setReduced] = useState(
      () => matchMedia("(prefers-reduced-motion: reduce)").matches,
    );
  const played = useRef({
      id: d.flight?.id ?? 0,
      time: d.flight ? (clock.serverNow - d.flight.start) / 1000 : 0,
    }),
    lastPhase = useRef(d.phase);
  const frameRef = useRef(frame),
    pendingSerial = useRef(0);
  const scope = `${room.id}:${clock.epoch ?? 0}`;
  const previousScope = useRef(scope);
  useLayoutEffect(() => {
    if (previousScope.current === scope) return;
    previousScope.current = scope;
    const observed = d.flight
      ? (clock.serverNow +
          performance.now() -
          clock.receivedAt -
          d.flight.start) /
        1000
      : 0;
    const elapsed =
      d.flight && frameRef.current.id === d.flight.id
        ? Math.max(frameRef.current.elapsed, observed)
        : observed;
    played.current = { id: d.flight?.id ?? 0, time: elapsed };
    lastPhase.current = d.phase;
    frameRef.current = { id: d.flight?.id ?? 0, elapsed, balls: d.balls };
    setFrame(frameRef.current);
    pendingSerial.current++;
    setPendingShot(undefined);
    setRespotted({ id: 0, ids: [], at: 0 });
    setSpotStrength(0);
  }, [scope]);
  useEffect(() => {
    if (!pendingShot) return;
    let request = 0;
    const tick = () => {
      const elapsed = (performance.now() - pendingShot.at) / 1000;
      setPendingAge(elapsed);
      if (elapsed < 0.2) request = requestAnimationFrame(tick);
    };
    tick();
    return () => cancelAnimationFrame(request);
  }, [pendingShot]);
  async function shoot(angle: number, power: number) {
    if (pendingShot || busy || !d.controls.canShoot) return;
    const serial = ++pendingSerial.current;
    setPendingAge(0);
    setPendingShot({ serial, angle, power, at: performance.now() });
    try {
      await send("shoot", { angle, power, top, side, freeBall });
    } finally {
      if (pendingSerial.current === serial) setPendingShot(undefined);
    }
  }
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
      const next = {
        id: flight?.id ?? 0,
        elapsed: flight?.motion.duration ?? 0,
        balls: d.balls,
      };
      frameRef.current = next;
      setFrame(next);
      return;
    }
    let request = 0;
    const fresh = played.current.id !== flight.id;
    if (fresh) {
      played.current = { id: flight.id, time: -Infinity };
      frameRef.current = {
        id: flight.id,
        elapsed: -Infinity,
        balls: flight.motion.frames[0].balls,
      };
    }
    const animate = () => {
      const age =
        (clock.serverNow +
          performance.now() -
          clock.receivedAt -
          flight.start) /
        1000;
      const elapsed = Math.max(
        frameRef.current.id === flight.id
          ? frameRef.current.elapsed
          : -Infinity,
        age,
      );
      const next = {
        id: flight.id,
        elapsed,
        balls: sampleFlight(flight, Math.max(0, elapsed)),
      };
      frameRef.current = next;
      setFrame(next);
      if (!document.hidden) {
        const events = [
          { time: 0, kind: "cue", ball: 0, speed: 0 },
          ...flight.motion.events,
        ];
        const event = recentImpacts(events, played.current.time, elapsed).at(
          -1,
        );
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
  }, [
    d.flight?.id,
    d.phase,
    d.balls,
    clock.serverNow,
    clock.receivedAt,
    scope,
  ]);
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
    ? frame.id === d.flight.id
      ? frame.elapsed
      : (clock.serverNow - d.flight.start) / 1000
    : 1;
  const cueSeconds = (d.flight?.cueDuration ?? cueLeadMs) / 1000;
  const confirmedCue =
    d.phase === "rolling" && d.flight && flightAge < 0.16
      ? {
          pos: d.flight.motion.frames[0].balls.find((b) => b.id === 0)!.pos,
          angle: d.flight.strike.angle,
          power: d.flight.strike.power,
          offset: reduced
            ? flightAge < 0
              ? -d.flight.strike.power * 0.16
              : 0.028
            : flightAge < 0
              ? -d.flight.strike.power * 0.16 +
                (d.flight.strike.power * 0.16 + 0.028) *
                  Math.max(
                    0,
                    Math.min(
                      1,
                      (flightAge + cueSeconds) / Math.max(0.001, cueSeconds),
                    ),
                  )
              : 0.028 +
                Math.min(1, flightAge / 0.16) * 0.035 * d.flight.strike.power,
        }
      : undefined;
  const strikeCue =
    confirmedCue ??
    (pendingShot && d.phase === "aim"
      ? {
          pos: d.balls.find((b) => b.id === 0)!.pos,
          angle: pendingShot.angle,
          power: pendingShot.power,
          offset:
            -pendingShot.power * 0.16 -
            (reduced
              ? 0
              : 0.04 * Math.sin(Math.min(1, pendingAge / 0.2) * Math.PI)),
        }
      : undefined);
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
          ? "任意彩球"
          : (colourNames[d.clear] ?? "清台");
  return (
    <section
      className="snooker-board"
      data-phase={d.phase}
      data-shot-pending={!!pendingShot}
      data-flight-time={flightAge}
    >
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
          第 {d.frame} 局 ·{" "}
          {d.phase === "rolling" && flightAge < 0
            ? "正在出杆"
            : phases[d.phase]}
        </strong>
        <span>
          {player(d.turn)}
          {d.phase === "aim" ? ` · 打${target}` : ""}
        </span>
        <span>
          单杆 {d.break} · 台面余分 {d.remaining}
        </span>
      </div>
      {d.endExplanation && (
        <div className="snooker-end-explanation" role="status">
          <strong>结束原因</strong>
          <p>{d.endExplanation}</p>
        </div>
      )}
      {error && (
        <p className="snooker-error" role="alert">
          {error}
        </p>
      )}
      <div className="snooker-game-layout">
        <Table
          balls={d.phase === "rolling" ? frame.balls : d.balls}
          angle={(angle * Math.PI) / 180}
          power={power / 100}
          canAim={
            d.controls.canShoot && !busy && !room.seats[room.you]?.autoPlay
          }
          finished={d.phase === "finished"}
          gestureKey={`${scope}:${d.window}`}
          onPower={(p) => setPower(p * 100)}
          onShoot={(angle, power) => void shoot(angle, power)}
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
                      <p>点击 D 区摆放白球，完成后瞄准并向后拉杆。</p>
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
                <p>本杆可打任意彩球，按实际先碰到的彩球判定。</p>
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
              <details className="snooker-spin">
                <summary>击点 · 高低杆与侧旋</summary>
                <CueTip
                  top={top}
                  side={side}
                  disabled={
                    !d.controls.canShoot ||
                    busy ||
                    !!room.seats[room.you]?.autoPlay
                  }
                  onChange={(value) => {
                    setTop(value.top);
                    setSide(value.side);
                  }}
                />
              </details>
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
