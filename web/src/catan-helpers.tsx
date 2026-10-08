import { useEffect, useState } from "react";
import type { Act, CatanOptions, Room } from "./types";
import "./catan-helpers.css";
import { CatanProgressArt } from "./catan-progress";
import { catanProgressNames } from "./catan-progress-names";
import { cityTracks } from "./catan-city-state";
const names = ["木材", "砖块", "羊毛", "粮食", "矿石"];
const devs = ["骑士", "道路建设", "丰收", "垄断", "胜利点"];
const sum = (a: number[]) => a.reduce((n, v) => n + v, 0);
export function CatanOptionPicker({
  value = {},
  onChange,
  disabled = false,
  seafarers = false,
  citiesKnights = false,
  harbors = false,
  friendlyRobber = false,
  fiveSixAvailable = true,
  fishing = false,
  helpersAvailable = true,
  explorer = false,
}: {
  value?: CatanOptions;
  onChange: (v: CatanOptions) => void;
  disabled?: boolean;
  seafarers?: boolean;
  citiesKnights?: boolean;
  harbors?: boolean;
  friendlyRobber?: boolean;
  fiveSixAvailable?: boolean;
  fishing?: boolean;
  helpersAvailable?: boolean;
  explorer?: boolean;
}) {
  return (
    <fieldset className="catan-helper-options" disabled={disabled}>
      <legend>卡坦岛扩展</legend>
      {fiveSixAvailable && (
        <label>
          <input
            type="checkbox"
            checked={!!value.fiveSix}
            onChange={(e) => onChange({ ...value, fiveSix: e.target.checked })}
          />{" "}
          五至六人扩充 · 新版配对回合
        </label>
      )}
      {helpersAvailable && (
        <label>
          <input
            type="checkbox"
            checked={!!value.helpers}
            onChange={(e) =>
              onChange({
                ...value,
                helpers: e.target.checked,
                allHelpers: false,
              })
            }
          />{" "}
          Helpers · 十二位助手
        </label>
      )}
      {helpersAvailable && value.helpers && (
        <label>
          <input
            type="checkbox"
            checked={!!value.allHelpers}
            onChange={(e) =>
              onChange({ ...value, allHelpers: e.target.checked })
            }
          />{" "}
          展示全部备用助手
        </label>
      )}
      {value.fiveSix && (
        <small>
          {seafarers
            ? "使用所选航海家剧本的五至六人地图。"
            : "30块陆地，5–6人。"}
          ①号正常行动后，左侧第三位②号玩家进行一次不掷骰、不自由交易的行动。
        </small>
      )}
      {helpersAvailable && value.helpers && (
        <small>
          使用后可翻面保留一次，或与展示区交换；新获得的助手需等下一回合。
        </small>
      )}
      {explorer && value.helpers && (
        <small>
          本站探索者适配：造船、人员建设与补给替代发展卡和强盗能力；支持渔夫、城市与骑士，人数按探索者规则处理。
        </small>
      )}
      {!explorer && citiesKnights && value.helpers && (
        <small>
          本站骑士助手：资源能力不处理商品；迪亚拉择选进步牌，卡拉更换进步牌，格雷戈尔归还实体骑士建造。
          {fishing && "鱼不取消希尔达与引水渠补偿；7鱼抽牌不能叠加助手择牌。"}
        </small>
      )}
      {value.helpers && harbors && (
        <small>助手建造的港口建筑同样计入港口点，奖励和获胜门槛不变。</small>
      )}
      {value.helpers && friendlyRobber && (
        <small>
          友善保护只限制强盗与海盗，不限制助手交易或取牌。
          {fishing
            ? "本站渔夫规则：迪古尔返回沙漠；无沙漠时移到场外。"
            : "迪古尔仍按助手效果返回沙漠；无沙漠时不可使用，不自动改为场外驱逐。"}
        </small>
      )}
    </fieldset>
  );
}
function ResourceSelect({
  value,
  onChange,
  label,
}: {
  value: number;
  onChange: (n: number) => void;
  label: string;
}) {
  return (
    <label>
      {label}
      <select value={value} onChange={(e) => onChange(Number(e.target.value))}>
        {names.map((n, i) => (
          <option key={n} value={i}>
            {n}
          </option>
        ))}
      </select>
    </label>
  );
}
function Counts({
  values,
  limits,
  onChange,
  label,
}: {
  values: number[];
  limits: number[];
  onChange: (v: number[]) => void;
  label: string;
}) {
  return (
    <fieldset className="helper-counts">
      <legend>
        {label} · {sum(values)} 张
      </legend>
      {names.map((n, i) => (
        <label key={n}>
          {n}
          <input
            aria-label={`${label}${n}`}
            type="number"
            min={0}
            max={limits[i]}
            value={values[i]}
            onChange={(e) =>
              onChange(
                values.map((v, j) =>
                  i === j
                    ? Math.max(
                        0,
                        Math.min(limits[i], Number(e.target.value) || 0),
                      )
                    : v,
                ),
              )
            }
          />
        </label>
      ))}
    </fieldset>
  );
}
export function CatanHelpers({
  room,
  act,
  busy,
  assets,
  onBuild,
  onMove,
  onDesert,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
  onBuild: (kind: string, payment: number[], knight?: number) => void;
  onMove: () => void;
  onDesert: (color: number) => void;
}) {
  const g = room.game!.catan!,
    p = g.players[room.you],
    rules = g.helperRules || [],
    helper = p?.helper,
    rule = rules.find((h) => h.id === helper?.id),
    pending = g.helperPending;
  const [open, setOpen] = useState(false),
    [collapsed, setCollapsed] = useState(false),
    [color, setColor] = useState(0),
    [target, setTarget] = useState(-1),
    [target2, setTarget2] = useState(-1),
    [offer, setOffer] = useState(0),
    [offer2, setOffer2] = useState(0),
    [card, setCard] = useState(-1),
    [progressTrack, setProgressTrack] = useState(0),
    [knight, setKnight] = useState(-1),
    [payment, setPayment] = useState([0, 0, 0, 0, 0]),
    [take, setTake] = useState([0, 0, 0, 0, 0]);
  useEffect(() => {
    setCollapsed(false);
    setOpen(false);
  }, [room.id, helper?.id, pending?.kind, pending?.player]);
  if (!g.options?.helpers) return null;
  const city = g.citiesKnights;
  const cityHelpers = city?.helpers?.rules === "wire-board-helpers-knights-v1";
  const progressCards = city?.players[room.you]?.progress || [];
  const fishingHelpers = g.fishing?.helpers === "wire-board-fishing-helpers-v1";
  const hand = p?.resources || [0, 0, 0, 0, 0],
    active =
      room.status === "playing" &&
      !room.game!.finished &&
      !room.spectating &&
      !room.seats[room.you]?.autoPlay &&
      !p?.eliminated;
  const response = active && pending?.player === room.you;
  const canUse =
    active &&
    room.game!.turn === room.you &&
    p?.helperReady &&
    (room.game!.phase === "catan_turn" ||
      (room.game!.phase === "catan_roll" && helper?.id === 10));
  const send = async (a: Record<string, unknown>) => {
    await act(a);
    setOpen(false);
  };
  const choice = (a: Record<string, unknown>) =>
    void send({ type: "catan_helper_choice", ...a });
  const action = (a: Record<string, unknown> = {}) =>
    void send({ type: "catan_helper", ...a });
  const opponents = g.players
    .map((v, i) => ({ v, i }))
    .filter(({ v, i }) => i !== room.you && !v.eliminated);
  const targetPicker = (
    v: number,
    change: (n: number) => void,
    optional = false,
  ) => (
    <label>
      {optional ? "第二位对手（可选）" : "选择对手"}
      <select value={v} onChange={(e) => change(Number(e.target.value))}>
        <option value={-1}>{optional ? "不选择" : "请选择"}</option>
        {opponents.map(({ i }) => (
          <option key={i} value={i}>
            {room.seats[i].name}
          </option>
        ))}
      </select>
    </label>
  );
  const badge = (id: number) => {
    const r = rules.find((h) => h.id === id);
    return (
      <span className="helper-card-copy">
        {assets && (
          <img
            className="helper-portrait"
            src={`${assets}/catan/helpers/helper-${id}.webp`}
            alt=""
          />
        )}
        <b>{r?.name}</b>
        <span>{r?.title}</span>
        <small>{r?.description}</small>
      </span>
    );
  };
  return (
    <section className="catan-helpers">
      <header>
        <strong>Helpers 助手</strong>
        <button type="button" className="subtle" onClick={() => setOpen(!open)}>
          {open ? "收起" : "查看 / 使用"}
        </button>
      </header>
      {g.two?.afterHelper && (
        <p>
          完成助手翻面或交换后，还需为中立势力建造
          {g.two.afterHelper === "road" ? "道路" : "村庄"}。
        </p>
      )}
      {rule ? (
        <p className="helper-owned">
          {assets && (
            <img
              className="helper-portrait"
              src={`${assets}/catan/helpers/helper-${rule.id}.webp`}
              alt=""
            />
          )}
          <b>
            {rule.name} · {rule.title}
          </b>{" "}
          <span>{helper?.moon ? "☾ 月面" : "☀ 太阳面"}</span>
          <small>{p.helperReady ? "本回合尚未使用" : "下一回合可用"}</small>
        </p>
      ) : (
        <p>完成起始建设后领取助手</p>
      )}
      {open && (
        <div className="helper-details">
          <p>{rule?.description}</p>
          {canUse && rule && (
            <fieldset disabled={busy} className="helper-use">
              {rule.id === 1 && (
                <>
                  <ResourceSelect
                    label="索取"
                    value={color}
                    onChange={setColor}
                  />
                  {targetPicker(target, setTarget)}
                  <ResourceSelect
                    label="给第一位"
                    value={offer}
                    onChange={setOffer}
                  />
                  {opponents.length > 1 &&
                    targetPicker(target2, setTarget2, true)}
                  <ResourceSelect
                    label="给第二位"
                    value={offer2}
                    onChange={setOffer2}
                  />
                  <button
                    disabled={target < 0 || target === target2}
                    onClick={() =>
                      action({
                        color,
                        targets: target2 < 0 ? [target] : [target, target2],
                        cards: target2 < 0 ? [offer] : [offer, offer2],
                      })
                    }
                  >
                    确认强制交易
                  </button>
                </>
              )}
              {[2, 6].includes(rule.id) && (
                <>
                  <Counts
                    label="实际支付（最多替换一张）"
                    values={payment}
                    limits={hand}
                    onChange={setPayment}
                  />
                  {rule.id === 6 && cityHelpers && (
                    <label>
                      进步牌堆
                      <select
                        value={progressTrack}
                        onChange={(e) =>
                          setProgressTrack(Number(e.target.value))
                        }
                      >
                        {cityTracks.map((name, i) => (
                          <option
                            key={name}
                            value={i}
                            disabled={!city?.progressRemaining[i]}
                          >
                            {name} · {city?.progressRemaining[i] || 0} 张
                          </option>
                        ))}
                      </select>
                    </label>
                  )}
                  <button
                    disabled={
                      sum(payment) !== (rule.id === 2 ? 2 : 3) ||
                      (rule.id === 2
                        ? g.legal.roads.length === 0
                        : cityHelpers
                          ? !city?.progressRemaining[progressTrack]
                          : g.devRemaining === 0)
                    }
                    onClick={() => {
                      if (rule.id === 6 && cityHelpers)
                        action({
                          choice: "progress_buy",
                          color: progressTrack,
                          tokens: payment,
                        });
                      else onBuild(rule.id === 2 ? "road" : "buy_dev", payment);
                      setOpen(false);
                    }}
                  >
                    确认支付，
                    {rule.id === 2
                      ? "在地图选修路位置"
                      : cityHelpers
                        ? "查看进步牌"
                        : "购买发展卡"}
                  </button>
                </>
              )}
              {rule.id === 4 && (
                <button
                  disabled={!Object.keys(g.helperRoadMoves || {}).length}
                  onClick={() => {
                    onMove();
                    setOpen(false);
                  }}
                >
                  在地图选择末端道路和新位置
                </button>
              )}
              {rule.id === 7 && (
                <>
                  {targetPicker(target, setTarget)}
                  <button
                    disabled={
                      target < 0 ||
                      g.players[target].publicScore <= p.publicScore ||
                      !g.players[target].resourceCount
                    }
                    onClick={() => action({ target })}
                  >
                    查看领先对手手牌
                  </button>
                </>
              )}
              {rule.id === 8 && (
                <>
                  {cityHelpers && (
                    <label>
                      归还的己方骑士
                      <select
                        value={knight}
                        onChange={(e) => setKnight(Number(e.target.value))}
                      >
                        <option value={-1}>请选择骑士</option>
                        {city?.knights
                          .filter((n) => n.owner === room.you)
                          .map((n) => (
                            <option key={n.vertex} value={n.vertex}>
                              交点 #{n.vertex + 1} · {n.strength} 级 ·{" "}
                              {n.active ? "已激活" : "未激活"}
                            </option>
                          ))}
                      </select>
                    </label>
                  )}
                  {(["settlement", "city"] as const).map((kind, i) => {
                    const cost = i ? [0, 0, 0, 1, 2] : [1, 1, 0, 0, 0];
                    return (
                      <button
                        key={kind}
                        disabled={
                          (cityHelpers ? knight < 0 : !p.knights) ||
                          cost.some((n, c) => n > hand[c]) ||
                          !(
                            cityHelpers
                              ? g.helperKnightBuilds?.[knight]?.[
                                  i ? "cities" : "settlements"
                                ] || []
                              : i
                                ? g.legal.cities
                                : g.legal.settlements
                          ).length
                        }
                        onClick={() => {
                          onBuild(kind, cost, cityHelpers ? knight : undefined);
                          setOpen(false);
                        }}
                      >
                        移除骑士，
                        {i
                          ? "矿石×2＋粮食×1 升级城市"
                          : "木材×1＋砖块×1 建村庄"}
                      </button>
                    );
                  })}
                </>
              )}
              {rule.id === 9 && (
                <>
                  <ResourceSelect
                    label="给出的资源（二比一）"
                    value={color}
                    onChange={setColor}
                  />
                  <Counts
                    label="从银行换取"
                    values={take}
                    limits={g.bank.map((n, i) => (i === color ? 0 : n))}
                    onChange={setTake}
                  />
                  <button
                    disabled={
                      !sum(take) ||
                      take[color] > 0 ||
                      sum(take) * 2 > hand[color]
                    }
                    onClick={() =>
                      action({
                        give: hand
                          .slice(0, 5)
                          .map((_, i) => (i === color ? sum(take) * 2 : 0)),
                        take,
                      })
                    }
                  >
                    支付 {sum(take) * 2} 张{names[color]}并交易
                  </button>
                </>
              )}
              {rule.id === 10 && (
                <>
                  {(g.tiles[g.robber]?.resource === 7 ||
                    (fishingHelpers && g.tiles[g.robber]?.resource === 9)) && (
                    <ResourceSelect
                      label="领取资源"
                      value={color}
                      onChange={setColor}
                    />
                  )}
                  <button
                    disabled={
                      g.robber < 0 ||
                      g.tiles[g.robber]?.resource === 5 ||
                      (!fishingHelpers &&
                        !g.tiles.some(
                          (t) =>
                            t.resource === 5 &&
                            (!g.seafarers?.cloth ||
                              g.seafarers.cloth.homeTiles.includes(t.id)),
                        ))
                    }
                    onClick={() => {
                      if (
                        g.tiles.filter(
                          (t) =>
                            t.resource === 5 &&
                            (!g.seafarers?.cloth ||
                              g.seafarers.cloth.homeTiles.includes(t.id)),
                        ).length > 1
                      ) {
                        onDesert(color);
                        setOpen(false);
                      } else action({ color });
                    }}
                  >
                    {fishingHelpers &&
                    !g.tiles.some(
                      (t) =>
                        t.resource === 5 &&
                        (!g.seafarers?.cloth ||
                          g.seafarers.cloth.homeTiles.includes(t.id)),
                    )
                      ? "将强盗驱逐到场外"
                      : "选择沙漠并驱逐强盗"}
                  </button>
                </>
              )}
              {rule.id === 11 && (
                <>
                  {(fishingHelpers ? [5, 7, 9] : [5, 7]).includes(
                    g.tiles[g.robber]?.resource,
                  ) && (
                    <ResourceSelect
                      label="领取资源"
                      value={color}
                      onChange={setColor}
                    />
                  )}
                  <button
                    disabled={g.robber < 0}
                    onClick={() => action({ color })}
                  >
                    领取强盗所在地资源
                  </button>
                </>
              )}
              {rule.id === 12 && (
                <>
                  <label>
                    换回牌堆底的{cityHelpers ? "进步牌" : "发展卡"}
                    <select
                      value={card}
                      onChange={(e) => setCard(Number(e.target.value))}
                    >
                      <option value={-1}>请选择卡牌</option>
                      {cityHelpers
                        ? [...new Set(progressCards)].map((id) => (
                            <option key={id} value={id}>
                              {catanProgressNames[id]}
                            </option>
                          ))
                        : devs.map((n, i) => (
                            <option key={n} value={i} disabled={!p.dev?.[i]}>
                              {n} · {p.dev?.[i] || 0} 张
                            </option>
                          ))}
                    </select>
                  </label>
                  <button
                    disabled={
                      cityHelpers
                        ? !progressCards.includes(card)
                        : !p.dev?.[card]
                    }
                    onClick={() => action({ card })}
                  >
                    确认交换一张{cityHelpers ? "进步牌" : "发展卡"}
                  </button>
                </>
              )}
              {[3, 5].includes(rule.id) && (
                <small>
                  {g.eventDeck
                    ? "事件效果结束并按牌面点数生产后，符合条件时自动提示你响应。"
                    : "掷骰结算符合条件时自动提示你响应。"}
                  {rule.id === 3 &&
                    g.seafarers &&
                    "所有玩家选完金矿资源后才判断补偿；金矿收入算生产，事件奖励不算。"}
                </small>
              )}
            </fieldset>
          )}
          <details>
            <summary>
              公共助手展示区 · {g.helperDisplay?.length || 0} 位
            </summary>
            <div className="helper-card-grid">
              {g.helperDisplay?.map((id) => (
                <article key={id}>{badge(id)}</article>
              ))}
            </div>
          </details>
          <details>
            <summary>所有玩家的助手</summary>
            {g.players.map((v, i) => (
              <p key={i}>
                {room.seats[i].name}：
                {rules.find((r) => r.id === v.helper?.id)?.name || "待领取"}{" "}
                {v.helper && (v.helper.moon ? "☾" : "☀")}
              </p>
            ))}
          </details>
        </div>
      )}
      {pending && (
        <small>
          {room.seats[pending.player]?.name} 正在
          {pending.kind === "exchange" ? "翻面或交换助手" : "使用助手"}
        </small>
      )}
      {response && (
        <section
          className={`helper-response ${collapsed ? "collapsed" : ""}`}
          role="dialog"
          aria-label="助手选择"
          aria-modal="false"
        >
          <header>
            <strong>{rule?.name} · 助手选择</strong>
            <button className="subtle" onClick={() => setCollapsed(!collapsed)}>
              {collapsed ? "展开选择" : "收起看地图"}
            </button>
          </header>
          {!collapsed && (
            <div className="helper-response-body">
              {pending.kind === "exchange" && (
                <>
                  <p>
                    {helper?.moon
                      ? "月面已使用，请交换另一位助手。"
                      : "翻至月面保留一次，或交换另一位助手。"}
                  </p>
                  {!helper?.moon && (
                    <button
                      disabled={busy}
                      onClick={() => choice({ choice: "flip" })}
                    >
                      保留并翻至月面
                    </button>
                  )}
                  <div className="helper-card-grid">
                    {g.helperDisplay?.map((id) => (
                      <button
                        key={id}
                        disabled={busy}
                        onClick={() => choice({ choice: "exchange", card: id })}
                      >
                        {badge(id)}
                      </button>
                    ))}
                  </div>
                </>
              )}
              {["resource", "leader"].includes(pending.kind) && (
                <>
                  <p>
                    {pending.kind === "leader"
                      ? "只有你能看到对方资源，选择拿走一张。"
                      : "选择领取一张资源。"}
                  </p>
                  <div className="helper-resources">
                    {(pending.resources || g.bank).slice(0, 5).map((n, i) => (
                      <button
                        key={i}
                        disabled={busy || n === 0}
                        onClick={() => choice({ color: i })}
                      >
                        {names[i]} <b>{n}</b>
                      </button>
                    ))}
                  </div>
                  {pending.optional && (
                    <button
                      disabled={busy}
                      onClick={() => choice({ choice: "skip" })}
                    >
                      本次不使用
                    </button>
                  )}
                </>
              )}
              {pending.kind === "progress" && (
                <>
                  <p>只有你能看到这些进步牌。选择一张，其余洗回同色牌堆。</p>
                  <div className="helper-dev-options">
                    {pending.cards?.map((id, i) => (
                      <button
                        key={i}
                        disabled={busy}
                        onClick={() => choice({ card: id })}
                      >
                        <CatanProgressArt card={id} assets={assets} />
                        <b>{catanProgressNames[id]}</b>
                      </button>
                    ))}
                  </div>
                </>
              )}
              {pending.kind === "development" && (
                <>
                  <p>只有你能看到这些牌。选择一张，其余洗回牌堆。</p>
                  <div className="helper-dev-options">
                    {pending.cards?.map((id, i) => (
                      <button
                        key={i}
                        disabled={busy}
                        onClick={() => choice({ card: id })}
                      >
                        {assets && (
                          <img
                            src={`${assets}/catan/dev-${id}-v1.webp`}
                            alt=""
                          />
                        )}
                        <b>{devs[id]}</b>
                      </button>
                    ))}
                  </div>
                </>
              )}
            </div>
          )}
        </section>
      )}
    </section>
  );
}
