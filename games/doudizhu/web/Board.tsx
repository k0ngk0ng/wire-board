import { useEffect, useState } from "react";
import type {
  BoardProps,
  OptionsProps,
} from "../../../web/src/game-api/module";
import { Modal } from "../../../web/src/ui/board-host";
import {
  CardStrip,
  PokerLayout,
  PokerOptions,
  useCardStyle,
  usePokerFeedback,
  useSelection,
  type PokerEvent,
} from "../../../web/src/poker/ui";
import { soundFor } from "./sounds";
export type Options = {
  mode: "classic3" | "shanghai4";
  rounds: number;
  turnSeconds: number;
  allowStraight: boolean;
  personalDoubling: boolean;
};
type Move = { cards: number[]; kind: string };
export type View = {
  ruleset: string;
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
  marker: number;
  markerSeat: number;
  huangFan: boolean;
  touLiao: boolean;
  openHand: boolean;
  landlordHand: number[];
  reports: number[][];
  reportPlay: boolean;
  players: {
    count: number;
    role: string;
    score: number;
    double: number;
    bombLimit: number;
    bombUsed: number;
  }[];
  table: { seat: number; cards: number[]; pass: boolean }[];
  events: PokerEvent[];
  roundResults: {
    round: number;
    scores: number[];
    spring: string;
    multiplier: number;
    reason?: string;
    reportCount?: number;
  }[];
  controls: {
    canAct: boolean;
    canPass: boolean;
    canBid: boolean;
    canDouble: boolean;
    canReady: boolean;
    canChoose: boolean;
    canOpen: boolean;
    reports: number[][];
    bombRemaining: number;
    bids: number[];
    moves: Move[];
  };
};
const phaseNames: Record<string, string> = {
  bid: "叫档决定地主",
  landlord_choice: "地主选择摊打／报到",
  double: "农民选择加倍",
  redouble: "地主选择再加倍",
  play: "轮流出牌",
  round_end: "本副已结束",
  finished: "本场已结束",
};
export const tierName = (n: number) =>
  ["未定", "一档", "二档", "三档"][n] ?? String(n);
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
  const huanle = d.ruleset === "doudizhu-shanghai-huanle-v2",
    style = useCardStyle(),
    { selected, setSelected } = useSelection(d.hand, room.id + ":" + d.window);
  const [error, setError] = useState(""),
    [hint, setHint] = useState(0),
    [open, setOpen] = useState(false),
    [reports, setReports] = useState<number[]>([]),
    [peek, setPeek] = useState<"bottom" | "open" | null>(null);
  useEffect(() => {
    setOpen(false);
    setReports([]);
    setError("");
  }, [room.id, d.window]);
  const fresh = usePokerFeedback(
      d.events,
      room.id,
      audio,
      assets,
      (e) => soundFor(e, huanle),
      clock,
    ),
    c = d.controls;
  async function send(type: string, extra: Record<string, unknown> = {}) {
    setError("");
    try {
      await act({ type, window: d.window, ...extra });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }
  return (
    <>
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
              {huanle
                ? "上海斗地主"
                : d.options.mode === "shanghai4"
                  ? "旧版四人上海"
                  : "旧版三人经典"}
            </span>
            <span>
              {d.phase === "bid" && !huanle
                ? "叫分决定地主"
                : phaseNames[d.phase]}
            </span>
            <small>
              {huanle ? `档位 ${tierName(d.bid)}` : `底分 ${d.bid || "未定"}`} ·
              倍率 ×{d.multiplier}
            </small>
            {huanle && (
              <>
                <small>
                  {[
                    d.huangFan && "荒番",
                    d.touLiao && "头撩",
                    d.marker % 54 >= 52 && "明王",
                    d.openHand && "摊打",
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </small>
                <details>
                  <summary>标记牌 · {room.seats[d.markerSeat]?.name}</summary>
                  <CardStrip
                    ids={[d.marker]}
                    skin={style.skin}
                    assets={assets}
                  />
                </details>
              </>
            )}
            {d.bottom.length > 0 && (
              <button
                className="poker-table-link"
                onClick={() => setPeek("bottom")}
              >
                {huanle ? "你的底牌" : "公开底牌"} · {d.bottom.length} 张
              </button>
            )}
            {d.openHand && (
              <button
                className="poker-table-link"
                onClick={() => setPeek("open")}
              >
                查看地主明牌 · {d.landlordHand.length} 张
              </button>
            )}
            {(d.reports ?? []).length > 0 && (
              <details>
                <summary>{d.reports.length === 2 ? "双报到" : "报到"}</summary>
                {d.reports.map((cards, i) => (
                  <CardStrip
                    key={i}
                    ids={cards}
                    skin={style.skin}
                    assets={assets}
                  />
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
            {c.canBid &&
              c.bids.map((bid) => (
                <button
                  type="button"
                  key={bid}
                  disabled={busy}
                  className={bid ? "primary" : ""}
                  onClick={() => void send("bid", { bid })}
                >
                  {bid ? (huanle ? tierName(bid) : `叫 ${bid} 分`) : "不叫"}
                </button>
              ))}
            {c.canChoose && (
              <div className="poker-landlord-choices">
                {c.canOpen && (
                  <label>
                    <input
                      type="checkbox"
                      checked={open}
                      disabled={busy}
                      onChange={(e) => setOpen(e.target.checked)}
                    />
                    摊打：公开地主手牌，输赢倍率 ×2
                  </label>
                )}
                {c.reports.length > 0 && (
                  <>
                    <p>可选一组报到，或两组双报到：</p>
                    <div className="poker-report-groups">
                      {c.reports.map((cards, i) => (
                        <button
                          key={i}
                          type="button"
                          aria-label={`报到牌组 ${i + 1}`}
                          aria-pressed={reports.includes(i)}
                          disabled={
                            busy ||
                            (!reports.includes(i) && reports.length >= 2)
                          }
                          onClick={() =>
                            setReports((v) =>
                              v.includes(i)
                                ? v.filter((n) => n !== i)
                                : [...v, i],
                            )
                          }
                        >
                          <span>
                            {cards.length === 4
                              ? "四王"
                              : `${cards.length} 张炸弹`}
                          </span>
                          <CardStrip
                            ids={cards}
                            skin={style.skin}
                            assets={assets}
                          />
                        </button>
                      ))}
                    </div>
                  </>
                )}
                <div className="poker-choice-buttons">
                  <button
                    className="primary"
                    disabled={busy}
                    onClick={() =>
                      void send("landlord", {
                        openHand: open,
                        reports: [],
                        continue: true,
                      })
                    }
                  >
                    {open ? "摊打，不报到" : "正常出牌"}
                  </button>
                  {c.reports.length > 0 && (
                    <>
                      <button
                        disabled={busy || !reports.length}
                        onClick={() =>
                          void send("landlord", {
                            openHand: open,
                            reports,
                            continue: true,
                          })
                        }
                      >
                        报到后继续打
                      </button>
                      <button
                        disabled={busy || !reports.length}
                        onClick={() =>
                          void send("landlord", {
                            openHand: false,
                            reports,
                            continue: false,
                          })
                        }
                      >
                        报到不打
                      </button>
                    </>
                  )}
                </div>
                {c.reports.length > 0 && (
                  <p className="poker-notice">
                    报到不打直接结算，不计荒番、摊打、明王倍率；继续打按官网详细公式结算。
                  </p>
                )}
              </div>
            )}
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
                    setSelected(c.moves[hint % c.moves.length].cards);
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
                {huanle && c.canAct && (
                  <p className="poker-notice">
                    {c.bombRemaining < 0
                      ? "本局炸弹次数不限"
                      : `本局还可出 ${c.bombRemaining} 次炸弹（含王炸）`}
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
              <p className="poker-notice">等待其他玩家；你的手牌始终在下方。</p>
            )}
            {d.roundResults.length > 0 &&
              (d.phase === "round_end" || d.phase === "finished") && (
                <p className="poker-notice">
                  {d.roundResults.at(-1)!.reason === "report_no_play"
                    ? "报到不打，地主获胜。"
                    : ""}
                  本副积分：
                  {d.roundResults
                    .at(-1)!
                    .scores.map(
                      (v, p) => `${room.seats[p].name} ${v > 0 ? "+" : ""}${v}`,
                    )
                    .join(" · ")}
                </p>
              )}
            {!huanle && (
              <p className="poker-notice">
                这是更新前开始的对局，继续按原规则结束；新房间仅开放四人上海斗地主。
              </p>
            )}
          </>
        }
      />
      {peek && (
        <Modal
          title={
            peek === "open"
              ? "地主明牌"
              : huanle
                ? "你的底牌（仅你可见）"
                : "公开底牌"
          }
          onClose={() => setPeek(null)}
        >
          <div className="poker-open-cards">
            <CardStrip
              ids={peek === "open" ? d.landlordHand : d.bottom}
              skin={style.skin}
              assets={assets}
            />
          </div>
        </Modal>
      )}
    </>
  );
}
export function OptionsForm({
  value: v,
  onChange,
  capacity,
  disabled,
}: OptionsProps<Options>) {
  return (
    <PokerOptions>
      <label>
        玩法规则
        <select
          aria-label="玩法规则"
          disabled={disabled}
          value="shanghai4"
          onChange={() =>
            onChange({
              ...v,
              mode: "shanghai4",
              allowStraight: true,
              personalDoubling: false,
            })
          }
        >
          <option value="shanghai4">上海斗地主</option>
        </select>
      </label>
      <p>
        四人欢乐三打一：两副牌，一档／二档／三档叫档，底牌仅地主可见；支持炸弹次数限制、头撩、摊打和报到。
      </p>
      {capacity !== 4 && (
        <p role="alert">此旧等待房间人数不符合新规则，请重新创建四人房间。</p>
      )}
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
    </PokerOptions>
  );
}
