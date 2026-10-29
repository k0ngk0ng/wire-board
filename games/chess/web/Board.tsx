import { useControllerCancel } from "../../../web/src/controllers/use-controller-cancel";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import type { View, LegalMove } from "./types";
const name: Record<string, string> = {
  K: "王",
  Q: "后",
  R: "车",
  B: "象",
  N: "马",
  P: "兵",
};
const glyph: Record<string, string> = {
  wK: "♔",
  wQ: "♕",
  wR: "♖",
  wB: "♗",
  wN: "♘",
  wP: "♙",
  bK: "♚",
  bQ: "♛",
  bR: "♜",
  bB: "♝",
  bN: "♞",
  bP: "♟",
};
const label = (sq: number) =>
  String.fromCharCode(97 + (sq % 8)) + (Math.floor(sq / 8) + 1);
const local = (path: string) => "/chess/v1/" + path;
const source = (base: string, path: string) =>
  base.replace(/\/$/, "") + local(path);
const pieceName = (code: string) =>
  code ? (code[0] === "w" ? "白" : "黑") + name[code[1]] : "空格";
const reasons: Record<string, string> = {
  checkmate: "将杀",
  stalemate: "逼和",
  dead_position: "无法将杀的死局",
  fivefold: "五次重复局面",
  seventy_five_move: "75 回合未动兵或吃子",
  threefold_claim: "三次重复局面申请",
  fifty_move_claim: "50 回合规则申请",
  agreement: "双方同意和棋",
  resign: "认输",
  resign_no_mate: "对手无法将杀，认输按和棋处理",
};
function Piece({ code, assets }: { code: string; assets: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [assets, code]);
  return (
    <span
      className={"chess-piece " + (code[0] === "w" ? "white" : "black")}
      aria-hidden="true"
    >
      {failed ? (
        <span>{glyph[code]}</span>
      ) : (
        <img
          draggable={false}
          alt=""
          src={source(assets, "images/" + code + ".webp")}
          onError={(e) => {
            const path = local("images/" + code + ".webp");
            if (e.currentTarget.getAttribute("src") !== path)
              e.currentTarget.src = path;
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
    [promotion, setPromotion] = useState(""),
    [offer, setOffer] = useState(false),
    [resign, setResign] = useState(false),
    [focus, setFocus] = useState(12);
  useControllerCancel(controllerRoot, () => {
    setSelected(null);
    setTarget(null);
    setPromotion("");
  });
  const cells = useRef<(HTMLButtonElement | null)[]>([]);
  const black = !room.spectating && room.you !== data.white;
  const scope = `${room.id}:${data.round}:${room.you}:${clock.epoch ?? 0}`;
  const seen = useRef({ scope, id: data.last.id, phase: data.phase });
  const canMove =
    data.canMove &&
    !busy &&
    !room.spectating &&
    !room.seats[room.you]?.autoPlay;
  const canAct = !busy && !room.spectating && !room.seats[room.you]?.autoPlay;
  const send = (type: string, extra: Record<string, unknown> = {}) =>
    act({ type, round: data.round, ply: data.ply, ...extra });
  useEffect(() => {
    setSelected(null);
    setTarget(null);
    setPromotion("");
    setOffer(false);
    setResign(false);
  }, [scope, data.ply, data.phase]);
  useEffect(() => {
    if (!canMove) {
      setSelected(null);
      setTarget(null);
      setPromotion("");
    }
  }, [canMove]);
  useEffect(() => {
    const before = seen.current;
    seen.current = { scope, id: data.last.id, phase: data.phase };
    if (before.scope !== scope) {
      audio.stop();
      return;
    }
    if (
      data.last.id > before.id ||
      (data.phase === "finished" && before.phase !== "finished")
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
  }, [scope, data.last.id, data.phase, audio, assets]);
  useEffect(() => () => audio.stop(), [audio]);
  const available =
    selected === null ? [] : data.legal.filter((m) => m.from === selected);
  const candidates = available.filter((m) => m.to === target);
  const choice = candidates.find(
    (m) => !m.promotion || m.promotion === promotion,
  );
  const claims = choice ? (data.claimMoves[choice.uci] ?? []) : data.claims;
  const xy = (sq: number) => ({
    x: black ? 7 - (sq % 8) : sq % 8,
    y: black ? Math.floor(sq / 8) : 7 - Math.floor(sq / 8),
  });
  const pick = (sq: number) => {
    if (!canMove) return;
    if (available.some((m) => m.to === sq)) {
      setTarget(sq);
      setPromotion("");
      return;
    }
    setSelected(data.legal.some((m) => m.from === sq) ? sq : null);
    setTarget(null);
    setPromotion("");
  };
  const event = data.motion?.event;
  const moving = (from: number, to: number, code: string, key: string) => {
    const a = xy(from),
      b = xy(to);
    return (
      <div
        key={key}
        className="chess-moving"
        style={
          {
            left: a.x * 12.5 + "%",
            top: a.y * 12.5 + "%",
            "--dx": (b.x - a.x) * 100 + "%",
            "--dy": (b.y - a.y) * 100 + "%",
            "--duration": data.motion!.duration + "ms",
          } as CSSProperties
        }
      >
        <Piece code={code} assets={assets} />
      </div>
    );
  };
  return (
    <section
      ref={controllerRoot}
      className="chess-board"
      data-ply={data.ply}
      data-moving={event?.id ?? ""}
    >
      <div className="chess-heading">
        <div>
          <h2>国际象棋</h2>
          <p>标准规则 · 每步 {data.options.turnSeconds} 秒，超时托管</p>
        </div>
        <strong>
          {data.phase === "finished"
            ? data.winner < 0
              ? "本局和棋"
              : "本局已结束"
            : `第 ${data.ply + 1} 手 · ${data.turn === data.white ? "白" : "黑"}方回合`}
        </strong>
      </div>
      <div className="chess-workspace">
        <div className="chess-play-area">
          <div className="chess-orientation">
            {room.spectating
              ? "白方在下方"
              : `你执${black ? "黑" : "白"}，己方在下方`}
            {data.checkSquare >= 0 ? " · 王被将军" : ""}
          </div>
          <div
            data-controller-group="board"
            className="chess-grid"
            role="group"
            aria-label="国际象棋棋盘"
          >
            {Array.from({ length: 64 }, (_, display) => {
              const sq = black
                ? Math.floor(display / 8) * 8 + 7 - (display % 8)
                : (7 - Math.floor(display / 8)) * 8 + (display % 8);
              const code = data.board[sq],
                dest = available.some((m) => m.to === sq),
                hidden = event && (sq === event.from || sq === event.rookFrom);
              return (
                <button
                  type="button"
                  key={sq}
                  ref={(e) => {
                    cells.current[sq] = e;
                  }}
                  data-chess-square={label(sq)}
                  className={
                    "chess-square " +
                    (((sq % 8) + Math.floor(sq / 8)) % 2 ? "light" : "dark") +
                    (selected === sq ? " selected" : "") +
                    (target === sq ? " target" : "") +
                    (data.checkSquare === sq ? " checked" : "") +
                    (data.last.id &&
                    (sq === data.last.from || sq === data.last.to)
                      ? " last"
                      : "")
                  }
                  aria-label={`${label(sq)}，${pieceName(code)}${dest ? "，可到达" : ""}`}
                  aria-pressed={selected === sq || target === sq}
                  aria-disabled={!canMove}
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
                        e.key === "ArrowUp"
                          ? -1
                          : e.key === "ArrowDown"
                            ? 1
                            : 0;
                    if (dx || dy) {
                      e.preventDefault();
                      const p = xy(sq),
                        x = Math.max(0, Math.min(7, p.x + dx)),
                        y = Math.max(0, Math.min(7, p.y + dy));
                      const n = black ? y * 8 + 7 - x : (7 - y) * 8 + x;
                      setFocus(n);
                      cells.current[n]?.focus();
                    }
                  }}
                >
                  {code && !hidden && <Piece code={code} assets={assets} />}
                  {dest && (
                    <span
                      className={"chess-destination " + (code ? "capture" : "")}
                    />
                  )}
                  {display % 8 === 0 && (
                    <small className="chess-rank">
                      {Math.floor(sq / 8) + 1}
                    </small>
                  )}
                  {display >= 56 && (
                    <small className="chess-file">{label(sq)[0]}</small>
                  )}
                </button>
              );
            })}
            {event &&
              moving(event.from, event.to, event.piece, "piece-" + event.id)}
            {event &&
              event.rookFrom >= 0 &&
              moving(
                event.rookFrom,
                event.rookTo,
                event.piece[0] + "R",
                "rook-" + event.id,
              )}
          </div>
          <p className="chess-hint">
            选择棋子与目标格后确认。方向键移动焦点，回车选择。
          </p>
        </div>
        <aside data-controller-group="actions" className="chess-actions">
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
              ? "点选己方棋子，查看合法目标格。"
              : target === null
                ? `已选择 ${label(selected)}，请选择目标格。`
                : `${label(selected)} → ${label(target)}，尚未提交。`}
          </p>
          {candidates.some((m) => m.promotion) && (
            <div
              className="chess-promotion"
              role="group"
              aria-label="选择升变棋子"
            >
              <strong>兵升变，请选择</strong>
              {["q", "r", "b", "n"].map((p) => (
                <button
                  type="button"
                  key={p}
                  aria-label={"升变为" + name[p.toUpperCase()]}
                  aria-pressed={promotion === p}
                  onClick={() => setPromotion(p)}
                >
                  {name[p.toUpperCase()]}
                </button>
              ))}
            </div>
          )}
          <button
            type="button"
            className="primary"
            data-chess-confirm
            disabled={!canMove || !choice}
            onClick={() => {
              if (choice)
                void send("move", { uci: choice.uci, offerDraw: offer });
            }}
          >
            确认走子
          </button>
          <label className="chess-offer">
            <input
              type="checkbox"
              checked={offer}
              disabled={!canMove}
              onChange={(e) => setOffer(e.target.checked)}
            />
            随本步提议和棋
          </label>
          {data.canRespond && (
            <div className="chess-offer-response">
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
          {claims.map((reason) => (
            <button
              key={reason}
              disabled={!canMove}
              onClick={() =>
                void send("claim_draw", {
                  reason,
                  ...(choice ? { uci: choice.uci } : {}),
                })
              }
            >
              {choice ? "预告本步并申请" : "申请"}
              {reason === "threefold" ? "三次重复和棋" : "50 回合和棋"}
            </button>
          ))}
          {!!claims.length && <p>申请成立即和棋。预告走子不会实际执行。</p>}
          <button
            type="button"
            disabled={!canAct || !data.canResign}
            data-chess-resign
            onClick={() => setResign(true)}
          >
            认输
          </button>
          <div className="chess-captured">
            <h3>吃子记录</h3>
            {[data.white, 1 - data.white].map((seat) => (
              <p key={seat}>
                <b>{seat === data.white ? "白" : "黑"}方吃掉：</b>
                {data.captured[seat]?.map(pieceName).join("、") || "暂无"}
              </p>
            ))}
          </div>
          <details>
            <summary>着法记录与棋谱</summary>
            <div className="chess-history">
              {data.san.length
                ? data.san.map((m, i) => (
                    <span key={i}>
                      {i % 2 === 0 ? `${Math.floor(i / 2) + 1}. ` : ""}
                      {m}
                    </span>
                  ))
                : "尚未走子"}
            </div>
            <label>
              PGN 棋谱
              <textarea aria-label="PGN 棋谱" readOnly value={data.pgn} />
            </label>
          </details>
          <p>真人和棋双方 0 积分，不计胜场。</p>
        </aside>
      </div>
      {resign && (
        <Modal title="确认认输" onClose={() => setResign(false)}>
          <p>认输会结束本局。对手无法通过任何合法走子将杀时按和棋处理。</p>
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
