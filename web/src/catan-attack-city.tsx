import { AttackFishPayment } from "./catan-attack-fish";
import { useEffect, useState } from "react";
import { twoNeutralName } from "./catan-two-state";
import type { Room } from "./types";
import type { AttackSelection } from "./catan-attack-state";
import { emptyAttackSelection } from "./catan-attack-state";
import {
  catanSeatColor,
  catanPieceColors,
  catanColorIndex,
} from "./catan-player-colors";

type Props = {
  room: Room;
  busy: boolean;
  selected: AttackSelection;
  onSelect: (s: AttackSelection) => void;
};
const canAct = (room: Room) =>
  room.status === "playing" &&
  !room.game?.finished &&
  !room.spectating &&
  room.you >= 0 &&
  !room.seats[room.you]?.autoPlay &&
  !room.game?.catan?.players[room.you]?.eliminated;
export function CatanAttackCityMap({
  room,
  busy,
  selected,
  onSelect,
  assets,
}: Props & { assets: string }) {
  const g = room.game!.catan!,
    c = g.attack?.city;
  if (!c) return null;
  const choices = c.choices,
    can = canAct(room) && !busy && !!choices;
  const move = choices?.moves?.find((m) => m.from === selected.from);
  const targets = [
    ...(selected.fish
      ? move?.fish || []
      : [...(move?.move || []), ...(move?.displace || [])]),
    ...(choices?.retreat || []),
    ...(choices?.recruit || []),
    ...(choices?.activate || []),
    ...(choices?.promote || []),
    ...(choices?.remove || []),
  ];
  const pick = (edge: number) => {
    if (choices?.moves?.some((m) => m.from === edge) && !targets.includes(edge))
      onSelect({ ...selected, from: edge, target: null });
    else onSelect({ ...selected, target: edge });
  };
  const button = (edge: number, label: string) => ({
    role: "button",
    tabIndex: 0,
    "aria-label": label,
    onClick: () => pick(edge),
    onKeyDown: (e: React.KeyboardEvent) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        pick(edge);
      }
    },
  });
  return (
    <g className="attack-city-map">
      {can &&
        [...new Set(targets)].map((id) => {
          const e = g.edges[id],
            a = g.vertices[e.a],
            b = g.vertices[e.b];
          return (
            <g
              key={id}
              {...button(id, `选择道路骑士路线 ${id + 1}`)}
              className="attack-city-target"
            >
              <line
                x1={a.x}
                y1={a.y}
                x2={b.x}
                y2={b.y}
                stroke={selected.target === id ? "#fff4a1" : "#f5ca59"}
                strokeWidth="11"
                strokeLinecap="round"
                opacity=".85"
              />
              <title>路线 #{id + 1}</title>
            </g>
          );
        })}
      {c.knights.map((k) => {
        const e = g.edges[k.edge],
          a = g.vertices[e.a],
          b = g.vertices[e.b];
        const selectable =
          can &&
          (targets.includes(k.edge) ||
            choices?.moves?.some((m) => m.from === k.edge));
        return (
          <g
            key={k.edge}
            transform={`translate(${(a.x + b.x) / 2},${(a.y + b.y) / 2})`}
            {...(selectable
              ? button(
                  k.edge,
                  `${k.owner < -1 ? twoNeutralName(k.owner) : room.seats[k.owner]?.name}的 ${k.strength} 级${k.active ? "已激活" : "未激活"}骑士，路线 ${k.edge + 1}`,
                )
              : { pointerEvents: "none" as const })}
            className={selectable ? "attack-city-target" : ""}
          >
            <circle
              r="16"
              fill={k.active ? "#fff2b8" : "#c9c2b5"}
              stroke={catanSeatColor(g, k.owner)}
              strokeWidth={
                selected.from === k.edge || selected.target === k.edge ? 5 : 3
              }
            />
            {assets ? (
              <image
                href={`${assets}/catan/cities-knights/knight-${catanPieceColors[catanColorIndex(g, k.owner)]}-${k.strength}-v1.webp`}
                x="-14"
                y="-23"
                width="28"
                height="33"
                pointerEvents="none"
              />
            ) : (
              <text
                y="5"
                textAnchor="middle"
                fontSize="21"
                fill={catanSeatColor(g, k.owner)}
              >
                ♞
              </text>
            )}
            <circle cx="12" cy="12" r="9" fill={catanSeatColor(g, k.owner)} />
            <text
              x="12"
              y="16"
              textAnchor="middle"
              fill="white"
              fontSize="12"
              fontWeight="800"
            >
              {k.strength}
            </text>
            <title>
              {k.owner < -1
                ? twoNeutralName(k.owner)
                : room.seats[k.owner]?.name}{" "}
              · {k.strength} 级 · {k.active ? "已激活" : "未激活"} · 路线 #
              {k.edge + 1}
            </title>
          </g>
        );
      })}
    </g>
  );
}
export function CatanAttackCityPanel({
  room,
  busy,
  selected,
  onSelect,
  act,
}: Props & { act: (a: Record<string, unknown>) => Promise<unknown> }) {
  const c = room.game!.catan!.attack!.city!,
    q = c.choices,
    plan = c.plan,
    treason = c.treason;
  const [kind, setKind] = useState("recruit");
  useEffect(
    () => onSelect(emptyAttackSelection()),
    [
      room.id,
      room.you,
      room.game?.phase,
      plan?.orders.length,
      plan?.actor,
      treason?.actor,
    ],
  );
  const submit = async (a: Record<string, unknown>) => {
    if ((await act(a)) !== false) onSelect(emptyAttackSelection());
  };
  const can = canAct(room) && !!q && !busy,
    actor = plan?.actor ?? treason?.actor ?? room.game!.turn;
  const legal =
    kind === "recruit"
      ? q?.recruit
      : kind === "activate"
        ? q?.activate
        : q?.promote;
  const move = q?.moves?.find((m) => m.from === selected.from);
  const target = selected.target;
  const selectedMove =
    target != null &&
    (selected.fish
      ? move?.fish?.includes(target) &&
        (q?.fishTokens || [])
          .filter((t) => selected.tokens?.includes(t.id))
          .reduce((sum, t) => sum + t.fish, 0) >= (q?.fishCost || 2)
      : move?.move.includes(target) || move?.displace.includes(target));
  return (
    <div className="attack-city-panel">
      <p>
        <strong>道路骑士</strong> · 每 3 个俘虏计 1 分 ·{" "}
        {room.game!.catan!.victoryTarget ?? 13} 分获胜
      </p>
      {!q && (plan || treason) && (
        <p>
          等待 {room.seats[actor]?.name}{" "}
          {plan?.awaitingRetreat
            ? "选择退让位置"
            : treason
              ? "处理叛变"
              : "安排骑士行动"}
          。
        </p>
      )}
      {q && plan && (
        <>
          <p>
            {plan.awaitingRetreat
              ? "你的骑士被驱逐，请选择最近的空路线。"
              : "激活骑士最多走 5 步，未激活走 3 步；移动后失活。每回合最多驱逐一名较弱对手。"}
          </p>
          {!plan.awaitingRetreat && (
            <div className="attack-picks">
              {q.moves?.map((m) => (
                <button
                  key={m.from}
                  disabled={!can}
                  className={selected.from === m.from ? "selected" : ""}
                  onClick={() =>
                    onSelect({ ...selected, from: m.from, target: null })
                  }
                >
                  路线 #{m.from + 1}
                  {m.required ? " · 必须离开" : ""}
                </button>
              ))}
            </div>
          )}
          {!plan.awaitingRetreat && q.fishTokens && selected.from !== null && (
            <AttackFishPayment
              pick={selected}
              onSelect={onSelect}
              tokens={q.fishTokens}
              cost={q.fishCost || 2}
              available={!!move?.fish?.length}
              disabled={!can}
            />
          )}
          {plan.awaitingRetreat && (
            <div className="attack-picks">
              {q.retreat?.map((edge) => (
                <button
                  key={edge}
                  disabled={!can}
                  className={target === edge ? "selected" : ""}
                  onClick={() => onSelect({ ...selected, target: edge })}
                >
                  退至 #{edge + 1}
                </button>
              ))}
            </div>
          )}
          <p>
            {target != null
              ? `已选路线 #${target + 1}`
              : "在地图上选择亮起的目的地"}
          </p>
          <div className="attack-picks">
            <button
              disabled={
                !can ||
                (plan.awaitingRetreat
                  ? target == null || !q.retreat?.includes(target)
                  : !selectedMove)
              }
              onClick={() =>
                submit({
                  type: "catan_attack_city_move",
                  prompt: plan.id,
                  choice: plan.awaitingRetreat
                    ? "retreat"
                    : selected.fish
                      ? "fish"
                      : move?.displace.includes(target!)
                        ? "displace"
                        : "move",
                  edge: plan.awaitingRetreat ? target : selected.from,
                  target,
                  ...(selected.fish && !plan.awaitingRetreat
                    ? { tokens: selected.tokens || [] }
                    : {}),
                })
              }
            >
              {plan.awaitingRetreat ? "确认退让" : "确认移动／驱逐"}
            </button>
            {!plan.awaitingRetreat && (
              <>
                <button
                  disabled={!can || !q.canUndo}
                  onClick={() =>
                    submit({
                      type: "catan_attack_city_move",
                      prompt: plan.id,
                      choice: "undo",
                    })
                  }
                >
                  撤销上一步
                </button>
                <button
                  className="primary"
                  disabled={!can || !q.canConfirm}
                  onClick={() =>
                    submit({
                      type: "catan_attack_city_move",
                      prompt: plan.id,
                      choice: "confirm",
                    })
                  }
                >
                  结束移动并结算战斗
                </button>
              </>
            )}
          </div>
          <small>
            对手确认退让后，之前的移动不能撤销。本站补充规则：城堡完全堵塞时可暂留，下回合重新检查。
          </small>
        </>
      )}
      {q && treason && (
        <>
          <p>
            {q.remove
              ? "选择移除自己的一名道路骑士。"
              : `在原路线 #${(q.edge ?? 0) + 1} 放置有库存的同级或低级骑士。`}
          </p>
          <div className="attack-picks">
            {q.remove?.map((edge) => (
              <button
                key={edge}
                disabled={!can}
                onClick={() =>
                  submit({
                    type: "catan_attack_city_treason",
                    prompt: treason.id,
                    choice: "remove",
                    edge,
                  })
                }
              >
                移除路线 #{edge + 1}
              </button>
            ))}
            {q.ranks?.map((rank) => (
              <button
                key={rank}
                disabled={!can}
                onClick={() =>
                  submit({
                    type: "catan_attack_city_treason",
                    prompt: treason.id,
                    choice: "place",
                    edge: q.edge,
                    color: rank,
                  })
                }
              >
                放置 {rank} 级骑士
              </button>
            ))}
            {q.ranks && (
              <button
                disabled={!can}
                onClick={() =>
                  submit({
                    type: "catan_attack_city_treason",
                    prompt: treason.id,
                    choice: "skip",
                  })
                }
              >
                不放置
              </button>
            )}
          </div>
        </>
      )}
      {q && !plan && !treason && (
        <>
          <div className="attack-picks">
            {[
              ["recruit", "招募 · 羊毛＋矿石"],
              ["activate", "激活 · 粮食"],
              ["promote", "升级 · 羊毛＋矿石"],
            ].map(([key, label]) => (
              <button
                key={key}
                disabled={busy}
                className={kind === key ? "selected" : ""}
                onClick={() => {
                  setKind(key);
                  onSelect(emptyAttackSelection());
                }}
              >
                {label}
              </button>
            ))}
          </div>
          <div className="attack-picks">
            {legal?.map((edge) => (
              <button
                key={edge}
                disabled={!can}
                className={target === edge ? "selected" : ""}
                onClick={() => onSelect({ ...selected, target: edge })}
              >
                路线 #{edge + 1}
              </button>
            ))}
          </div>
          <p>
            {legal?.length
              ? "选择路线后确认；也可点击地图上的骑士。"
              : "当前没有符合资源、等级和库存条件的路线。"}
          </p>
          <button
            disabled={!can || target == null || !legal?.includes(target)}
            onClick={() =>
              submit({ type: `catan_attack_knight_${kind}`, edge: target })
            }
          >
            确认
            {kind === "recruit"
              ? "招募"
              : kind === "activate"
                ? "激活"
                : "升级"}
          </button>
        </>
      )}
      {!!c.end?.battles.length && (
        <details>
          <summary>上次战斗 · {c.end.battles.length} 块地</summary>
          {c.end.battles.map((b) => (
            <div key={b.tile}>
              <p>
                地块 #{b.tile + 1}：力量 {b.strength.reduce((a, n) => a + n, 0)}
                ，击退 {b.barbarians} 个蛮族
              </p>
              {b.prisoners.map(
                (n, p) =>
                  (n > 0 || b.gold[p] > 0) && (
                    <p key={p}>
                      {room.seats[p]?.name}：俘虏 +{n}，金币 +{b.gold[p]}
                    </p>
                  ),
              )}
              <small>
                {b.lossDie
                  ? `损失骰 ${b.lossDie}；移除 ${b.lost.length}，降级 ${b.downgraded.length}`
                  : "达到胜利条件，不再掷损失骰"}
              </small>
            </div>
          ))}
        </details>
      )}
    </div>
  );
}
