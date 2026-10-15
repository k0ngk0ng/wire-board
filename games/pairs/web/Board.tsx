import { PairInput, type InputSnapshot } from "./input";
import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  useReducer,
  type CSSProperties,
} from "react";
import { Hammer, Lightbulb, Shuffle } from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import type { PairEvent, View } from "./types";
import { asset, localAsset, play } from "./media";

export function Board({
  room,
  data,
  act,
  busy,
  assets,
  audio,
  clock,
}: BoardProps<View>) {
  const [spectatorSeat, setSpectatorSeat] = useState(0),
    [hammer, setHammer] = useState(false),
    [notice, setNotice] = useState("");
  const [narrow, setNarrow] = useState(
    () => matchMedia("(max-width: 600px)").matches,
  );
  const [reduced, setReduced] = useState(
    () => matchMedia("(prefers-reduced-motion: reduce)").matches,
  );
  const [now, setNow] = useState(clock.serverNow),
    [bolt, setBolt] = useState<PairEvent | null>(null),
    [frame, setFrame] = useState(0);
  const field = useRef<HTMLDivElement>(null),
    [size, setSize] = useState({ w: 0, h: 0 });
  const seat = room.spectating ? spectatorSeat : Math.max(0, room.you),
    player = data.players[seat],
    grid = player.board;
  const transposed = !!grid && narrow && grid.cols > grid.rows,
    cols = grid ? (transposed ? grid.rows : grid.cols) : 1,
    rows = grid ? (transposed ? grid.cols : grid.rows) : 1;
  const [input] = useState(() => new PairInput());
  const [inputTick, redrawInput] = useReducer((n: number) => n + 1, 0);
  const manual = useRef(false);
  const gesture = useRef<{
    id: number;
    x: number;
    y: number;
    moved: boolean;
    scope: string;
    revision: number;
  } | null>(null);
  const scope = `${room.id}:${seat}:${!!room.spectating}:${data.round}:${clock.epoch ?? 0}`;
  const playable =
    room.status === "playing" &&
    data.canAct &&
    !room.spectating &&
    !room.seats[room.you]?.autoPlay;
  const snapshot = useMemo<InputSnapshot>(
    () => ({
      scope,
      round: data.round,
      revision: player.revision,
      stamp: clock.receivedAt,
      playable,
      board: grid,
      event: player.event,
    }),
    [
      scope,
      data.round,
      player.revision,
      clock.receivedAt,
      playable,
      grid,
      player.event,
    ],
  );
  const canAct = playable && !manual.current;
  const selected = input.selected;
  const toolsBusy = busy || manual.current || input.hasWork;
  useLayoutEffect(() => {
    const before = input.version;
    input.sync(snapshot);
    if (before !== input.version) redrawInput();
  }, [input, snapshot]);
  useEffect(() => {
    const before = input.version;
    const request = input.next(snapshot, busy || manual.current);
    if (before !== input.version) redrawInput();
    if (!request) return;
    const { token, ...action } = request;
    void act(action).then(
      () => {
        input.complete(token);
        redrawInput();
      },
      () => {
        input.cancel();
        input.complete(token);
        redrawInput();
      },
    );
  }, [input, snapshot, busy, inputTick, act]);
  const seen = useRef({
    round: data.round,
    seat,
    id: player.event.id,
    phase: data.phase,
    epoch: clock.epoch,
  });
  useEffect(() => {
    const small = matchMedia("(max-width: 600px)"),
      motion = matchMedia("(prefers-reduced-motion: reduce)");
    const sync = () => {
      setNarrow(small.matches);
      setReduced(motion.matches);
    };
    small.addEventListener("change", sync);
    motion.addEventListener("change", sync);
    return () => {
      small.removeEventListener("change", sync);
      motion.removeEventListener("change", sync);
    };
  }, []);
  useEffect(() => {
    const tick = () =>
      setNow(
        clock.serverNow + Math.max(0, performance.now() - clock.receivedAt),
      );
    tick();
    const id = setInterval(tick, 100);
    return () => clearInterval(id);
  }, [clock]);
  useEffect(() => {
    gesture.current = null;
    setHammer(false);
  }, [scope, playable]);
  useEffect(() => {
    setHammer(false);
  }, [player.revision]);
  useEffect(() => {
    const el = field.current;
    if (!el) return;
    const update = () => setSize({ w: el.clientWidth, h: el.clientHeight });
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, [!!grid, cols, rows]);
  useEffect(() => {
    const previous = seen.current;
    seen.current = {
      round: data.round,
      seat,
      id: player.event.id,
      phase: data.phase,
      epoch: clock.epoch,
    };
    if (
      document.hidden ||
      previous.round !== data.round ||
      previous.seat !== seat ||
      previous.epoch !== clock.epoch
    ) {
      setBolt(null);
      return;
    }
    if (previous.phase === "countdown" && data.phase === "playing")
      void play(audio, assets, "start");
    if (previous.phase !== "finished" && data.phase === "finished")
      void play(audio, assets, "finish");
    if (previous.id === player.event.id) return;
    const event = player.event;
    // Only new, recent actions produce feedback. Restoring a saved room is silent.
    if (Math.abs(now - event.at) > 5000) return;
    if (event.path) {
      setBolt(event);
      setFrame(0);
    }
    if (event.recovery) setNotice("没有可配对的图案，已免费重排。");
    else if (event.type === "hint") setNotice("发光的两个图案可以连接。");
    else if (event.type === "hammer") setNotice("障碍已清除。");
    else if (event.type === "shuffle") setNotice("棋盘已重排。");
    else setNotice(player.combo > 1 ? `${player.combo} 连击！` : "");
    const kind = data.phase === "finished" ? "" : event.type;
    if (
      kind === "match" ||
      kind === "hammer" ||
      kind === "shuffle" ||
      kind === "hint"
    )
      void play(audio, assets, kind);
  }, [data, seat, clock.epoch]);
  useEffect(() => {
    if (!bolt) return;
    const interval = setInterval(() => setFrame((v) => (v + 1) % 5), 55);
    const timeout = setTimeout(
      () => setBolt(null),
      bolt.type === "hint" ? 1600 : reduced ? 180 : 420,
    );
    return () => {
      clearInterval(interval);
      clearTimeout(timeout);
    };
  }, [bolt, reduced]);
  useEffect(() => {
    const hide = () => {
      setBolt(null);
      input.cancel();
      gesture.current = null;
      redrawInput();
    };
    document.addEventListener("visibilitychange", hide);
    return () => document.removeEventListener("visibilitychange", hide);
  }, [input]);
  const send = (type: string, a?: number, b?: number) => {
    if (!canAct || toolsBusy) return;
    input.cancel();
    manual.current = true;
    redrawInput();
    const complete = () => {
      manual.current = false;
      redrawInput();
    };
    void act({
      type,
      round: data.round,
      revision: player.revision,
      ...(a === undefined ? {} : { a }),
      ...(b === undefined ? {} : { b }),
    }).then(complete, complete);
  };
  const ensureMusic = () => {
    if (data.phase === "playing")
      void audio.music([
        asset(assets, "audio/bg-legacy-2015.mp3"),
        localAsset("audio/bg-legacy-2015.mp3"),
      ]);
  };
  useEffect(() => {
    ensureMusic();
    if (data.phase === "finished") audio.stopMusic();
    return () => audio.stopMusic();
  }, [data.phase, assets, audio]);
  const choose = (index: number) => {
    if (!canAct || !grid || input.reserved(index)) return;
    input.sync(snapshot);
    const cell = grid.cells[index];
    if (!cell) return;
    if (hammer) {
      if (cell.kind === "tile" && !cell.ice) {
        setNotice("破障锤只能用于石块、木箱或冰层。");
        return;
      }
      send("hammer", index);
      return;
    }
    if (cell.kind !== "tile" || cell.ice) {
      setNotice(
        cell.ice
          ? "先在旁边配对解冻，或使用破障锤。"
          : "木箱可通过相邻消除击碎；石块可绕行或用锤子破除。",
      );
      return;
    }
    input.click(index);
    redrawInput();
    setNotice("");
    ensureMusic();
    if (input.selected !== null) void play(audio, assets, "select");
  };
  const hitCell = (x: number, y: number) => {
    const elements =
      field.current?.querySelectorAll<HTMLButtonElement>(
        ".pairs-cell:not(:disabled)",
      ) ?? [];
    let best: number | null = null,
      distance = Infinity;
    for (const el of elements) {
      const r = el.getBoundingClientRect();
      if (
        x < r.left - 0.5 ||
        x > r.right + 0.5 ||
        y < r.top - 0.5 ||
        y > r.bottom + 0.5
      )
        continue;
      const d = Math.hypot(
        x - (r.left + r.width / 2),
        y - (r.top + r.height / 2),
      );
      if (d < distance) {
        best = Number(el.dataset.index);
        distance = d;
      }
    }
    return best;
  };
  const displayIndex = (x: number, y: number) =>
    grid ? (transposed ? x * grid.cols + y : y * grid.cols + x) : 0;
  const elapsed = Math.max(0, (data.finishAt || now) - data.startAt) / 1000;
  const pad = 6,
    gap = 1,
    cw = (size.w - pad * 2 - gap * (cols - 1)) / cols,
    ch = (size.h - pad * 2 - gap * (rows - 1)) / rows;
  const coordinate = (p: { x: number; y: number }) => {
    const x = transposed ? p.y : p.x,
      y = transposed ? p.x : p.y;
    return {
      x:
        x < 0
          ? pad / 2
          : x >= cols
            ? size.w - pad / 2
            : pad + (x + 0.5) * cw + x * gap,
      y:
        y < 0
          ? pad / 2
          : y >= rows
            ? size.h - pad / 2
            : pad + (y + 0.5) * ch + y * gap,
    };
  };
  const points = bolt?.path?.map(coordinate) ?? [];
  return (
    <section
      className="pairs-board"
      data-revision={player.revision}
      aria-label="连连看棋盘"
    >
      <div className="pairs-heading">
        <div>
          <h2>{data.name}</h2>
          <p>{data.difficulty} · 同图竞速</p>
        </div>
        <strong className="pairs-clock">
          {data.phase === "countdown"
            ? `准备 ${Math.max(0, Math.ceil((data.startAt - now) / 1000))} 秒`
            : `${elapsed.toFixed(1)} 秒`}
        </strong>
      </div>
      {room.spectating && (
        <label className="pairs-watch">
          观看棋盘
          <select
            aria-label="观看棋盘"
            value={seat}
            onChange={(e) => setSpectatorSeat(+e.target.value)}
          >
            {room.seats.map((p, i) => (
              <option key={p.id} value={i}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
      )}
      {data.phase === "finished" && (
        <div className="pairs-result" role="status">
          {room.seats[data.winner]?.name} 最先清空棋盘，获得胜利！用时{" "}
          {elapsed.toFixed(2)} 秒。
        </div>
      )}
      {!grid ? (
        <div className="pairs-countdown">
          <strong>{Math.max(1, Math.ceil((data.startAt - now) / 1000))}</strong>
          <p>所有玩家将在倒计时结束后看到同一张棋盘</p>
        </div>
      ) : (
        <>
          <div className="pairs-toolbar">
            <button
              className="outline"
              disabled={!canAct || toolsBusy || player.tools.hint < 1}
              onClick={() => send("hint")}
            >
              <Lightbulb size={15} />
              提示 {player.tools.hint}
            </button>
            <button
              className="outline"
              disabled={!canAct || toolsBusy || player.tools.shuffle < 1}
              onClick={() => send("shuffle")}
            >
              <Shuffle size={15} />
              重排 {player.tools.shuffle}
            </button>
            <button
              className="outline"
              aria-pressed={hammer}
              disabled={!canAct || toolsBusy || player.tools.hammer < 1}
              onClick={() => {
                setHammer(!hammer);
                input.cancel();
                redrawInput();
                setNotice(hammer ? "" : "点击石块、木箱或冰层使用破障锤。");
              }}
            >
              <Hammer size={15} />
              破障 {player.tools.hammer}
            </button>
          </div>
          <div
            className={`pairs-field ${hammer ? "hammering" : ""}`}
            ref={field}
            onPointerDown={(e) => {
              if (e.button !== 0 || !canAct) return;
              if (!e.isPrimary || gesture.current) {
                gesture.current = null;
                return;
              }
              gesture.current = {
                id: e.pointerId,
                x: e.clientX,
                y: e.clientY,
                moved: false,
                scope,
                revision: player.revision,
              };
              e.currentTarget.setPointerCapture(e.pointerId);
            }}
            onPointerMove={(e) => {
              const g = gesture.current;
              if (
                g?.id === e.pointerId &&
                Math.hypot(e.clientX - g.x, e.clientY - g.y) > 12
              )
                g.moved = true;
            }}
            onPointerUp={(e) => {
              const g = gesture.current;
              gesture.current = null;
              if (e.currentTarget.hasPointerCapture(e.pointerId))
                e.currentTarget.releasePointerCapture(e.pointerId);
              if (
                !g ||
                g.id !== e.pointerId ||
                g.scope !== scope ||
                g.moved ||
                Math.hypot(e.clientX - g.x, e.clientY - g.y) > 12
              )
                return;
              if (
                g.revision !== player.revision &&
                (player.event.recovery || player.event.type === "shuffle")
              )
                return;
              const index = hitCell(e.clientX, e.clientY);
              if (index !== null) choose(index);
            }}
            onPointerCancel={() => {
              gesture.current = null;
            }}
            onLostPointerCapture={(e) => {
              if (gesture.current?.id === e.pointerId) gesture.current = null;
            }}
            style={
              {
                "--pairs-cols": cols,
                aspectRatio: `${cols + 0.5}/${rows + 0.5}`,
              } as CSSProperties
            }
          >
            {Array.from({ length: cols * rows }, (_, i) => {
              const index = displayIndex(i % cols, Math.floor(i / cols)),
                c = grid.cells[index],
                highlight = bolt?.type === "hint" && bolt.pair?.includes(index);
              return (
                <button
                  key={index}
                  type="button"
                  className={`pairs-cell ${c?.kind ?? "empty"} ${c?.ice ? "iced" : ""} ${selected === index ? "selected" : ""} ${highlight ? "hinted" : ""} ${input.reserved(index) ? "queued" : ""}`}
                  data-index={index}
                  aria-label={
                    c
                      ? c.kind === "tile"
                        ? `图案 ${c.icon}${c.ice ? "，冰封" : ""}，第 ${index + 1} 格`
                        : `${c.kind === "stone" ? "石块" : "木箱"}，第 ${index + 1} 格`
                      : "空格"
                  }
                  aria-pressed={selected === index}
                  disabled={!c || !canAct || input.reserved(index)}
                  onClick={(e) => {
                    if (e.detail === 0) choose(index);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Escape") {
                      input.cancel();
                      gesture.current = null;
                      redrawInput();
                      return;
                    }
                    let dx = 0,
                      dy = 0;
                    if (e.key === "ArrowLeft") dx = -1;
                    else if (e.key === "ArrowRight") dx = 1;
                    else if (e.key === "ArrowUp") dy = -1;
                    else if (e.key === "ArrowDown") dy = 1;
                    else return;
                    e.preventDefault();
                    let x = (i % cols) + dx,
                      y = Math.floor(i / cols) + dy;
                    while (x >= 0 && y >= 0 && x < cols && y < rows) {
                      const el =
                        field.current?.querySelector<HTMLButtonElement>(
                          `[data-index="${displayIndex(x, y)}"]`,
                        );
                      if (el && !el.disabled) {
                        el.focus();
                        break;
                      }
                      x += dx;
                      y += dy;
                    }
                  }}
                >
                  {c?.kind === "tile" ? (
                    <img
                      src={asset(assets, `icons/${c.icon}.webp`)}
                      alt=""
                      draggable={false}
                      onError={(e) => {
                        const img = e.currentTarget;
                        if (!img.dataset.fallback) {
                          img.dataset.fallback = "true";
                          img.src = localAsset(`icons/${c.icon}.webp`);
                        }
                      }}
                    />
                  ) : c?.kind === "crate" ? (
                    <span className="pairs-crate" />
                  ) : c?.kind === "stone" ? (
                    <span className="pairs-stone" />
                  ) : null}
                  {c?.ice && <span className="pairs-ice" aria-hidden="true" />}
                </button>
              );
            })}
            {!!points.length && (
              <svg
                className="pairs-bolt"
                viewBox={`0 0 ${size.w} ${size.h}`}
                aria-hidden="true"
              >
                <polyline
                  points={points.map((p) => `${p.x},${p.y}`).join(" ")}
                  fill="none"
                  stroke="#e2b647"
                  strokeWidth={reduced ? 3 : 6}
                  strokeLinejoin="round"
                />
                <polyline
                  points={points.map((p) => `${p.x},${p.y}`).join(" ")}
                  fill="none"
                  stroke="#fffbea"
                  strokeWidth="2"
                />
                {!reduced &&
                  points.slice(1).map((p, i) => {
                    const start = points[i],
                      length = Math.hypot(p.x - start.x, p.y - start.y),
                      angle =
                        (Math.atan2(p.y - start.y, p.x - start.x) * 180) /
                        Math.PI;
                    return (
                      <image
                        key={i}
                        href={asset(
                          assets,
                          `effects/lightning-${frame + 1}.webp`,
                        )}
                        x="0"
                        y="-9"
                        width={length}
                        height="18"
                        preserveAspectRatio="none"
                        transform={`translate(${start.x} ${start.y}) rotate(${angle})`}
                        onError={(e) => {
                          e.currentTarget.setAttribute(
                            "href",
                            localAsset(`effects/lightning-${frame + 1}.webp`),
                          );
                        }}
                      />
                    );
                  })}
              </svg>
            )}
          </div>
        </>
      )}
      <p className="pairs-notice" role="status">
        {notice || data.description}
      </p>
      <details className="pairs-help">
        <summary>玩法与操作</summary>
        <p>
          点击两个相同图案，连线最多转两次弯，可经过棋盘外侧。先清空全部图案即获胜，无解时自动免费重排。键盘方向键移动焦点，空格或回车选择。
        </p>
        <p>
          石块挡路；相邻消除能击碎木箱、解冻冰层。每人独立拥有 3 次提示、2
          次重排和 2 次破障。120 秒未操作将自动托管，可随时取消。
        </p>
      </details>
    </section>
  );
}
