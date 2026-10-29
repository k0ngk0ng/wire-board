import { Modal } from "../../../web/src/ui/board-host";
import { soundFor } from "./sounds";
import { useEffect, useState } from "react";
import type {
  BoardProps,
  OptionsProps,
} from "../../../web/src/game-api/module";
import {
  CardStrip,
  PokerLayout,
  PokerNotice,
  PokerOptions,
  rankName,
  useCardStyle,
  usePokerFeedback,
  useSelection,
  type PokerEvent,
} from "../../../web/src/poker/ui";
export type Options = {
  rounds: number;
  startLevel: number;
  turnSeconds: number;
};
type Combo = {
  key: string;
  kind: string;
  high: number;
  wild: { rank: number; suit: number }[];
};
type Move = Combo & { cards: number[] };
export type View = {
  options: Options;
  phase: string;
  turn: number;
  window: number;
  round: number;
  dealSerial?: number;
  hand: number[];
  level: number;
  levels: number[];
  players: { count: number; team: number; level: number; place: number }[];
  table: { seat: number; cards: number[]; pass: boolean }[];
  exchanges: {
    from: number;
    to: number;
    given: number;
    returned: number;
    done: boolean;
  }[];
  antiTribute: boolean;
  events: PokerEvent[];
  controls: {
    canAct: boolean;
    canPass: boolean;
    canReady: boolean;
    moves: Move[];
    choices: number[];
    previewCards: number[];
    interpretations: Combo[];
  };
  roundResults: { head: number; partnerPlace: number; promotion: number }[];
};

const names: Record<string, string> = {
  single: "单张",
  pair: "对子",
  triple: "三张",
  full_house: "三带对",
  straight: "顺子",
  tube: "三连对",
  plate: "二连三",
  straight_flush: "同花顺",
  bomb: "炸弹",
  rocket: "王炸",
};
const phases: Record<string, string> = {
  play: "轮流出牌",
  tribute: "进贡最大非配牌",
  return: "还贡",
  round_end: "本副已结束",
  finished: "本场已结束",
};
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
    { selected, setSelected } = useSelection(
      d.hand,
      JSON.stringify([
        room.id,
        room.matchId,
        room.you,
        !!room.spectating,
        d.dealSerial ?? d.round,
      ]),
    ),
    [key, setKey] = useState(""),
    [error, setError] = useState(""),
    [hint, setHint] = useState(0),
    [chooseOpen, setChooseOpen] = useState(false),
    [pendingPlay, setPendingPlay] = useState<{
      cards: number[];
      window: number;
    } | null>(null);
  useEffect(() => {
    setHint(0);
    setChooseOpen(false);
    setPendingPlay(null);
  }, [room.id, d.window]);
  const fresh = usePokerFeedback(
      d.events,
      room.id,
      audio,
      assets,
      soundFor,
      clock,
    ),
    c = d.controls;
  const matches = (a: number[], b: number[]) =>
    a.length === b.length && a.every((id) => b.includes(id));
  const preview = matches(selected, c.previewCards) ? c.interpretations : [];
  const interpretations = preview.length
    ? preview
    : c.moves.filter((m) => matches(selected, m.cards));
  const chosen =
    interpretations.find((v) => v.key === key) ?? interpretations[0];
  useEffect(() => {
    if (
      busy ||
      !pendingPlay ||
      pendingPlay.window !== d.window ||
      !matches(pendingPlay.cards, selected) ||
      !matches(pendingPlay.cards, c.previewCards) ||
      !c.interpretations.length
    )
      return;
    setPendingPlay(null);
    if (c.interpretations.length === 1)
      void send("play", {
        cards: pendingPlay.cards,
        interpretation: c.interpretations[0].key,
      });
    else {
      setKey(c.interpretations[0].key);
      setChooseOpen(true);
    }
  }, [
    busy,
    pendingPlay,
    d.window,
    selected,
    c.previewCards,
    c.interpretations,
  ]);
  function playSelected() {
    if (interpretations.length === 1) {
      void send("play", {
        cards: selected,
        interpretation: interpretations[0].key,
      });
      return;
    }
    if (interpretations.length > 1) {
      setChooseOpen(true);
      return;
    }
    setPendingPlay({ cards: [...selected], window: d.window });
    void send("preview", { cards: selected });
  }
  const highlights: Record<number, string> = {};
  for (const id of d.hand)
    if (id % 54 === 26 + d.level - 2) highlights[id] = "配";
  async function send(type: string, extra: Record<string, unknown> = {}) {
    setError("");
    try {
      await act({ type, window: d.window, ...extra });
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
      handCapacity={28}
      actionColumns={3}
      selected={selected}
      onSelect={(v) => {
        setSelected(v);
        setKey("");
        setPendingPlay(null);
        setChooseOpen(false);
      }}
      highlights={highlights}
      fresh={fresh}
      labels={d.players.map(
        (p, i) =>
          `${i % 2 === room.you % 2 && !room.spectating ? "己方" : "队伍 " + (p.team + 1)}${p.place ? " · 第 " + p.place + " 名" : ""}`,
      )}
      counts={d.players.map((p) => p.count)}
      plays={d.table}
      center={
        <>
          <strong>打 {rankName(d.level)}</strong>
          <small>
            {rankName(d.levels[0])} : {rankName(d.levels[1])}
          </small>
          {d.phase !== "play" && <span>{phases[d.phase]}</span>}
          {d.antiTribute && <small>本副抗贡</small>}
        </>
      }
      actions={
        <>
          {error && (
            <Modal title="无法出牌" onClose={() => setError("")}>
              <p role="alert">{error}</p>
            </Modal>
          )}
          {(d.phase === "tribute" || d.phase === "return") && (
            <>
              <PokerNotice
                label={d.phase === "tribute" ? "进贡说明" : "还贡说明"}
              >
                {d.phase === "tribute"
                  ? "进贡最大的非配牌；可选的同点牌由你决定。"
                  : "还 10 以下非级牌，不能还刚收到的贡牌；无符合牌时还最小非配牌。还牌仅双方可见。"}
              </PokerNotice>
              {c.canAct && (
                <>
                  <button
                    disabled={busy}
                    onClick={() => setSelected([c.choices[0]])}
                  >
                    提示
                  </button>
                  <button
                    className="primary"
                    disabled={
                      busy ||
                      selected.length !== 1 ||
                      !c.choices.includes(selected[0])
                    }
                    onClick={() => void send(d.phase, { card: selected[0] })}
                  >
                    {d.phase === "tribute" ? "确认进贡" : "确认还贡"}
                  </button>
                </>
              )}
            </>
          )}
          {d.phase === "play" && !room.spectating && (
            <>
              <button
                disabled={busy || !c.canPass}
                data-controller-action="pass"
                onClick={() => void send("pass")}
              >
                不出
              </button>
              <button
                data-controller-action="hint"
                disabled={busy || !c.canAct || !c.moves.length}
                onClick={() => {
                  const m = c.moves[hint % c.moves.length];
                  setSelected(m.cards);
                  setKey(m.key);
                  setHint(hint + 1);
                }}
              >
                提示
              </button>
              <button
                className="primary"
                disabled={busy || !c.canAct || !selected.length}
                data-controller-action="play"
                onClick={playSelected}
              >
                出牌（{selected.length}）
              </button>

              {chooseOpen && (
                <Modal
                  title="选择出牌方式"
                  onClose={() => setChooseOpen(false)}
                >
                  <CardStrip ids={selected} skin={style.skin} assets={assets} />
                  <label>
                    牌型
                    <select
                      aria-label="百搭牌型"
                      value={chosen?.key ?? ""}
                      onChange={(e) => setKey(e.target.value)}
                    >
                      {interpretations.map((v) => (
                        <option key={v.key} value={v.key}>
                          {names[v.kind] ?? v.kind} ·{" "}
                          {rankName(v.high === 15 ? d.level : v.high)}
                          {v.wild.length
                            ? " · 配牌代 " +
                              v.wild
                                .map(
                                  (w) =>
                                    ["♣", "♦", "♥", "♠"][w.suit] +
                                    rankName(w.rank),
                                )
                                .join("、")
                            : ""}
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    className="primary"
                    disabled={busy || !chosen}
                    onClick={() => {
                      setChooseOpen(false);
                      void send("play", {
                        cards: selected,
                        interpretation: chosen?.key ?? "",
                      });
                    }}
                  >
                    确认出牌
                  </button>
                </Modal>
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

          {d.roundResults.length > 0 &&
            (d.phase === "round_end" || d.phase === "finished") && (
              <PokerNotice>
                上游：{room.seats[d.roundResults.at(-1)!.head].name}；同伴第{" "}
                {d.roundResults.at(-1)!.partnerPlace} 位，升{" "}
                {d.roundResults.at(-1)!.promotion} 级。
              </PokerNotice>
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
      <p>
        江苏规则，固定对家，红心级牌配牌；头二／头三／头末升 3／2／1 级，必须打
        A，头二或头三过 A。
      </p>
    </PokerOptions>
  );
}
