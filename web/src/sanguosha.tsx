import { useEffect, useState } from "react";
import { Bot } from "lucide-react";
import type { Act, Room, SanguoshaState, SGOptions } from "./types";
import { PlayerName } from "./profiles";
import "./sanguosha.css";
const roles: Record<string, string> = {
  lord: "主公",
  loyalist: "忠臣",
  rebel: "反贼",
  renegade: "内奸",
};
const kingdoms: Record<string, string> = {
  wei: "魏",
  shu: "蜀",
  wu: "吴",
  qun: "群",
};
const militaryArt = new Set([
  "fire_slash",
  "thunder_slash",
  "analeptic",
  "fire_attack",
  "iron_chain",
  "supply_shortage",
  "fan",
  "guding_blade",
  "vine",
  "silver_lion",
  "hualiu",
]);
const windGenerals = new Set([
  "caoren",
  "xiahouyuan",
  "huangzhong",
  "weiyan",
  "xiaoqiao",
  "zhoutai",
  "zhangjiao",
  "yuji",
]);
const fireGenerals = new Set([
  "dianwei",
  "xunyu",
  "wolong",
  "pangtong",
  "taishici",
  "yuanshao",
  "yanliangwenchou",
  "pangde",
]);
const thicketGenerals = new Set([
  "caopi",
  "xuhuang",
  "menghuo",
  "zhurong",
  "sunjian",
  "lusu",
  "dongzhuo",
  "jiaxu",
]);
const mountainGenerals = new Set([
  "zhanghe",
  "dengai",
  "jiangwei",
  "liushan",
  "sunce",
  "erzhang",
  "caiwenji",
  "zuoci",
]);
const generalArt = (assets: string, id: string) =>
  `${assets}/sanguosha/${mountainGenerals.has(id) ? "v6" : thicketGenerals.has(id) ? "v5" : fireGenerals.has(id) ? "v4" : windGenerals.has(id) ? "v3" : "v1"}/generals/${id}.webp`;
const suits = ["♠", "♥", "♣", "♦"];
const transforms = [
  "duanliang",
  "jiuchi",
  "huoji",
  "kanpo",
  "lianhuan",
  "luanji",
  "shuangxiong",
  "guhuo",
  "wusheng",
  "qingguo",
  "longdan",
  "qixi",
  "guose",
  "jijiu",
];
const activeSkills = [
  "jixi",
  "tiaoxin",
  "zhijian",
  "zhiba_pindian",
  "dimeng",
  "luanwu",
  "qiangxi",
  "quhu",
  "tianyi",
  "rende",
  "zhiheng",
  "kurou",
  "fanjian",
  "jieyin",
  "qingnang",
  "lijian",
  "jijiang",
  "huangtian_give",
];
function Card({
  id,
  g,
  assets,
  selected,
  onClick,
  small = false,
}: {
  id: number;
  g: SanguoshaState;
  assets: string;
  selected?: boolean;
  onClick?: () => void;
  small?: boolean;
}) {
  const c = g.cards.find((c) => c.id === id)!;
  const info = g.cardTypes[c.kind];
  const [failed, setFailed] = useState(false);
  return (
    <button
      type="button"
      className={`sg-card ${selected ? "selected" : ""} ${small ? "small" : ""} ${c.suit === 1 || c.suit === 3 ? "red-suit" : ""}`}
      onClick={onClick}
      aria-pressed={onClick ? !!selected : undefined}
      title={`${suits[c.suit]}${c.rank} ${info.name}：${info.text}`}
    >
      <span className="sg-card-heading">
        <b>
          {suits[c.suit]}
          {
            [
              "",
              "A",
              "2",
              "3",
              "4",
              "5",
              "6",
              "7",
              "8",
              "9",
              "10",
              "J",
              "Q",
              "K",
            ][c.rank]
          }
        </b>
        <span>{info.name}</span>
      </span>
      {assets && !failed ? (
        <img
          src={`${assets}/sanguosha/${militaryArt.has(c.kind) ? "v2" : "v1"}/cards/${c.kind}.webp`}
          alt={info.name}
          onError={() => setFailed(true)}
          draggable={false}
        />
      ) : (
        <span className="sg-card-fallback">
          {info.name}
          <small>{info.text}</small>
        </span>
      )}
      {selected && <span className="sg-selected-check">✓</span>}
    </button>
  );
}
export function SanguoshaBoard({
  room,
  act,
  busy,
  assets,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
}) {
  const game = room.game!,
    g = game.sanguosha!,
    you = room.you,
    me = g.players[you],
    prompt = g.pending;
  const responding = !!prompt && prompt.canRespond && !game.finished;
  const playing =
    !prompt &&
    game.turn === you &&
    !game.finished &&
    room.status === "playing" &&
    !me?.dead;
  const enabled = (responding || playing) && !busy && !room.spectating;
  const [cards, setCards] = useState<number[]>([]),
    [targets, setTargets] = useState<number[]>([]),
    [skill, setSkill] = useState(""),
    [declaration, setDeclaration] = useState("slash"),
    [help, setHelp] = useState(""),
    [error, setError] = useState("");
  const [detail, setDetail] = useState<number>();
  const mineGeneral = g.generals.find((x) => x.id === me?.general);
  const skills = [
    ...(me?.skills || mineGeneral?.skills || []),
    ...(g.zhibaPindian ? ["zhiba_pindian"] : []),
    ...(g.huangtianGive ? ["huangtian_give"] : []),
  ].filter(
    (x) =>
      ![
        "hujia",
        "jijiang",
        "jiuyuan",
        "huangtian",
        "xueyi",
        "songwei",
        "baonue",
        "ruoyu",
        "zhiba",
      ].includes(x) || me?.role === "lord",
  );
  useEffect(() => {
    setCards([]);
    setTargets([]);
    setSkill("");
    setError("");
  }, [game.turn, game.round, prompt?.id, responding]);
  const selectCard = (id: number) => {
    if (!enabled) {
      setDetail(id);
      return;
    }
    setCards((old) =>
      old.includes(id) ? old.filter((x) => x !== id) : [...old, id],
    );
  };
  const selectTarget = (i: number) => {
    if (!enabled || g.players[i].dead) return;
    setTargets((old) =>
      old.includes(i) ? old.filter((x) => x !== i) : [...old, i],
    );
  };
  const send = async (extra: Record<string, unknown> = {}) => {
    setError("");
    try {
      await act({
        type: playing
          ? activeSkills.includes(skill)
            ? "sg_skill"
            : "sg_play"
          : "sg_respond",
        cards,
        targets,
        skill,
        choice: skill === "guhuo" ? declaration : undefined,
        prompt: prompt?.id,
        card: cards[0] || 0,
        ...extra,
      });
      setCards([]);
      setTargets([]);
      setSkill("");
    } catch (e) {
      setError((e as Error).message);
    }
  };
  const ask = prompt?.kind;
  const optional =
    (ask === "huashen" && !prompt?.required) ||
    [
      "qiaobian",
      "qiaobian_draw",
      "qiaobian_move",
      "fangquan",
      "fangquan_give",
      "xiangle",
      "tiaoxin",
      "guzheng",
      "beige",
      "zhiba_accept",
      "zhiba_obtain",
      "xingshang",
      "fangzhu",
      "songwei",
      "baonue",
      "lieren",
      "yinghun",
      "luanwu",
      "niepan",
      "mengjin",
      "jieming",
      "invoke",
      "keji",
      "nullification",
      "peach",
      "card",
      "support",
      "liuli",
      "double_sword",
      "sword_discard",
      "tieji",
      "weapon_after_jink",
      "ice_sword",
      "kylin_bow",
      "guicai",
      "tiandu",
      "ganglie",
      "fire_discard",
      "shensu_judge",
      "shensu_play",
      "liegong",
      "tianxiang",
      "leiji",
      "guidao",
      "buqu",
      "guhuo_question",
    ].includes(ask || "");
  const simple = [
    "fangquan",
    "zhiba_accept",
    "zhiba_obtain",
    "xingshang",
    "songwei",
    "baonue",
    "niepan",
    "mengjin",
    "invoke",
    "keji",
    "double_sword",
    "tieji",
    "ice_sword",
    "tiandu",
    "liegong",
    "buqu",
  ].includes(ask || "");
  const extraCards =
    responding &&
    ["guanxing", "yiji", "grace", "buqu_remove", "guzheng"].includes(ask || "")
      ? prompt?.cards || []
      : [];
  const instructions: Record<string, string> = {
    qiaobian: "选择一张手牌弃置，跳过提示中的阶段；也可以放弃。",
    qiaobian_draw: "选择一至两名有手牌的其他角色，再确认获得各一张随机手牌。",
    qiaobian_move: "展开要移动的牌，再选择新的装备或判定区。只显示合法位置。",
    fangquan: "跳过出牌阶段，正常弃牌后可以弃一张手牌，交出额外回合。",
    fangquan_give: "选择一张手牌和另一名角色。其额外回合结束后恢复正常座次。",
    xiangle: "必须额外弃一张基本手牌；放弃后，这张杀对享乐角色无效。",
    tiaoxin:
      "选择一张杀或可转化为杀的牌；目标固定为挑衅者。放弃会让其弃你的牌。",
    guzheng: "在展示的弃牌中选择一张返还，自己获得该弃牌阶段的其他弃置牌。",
    beige: "弃一张手牌或装备，让受到杀伤害的角色判定。",
    beige_discard: `选择 ${prompt?.amount} 张手牌或装备弃置。`,
    huashen:
      "备选只有你能看到。点击技能完成选择；当前化身、技能、势力和性别会公开。",
    zhiji: "减体力上限后，选择回复1点体力或摸两张牌；本回合即可发动观星。",

    fangzhu: "选择另一名角色，摸你已损失体力数的牌并翻面。背面角色会翻回正面。",
    lieren: "选择一张手牌拼点，胜出后可获得伤害目标的一张手牌或装备。",
    yinghun: "先选择另一名角色，再选择摸牌与弃牌方式。弃牌可以包含装备。",
    yinghun_discard: `选择 ${prompt?.amount} 张手牌或装备弃置。`,
    haoshi_give: `选择 ${prompt?.amount} 张手牌，交给可选择的角色之一。`,
    luanwu: "选择杀与一名距离最近的角色；需符合杀的范围。放弃则失去1点体力。",
    benghuai: "有其他角色体力低于你，须失去1点体力或1点体力上限。",

    pindian: "选择一张手牌拼点；双方选好后才会一起揭示。同点时发起者未胜出。",
    quhu_target: "选择对方攻击范围内的一名角色，由对方对其造成1点伤害。",
    jieming: "选择一名角色，将其手牌补至体力上限（最多5张）。",
    niepan:
      "整局限一次。弃置手牌、装备与判定牌，回复至3点体力，摸三张并解除横置和翻面。",
    shensu_judge: "选择一名目标，跳过判定和摸牌，视为使用无距离限制的杀。",
    shensu_play:
      "选择一张手牌或装备区的装备牌，再选择一名目标；跳过出牌阶段并使用无距离限制的杀。",
    tianxiang:
      "选择一张红桃手牌及另一名角色，转移本次伤害。红颜会将黑桃视为红桃。",
    leiji: "选择一名角色判定；黑桃将造成2点雷电伤害。",
    guidao: "选择一张黑色手牌或装备牌，替换并获得原判定牌。",
    buqu_remove: `选择 ${prompt?.amount} 张不屈牌移去，避免留下重复点数。`,
    guhuo_question:
      "牌面尚未公开。质疑真牌会失去1点体力；质疑假牌摸一张牌。有人质疑时，仅真实的红桃牌生效。",
    card: `需要 ${prompt?.count || 1} 张${prompt?.wanted === "jink" ? "闪" : "杀"}，每次响应一张。`,
    nullification: prompt?.cancelled
      ? "当前效果已被抵消；再次无懈会恢复效果。"
      : "无懈可抵消此次锦囊对这一名角色的效果。",
    peach:
      prompt?.target === you
        ? "每次使用一张桃或酒自救，体力恢复到 1 才脱离濒死。"
        : "每次使用一张桃，体力恢复到 1 才脱离濒死。酒只能自救。",
    fire_reveal: "请选择一张手牌公开展示；这张牌仍留在你手中。",
    fire_discard:
      "弃置一张与展示牌花色相同的手牌，造成 1 点火焰伤害；也可放弃。",
    discard: `请选择 ${prompt?.amount} 张手牌。`,
    liuli: "选一张自己的牌，再选择另一名目标；距离按弃牌后计算。",
    yiji: "选要分配的牌，再选一位获得者，可分多次。",
    guanxing: "依次点击想放在牌堆顶的牌；未选的牌按原顺序放至底部。",
    steal: "可获得/弃置一张随机手牌，或点击目标的装备与判定牌。",
    weapon_after_jink:
      "青龙偃月刀：再出一张杀；贯石斧：弃两张牌，不可弃斧本身。",
    guicai: "选择一张自己的手牌替换判定牌。",
    ganglie: "选择两张手牌弃置，或放弃并受到 1 点伤害。",
  };
  const getCard = (id: number) => g.cards.find((c) => c.id === id)!;
  const actionHint = skill
    ? g.skills[skill]?.text ||
      (skill === "fan" ? "选择一张普通杀，当火杀使用。" : "选择两张手牌当杀。")
    : cards.length === 1
      ? g.cardTypes[getCard(cards[0]).kind].text
      : "选择手牌、技能和目标，再确认出牌。";
  return (
    <section className="sg-board">
      <header className="sg-heading">
        <div>
          <span className="eyebrow">
            三国杀 · 经典身份局{g.options?.deck === "military" ? " · 军争" : ""}
          </span>
          <h2>{g.selecting ? "群雄集结" : "一桌风云，各有所谋"}</h2>
        </div>
        <div className="sg-piles">
          <span>
            牌堆 <b>{g.remaining}</b>
          </span>
          <span>
            弃牌 <b>{g.discardCount}</b>
          </span>
        </div>
      </header>
      {g.bluff && !g.bluff.resolved && (
        <aside className="sg-help">
          <strong>
            {room.seats[g.bluff.player]?.name} 发动蛊惑：
            {g.cardTypes[g.bluff.declared]?.name}
          </strong>
          <p>{g.bluff.context}</p>
          <p>
            牌面扣置，正在询问质疑。
            {g.bluff.questioned?.length
              ? `已有 ${g.bluff.questioned.length} 人质疑。`
              : ""}
          </p>
        </aside>
      )}
      <div
        className="sg-seats"
        style={{ "--sg-seat-count": room.seats.length } as React.CSSProperties}
      >
        {room.seats.map((seat, i) => {
          const p = g.players[i],
            general = g.generals.find((x) => x.id === p.general),
            effectiveSkills = p.skills || general?.skills || [],
            kingdom = p.kingdom || general?.kingdom || "unknown";
          return (
            <article
              key={seat.id}
              data-player-seat={i}
              className={`sg-seat kingdom-${kingdom} ${i === game.turn ? "turn" : ""} ${p.dead ? "dead" : ""} ${targets.includes(i) ? "targeted" : ""} ${i === you ? "self" : ""}`}
            >
              <button
                className="sg-seat-target"
                disabled={
                  !enabled ||
                  p.dead ||
                  ([
                    "quhu_target",
                    "haoshi_give",
                    "luanwu",
                    "qiaobian_draw",
                  ].includes(ask || "") &&
                    !prompt?.targets?.includes(i))
                }
                onClick={() => selectTarget(i)}
                aria-label={`选择${seat.name}为目标`}
                aria-pressed={targets.includes(i)}
              >
                {general && assets && (
                  <img
                    src={generalArt(assets, general.id)}
                    alt=""
                    draggable={false}
                  />
                )}
                <span className="sg-general-name">
                  {general?.name || "待选武将"}{" "}
                  <small>{general ? kingdoms[kingdom] : ""}</small>
                </span>
                <span className="sg-target-number">
                  {targets.includes(i)
                    ? `目标 ${targets.indexOf(i) + 1}`
                    : i === you
                      ? "你"
                      : `${i + 1} 号位`}
                </span>
              </button>
              <div className="sg-seat-info">
                <strong>
                  <PlayerName user={seat} />
                  {seat.bot && <small>电脑</small>}
                  {seat.autoPlay && (
                    <span className="autoplay-badge" title="由电脑代为行动">
                      <Bot size={12} aria-hidden="true" />
                      托管
                    </span>
                  )}
                </strong>
                <span className={`sg-role role-${p.role || "hidden"}`}>
                  {p.role ? roles[p.role] : "身份未知"}
                  {p.dead ? " · 阵亡" : ""}
                </span>
                <div
                  className="sg-health"
                  aria-label={`体力 ${p.hp}/${p.maxHP}`}
                >
                  <span>
                    {"♥".repeat(Math.max(0, p.hp))}
                    {"♡".repeat(Math.max(0, p.maxHP - Math.max(0, p.hp)))}
                  </span>
                  <b>
                    {p.hp}/{p.maxHP}
                  </b>
                </div>
                <small>
                  手牌 {p.handCount}
                  {p.handLimit !== undefined && ` / 上限 ${p.handLimit}`}
                  {i !== you && g.distances ? ` · 距离 ${g.distances[i]}` : ""}
                </small>
              </div>
              <div className="sg-status-marks">
                {p.skillsLost && (
                  <span className="sg-flipped">断肠 · 技能已失去</span>
                )}
                {["zaoxian", "zhiji", "hunzi", "ruoyu"]
                  .filter((k) => p.marks?.[k])
                  .map((k) => (
                    <span className="sg-limited" key={k}>
                      {g.skills[k].name} · 已觉醒
                    </span>
                  ))}
                {!!p.avatarCount && (
                  <span className="sg-avatar-mark">
                    化身 {p.avatarCount} 张
                    {p.avatar
                      ? ` · ${g.generals.find((x) => x.id === p.avatar)?.name || p.avatar} · ${g.skills[p.avatarSkill || ""]?.name || "无技能"}`
                      : ""}
                  </span>
                )}

                {p.flipped && (
                  <span className="sg-flipped">背面 · 下回合跳过</span>
                )}
                {effectiveSkills.includes("luanwu") && (
                  <span className="sg-limited">
                    乱武 · {p.marks?.luanwu ? "已使用" : "可用"}
                  </span>
                )}
                {effectiveSkills.includes("niepan") && (
                  <span className="sg-limited">
                    涅槃 · {p.marks?.niepan ? "已使用" : "可用"}
                  </span>
                )}
                {i === game.turn && !!p.used.tianyi_result && (
                  <span className="sg-limited">
                    天义 ·{" "}
                    {p.used.tianyi_result > 0
                      ? "杀 +1／目标 +1／不限距离"
                      : "不能使用杀"}
                  </span>
                )}
                {i === game.turn && !!p.used.shuangxiong && (
                  <span className="sg-limited">
                    双雄 · {p.used.shuangxiong === 1 ? "黑牌" : "红牌"}可作决斗
                  </span>
                )}
                {p.chained && <span className="sg-chained">连环 · 已横置</span>}
                {!!p.drank && (
                  <span className="sg-drank">酒 · 下一张杀伤害 +{p.drank}</span>
                )}
              </div>
              {!!p.buqu?.length && (
                <div className="sg-buqu-pile">
                  <span>不屈</span>
                  {p.buqu.map((id) => (
                    <button
                      key={id}
                      title={g.cardTypes[getCard(id).kind].name}
                      onClick={() => setDetail(id)}
                    >
                      {getCard(id).rank}
                    </button>
                  ))}
                </div>
              )}
              {!!p.fields?.length && (
                <div className="sg-field-pile">
                  <span>田 · {p.fields.length}</span>
                  {p.fields.map((id) => (
                    <button
                      key={id}
                      className={cards.includes(id) ? "selected" : ""}
                      title={`${suits[getCard(id).suit]}${getCard(id).rank} ${g.cardTypes[getCard(id).kind].name}${i === you && skills.includes("jixi") ? " · 点击发动急袭" : ""}`}
                      onClick={() => {
                        if (
                          i === you &&
                          playing &&
                          enabled &&
                          skills.includes("jixi")
                        ) {
                          setSkill("jixi");
                          setCards([id]);
                        } else setDetail(id);
                      }}
                    >
                      {suits[getCard(id).suit]}
                      {getCard(id).rank}
                    </button>
                  ))}
                </div>
              )}
              <div className="sg-seat-skills">
                {effectiveSkills.map((k) => (
                  <button
                    key={k}
                    className="sg-skill-tag"
                    onClick={() => setHelp(k)}
                  >
                    {g.skills[k].name}
                  </button>
                ))}
              </div>
              <div className="sg-equipment">
                {p.equip.map((id) => (
                  <button
                    key={id}
                    disabled={busy}
                    className={cards.includes(id) ? "selected" : ""}
                    title={g.cardTypes[getCard(id).kind].text}
                    onClick={() => {
                      if (
                        responding &&
                        ask === "steal" &&
                        prompt?.target === i
                      ) {
                        void send({ card: id, choice: "equipment" });
                      } else if (
                        responding &&
                        ask === "kylin_bow" &&
                        prompt?.target === i
                      ) {
                        void send({ cards: [id] });
                      } else if (i === you) {
                        selectCard(id);
                      } else {
                        setDetail(id);
                      }
                    }}
                  >
                    {g.cardTypes[getCard(id).kind].name}
                  </button>
                ))}
              </div>
              <div className="sg-delays">
                {p.judgment.map((d) => (
                  <button
                    key={d.card}
                    onClick={() =>
                      responding && ask === "steal" && prompt?.target === i
                        ? void send({ card: d.card, choice: "judgment" })
                        : setDetail(d.card)
                    }
                  >
                    {g.cardTypes[d.kind].name}
                  </button>
                ))}
              </div>
            </article>
          );
        })}
      </div>
      {help && (
        <aside className="sg-help">
          <strong>{g.skills[help].name}</strong>
          <p>{g.skills[help].text}</p>
          <button className="subtle" onClick={() => setHelp("")}>
            收起说明
          </button>
        </aside>
      )}
      {detail !== undefined && (
        <aside className="sg-help">
          <strong>{g.cardTypes[getCard(detail).kind].name}</strong>
          <p>{g.cardTypes[getCard(detail).kind].text}</p>
          <button className="subtle" onClick={() => setDetail(undefined)}>
            收起说明
          </button>
        </aside>
      )}
      {g.selecting && responding && (
        <div className="sg-general-options">
          {me?.choices?.map((id) => {
            const general = g.generals.find((x) => x.id === id)!;
            return (
              <button
                disabled={busy}
                className={`sg-general-choice kingdom-${general.kingdom}`}
                key={id}
                onClick={() => void send({ choice: id })}
              >
                {assets && <img src={generalArt(assets, id)} alt="" />}
                <strong>
                  {general.name} · {kingdoms[general.kingdom]}
                </strong>
                <small>体力 {general.hp}</small>
                {general.skills.map((k) => (
                  <span key={k}>
                    <b>{g.skills[k].name}</b> {g.skills[k].text}
                  </span>
                ))}
              </button>
            );
          })}
        </div>
      )}
      {!g.selecting && (
        <div className="sg-center">
          <span className="eyebrow">
            {g.revealed?.length
              ? "火攻 · 公开展示"
              : g.grace?.length
                ? "五谷丰登"
                : "正在结算"}
          </span>
          <div className="sg-public-cards">
            {(g.revealed?.length
              ? g.revealed
              : g.grace?.length
                ? g.grace
                : g.table || []
            ).map((id) => (
              <Card
                key={id}
                id={id}
                g={g}
                assets={assets}
                small
                onClick={() => setDetail(id)}
              />
            ))}
          </div>
          {!g.table?.length && !g.grace?.length && (
            <p>出牌、响应与判定将在这里展示。</p>
          )}
        </div>
      )}
      {!!extraCards.length && (
        <div className="sg-choice-cards">
          {extraCards.map((id) => (
            <div key={id}>
              <Card
                id={id}
                g={g}
                assets={assets}
                selected={cards.includes(id)}
                onClick={() => selectCard(id)}
              />
              {ask === "guanxing" && cards.includes(id) && (
                <small>牌堆顶第 {cards.indexOf(id) + 1} 张</small>
              )}
            </div>
          ))}
        </div>
      )}
      {me && !g.selecting && (
        <section className="sg-hand-section">
          <header>
            <h3>
              我的手牌 <small>{me.handCount} 张</small>
            </h3>
            <span className={`sg-role role-${me.role}`}>
              你的身份：{roles[me.role || ""]}
            </span>
            <span>攻击范围 {g.range}</span>
          </header>
          <div className="sg-hand">
            {me.hand?.map((id) => (
              <Card
                key={id}
                id={id}
                g={g}
                assets={assets}
                selected={cards.includes(id)}
                onClick={() => selectCard(id)}
              />
            ))}
          </div>
          {!me.hand?.length && <p className="muted">暂时没有手牌</p>}
          {skill === "guhuo" && (
            <label className="sg-declaration">
              蛊惑声明
              <select
                value={declaration}
                onChange={(e) => setDeclaration(e.target.value)}
                disabled={!enabled}
              >
                {g.guhuoKinds?.map((k) => (
                  <option key={k} value={k}>
                    {g.cardTypes[k].name}
                  </option>
                ))}
              </select>
              <small>选择一张手牌扣下，按声明的牌选择目标，再确认。</small>
            </label>
          )}
          <div className="sg-skills">
            {me.equip.some((id) => getCard(id).kind === "fan") && (
              <button
                disabled={
                  !enabled ||
                  (responding &&
                    ask === "card" &&
                    prompt?.effect !== "collateral")
                }
                className={skill === "fan" ? "selected" : ""}
                onClick={() => setSkill(skill === "fan" ? "" : "fan")}
              >
                朱雀羽扇 · 转火杀
              </button>
            )}
            {skills.map((k) => (
              <button
                key={k}
                disabled={!enabled}
                className={`${skill === k ? "selected" : ""} ${activeSkills.includes(k) || transforms.includes(k) ? "active-skill" : ""}`}
                title={g.skills[k].text}
                onClick={() => {
                  if (activeSkills.includes(k) || transforms.includes(k)) {
                    setSkill(skill === k ? "" : k);
                  } else {
                    setHelp(k);
                  }
                }}
              >
                {g.skills[k].name}
                {skill === k ? " ✓" : ""}
              </button>
            ))}
            {me.equip.some((id) => getCard(id).kind === "spear") && (
              <button
                className={skill === "spear" ? "selected" : ""}
                disabled={!enabled}
                onClick={() => setSkill(skill === "spear" ? "" : "spear")}
              >
                丈八蛇矛
              </button>
            )}
          </div>
        </section>
      )}
      <div className={`sg-actions ${enabled ? "active" : ""}`}>
        <div aria-live="polite">
          <strong>
            {game.finished
              ? "本局已结束"
              : responding
                ? prompt?.message
                : playing
                  ? "轮到你出牌"
                  : prompt
                    ? prompt.kind === "nullification"
                      ? "等待其他玩家响应锦囊"
                      : `等待 ${room.seats[prompt.player]?.name} 响应`
                    : `${room.seats[game.turn]?.name} 的回合`}
          </strong>
          <p>
            {responding
              ? instructions[ask || ""] || "根据提示选择后确认。"
              : playing
                ? actionHint
                : me?.dead
                  ? "你已阵亡，可以继续观战与聊天。"
                  : "可以点击技能或牌面查看说明。"}
          </p>
        </div>
        {error && (
          <p role="alert" className="sg-error">
            {error}
          </p>
        )}
        {responding && ask === "huashen" && (
          <div className="sg-avatar-options">
            {prompt?.choices?.map((id) => {
              const general = g.generals.find((x) => x.id === id);
              return (
                <article
                  key={id}
                  className={`sg-avatar-choice kingdom-${general?.kingdom || "qun"}`}
                >
                  {assets && <img src={generalArt(assets, id)} alt="" />}
                  <div>
                    <strong>
                      {general?.name} · {kingdoms[general?.kingdom || ""]} ·{" "}
                      {general?.female ? "女" : "男"}
                    </strong>
                    <div className="sg-avatar-skills">
                      {(prompt.avatarSkills?.[id]?.length
                        ? prompt.avatarSkills[id]
                        : [""]
                      ).map((k) => (
                        <button
                          key={k || "none"}
                          disabled={busy}
                          title={g.skills[k]?.text}
                          onClick={() =>
                            void send({
                              choice: id,
                              skill: k,
                              cards: [],
                              targets: [],
                            })
                          }
                        >
                          {g.skills[k]?.name || "选择此化身"}
                        </button>
                      ))}
                    </div>
                    {prompt.avatarSkills?.[id]?.map((k) => (
                      <p key={k}>
                        <b>{g.skills[k].name}</b>：{g.skills[k].text}
                      </p>
                    ))}
                  </div>
                </article>
              );
            })}
          </div>
        )}
        {responding && ask === "qiaobian_move" && (
          <div className="sg-move-options">
            {[...new Set(prompt?.moves?.map((m) => m.card))].map((id) => {
              const moves = prompt!.moves!.filter((m) => m.card === id),
                from = moves[0].targets[0];
              return (
                <details key={id}>
                  <summary>
                    {room.seats[from]?.name} ·{" "}
                    {g.cardTypes[getCard(id).kind].name}{" "}
                    {suits[getCard(id).suit]}
                    {getCard(id).rank}
                  </summary>
                  <div className="sg-action-buttons">
                    {moves.map((m) => (
                      <button
                        key={m.targets[1]}
                        disabled={busy}
                        onClick={() =>
                          void send({
                            card: id,
                            targets: m.targets,
                            cards: [],
                            skill: "",
                          })
                        }
                      >
                        移至 {room.seats[m.targets[1]]?.name}
                      </button>
                    ))}
                  </div>
                </details>
              );
            })}
          </div>
        )}
        {responding && ask === "zhiji" && (
          <div className="sg-action-buttons">
            <button
              disabled={busy}
              onClick={() => void send({ choice: "draw" })}
            >
              摸两张牌
            </button>
            {prompt?.choices?.includes("heal") && (
              <button
                disabled={busy}
                onClick={() => void send({ choice: "heal" })}
              >
                回复 1 点体力
              </button>
            )}
          </div>
        )}
        {responding && ask === "draw_phase" && (
          <div className="sg-action-buttons">
            <button
              disabled={busy}
              onClick={() => void send({ choice: "normal" })}
            >
              正常摸两张
            </button>
            {["yingzi", "luoyi", "tuxi", "shuangxiong", "zaiqi", "haoshi"]
              .filter((k) => skills.includes(k))
              .map((k) => (
                <button
                  key={k}
                  disabled={busy}
                  onClick={() => void send({ choice: k })}
                >
                  {g.skills[k].name}
                </button>
              ))}
          </div>
        )}
        {responding && ask === "yinghun" && (
          <div className="sg-action-buttons">
            <button
              className="outline"
              disabled={busy || targets.length !== 1}
              onClick={() => void send({ choice: "draw_many" })}
            >
              摸 {prompt?.amount} 张 · 弃 1 张
            </button>
            <button
              className="outline"
              disabled={busy || targets.length !== 1}
              onClick={() => void send({ choice: "draw_one" })}
            >
              摸 1 张 · 弃 {prompt?.amount} 张
            </button>
          </div>
        )}
        {responding && ask === "benghuai" && (
          <div className="sg-action-buttons">
            <button disabled={busy} onClick={() => void send({ choice: "hp" })}>
              失去 1 点体力
            </button>
            <button
              className="outline"
              disabled={busy}
              onClick={() => void send({ choice: "maxhp" })}
            >
              失去 1 点体力上限
            </button>
          </div>
        )}
        {responding &&
          ["luanwu", "tiaoxin"].includes(ask || "") &&
          skills.includes("jijiang") &&
          !(prompt.step! & 2) && (
            <button
              disabled={busy || (ask === "luanwu" && targets.length !== 1)}
              onClick={() =>
                void send({ choice: "jijiang", cards: [], skill: "" })
              }
            >
              激将
            </button>
          )}
        {responding && ask === "guhuo_question" && (
          <button
            className="primary"
            disabled={busy}
            onClick={() =>
              void send({
                choice: "challenge",
                cards: [],
                targets: [],
                skill: "",
              })
            }
          >
            质疑
          </button>
        )}
        {responding && ask === "fanjian" && (
          <div className="sg-action-buttons">
            {["spade", "heart", "club", "diamond"].map((v, i) => (
              <button
                key={v}
                disabled={busy}
                onClick={() => void send({ choice: v })}
              >
                {suits[i]}
              </button>
            ))}
          </div>
        )}
        {responding && ask === "steal" && (
          <button
            disabled={busy || !g.players[prompt?.target ?? -1]?.handCount}
            onClick={() => void send({ choice: "hand" })}
          >
            选择一张随机手牌
          </button>
        )}
        {responding && (ask === "card" || ask === "support") && (
          <div className="sg-response-extras">
            {prompt.wanted === "jink" &&
              !prompt.ignoreArmor &&
              !(prompt.step! & 1) &&
              g.armor === "eight_diagram" && (
                <button
                  disabled={busy}
                  onClick={() => void send({ choice: "eight_diagram" })}
                >
                  {skills.includes("bazhen") &&
                  !me?.equip.some(
                    (id) => g.cardTypes[getCard(id).kind].slot === "armor",
                  )
                    ? "八阵判定"
                    : "八卦阵判定"}
                </button>
              )}
            {!(prompt.step! & 2) &&
              skills.includes(
                prompt.wanted === "jink" ? "hujia" : "jijiang",
              ) && (
                <button
                  disabled={busy}
                  onClick={() =>
                    void send({
                      choice: prompt.wanted === "jink" ? "hujia" : "jijiang",
                    })
                  }
                >
                  {prompt.wanted === "jink" ? "护驾" : "激将"}
                </button>
              )}
          </div>
        )}
        {enabled && ask !== "general" && (
          <div className="sg-action-buttons">
            {(!responding ||
              ![
                "draw_phase",
                "fanjian",
                "steal",
                "guhuo_question",
                "yinghun",
                "benghuai",
                "huashen",
                "qiaobian_move",
                "zhiji",
              ].includes(ask || "")) && (
              <button
                className="primary"
                disabled={
                  busy ||
                  (!cards.length &&
                    !simple &&
                    ask !== "guanxing" &&
                    !(
                      ask === "qiaobian_draw" &&
                      targets.length > 0 &&
                      targets.length <= 2
                    ) &&
                    !(
                      [
                        "shensu_judge",
                        "leiji",
                        "jieming",
                        "quhu_target",
                        "fangzhu",
                      ].includes(ask || "") && targets.length === 1
                    ) &&
                    !(
                      playing &&
                      [
                        "kurou",
                        "fanjian",
                        "jijiang",
                        "qiangxi",
                        "luanwu",
                        "dimeng",
                        "tiaoxin",
                      ].includes(skill)
                    ))
                }
                onClick={() =>
                  void send(
                    ask === "guanxing"
                      ? { take: extraCards.filter((id) => !cards.includes(id)) }
                      : simple
                        ? { choice: "yes" }
                        : {},
                  )
                }
              >
                {simple
                  ? "发动"
                  : ask === "guanxing"
                    ? "确认牌序"
                    : responding
                      ? "确认响应"
                      : activeSkills.includes(skill)
                        ? "发动技能"
                        : "确认出牌"}
              </button>
            )}
            {optional && (
              <button
                className="outline"
                disabled={busy}
                onClick={() =>
                  void send({
                    choice: "pass",
                    cards: [],
                    targets: [],
                    skill: "",
                  })
                }
              >
                {ask === "guhuo_question" ? "不质疑" : "放弃"}
              </button>
            )}
            {playing && (
              <button
                className="outline"
                disabled={busy}
                onClick={() =>
                  void send({
                    type: "sg_end",
                    cards: [],
                    targets: [],
                    skill: "",
                  })
                }
              >
                结束出牌
              </button>
            )}
            {(cards.length > 0 || targets.length > 0 || skill) && (
              <button
                className="subtle"
                onClick={() => {
                  setCards([]);
                  setTargets([]);
                  setSkill("");
                }}
              >
                清空选择
              </button>
            )}
          </div>
        )}
      </div>
    </section>
  );
}
export function SanguoshaCover() {
  return (
    <svg preserveAspectRatio="xMidYMid slice" viewBox="0 0 520 260">
      <rect width="520" height="260" fill="#e9d8b5" />
      <circle cx="413" cy="57" r="38" fill="#bd6650" />
      <path
        d="M0 161L93 91 182 158 281 80 399 150 520 109V260H0"
        fill="#b1b899"
      />
      <path d="M0 203Q120 160 260 201T520 178V260H0" fill="#71866a" />
      {[
        { x: 124, y: 38, r: -13, c: "#577c94", v: "魏" },
        { x: 218, y: 29, r: 0, c: "#b6604d", v: "蜀" },
        { x: 314, y: 40, r: 13, c: "#477a62", v: "吴" },
      ].map((c) => (
        <g
          key={c.v}
          transform={`translate(${c.x} ${c.y}) rotate(${c.r} 40 85)`}
        >
          <path d="M6 9H88V186H6Z" fill="#283e37" opacity=".18" />
          <rect width="82" height="176" rx="7" fill="#fbf1d9" />
          <rect x="6" y="6" width="70" height="164" rx="4" fill={c.c} />
          <path
            d="M17 140L41 104 65 140M41 42V100M24 67H58"
            fill="none"
            stroke="#eed8a3"
            strokeWidth="4"
          />
          <circle
            cx="41"
            cy="60"
            r="24"
            fill={c.c}
            stroke="#eed8a3"
            strokeWidth="2"
          />
          <text
            x="41"
            y="70"
            textAnchor="middle"
            fill="#fff1cf"
            fontFamily="serif"
            fontSize="28"
          >
            {c.v}
          </text>
        </g>
      ))}
    </svg>
  );
}
export function SanguoshaRules({ options }: { options?: SGOptions }) {
  return (
    <>
      <p>
        经典身份局，4–8 人，25 名标准武将。
        {options?.deck === "military"
          ? "标准＋军争：160 张牌。"
          : "标准牌堆：108 张牌（含 EX）。"}
      </p>
      <ol>
        <li>
          主公与忠臣消灭全部反贼和内奸获胜；反贼令主公死亡获胜；内奸须先消灭其他角色，最后击败主公。除主公外的身份起初保密。
        </li>
        <li>
          主公先选将；五人及以上主公体力上限
          +1。每人起手四张，正常每回合摸两张，弃牌后手牌不超过当前体力。
        </li>
        <li>
          出牌阶段通常只能使用一次杀。选择手牌、技能、目标后确认；武将技能和装备可能改变规则。
        </li>
        <li>
          杀可用闪响应，锦囊可用无懈可击反制。体力为零或负数时进入求桃，救到 1
          点以上才能继续。
        </li>
        <li>
          击杀反贼摸三张；主公误杀忠臣弃掉全部手牌和装备。阵亡角色仍属于自己的阵营。
        </li>
        <li>
          主动操作 120 秒，回合外响应 20
          秒，等待响应期间暂停主动计时。超时默认放弃可选响应，必要操作自动完成。
        </li>
        <li>
          纯真人完整对局：获胜阵营各 +20 积分，失败阵营各
          −10；含电脑或中止不计分。
        </li>
      </ol>
      {options?.deck === "military" && (
        <>
          <h3>军争篇</h3>
          <ul>
            <li>
              火杀与雷杀造成属性伤害，与普通杀共用出杀次数。龙胆等需要杀的技能可以使用属性杀。
            </li>
            <li>
              酒：每回合主动使用一次，令本回合下一张杀伤害加一；自己濒死时可以用酒自救，救命不计次数。
            </li>
            <li>
              铁索连环：选择一至两人横置或重置；横置角色受到属性伤害，会传导给其他横置角色并重置。可以不选目标，重铸摸一张。
            </li>
            <li>
              火攻：目标展示一张手牌，你弃同花色手牌才能造成火焰伤害。兵粮寸断：判定不为梅花，跳过摸牌阶段。
            </li>
            <li>
              藤甲抵挡普通杀、南蛮与万箭，但增加火伤；白银狮子将伤害限制为一点，失去装备后回复一点体力。
            </li>
            <li>
              朱雀羽扇可将普通杀转为火杀；古锭刀对没有手牌的目标出杀，伤害加一。
            </li>
          </ul>
        </>
      )}
    </>
  );
}
