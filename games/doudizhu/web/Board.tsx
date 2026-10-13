import { useState } from "react";
import type {
  BoardProps,
  OptionsProps,
} from "../../../web/src/game-api/module";
import {
  CardStrip,
  PokerLayout,
  PokerOptions,
  useCardStyle,
  usePokerFeedback,
  useSelection,
  type PokerEvent,
} from "../../../web/src/poker/ui";
export type Options = {
  mode: "classic3" | "shanghai4";
  rounds: number;
  turnSeconds: number;
  allowStraight: boolean;
  personalDoubling: boolean;
};
type Move = { cards: number[]; kind: string };
export type View = {
  options: Options;
  phase: string;
  turn: number;
  window: number;
  round: number;
  hand: number[];
  bottom: number[];
  landlord: number;
  bid: number;
  multiplier: number;
  players: { count: number; role: string; score: number; double: number }[];
  table: { seat: number; cards: number[]; pass: boolean }[];
  events: PokerEvent[];
  roundResults: {
    round: number;
    scores: number[];
    spring: string;
    multiplier: number;
  }[];
  controls: {
    canAct: boolean;
    canPass: boolean;
    canBid: boolean;
    canDouble: boolean;
    canReady: boolean;
    bids: number[];
    moves: Move[];
  };
};
const sounds = {
  deal: "card-shuffle",
  play: "card-place-1",
  pass: "card-slide-1",
  bid: "card-fan-1",
  round_end: "card-fan-1",
};
const phaseNames: Record<string, string> = {
  bid: "叫分决定地主",
  double: "农民选择加倍",
  redouble: "地主选择再加倍",
  play: "轮流出牌",
  round_end: "本副已结束",
  finished: "本场已结束",
};
export const roleName = (role: string) =>
  ({ landlord: "地主", farmer: "农民", undecided: "待定身份" })[role] ?? role;
export function Board({
  room,
  data: d,
  act,
  busy,
  assets,
  audio,
  clock,
}: BoardProps<View>) {
  const style = useCardStyle(),
    { selected, setSelected } = useSelection(d.hand, room.id + ":" + d.window),
    [error, setError] = useState("");
  const [hint, setHint] = useState(0);
  const fresh = usePokerFeedback(
    d.events,
    room.id,
    audio,
    assets,
    sounds,
    clock,
  );
  const c = d.controls;
  async function send(type: string, extra: Record<string, unknown> = {}) {
    setError("");
    try {
      await act({ type, window: d.window, ...extra });
      if (type === "play") setSelected([]);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }
  return (
    <PokerLayout
      room={room}
      phase={d.phase}
      assets={assets}
      style={style}
      hand={d.hand}
      selected={selected}
      onSelect={setSelected}
      fresh={fresh}
      labels={d.players.map((p) => `${roleName(p.role)} · ${p.count} 张`)}
      plays={d.table}
      center={
        <>
          <strong>
            第 {d.round} / {d.options.rounds} 副
          </strong>
          <span>
            {d.options.mode === "shanghai4" ? "四人上海" : "三人经典"}
          </span>
          <span>{phaseNames[d.phase]}</span>
          <small>
            底分 {d.bid || "未定"} · 倍数 ×{d.multiplier}
          </small>
          {d.bottom.length > 0 && (
            <details>
              <summary>公开底牌 · {d.bottom.length} 张</summary>
              <CardStrip ids={d.bottom} skin={style.skin} assets={assets} />
            </details>
          )}
        </>
      }
      actions={
        <>
          {error && (
            <p className="poker-error" role="alert">
              {error}
            </p>
          )}
          {c.canBid &&
            c.bids.map((bid) => (
              <button
                type="button"
                key={bid}
                disabled={busy}
                className={bid ? "primary" : ""}
                onClick={() => void send("bid", { bid })}
              >
                {bid ? `叫 ${bid} 分` : "不叫"}
              </button>
            ))}
          {c.canDouble && (
            <>
              <button
                disabled={busy}
                onClick={() => void send("double", { double: false })}
              >
                不加倍
              </button>
              <button
                className="primary"
                disabled={busy}
                onClick={() => void send("double", { double: true })}
              >
                {d.phase === "redouble" ? "再加倍" : "加倍"}
              </button>
            </>
          )}
          {d.phase === "play" && !room.spectating && (
            <>
              <button
                disabled={busy || !c.canPass}
                onClick={() => void send("pass")}
              >
                不出
              </button>
              <button
                disabled={!c.canAct || busy || !c.moves.length}
                onClick={() => {
                  const move = c.moves[hint % c.moves.length];
                  setSelected(move.cards);
                  setHint(hint + 1);
                }}
              >
                提示
              </button>
              <button
                disabled={!selected.length || busy}
                onClick={() => setSelected([])}
              >
                取消选择
              </button>
              <button
                className="primary"
                disabled={busy || !c.canAct || !selected.length}
                onClick={() => void send("play", { cards: selected })}
              >
                出牌（{selected.length}）
              </button>
            </>
          )}
          {c.canReady && (
            <button
              className="primary"
              disabled={busy}
              onClick={() => void send("ready")}
            >
              准备下一副
            </button>
          )}
          {!c.canAct && d.phase !== "finished" && (
            <p className="poker-notice">等待其他玩家；你的手牌始终在下方。</p>
          )}
          {d.roundResults.length > 0 &&
            (d.phase === "round_end" || d.phase === "finished") && (
              <p className="poker-notice">
                本副积分：
                {d.roundResults
                  .at(-1)!
                  .scores.map(
                    (v, p) => `${room.seats[p].name} ${v > 0 ? "+" : ""}${v}`,
                  )
                  .join(" · ")}
              </p>
            )}
          {d.options.mode === "shanghai4" && (
            <p className="poker-notice">
              四王王炸最大，压过八张炸弹；一个地主对三位农民。
            </p>
          )}
        </>
      }
    />
  );
}
export function OptionsForm({
  value: v,
  onChange,
  capacity,
  disabled,
}: OptionsProps<Options>) {
  // Mode is derived from the shared seat selector, never a separate capacity.
  const expected = capacity === 4 ? "shanghai4" : "classic3";
  // Sync after shared capacity changes; the host prevents a 3/4 mismatch too.
  return (
    <PokerOptions>
      <ModeSync value={v} expected={expected} onChange={onChange} />
      <p>
        {capacity === 4
          ? "四人上海：两副牌，地主 33 张，对三位农民，四王王炸最大。"
          : "三人经典：一副牌，地主 20 张，对两位农民。"}
      </p>
      <label>
        对局副数
        <select
          disabled={disabled}
          value={v.rounds}
          onChange={(e) => onChange({ ...v, rounds: +e.target.value })}
        >
          {[1, 3, 5, 10].map((n) => (
            <option key={n} value={n}>
              {n} 副
            </option>
          ))}
        </select>
      </label>
      <label>
        行动时限
        <select
          aria-label="行动时限"
          disabled={disabled}
          value={v.turnSeconds}
          onChange={(e) => onChange({ ...v, turnSeconds: +e.target.value })}
        >
          {[15, 30, 60, 120].map((n) => (
            <option key={n} value={n}>
              {n} 秒
            </option>
          ))}
        </select>
      </label>
      <label>
        个人加倍
        <select
          disabled={disabled}
          value={String(v.personalDoubling)}
          onChange={(e) =>
            onChange({ ...v, personalDoubling: e.target.value === "true" })
          }
        >
          <option value="false">关闭</option>
          <option value="true">开启</option>
        </select>
      </label>
      {capacity === 4 && (
        <label>
          单顺
          <select
            disabled={disabled}
            value={String(v.allowStraight)}
            onChange={(e) =>
              onChange({ ...v, allowStraight: e.target.value === "true" })
            }
          >
            <option value="true">允许五张起单顺</option>
            <option value="false">不允许单顺</option>
          </select>
        </label>
      )}
    </PokerOptions>
  );
}
import { useEffect } from "react";
function ModeSync({
  value,
  expected,
  onChange,
}: {
  value: Options;
  expected: Options["mode"];
  onChange: (v: Options) => void;
}) {
  useEffect(() => {
    if (
      value.mode !== expected ||
      (expected === "classic3" && !value.allowStraight)
    )
      onChange({
        ...value,
        mode: expected,
        allowStraight: expected === "classic3" ? true : value.allowStraight,
      });
  }, [value, expected, onChange]);
  return null;
}
