import { useState } from "react";
import type { Room, SanguoshaState, SGPlayer } from "./types";
import { generalArt } from "./sanguosha-art";

const kingdoms: Record<string, string> = {
  wei: "魏",
  shu: "蜀",
  wu: "吴",
  qun: "群",
};
type Send = (extra?: Record<string, unknown>) => Promise<void>;

export const hegemonyInstructions: Record<string, string> = {
  heg_guzheng_obtain: "已返还一张手牌。可以获得下面所有剩余弃牌，也可以放弃。",
  heg_generals:
    "先选主将，再选同势力副将。两将体力相加除以二、向下取整；确认前可交换主副将。",
  heg_reveal_turn:
    "可以亮出一张或两张武将牌，也可以继续隐藏。未明置的玩家不算公开队友。",
  heg_reward: "奖励在这次明置时选择；放弃后不能补领。",
  heg_intel: "这次查看仅你可见，不会明置对方武将。武将情报保留在“我的情报”中。",
  heg_known_both: "选择对方手牌或一张暗置武将私下查看。",
  heg_await_discard: "从自己的手牌或已装备的牌中，选择提示要求的数量弃置。",
  heg_triblade:
    "选一张手牌和一个可选目标。目标须与刚受到杀伤害的角色距离为一。",
  heg_shushen: "选择一名其他已明置的同势力角色，让其摸一张。",
  heg_sijian: "先选一名角色；确认后再选择弃掉其手牌或装备。",
  heg_xiaoguo: "从下方列出的基本手牌中选一张弃置；目标须弃装备或受到一点伤害。",
  heg_xiaoguo_discard: "弃一张装备牌（可以来自手牌或装备区），或承受一点伤害。",
  heg_lirang:
    "只可分配本次仍在弃牌堆的牌。选择牌和一名获得者，可以分多次交出。",
  heg_shuangren: "选择一张手牌和拼点对象。拼点未赢会跳过出牌阶段。",
  heg_shuangren_slash:
    "选择一名可选目标使用虚拟杀，无需出手牌，不计普通杀次数。",
  heg_kuangfu: "选择对方一张装备，将其弃置，或移到自己空着的对应装备栏。",
  heg_duanchang: "选择伤害来源的主将或副将失去技能；暗将身份不会因此公开。",
  heg_invoke: "发动会明置拥有此技能的武将，之后该锁定技按规则生效。",
  heg_mingshi: "可以明置并发动名士，令本次伤害减少一点。",
  heg_suishi: "明置后按随势执行：友方造成濒死则摸牌；友方阵亡则失去体力。",
  heg_shenzhi: "弃掉全部手牌；数量不少于当前体力时回复一点体力。",
  heg_luoshen: "继续判定，或停止并获得本段判定中仍在处理区的黑牌。",
  heg_luoshen_tiandu: "现在拿走这张判定牌，或留到洛神结束统一获得。",
  kuanggu: "明置并发动狂骨，回复本次造成的伤害数对应的体力。",
};

export function HegemonyPortraits({
  player,
  g,
  assets,
}: {
  player: SGPlayer;
  g: SanguoshaState;
  assets: string;
}) {
  return (
    <span className="sg-dual-portraits">
      {[player.general, player.deputy].map((id, slot) => {
        const general = g.generals.find((x) => x.id === id);
        return (
          <span
            className={`sg-general-slot ${player.shown?.[slot] ? "shown" : "hidden"}`}
            key={slot}
          >
            {assets && (
              <img
                src={generalArt(assets, id || "heg_anjiang")}
                alt=""
                draggable={false}
              />
            )}
            <span className="sg-slot-state">
              {slot === 0 ? "主将" : "副将"} ·{" "}
              {player.lost?.[slot]
                ? "技能失去"
                : player.shown?.[slot]
                  ? "明置"
                  : "暗置"}
            </span>
            <span className="sg-slot-name">
              {general?.name || (g.selecting ? "待选" : "暗将")}
            </span>
          </span>
        );
      })}
    </span>
  );
}

export function HegemonyDraft({
  g,
  player,
  assets,
  busy,
  send,
}: {
  g: SanguoshaState;
  player: SGPlayer;
  assets: string;
  busy: boolean;
  send: Send;
}) {
  const [pair, setPair] = useState<string[]>([]);
  const first = g.generals.find((x) => x.id === pair[0]);
  const second = g.generals.find((x) => x.id === pair[1]);
  const companion = g.companions?.some((p) =>
    p.every((id) => pair.includes(id)),
  );
  const choose = (id: string) =>
    setPair((old) =>
      old[0] === id
        ? []
        : old[1] === id
          ? old.slice(0, 1)
          : old.length
            ? [old[0], id]
            : [id],
    );
  return (
    <section className="sg-heg-draft">
      <h3>
        选择你的双将 <small>候选仅你可见</small>
      </h3>
      <p>先选主将，再选同势力副将；主将明置后以主将性别为准。</p>
      <div className="sg-heg-factions">
        {Object.entries(kingdoms).map(([kingdom, label]) => {
          const candidates = g.generals.filter(
            (x) => x.kingdom === kingdom && player.choices?.includes(x.id),
          );
          if (!candidates.length) return null;
          return (
            <section
              className={`sg-draft-faction kingdom-${kingdom}`}
              key={kingdom}
              style={
                { "--sg-draft-cols": candidates.length } as React.CSSProperties
              }
            >
              <h4>
                {label}{" "}
                <small>
                  {candidates.length < 2
                    ? "不足两名，无法组成双将"
                    : "同势力可组合"}
                </small>
              </h4>
              <div className="sg-heg-candidates">
                {candidates.map((general) => (
                  <article
                    className={pair.includes(general.id) ? "selected" : ""}
                    key={general.id}
                  >
                    <button
                      className="sg-heg-candidate"
                      disabled={
                        busy ||
                        candidates.length < 2 ||
                        (!!first && first.kingdom !== kingdom)
                      }
                      onClick={() => choose(general.id)}
                      aria-pressed={pair.includes(general.id)}
                    >
                      {assets && (
                        <img
                          src={generalArt(assets, general.id)}
                          alt=""
                          draggable={false}
                        />
                      )}
                      <strong>{general.name}</strong>
                      <small>
                        体力 {general.hp} · {general.female ? "女" : "男"}
                      </small>
                      {pair.includes(general.id) && (
                        <b className="sg-draft-slot">
                          {pair.indexOf(general.id) === 0 ? "主将" : "副将"}
                        </b>
                      )}
                    </button>
                    <details>
                      <summary>
                        {general.skills
                          .map((k) => g.skills[k].name)
                          .join(" · ")}
                      </summary>
                      {general.skills.map((k) => (
                        <p key={k}>
                          <b>{g.skills[k].name}</b>：{g.skills[k].text}
                        </p>
                      ))}
                    </details>
                  </article>
                ))}
              </div>
            </section>
          );
        })}
      </div>
      <div className="sg-draft-confirm">
        <div>
          <strong>
            {first ? `主将 ${first.name}` : "请选择主将"}
            {second && ` ＋ 副将 ${second.name}`}
          </strong>
          {first && second && (
            <p>
              体力上限 {Math.floor((first.hp + second.hp) / 2)}
              {companion ? " · 珠联璧合" : ""}
              {(first.hp + second.hp) % 2 ? " · 半阴阳鱼奖励" : ""}
            </p>
          )}
        </div>
        <div className="sg-action-buttons">
          <button
            className="primary"
            disabled={busy || pair.length !== 2}
            onClick={() =>
              void send({
                choice: pair.join("+"),
                cards: [],
                targets: [],
                skill: "",
              })
            }
          >
            确认双将
          </button>
          <button
            disabled={busy || pair.length !== 2}
            onClick={() => setPair([pair[1], pair[0]])}
          >
            交换主副将
          </button>
          <button
            className="subtle"
            disabled={busy || !pair.length}
            onClick={() => setPair([])}
          >
            重新选择
          </button>
        </div>
      </div>
    </section>
  );
}

function GeneralIntel({
  id,
  g,
  assets,
}: {
  id: string;
  g: SanguoshaState;
  assets: string;
}) {
  const general = g.generals.find((x) => x.id === id);
  if (!general) return null;
  return (
    <div className={`sg-general-intel kingdom-${general.kingdom}`}>
      {assets && <img src={generalArt(assets, id)} alt={general.name} />}
      <div>
        <strong>
          {general.name} · {kingdoms[general.kingdom]}
        </strong>
        {general.skills.map((k) => (
          <p key={k}>
            <b>{g.skills[k].name}</b>：{g.skills[k].text}
          </p>
        ))}
      </div>
    </div>
  );
}

export function HegemonyIntel({
  player,
  g,
  room,
  assets,
}: {
  player: SGPlayer;
  g: SanguoshaState;
  room: Room;
  assets: string;
}) {
  return (
    <details className="sg-private-stars sg-heg-intel">
      <summary>我的双将与情报 · 仅自己可见</summary>
      <div className="sg-intel-grid">
        {[player.general, player.deputy].map(
          (id, slot) =>
            id && (
              <section key={slot}>
                <h4>
                  {slot === 0 ? "主将" : "副将"} ·{" "}
                  {player.shown?.[slot] ? "明置" : "暗置"}
                  {player.lost?.[slot] ? " · 技能已失去" : ""}
                </h4>
                <GeneralIntel id={id} g={g} assets={assets} />
              </section>
            ),
        )}
      </div>
      <p>
        {player.companion
          ? `珠联璧合 · ${player.companionClaimed ? "已处理奖励" : "双将明置时可摸二或回复一"}`
          : "本组合无珠联璧合"}
        {player.halfHP
          ? `；半阴阳鱼 · ${player.halfClaimed ? "已处理奖励" : "双将明置时可摸一"}`
          : ""}
      </p>
      {Object.entries(player.knownGenerals || {}).map(([seat, ids]) => (
        <section key={seat}>
          <h4>知己知彼 · {room.seats[Number(seat)]?.name}</h4>
          <div className="sg-intel-grid">
            {ids.map(
              (id, slot) =>
                id && (
                  <div key={slot}>
                    <h4>已知{slot === 0 ? "主将" : "副将"}</h4>
                    <GeneralIntel id={id} g={g} assets={assets} />
                  </div>
                ),
            )}
          </div>
        </section>
      ))}
    </details>
  );
}

export function HegemonyResponseActions({
  g,
  player,
  assets,
  cards,
  targets,
  busy,
  send,
}: {
  g: SanguoshaState;
  player: SGPlayer;
  assets: string;
  cards: number[];
  targets: number[];
  busy: boolean;
  send: Send;
}) {
  const prompt = g.pending!;
  const ask = prompt.kind;
  const allIn = (available: number[] | undefined) =>
    cards.every((id) => available?.includes(id));
  const own = [...(player.hand || []), ...player.equip];
  const oneTarget =
    targets.length === 1 && !!prompt.targets?.includes(targets[0]);
  const choices: { value: string; label: string; disabled?: boolean }[] = [];
  let selected = false,
    valid = false,
    label = "确认响应";
  switch (ask) {
    case "heg_reveal_turn":
      choices.push(
        ...(prompt.choices || []).map((value) => ({
          value,
          label:
            {
              head: "明置主将",
              deputy: "明置副将",
              both: "明置双将",
              pass: "继续暗置",
            }[value] || value,
        })),
      );
      break;
    case "heg_reward":
      choices.push(
        ...(prompt.choices || []).map((value) => ({
          value,
          label:
            value === "draw"
              ? `摸${prompt.effect === "half" ? "一" : "两"}张牌`
              : value === "heal"
                ? "回复一点体力"
                : "放弃奖励",
        })),
      );
      break;
    case "heg_known_both":
      choices.push(
        ...(prompt.choices || []).map((value) => ({
          value,
          label: `查看${{ hand: "手牌", head: "主将", deputy: "副将" }[value] || value}`,
        })),
      );
      break;
    case "heg_intel":
      choices.push({ value: "ok", label: "已查看，继续" });
      break;
    case "heg_guzheng_obtain":
      choices.push({ value: "yes", label: "获得其余弃牌" });
      break;
    case "heg_duanchang":
      choices.push(
        { value: "head", label: "令主将失去技能" },
        { value: "deputy", label: "令副将失去技能" },
      );
      break;
    case "heg_invoke":
    case "heg_mingshi":
    case "heg_suishi":
    case "heg_shenzhi":
    case "heg_luoshen":
    case "heg_luoshen_tiandu":
      choices.push({
        value: "yes",
        label:
          ask === "heg_shenzhi"
            ? "发动 · 弃全部手牌"
            : ask === "heg_luoshen"
              ? "继续判定"
              : ask === "heg_luoshen_tiandu"
                ? "发动天妒 · 立即获得"
                : "明置并发动",
      });
      break;
    case "heg_kuangfu": {
      const card = g.cards.find((c) => c.id === cards[0]);
      const slot = card && g.cardTypes[card.kind].slot;
      valid = cards.length === 1 && allIn(prompt.cards);
      choices.push(
        { value: "discard", label: "弃置所选装备", disabled: !valid },
        {
          value: "move",
          label: "移入我的装备区",
          disabled:
            !valid ||
            player.equip.some(
              (id) =>
                g.cardTypes[g.cards.find((c) => c.id === id)!.kind].slot ===
                slot,
            ),
        },
      );
      break;
    }
    case "heg_await_discard":
      selected = true;
      valid = cards.length === prompt.amount && allIn(own);
      label = `弃置所选 ${prompt.amount} 张牌`;
      break;
    case "heg_lirang":
      selected = true;
      valid = cards.length > 0 && allIn(prompt.cards) && oneTarget;
      label = "交给所选角色";
      break;
    case "heg_xiaoguo":
      selected = true;
      valid = cards.length === 1 && allIn(prompt.cards);
      break;
    case "heg_xiaoguo_discard":
      selected = true;
      valid =
        cards.length === 1 &&
        allIn(own) &&
        !!g.cardTypes[g.cards.find((c) => c.id === cards[0])!.kind].slot;
      label = "弃置装备";
      break;
    case "heg_triblade":
    case "heg_shuangren":
      selected = true;
      valid = cards.length === 1 && allIn(player.hand) && oneTarget;
      break;
    case "heg_shushen":
    case "heg_sijian":
    case "heg_shuangren_slash":
      selected = true;
      valid = oneTarget;
      break;
  }
  const optional = [
    "heg_guzheng_obtain",
    "heg_invoke",
    "heg_mingshi",
    "heg_suishi",
    "heg_shenzhi",
    "heg_luoshen",
    "heg_luoshen_tiandu",
    "heg_kuangfu",
    "heg_lirang",
    "heg_xiaoguo",
    "heg_xiaoguo_discard",
    "heg_triblade",
    "heg_shuangren",
    "heg_shushen",
    "heg_sijian",
  ].includes(ask);
  if (optional)
    choices.push({
      value: "pass",
      label:
        ask === "heg_lirang"
          ? "结束分配"
          : ask === "heg_luoshen"
            ? "停止并获得黑牌"
            : ask === "heg_xiaoguo_discard"
              ? "放弃 · 受到一点伤害"
              : "放弃",
    });
  return (
    <>
      {ask === "heg_intel" && prompt.general && (
        <GeneralIntel id={prompt.general} g={g} assets={assets} />
      )}
      <div className="sg-action-buttons">
        {selected && (
          <button
            className="primary"
            disabled={busy || !valid}
            onClick={() => void send({ skill: "" })}
          >
            {label}
          </button>
        )}
        {choices.map((choice) => (
          <button
            key={choice.value}
            className={choice.value === "pass" ? "outline" : "primary"}
            disabled={busy || choice.disabled}
            onClick={() =>
              void send({
                choice: choice.value,
                skill: "",
                cards:
                  ask === "heg_kuangfu" && choice.value !== "pass" ? cards : [],
                targets: [],
              })
            }
          >
            {choice.label}
          </button>
        ))}
      </div>
    </>
  );
}

export function HegemonyRules() {
  return (
    <>
      <p>
        基础国战，4–8 人，60 名专用武将、108
        张国战牌。双将与身份局使用独立规则。
      </p>
      <ol>
        <li>
          每人从七名私有候选中，依次选择同势力主将与副将。两将体力相加除以二、向下取整；起手四张。随机先手，无主公加血。
        </li>
        <li>
          武将起初暗置。准备阶段可选择明置，也可以在合法发动技能时明置对应武将；公开势力相同才算队友。祸水拥有者的回合内，其他角色不能新明置。
        </li>
        <li>
          首亮可摸二；珠联璧合双亮可摸二或回复一；总阴阳鱼为奇数，双亮可摸一。奖励在触发时选择，放弃后不补领。
        </li>
        <li>
          同势力已明置人数（含阵亡者）超过开局人数一半，后亮者成为独立的野心家。野心家之间也不属于同一阵营。
        </li>
        <li>
          消灭其他所有势力获胜，阵亡队友共享胜利。未明置杀手无击杀奖励；杀敌摸死者加其存活公开队友人数的牌；杀同势力弃全部手牌和装备。
        </li>
        <li>
          知己知彼仅自己查看；国无懈可抵消当前目标或该势力的后续目标；以逸待劳让公开队友各摸二再弃二；远交近攻要求双方已明置且不同势力。
        </li>
        <li>
          主动操作120秒，回合外响应20秒。超时放弃可选响应、自动完成必选操作；托管按电脑策略行动。
        </li>
        <li>
          纯真人完整对局：获胜势力成员各加20分，其余各减10分；含电脑或中止不计分。
        </li>
      </ol>
    </>
  );
}
