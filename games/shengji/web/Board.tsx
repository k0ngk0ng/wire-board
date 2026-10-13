import { useState } from "react";
import type {
  BoardProps,
  OptionsProps,
} from "../../../web/src/game-api/module";
import {
  CardStrip,
  PokerLayout,
  PokerOptions,
  cardRank,
  rankName,
  useCardStyle,
  usePokerFeedback,
  useSelection,
  type PokerEvent,
} from "../../../web/src/poker/ui";
export type Options = {
  decks: number;
  rounds: number;
  startLevel: number;
  turnSeconds: number;
  throwPenalty: number;
  bottomMode: string;
};
type Play = { seat: number; cards: number[] };
type Declaration = {
  seat: number;
  cards: number[];
  trump: number;
  strength: number;
};
export type View = {
  options: Options;
  phase: string;
  turn: number;
  window: number;
  round: number;
  hand: number[];
  bottom: number[];
  level: number;
  levels: number[];
  trump: number;
  dealer: number;
  attackPoints: number;
  penalty: number;
  trickNumber: number;
  trick: Play[];
  lastTrick: { plays: Play[]; winner: number; points: number } | null;
  declaration: Declaration | null;
  revealed: { seat: number; cards: number[]; reason: string }[];
  challengeSeat: number;
  players: { count: number; team: number; level: number; dealer: boolean }[];
  events: PokerEvent[];
  roundResults: {
    round: number;
    attackPoints: number;
    bottomPoints: number;
    bottomFactor: number;
    team: number;
    promotion: number;
  }[];
  controls: {
    canAct: boolean;
    canPass: boolean;
    canReady: boolean;
    canRebel: boolean;
    declarations: Declaration[];
    moves: { cards: number[]; label: string }[];
    buryCount: number;
  };
};
const sounds = {
  deal: "card-shuffle",
  declare: "card-fan-1",
  play: "card-place-1",
  bury: "card-slide-1",
  trick: "card-fan-1",
  round_end: "card-fan-1",
};
const phases: Record<string, string> = {
  declare: "亮主／反主",
  bury: "庄家扣底",
  play: "逐墩出牌",
  round_end: "本副已结束",
  finished: "本场已结束",
};
export function trumpName(n: number) {
  return n < 0 ? "无主" : ["梅花", "方块", "红心", "黑桃"][n];
}
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
    [error, setError] = useState(""),
    [hint, setHint] = useState(0);
  const fresh = usePokerFeedback(
      d.events,
      room.id,
      audio,
      assets,
      sounds,
      clock,
    ),
    c = d.controls;
  const highlights: Record<number, string> = {};
  for (const id of d.hand) {
    if (cardRank(id) === d.level) highlights[id] = "级";
    else if (cardRank(id) >= 16 || Math.floor((id % 54) / 13) === d.trump)
      highlights[id] = "主";
  }
  async function send(type: string, extra: Record<string, unknown> = {}) {
    setError("");
    try {
      await act({ type, window: d.window, ...extra });
      setSelected([]);
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
      highlights={highlights}
      fresh={fresh}
      labels={d.players.map(
        (p, i) =>
          `${i === d.dealer ? "庄家" : d.dealer < 0 ? "待定" : i % 2 === d.dealer % 2 ? "庄家搭档" : "抓分方"} · ${p.count} 张`,
      )}
      plays={d.trick.length ? d.trick : (d.lastTrick?.plays ?? [])}
      center={
        <>
          <strong>
            打 {rankName(d.level)} ·{" "}
            {d.phase === "declare" && !d.declaration
              ? "主牌待定"
              : trumpName(d.trump)}
          </strong>
          <span>
            第 {d.round} 副 · 第 {d.trickNumber} 墩
          </span>
          <span>{phases[d.phase]}</span>
          <strong>
            抓分 {d.attackPoints} / {d.options.decks === 1 ? 40 : 80}
          </strong>
          <small>
            双方级数 {rankName(d.levels[0])} / {rankName(d.levels[1])}
          </small>
          {d.lastTrick && (
            <small>
              上墩：{room.seats[d.lastTrick.winner].name} · {d.lastTrick.points}{" "}
              分
            </small>
          )}
          {d.bottom.length > 0 && (
            <details>
              <summary>
                {d.phase === "play" ? "你的底牌" : "底牌"} · {d.bottom.length}{" "}
                张
              </summary>
              <CardStrip ids={d.bottom} skin={style.skin} assets={assets} />
            </details>
          )}
          {d.revealed.length > 0 && (
            <details>
              <summary>本副明示牌</summary>
              {d.revealed.map((v, i) => (
                <div key={i}>
                  <small>
                    {v.seat >= 0 ? room.seats[v.seat].name : "底牌"} ·{" "}
                    {v.reason}
                  </small>
                  <CardStrip ids={v.cards} skin={style.skin} assets={assets} />
                </div>
              ))}
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
          {d.phase === "declare" && c.canAct && (
            <>
              <button disabled={busy} onClick={() => void send("pass")}>
                不亮／不反
              </button>
              {c.declarations.map((v) => (
                <button
                  className="primary"
                  key={v.cards.join(",")}
                  disabled={busy}
                  onClick={() => void send("declare", { cards: v.cards })}
                >
                  {v.strength === 1
                    ? "单亮"
                    : v.strength === 2
                      ? "对子反主"
                      : "双王"}{" "}
                  · {trumpName(v.trump)}
                </button>
              ))}
              {c.canRebel && (
                <button disabled={busy} onClick={() => void send("rebel")}>
                  明示无分牌并重发
                </button>
              )}
            </>
          )}
          {d.phase === "bury" && (
            <>
              <p className="poker-notice">
                庄家选择 {c.buryCount || (d.options.decks === 1 ? 6 : 8)}{" "}
                张牌扣底，其他玩家看不到底牌。
              </p>
              {c.canAct && (
                <button
                  className="primary"
                  disabled={busy || selected.length !== c.buryCount}
                  onClick={() => void send("bury", { cards: selected })}
                >
                  确认扣底（{selected.length}/{c.buryCount}）
                </button>
              )}
            </>
          )}
          {d.phase === "play" && !room.spectating && (
            <>
              <button
                disabled={busy || !c.canAct || !c.moves.length}
                onClick={() => {
                  setSelected(c.moves[hint % c.moves.length].cards);
                  setHint(hint + 1);
                }}
              >
                提示
              </button>
              <button
                disabled={busy || !selected.length}
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
              {d.challengeSeat === room.you && (
                <p className="poker-notice">
                  你能破甩，须跟同门更大的对应牌型。
                </p>
              )}
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
            <p className="poker-notice">等待其他玩家。对面玩家是你的搭档。</p>
          )}
          {d.roundResults.length > 0 &&
            (d.phase === "round_end" || d.phase === "finished") && (
              <p className="poker-notice">
                抓分 {d.roundResults.at(-1)!.attackPoints} 分（抠底{" "}
                {d.roundResults.at(-1)!.bottomPoints} 分，倍率 ×
                {d.roundResults.at(-1)!.bottomFactor}）；队伍{" "}
                {d.roundResults.at(-1)!.team + 1} 升{" "}
                {d.roundResults.at(-1)!.promotion} 级。
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
  disabled,
}: OptionsProps<Options>) {
  return (
    <PokerOptions>
      <label>
        牌副数
        <select
          aria-label="牌副数"
          disabled={disabled}
          value={v.decks}
          onChange={(e) => onChange({ ...v, decks: +e.target.value })}
        >
          <option value={2}>两副牌 · 80 分</option>
          <option value={1}>一副牌 · 40 分</option>
        </select>
      </label>
      <label>
        结束条件
        <select
          disabled={disabled}
          value={v.rounds}
          onChange={(e) => onChange({ ...v, rounds: +e.target.value })}
        >
          <option value={0}>打到过 A</option>
          {[3, 6, 12].map((n) => (
            <option key={n} value={n}>
              {n} 副休闲局
            </option>
          ))}
        </select>
      </label>
      <label>
        起始级牌
        <select
          disabled={disabled}
          value={v.startLevel}
          onChange={(e) => onChange({ ...v, startLevel: +e.target.value })}
        >
          {Array.from({ length: 13 }, (_, i) => i + 2).map((n) => (
            <option key={n} value={n}>
              {rankName(n)}
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
        甩牌失败罚分
        <select
          disabled={disabled}
          value={v.throwPenalty}
          onChange={(e) => onChange({ ...v, throwPenalty: +e.target.value })}
        >
          <option value={10}>每收回一张罚 10 分</option>
          <option value={0}>不罚分，仍强制领出</option>
        </select>
      </label>
      {v.decks === 2 && (
        <label>
          抠底倍率
          <select
            disabled={disabled}
            value={v.bottomMode}
            onChange={(e) => onChange({ ...v, bottomMode: e.target.value })}
          >
            <option value="cards">末墩领出张数 × 2</option>
            <option value="structure">最大结构指数倍率</option>
          </select>
        </label>
      )}
      <p>
        上海预设：无常主 2、无勾
        J，级牌始终为主。所有其他玩家（含对家）都可破甩；首版发牌完成后集中亮主。A
        必须坐庄打过。
      </p>
    </PokerOptions>
  );
}
