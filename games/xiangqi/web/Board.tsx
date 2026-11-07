import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import type { View } from "./types";
const red = ["", "帅", "仕", "相", "马", "车", "炮", "兵"],
  blackNames = ["", "将", "士", "象", "马", "车", "炮", "卒"];
const glyph = (code: number) => (code > 0 ? red : blackNames)[Math.abs(code)];
const pieceName = (code: number) =>
  code ? (code > 0 ? "红" : "黑") + glyph(code) : "空点";
const label = (sq: number) =>
  String.fromCharCode(97 + (sq % 9)) + (9 - Math.floor(sq / 9));
const local = (file: string) => "/xiangqi/v1/" + file;
const source = (base: string, file: string) =>
  base.replace(/\/$/, "") + local(file);
const reasons: Record<string, string> = {
  checkmate: "将杀",
  stalemate_loss: "困毙，无法走子方判负",
  dead_position: "双方无法取胜",
  perpetual_check: "单方长将未变着",
  perpetual_chase: "单方长捉未变着",
  repetition: "循环局面和棋",
  fifty_move: "连续 50 回合没有吃子",
  agreement: "双方同意和棋",
  resign: "认输",
};
function Piece({ code, assets }: { code: number; assets: string }) {
  const [failed, setFailed] = useState(false),
    file = "images/" + (code > 0 ? "r" : "b") + Math.abs(code) + ".webp";
  useEffect(() => setFailed(false), [assets, file]);
  return (
    <span
      className={"xiangqi-piece " + (code > 0 ? "red" : "black")}
      aria-hidden="true"
    >
      {failed ? (
        <span className="xiangqi-piece-fallback">{glyph(code)}</span>
      ) : (
        <img
          alt=""
          draggable={false}
          src={source(assets, file)}
          onError={(e) => {
            if (e.currentTarget.getAttribute("src") !== local(file))
              e.currentTarget.src = local(file);
            else setFailed(true);
          }}
        />
      )}
    </span>
  );
}
export function Board({
  room,
  data,
  act,
  busy,
  assets,
  audio,
  clock,
}: BoardProps<View>) {
  const controllerRoot = useRef<HTMLElement>(null);
  const [selected, setSelected] = useState<number | null>(null),
    [target, setTarget] = useState<number | null>(null),
    [focus, setFocus] = useState(67),
    [offer, setOffer] = useState(false),
    [resign, setResign] = useState(false);
  useControllerCancel(controllerRoot, () => {
    setSelected(null);
    setTarget(null);
  });
  const cells = useRef<(HTMLButtonElement | null)[]>([]);
  const flipped = !room.spectating && room.you !== data.red;
  const scope = `${room.id}:${data.round}:${room.you}:${clock.epoch ?? 0}`;
  const seen = useRef({ scope, id: data.last.id, phase: data.phase });
  const canAct = !busy && !room.spectating && !room.seats[room.you]?.autoPlay,
    canMove = canAct && data.canMove;
  const send = (type: string, extra: Record<string, unknown> = {}) =>
    act({ type, round: data.round, ply: data.ply, ...extra });
  useEffect(() => {
    setSelected(null);
    setTarget(null);
    setOffer(false);
    setResign(false);
  }, [scope, data.ply, data.phase]);
  useEffect(() => {
    if (!canMove) {
      setSelected(null);
      setTarget(null);
    }
  }, [canMove]);
  useEffect(() => {
    const old = seen.current;
    seen.current = { scope, id: data.last.id, phase: data.phase };
    if (old.scope !== scope) {
      audio.stop();
      return;
    }
    if (
      data.last.id > old.id ||
      (data.phase === "finished" && old.phase !== "finished")
    ) {
      const cue =
        data.phase === "finished"
          ? data.winner < 0
            ? "draw"
            : "win"
          : data.last.check
            ? "check"
            : data.last.capture
              ? "capture"
              : "move";
      void audio.playWithFallback([
        source(assets, "audio/" + cue + ".wav"),
        local("audio/" + cue + ".wav"),
      ]);
    }
  }, [scope, data.last.id, data.phase, assets, audio]);
  useEffect(() => () => audio.stop(), [audio]);
  const available =
      selected === null ? [] : data.legal.filter((m) => m.from === selected),
    choice = available.find((m) => m.to === target);
  const xy = (sq: number) => ({
    x: flipped ? 8 - (sq % 9) : sq % 9,
    y: flipped ? 9 - Math.floor(sq / 9) : Math.floor(sq / 9),
  });
  const pos = (sq: number) => {
    const p = xy(sq);
    return {
      left: ((25 + 50 * p.x) / 450) * 100 + "%",
      top: ((25 + 50 * p.y) / 500) * 100 + "%",
    };
  };
  const pick = (sq: number) => {
    if (!canMove) return;
    if (available.some((m) => m.to === sq)) {
      setTarget(sq);
      return;
    }
    setSelected(data.legal.some((m) => m.from === sq) ? sq : null);
    setTarget(null);
  };
  const event = data.motion?.event;
  return (
    <section
      ref={controllerRoot}
      className="xiangqi-board"
      data-ply={data.ply}
      data-moving={event?.id ?? ""}
    >
      <div className="xiangqi-heading">
        <div>
          <h2>中国象棋</h2>
          <p>红先黑后 · 每步 {data.options.turnSeconds} 秒，超时托管</p>
        </div>
        <strong>
          {data.phase === "finished"
            ? data.winner < 0
              ? "本局和棋"
              : "本局已结束"
            : `第 ${data.ply + 1} 手 · ${data.turn === data.red ? "红" : "黑"}方回合`}
        </strong>
      </div>
      {data.warning && data.phase === "playing" && (
        <p className="xiangqi-warning" role="alert">
          {data.warning}
        </p>
      )}
      <div className="xiangqi-workspace">
        <div className="xiangqi-play-area">
          <div className="xiangqi-orientation">
            {room.spectating
              ? "红方在下方"
              : `你执${flipped ? "黑" : "红"}，己方在下方`}
            {data.checkSquare >= 0 ? " · 将军" : ""}
          </div>
          <div
            data-controller-group="board"
            className="xiangqi-map"
            role="group"
            aria-label="中国象棋棋盘"
          >
            <svg
              className="xiangqi-grid"
              viewBox="0 0 450 500"
              aria-hidden="true"
            >
              <rect width="450" height="500" rx="12" fill="#e6cca0" />
              <image
                href={source(assets, "images/board.webp")}
                width="450"
                height="500"
                onError={(e) => {
                  if (
                    e.currentTarget.getAttribute("href") !==
                    local("images/board.webp")
                  )
                    e.currentTarget.setAttribute(
                      "href",
                      local("images/board.webp"),
                    );
                  else e.currentTarget.style.display = "none";
                }}
              />
              <g stroke="#745536" strokeWidth="1.5" fill="none">
                {Array.from({ length: 10 }, (_, i) => (
                  <path key={"h" + i} d={`M25 ${25 + i * 50}H425`} />
                ))}
                {Array.from({ length: 9 }, (_, i) => (
                  <path
                    key={"v" + i}
                    d={
                      i === 0 || i === 8
                        ? `M${25 + i * 50} 25V475`
                        : `M${25 + i * 50} 25V225M${25 + i * 50} 275V475`
                    }
                  />
                ))}
                <path d="M175 25 275 125M275 25 175 125M175 375 275 475M275 375 175 475" />
                {[
                  ...[1, 7].flatMap((x) => [2, 7].map((y) => [x, y])),
                  ...[0, 2, 4, 6, 8].flatMap((x) => [3, 6].map((y) => [x, y])),
                ].map(([x, y]) => (
                  <g key={x + "-" + y}>
                    {x > 0 && (
                      <path
                        d={`M${25 + x * 50 - 5} ${25 + y * 50 - 12}v7h-7M${25 + x * 50 - 5} ${25 + y * 50 + 12}v-7h-7`}
                      />
                    )}
                    {x < 8 && (
                      <path
                        d={`M${25 + x * 50 + 5} ${25 + y * 50 - 12}v7h7M${25 + x * 50 + 5} ${25 + y * 50 + 12}v-7h7`}
                      />
                    )}
                  </g>
                ))}
              </g>
              <g
                fill="#674b2e"
                fontFamily="serif"
                fontSize="22"
                textAnchor="middle"
              >
                <text x="125" y="257">
                  楚 河
                </text>
                <text x="325" y="257">
                  汉 界
                </text>
              </g>
              <g fill="#5c462e" fontSize="10" textAnchor="middle">
                {Array.from({ length: 9 }, (_, i) => (
                  <text key={i} x={25 + i * 50} y="495">
                    {String.fromCharCode(97 + (flipped ? 8 - i : i))}
                  </text>
                ))}
                {Array.from({ length: 10 }, (_, i) => (
                  <text key={i} x="8" y={29 + i * 50}>
                    {flipped ? i : 9 - i}
                  </text>
                ))}
              </g>
            </svg>
            {data.board.map((code, sq) => (
              <button
                key={sq}
                type="button"
                ref={(e) => {
                  cells.current[sq] = e;
                }}
                data-xiangqi-square={label(sq)}
                className={
                  "xiangqi-point" +
                  (sq === selected ? " selected" : "") +
                  (sq === target ? " target" : "") +
                  (sq === data.checkSquare ? " checked" : "") +
                  (data.last.id &&
                  (sq === data.last.from || sq === data.last.to)
                    ? " last"
                    : "")
                }
                style={pos(sq)}
                aria-label={`${label(sq)}，${pieceName(code)}${available.some((m) => m.to === sq) ? "，可到达" : ""}`}
                aria-disabled={!canMove}
                aria-pressed={selected === sq || target === sq}
                tabIndex={focus === sq ? 0 : -1}
                onFocus={() => setFocus(sq)}
                onClick={() => pick(sq)}
                onKeyDown={(e) => {
                  const dx =
                      e.key === "ArrowLeft"
                        ? -1
                        : e.key === "ArrowRight"
                          ? 1
                          : 0,
                    dy =
                      e.key === "ArrowUp" ? -1 : e.key === "ArrowDown" ? 1 : 0;
                  if (dx || dy) {
                    e.preventDefault();
                    const p = xy(sq),
                      x = Math.max(0, Math.min(8, p.x + dx)),
                      y = Math.max(0, Math.min(9, p.y + dy)),
                      n = flipped ? (9 - y) * 9 + 8 - x : y * 9 + x;
                    setFocus(n);
                    cells.current[n]?.focus();
                  }
                }}
              >
                {!!code && event?.from !== sq && (
                  <Piece code={code} assets={assets} />
                )}
                {available.some((m) => m.to === sq) && (
                  <span
                    className={"xiangqi-destination " + (code ? "capture" : "")}
                  />
                )}
              </button>
            ))}
            {event && (
              <div
                key={event.id}
                className="xiangqi-moving"
                style={
                  {
                    ...pos(event.from),
                    "--from-x": pos(event.from).left,
                    "--from-y": pos(event.from).top,
                    "--to-x": pos(event.to).left,
                    "--to-y": pos(event.to).top,
                    "--duration": data.motion!.duration + "ms",
                  } as CSSProperties
                }
              >
                <Piece code={event.piece} assets={assets} />
              </div>
            )}
          </div>
          <p className="xiangqi-hint">
            选择棋子和目标点后确认。方向键移动焦点，回车选择。
          </p>
        </div>
        <aside data-controller-group="actions" className="xiangqi-actions">
          <h3>
            {data.phase === "finished"
              ? (reasons[data.reason] ?? "对局结束")
              : room.spectating
                ? "正在观战"
                : event
                  ? "正在走子"
                  : canMove
                    ? "轮到你走子"
                    : "等待对方走子"}
          </h3>
          <p role="status">
            {selected === null
              ? "点选己方棋子，查看合法目标点。"
              : target === null
                ? `已选择 ${label(selected)}，请选择目标点。`
                : `${label(selected)} → ${label(target)}，尚未提交。`}
          </p>
          <button
            type="button"
            className="primary"
            data-xiangqi-confirm
            disabled={!canMove || !choice}
            onClick={() => {
              if (choice) void send("move", { ...choice, offerDraw: offer });
            }}
          >
            确认走子
          </button>
          <label className="xiangqi-offer">
            <input
              type="checkbox"
              checked={offer}
              disabled={!canMove}
              onChange={(e) => setOffer(e.target.checked)}
            />
            随本步提议和棋
          </label>
          {data.canRespond && (
            <div className="xiangqi-offer-response">
              <p>对方提议和棋，计时继续。</p>
              <button
                disabled={!canAct}
                onClick={() => void send("respond_draw", { accept: true })}
              >
                同意和棋
              </button>
              <button
                disabled={!canAct}
                onClick={() => void send("respond_draw", { accept: false })}
              >
                继续对弈
              </button>
            </div>
          )}
          <button
            type="button"
            disabled={!canAct || !data.canResign}
            data-xiangqi-resign
            onClick={() => setResign(true)}
          >
            认输
          </button>
          <div className="xiangqi-captured">
            <h3>吃子记录</h3>
            {[data.red, 1 - data.red].map((seat) => (
              <p key={seat}>
                <b>{seat === data.red ? "红" : "黑"}方吃掉：</b>
                {data.captured[seat]?.map(pieceName).join("、") || "暂无"}
              </p>
            ))}
          </div>
          <p>已连续 {data.noCapture} 着未吃子；100 着自动和棋。</p>
          <details>
            <summary>着法记录</summary>
            <div className="xiangqi-history">
              {data.moves.length
                ? data.moves.map((m, i) => (
                    <span key={i}>
                      {i + 1}. {label(m.from)} → {label(m.to)}
                    </span>
                  ))
                : "尚未走子"}
            </div>
          </details>
          <p>真人和棋双方 0 积分，不计胜场。</p>
        </aside>
      </div>
      {resign && (
        <Modal title="确认认输" onClose={() => setResign(false)}>
          <p>认输会结束本局，对方获胜。</p>
          <div className="button-row">
            <button onClick={() => setResign(false)}>继续对弈</button>
            <button
              className="danger"
              disabled={!canAct}
              onClick={() => {
                setResign(false);
                void send("resign");
              }}
            >
              确认认输
            </button>
          </div>
        </Modal>
      )}
    </section>
  );
}
