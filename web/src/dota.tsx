import { useEffect, useState } from "react";
import {
  Bot,
  Check,
  ChevronDown,
  Coins,
  Flag,
  Heart,
  LockKeyhole,
  Sparkles,
  Swords,
  Undo2,
} from "lucide-react";
import type { Act, DotaOrder, DotaState, Room } from "./types";
import { PlayerName } from "./profiles";
import { LogLine } from "./log-line";
import "./dota.css";

const teams = ["近卫", "天灾"];
const laneName = (lane: number, n: number): string =>
  lane < 0
    ? "泉水"
    : n === 1
      ? "中路"
      : n === 2
        ? ["上路", "下路"][lane]
        : ["上路", "中路", "下路"][lane];
function Art({
  id,
  assets,
  label,
  className = "",
}: {
  id: string;
  assets: string;
  label: string;
  className?: string;
}) {
  const [fallback, setFallback] = useState(false);
  return (
    <img
      className={`dota-art ${className}`}
      src={`${fallback ? "" : assets}/dota/v1/${id}.webp`}
      alt={label}
      draggable={false}
      onError={() => setFallback(true)}
    />
  );
}
function cardSpec(g: DotaState, player: number, card: number) {
  return card < 5
    ? g.catalog.common[card]
    : g.catalog.heroes.find((h) => h.id === g.players[player].hero)!.skills[
        card - 5
      ];
}
function orderText(g: DotaState, room: Room, order: DotaOrder) {
  const parts = [];
  if (order.lane >= 0)
    parts.push(
      `${order.card === 0 ? "进军" : "移动到"}${laneName(order.lane, g.n)}`,
    );
  if (order.target >= 0) parts.push(`目标：${room.seats[order.target].name}`);
  if (order.buy)
    parts.push(`购买${g.catalog.items.find((i) => i.id === order.buy)!.name}`);
  return (
    parts.join(" · ") || (order.card === 4 ? "只整备，不购买" : "执行本牌")
  );
}
export function DotaBoard({
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
    g = game.dota!,
    me = g.players[room.you];
  const [draftTeams, setDraftTeams] = useState(g.players.map((p) => p.team));
  const [selected, setSelected] = useState<DotaOrder | null>(null);
  const [inspected, setInspected] = useState<number | null>(null);
  const [shop, setShop] = useState(false);
  const [report, setReport] = useState(false);
  const [mobileView, setMobileView] = useState("board");
  const canPlay =
    !room.spectating &&
    !busy &&
    !game.finished &&
    !room.seats[room.you]?.autoPlay;
  const send = (type: string, extra: Record<string, unknown> = {}) =>
    void act({ type, prompt: g.sequence, ...extra });
  useEffect(() => {
    setDraftTeams(g.players.map((p) => p.team));
    setSelected(null);
  }, [g.sequence]);
  const isLegal = (o: DotaOrder) =>
    g.legal.some((a) => JSON.stringify(a) === JSON.stringify(o));
  const choice =
    selected && isLegal(selected)
      ? selected
      : g.legal.find((o) => o.card === 0) || g.legal[0];
  const last = g.history.at(-1);
  const round = game.finished ? game.round - 1 : game.round;
  const changedTeams = draftTeams.some((t, i) => t !== g.players[i].team);

  if (game.phase === "dota_teams")
    return (
      <section className="dota-table dota-setup">
        <div className="dota-setup-heading">
          <span className="dota-overline">CHOOSE YOUR ALLIES</span>
          <h2>英雄尚未登场，先找到你的同伴</h2>
          <p>
            {g.n} 对 {g.n} · 每人控制一位英雄 · 摧毁敌方遗迹获胜
          </p>
        </div>
        <div className="dota-team-selection">
          {teams.map((name, t) => (
            <section className={`dota-faction faction-${t}`} key={t}>
              <h3>
                <Flag size={20} />
                {name}
                <small>
                  {draftTeams.filter((x) => x === t).length} / {g.n}
                </small>
              </h3>
              {room.seats.map((seat, i) =>
                draftTeams[i] !== t ? null : (
                  <div className="dota-seat" key={seat.id}>
                    <span>
                      <PlayerName user={seat} />
                      {(seat.bot || seat.autoPlay) && <Bot size={15} />}
                    </span>
                    {room.you === g.captain && canPlay ? (
                      <select
                        aria-label={`${seat.name}的队伍`}
                        value={draftTeams[i]}
                        onChange={(e) =>
                          setDraftTeams(
                            draftTeams.map((x, j) =>
                              j === i ? Number(e.target.value) : x,
                            ),
                          )
                        }
                      >
                        {teams.map((v, j) => (
                          <option value={j} key={j}>
                            {v}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <span>{g.confirmed[i] ? "✓ 已确认" : "等待确认"}</span>
                    )}
                  </div>
                ),
              )}
            </section>
          ))}
        </div>
        <div className="dota-setup-actions">
          {room.you === g.captain && changedTeams && (
            <button
              className="dota-primary"
              disabled={
                !canPlay || draftTeams.filter((x) => x === 0).length !== g.n
              }
              onClick={() => send("dota_teams", { targets: draftTeams })}
            >
              保存分队
            </button>
          )}
          {!room.spectating && (
            <button
              className="dota-primary"
              disabled={!canPlay || changedTeams || g.confirmed[room.you]}
              onClick={() => send("dota_confirm")}
            >
              <Check size={17} />
              {g.confirmed[room.you]
                ? "已确认，等待同伴"
                : `确认加入${teams[me.team]}`}
            </button>
          )}
          <p>
            房主可以调整分队，调整后所有人重新确认。确认完成后双方交替选英雄。
          </p>
        </div>
      </section>
    );

  if (game.phase === "dota_draft")
    return (
      <section className="dota-table dota-setup">
        <div className="dota-setup-heading">
          <span className="dota-overline">THE HEROES OF OLD</span>
          <h2>
            {game.turn === room.you
              ? "选择你的英雄"
              : `等待 ${room.seats[game.turn].name} 选择英雄`}
          </h2>
          <p>
            近卫与天灾交替选人 · 每位英雄全场唯一
            {me ? ` · 你属于${teams[me.team]}` : ""}
          </p>
        </div>
        <div className="dota-draft-progress">
          {g.draft.map((p, i) => (
            <span
              className={`${i === g.draftStep ? "active" : ""} faction-${g.players[p].team}`}
              key={p}
            >
              {i + 1}. {room.seats[p].name}
              {g.players[p].hero
                ? ` · ${g.catalog.heroes.find((h) => h.id === g.players[p].hero)?.name}`
                : ""}
            </span>
          ))}
        </div>
        <div className="dota-hero-picker">
          {g.catalog.heroes.map((h) => {
            const taken = g.players.some((p) => p.hero === h.id);
            return (
              <article
                key={h.id}
                className={`dota-hero-option ${taken ? "taken" : ""}`}
              >
                <div>
                  <Art id={`${h.id}_portrait`} assets={assets} label={h.name} />
                  <h3>
                    {h.name}
                    <small>
                      <Heart size={12} /> {h.hp} 生命{taken ? " · 已选" : ""}
                    </small>
                  </h3>
                </div>
                <p>{h.passive || h.skills[0].text}</p>
                <div className="dota-skill-preview">
                  {h.skills.map((s) => (
                    <span title={`${s.name}：${s.text}`} key={s.name}>
                      <Art id={s.art} assets={assets} label={s.name} />
                      {s.name}
                    </span>
                  ))}
                </div>
                <details className="dota-hero-skills">
                  <summary>查看技能效果</summary>
                  {h.skills.map((skill, i) => (
                    <p key={skill.name}>
                      <b>
                        {skill.name}
                        {i === 2 ? " · 大招" : ""}
                      </b>
                      {skill.text}
                      {i === 2 && " 消耗 3 蓄力，另提供 +1 推进。"}
                    </p>
                  ))}
                </details>
                <button
                  className="dota-primary"
                  disabled={!canPlay || game.turn !== room.you || taken}
                  onClick={() => send("dota_pick", { choice: h.id })}
                >
                  {taken ? "已被选走" : `选择${h.name}`}
                </button>
              </article>
            );
          })}
        </div>
      </section>
    );

  return (
    <section className="dota-table" data-mobile-view={mobileView}>
      <div className="dota-match-bar">
        <span>
          <Swords size={18} /> 第 {round} 轮{" "}
          <small>
            {game.finished
              ? "对局结束"
              : `${teams[(g.first + round - 1) % 2]}先手`}
          </small>
        </span>
        <div>
          <button onClick={() => setShop(!shop)} aria-expanded={shop}>
            <Coins size={16} />
            装备
          </button>
          <button onClick={() => setReport(!report)} aria-expanded={report}>
            战报
            <ChevronDown size={15} />
          </button>
        </div>
      </div>
      {!game.finished && !room.spectating && (
        <nav className="dota-mobile-tabs" aria-label="牌桌视图">
          <button
            className={mobileView === "board" ? "active" : ""}
            onClick={() => setMobileView("board")}
          >
            战场与玩家
          </button>
          <button
            className={mobileView === "hand" ? "active" : ""}
            onClick={() => setMobileView("hand")}
          >
            {g.locked[room.you] ? "已锁定行动" : "我的行动牌"}
            {!g.locked[room.you] && <span className="dota-action-dot" />}
          </button>
        </nav>
      )}
      <div className="dota-rosters">
        {teams.map((name, t) => (
          <section className={`dota-faction faction-${t}`} key={t}>
            <div className="dota-team-title">
              <strong>
                {name}
                {me?.team === t ? " · 我方" : ""}
              </strong>
              <span>
                遗迹 <b>{g.core[t]}</b> / {g.n + 1}
                <small>击杀 {g.kills[t]}</small>
              </span>
            </div>
            <div className="dota-players">
              {g.players.map((p, i) => {
                if (p.team !== t) return null;
                const spec = g.catalog.heroes.find((h) => h.id === p.hero)!;
                const old = last?.before[i];
                const lost = old
                  ? Math.max(
                      0,
                      old.hp -
                        (last!.after[i].lane < 0 ? 0 : last!.after[i].hp),
                    )
                  : 0;
                return (
                  <button
                    className={`dota-player ${i === room.you ? "mine" : ""} ${g.locked[i] ? "locked" : ""}`}
                    key={i}
                    onClick={() => setInspected(inspected === i ? null : i)}
                    aria-expanded={inspected === i}
                    title={`查看${room.seats[i].name}的英雄、装备和已用牌`}
                  >
                    <div className="dota-portrait-wrap">
                      <Art
                        id={`${p.hero}_portrait`}
                        assets={assets}
                        label={spec.name}
                      />
                      {lost > 0 && (
                        <span key={last!.round} className="dota-damage">
                          −{lost}
                        </span>
                      )}
                      <span className="dota-hp">
                        <Heart size={10} />
                        {p.hp}/{g.maximum[i]}
                      </span>
                    </div>
                    <div className="dota-player-summary">
                      <strong>
                        {room.seats[i].name}
                        {(room.seats[i].bot || room.seats[i].autoPlay) && (
                          <Bot size={13} />
                        )}
                      </strong>
                      <span>{spec.name}</span>
                      <small>
                        {laneName(p.lane, g.n)} · <Coins size={10} />
                        {p.gold} · <Sparkles size={10} />
                        {p.charge}/3
                      </small>
                    </div>
                    <span className="dota-lock">
                      {g.locked[i] ? <LockKeyhole size={13} /> : null}
                    </span>
                  </button>
                );
              })}
            </div>
          </section>
        ))}
      </div>
      {inspected !== null && (
        <section className="dota-inspector">
          <div>
            <PlayerName user={room.seats[inspected]} />
            <button onClick={() => setInspected(null)}>收起</button>
          </div>
          <p>
            {g.catalog.heroes.find((h) => h.id === g.players[inspected].hero)
              ?.passive || "通过技能与装备配合同伴。"}
            {g.players[inspected].wounded && " 上轮受伤，本轮跳刀不可用。"}
          </p>
          <div className="dota-public-hand">
            {Array.from({ length: 8 }, (_, c) => (
              <span
                className={g.players[inspected].spent & (1 << c) ? "spent" : ""}
                key={c}
                title={cardSpec(g, inspected, c).text}
              >
                {cardSpec(g, inspected, c).name}
                {g.players[inspected].spent & (1 << c) ? " · 已用" : ""}
              </span>
            ))}
          </div>
          <div className="dota-gear">
            {g.players[inspected].gear.length ? (
              g.players[inspected].gear.map((id) => (
                <span key={id}>
                  <Art
                    id={id}
                    assets={assets}
                    label={g.catalog.items.find((x) => x.id === id)!.name}
                  />
                  <span>
                    <b>{g.catalog.items.find((x) => x.id === id)!.name}</b>
                    {g.catalog.items.find((x) => x.id === id)!.text}
                  </span>
                </span>
              ))
            ) : (
              <p>尚未购买装备 · 最多两件</p>
            )}
          </div>
        </section>
      )}
      <div className="dota-battlefield">
        <div className="dota-map-header">
          <span>兵线战场</span>
          <small>
            净优势 1–2 推进一格 · 3+ 推进两格 · 攻城伤害{" "}
            {round >= 21 ? 3 : round >= 13 ? 2 : 1}
          </small>
        </div>
        {Array.from({ length: g.n }, (_, l) => (
          <div className="dota-lane" key={l}>
            <div
              className={`dota-tower faction-0 ${g.towers[0][l] === 0 ? "fallen" : ""}`}
            >
              <Tower />
              <span>{g.towers[0][l] || "已毁"}</span>
            </div>
            <div className="dota-lane-center">
              <span className="dota-lane-label">{laneName(l, g.n)}</span>
              <div className="dota-track">
                {[-2, -1, 0, 1, 2].map((x) => (
                  <span className={x === g.track[l] ? "active" : ""} key={x}>
                    {x === 0
                      ? "河道"
                      : x === -2
                        ? "近卫"
                        : x === 2
                          ? "天灾"
                          : ""}
                  </span>
                ))}
                <div
                  className="dota-wave"
                  style={{ left: `${(g.track[l] + 2) * 20 + 10}%` }}
                >
                  <Swords size={17} />
                </div>
              </div>
              <div className="dota-lane-heroes">
                {g.players.map((p, i) =>
                  p.lane !== l ? null : (
                    <span
                      className={`faction-${p.team}`}
                      key={i}
                      title={`${room.seats[i].name} · ${g.catalog.heroes.find((h) => h.id === p.hero)!.name}`}
                    >
                      <Art
                        id={`${p.hero}_portrait`}
                        assets={assets}
                        label={room.seats[i].name}
                      />
                    </span>
                  ),
                )}
              </div>
            </div>
            <div
              className={`dota-tower faction-1 ${g.towers[1][l] === 0 ? "fallen" : ""}`}
            >
              <Tower />
              <span>{g.towers[1][l] || "已毁"}</span>
            </div>
          </div>
        ))}
      </div>
      {last && (
        <details className="dota-reveal" key={last.round}>
          <summary>
            第 {last.round} 轮已结算 · 查看双方出牌与攻城结果
            <ChevronDown size={14} />
          </summary>
          <div className="dota-revealed-cards">
            {last.orders.map((o, i) => (
              <div key={i} className={`faction-${g.players[i].team}`}>
                <Art
                  id={cardSpec(g, i, o.card).art}
                  assets={assets}
                  label={cardSpec(g, i, o.card).name}
                />
                <span>
                  {room.seats[i].name}
                  <b>{cardSpec(g, i, o.card).name}</b>
                  <small>{orderText(g, room, o)}</small>
                </span>
              </div>
            ))}
          </div>
        </details>
      )}
      {report && (
        <section className="dota-battle-report">
          <h3>战报</h3>
          {game.log
            .slice(-100)
            .reverse()
            .map((line, i) => (
              <p key={`${room.version}-${i}`}>
                <LogLine line={line} seats={room.seats} />
              </p>
            ))}
        </section>
      )}
      {shop && (
        <section className="dota-shop">
          <div>
            <h3>装备商店</h3>
            <button onClick={() => setShop(false)}>收起</button>
          </div>
          <p>
            成功整备时可买一件。最多两格，买第三件会替换最早的装备。鞋子可补差价升级。
          </p>
          <div className="dota-shop-grid">
            {g.catalog.items
              .filter((i) => g.n > 1 || i.solo)
              .map((item) => {
                const order = g.legal.find((o) => o.buy === item.id);
                const price =
                  item.price -
                  (me?.gear.includes("boots") &&
                  ["phase_boots", "arcane_boots"].includes(item.id)
                    ? 3
                    : 0);
                return (
                  <button
                    key={item.id}
                    disabled={!canPlay || !order || g.locked[room.you]}
                    onClick={() => {
                      setSelected(order!);
                      setShop(false);
                    }}
                  >
                    <Art id={item.id} assets={assets} label={item.name} />
                    <span>
                      <b>
                        {item.name}
                        <em>{price} 金</em>
                      </b>
                      <small>{item.text}</small>
                      {me?.gear.includes(item.id) && <strong>已装备</strong>}
                    </span>
                  </button>
                );
              })}
          </div>
        </section>
      )}
      {!game.finished && me && !room.spectating && (
        <section className="dota-hand">
          <div className="dota-hand-heading">
            <h3>
              {g.catalog.heroes.find((h) => h.id === me.hero)!.name} ·{" "}
              {g.locked[room.you] ? "行动已锁定" : "暗选一张行动牌"}
            </h3>
            <span>
              <Coins size={14} />
              {me.gold} 金 <Sparkles size={14} />
              {me.charge}/3
            </span>
          </div>
          {g.locked[room.you] ? (
            <div className="dota-locked-plan">
              <LockKeyhole size={22} />
              <div>
                <b>{cardSpec(g, room.you, g.plans[room.you].card).name}</b>
                <p>{orderText(g, room, g.plans[room.you])} · 等待其他玩家</p>
              </div>
              <button disabled={!canPlay} onClick={() => send("dota_unlock")}>
                <Undo2 size={15} />
                撤回
              </button>
            </div>
          ) : (
            <>
              <div className="dota-action-cards">
                {Array.from({ length: 8 }, (_, c) => {
                  const spec = cardSpec(g, room.you, c),
                    order = g.legal.find((o) => o.card === c);
                  return (
                    <button
                      key={c}
                      className={`${choice?.card === c ? "selected" : ""} ${!order ? "unavailable" : ""}`}
                      disabled={!canPlay || !order}
                      title={spec.text}
                      aria-pressed={choice?.card === c}
                      onClick={() => setSelected(order!)}
                    >
                      <Art id={spec.art} assets={assets} label={spec.name} />
                      <strong>{spec.name}</strong>
                      <small>
                        {!order
                          ? me.lane < 0
                            ? "先回战场"
                            : me.spent & (1 << c)
                              ? "已使用"
                              : "蓄力不足"
                          : c === 7
                            ? "消耗 3 蓄力"
                            : c === 4
                              ? "回城 · 收牌"
                              : "行动"}
                      </small>
                    </button>
                  );
                })}
              </div>
              {choice && (
                <div className="dota-order">
                  <p>
                    {cardSpec(g, room.you, choice.card).text}
                    {choice.card === 7 && " 大招另提供 +1 推进。"}
                  </p>
                  <div>
                    <select
                      aria-label="行动目标与装备"
                      disabled={!canPlay}
                      value={g.legal.indexOf(
                        g.legal.find(
                          (o) => JSON.stringify(o) === JSON.stringify(choice),
                        )!,
                      )}
                      onChange={(e) =>
                        setSelected(g.legal[Number(e.target.value)])
                      }
                    >
                      {g.legal.map((o, i) =>
                        o.card === choice.card ? (
                          <option key={i} value={i}>
                            {orderText(g, room, o)}
                          </option>
                        ) : null,
                      )}
                    </select>
                    <button
                      className="dota-primary"
                      disabled={!canPlay}
                      onClick={() => send("dota_plan", { dota: choice })}
                    >
                      <LockKeyhole size={15} />
                      锁定行动
                    </button>
                  </div>
                </div>
              )}
            </>
          )}
          <div className="dota-allied-plans">
            {Object.entries(g.plans)
              .filter(([p]) => Number(p) !== room.you)
              .map(([p, o]) => (
                <span key={p}>
                  <Check size={12} />
                  {room.seats[Number(p)].name}：
                  {cardSpec(g, Number(p), o.card).name} ·{" "}
                  {orderText(g, room, o)}
                </span>
              ))}
          </div>
          <p className="dota-privacy">
            锁定的计划仅队友可见，全部锁定后公开。牌桌聊天对所有人可见。
          </p>
        </section>
      )}
      {room.spectating && !game.finished && (
        <p className="dota-spectating">
          正在观战 · 已锁定 {g.locked.filter(Boolean).length} /{" "}
          {g.players.length} · 揭牌前无法查看行动
        </p>
      )}
    </section>
  );
}
function Tower() {
  return (
    <svg viewBox="0 0 44 54" aria-hidden="true">
      <path
        d="M7 6h7v8h6V6h6v8h6V6h6v18l-5 5 2 17 5 3v4H4v-4l6-3 2-17-5-5Z"
        fill="currentColor"
      />
      <path
        d="M18 52V36a4 4 0 0 1 8 0v16M12 22h22"
        fill="#162a22"
        stroke="#162a22"
        strokeWidth="2"
      />
    </svg>
  );
}
export function DotaCover() {
  return (
    <svg viewBox="0 0 520 260" preserveAspectRatio="xMidYMid slice">
      <rect width="520" height="260" fill="#263f37" />
      <path d="M0 210Q140 80 520 30V260H0Z" fill="#473a43" />
      <path
        d="M70 260Q220 160 260 130T450 0"
        stroke="#82b9b0"
        strokeWidth="22"
        fill="none"
        opacity=".6"
      />
      <g stroke="#d0b773" strokeWidth="9" fill="none">
        <path d="M65 211V55H450M65 211H450V55M65 211L450 55" />
      </g>
      <g fill="#a8ce8a" stroke="#eae2b6" strokeWidth="3">
        <path d="M32 190h20v-25h25v25h20v40H32Z" />
        <path d="M427 38h20V13h25v25h20v40h-65Z" />
      </g>
      <g transform="translate(219 66) rotate(12 42 63)">
        <rect
          width="84"
          height="126"
          rx="9"
          fill="#ead4a1"
          stroke="#f6e5b9"
          strokeWidth="3"
        />
        <path
          d="M25 27 61 72M59 26 24 71M16 63l18 17M52 64l17 16"
          stroke="#6d493d"
          strokeWidth="7"
        />
        <circle cx="43" cy="99" r="9" fill="#b85244" />
      </g>
    </svg>
  );
}
export function DotaRules() {
  return (
    <>
      <p>
        兵线争锋是一款原创 DotA 主题桌游，使用经典 Warcraft III
        图标与桌游改编技能。支持
        1v1、2v2、3v3，每人一名英雄，所有模式必须拆掉敌方遗迹。
      </p>
      <ol>
        <li>
          开始后分队并全员确认，双方交替选择不重复的英雄。1v1 一条路，2v2
          两条路，3v3 三条路。
        </li>
        <li>
          每轮同时暗选一张行动牌和目标。队友可以看到已锁定的计划，对手和观战者不能。全员锁定后揭牌。
        </li>
        <li>
          先移动，再魔免、沉默、防护和治疗；伤害按先手队伍与另一队交替结算，队内起始玩家每轮轮换。先被击倒者不能发动未执行的伤害。同一张牌的范围伤害与反伤一起结算。
        </li>
        <li>
          推进压过固守，固守抵消突袭；受到实际伤害会取消该英雄的推进。各路净优势
          1–2 点走一格，3
          点以上走两格。到达端点就打塔，塔已毁才打遗迹，随后兵线回河道。
        </li>
        <li>
          通用牌：推进同路 +3、换路 +2、出泉水 +1；突袭伤害 3、推进 +1；固守护盾
          3、推进 +2；打钱存活且在兵线时 +3
          金；整备先承伤、存活才回城、回满生命、收牌并买一件装备。
        </li>
        <li>
          已用牌公开，整备或阵亡后收回。大招消耗 3 蓄力并提供 +1 推进；每轮恢复
          1 蓄力，上限 3。偶数轮得 1 金，上限 12。装备两格，满格时替换最早装备。
        </li>
        <li>
          阵亡损失 2 金，参与命中的敌人各得 1 金，并给敌方该路 3
          点援军推进。泉水里的英雄下一轮必须推进出征，不淘汰玩家。
        </li>
        <li>
          护盾依次抵消伤害；反伤不吃护盾也不再反弹。魔免抵消魔法伤害及沉默、减速。跳刀需要上轮没有受伤；原力法杖只能移动至相邻一路。
        </li>
        <li>
          1v1 塔生命 1、遗迹 2；2v2 塔 2、遗迹 3；3v3 塔 2、遗迹 4。攻城伤害第
          1–12 轮为 1，第 13–20 轮为 2，之后为
          3。同次攻城不穿透防御塔；两边同轮拆家时本轮先手方获胜。
        </li>
        <li>
          每阶段 120
          秒，超时由电脑接管，回来可取消托管。获胜队伍共同获胜；有电脑席位的练习局不计积分。
        </li>
      </ol>
      <p>
        点击玩家可以检查英雄特性、已用行动牌与装备；装备商店显示完整效果。界面保留原版素材颜色，数值采用桌游规则。
      </p>
    </>
  );
}
