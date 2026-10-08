import { useEffect, useState } from "react";
import type { Act, Room } from "./types";
import { ResourcePicker } from "./catan-resources";
import {
  explorerChoices,
  explorerCanRespond,
  explorerActionKey,
  explorerResources as names,
} from "./catan-explorer-state";
import type { ExplorerAction } from "./catan-explorer-state";
import "./catan-helpers.css";

export const isExplorerHelperAction = (a: ExplorerAction) =>
  a.skill === "helper" ||
  a.type === "catan_helper" ||
  a.type === "catan_helper_choice";

export function ExplorerHelpers({
  room,
  act,
  busy,
  assets,
  onMap,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
  onMap: () => void;
}) {
  const g = room.game!.catan!,
    x = g.explorer!,
    player = g.players[room.you],
    helper = player?.helper;
  const rules = g.helperRules || [],
    rule = rules.find((r) => r.id === helper?.id),
    pending = g.helperPending;
  const actions = explorerChoices(room).filter(isExplorerHelperAction);
  const [open, setOpen] = useState(false),
    [collapsed, setCollapsed] = useState(false);
  const [color, setColor] = useState(0),
    [offer, setOffer] = useState(0),
    [offer2, setOffer2] = useState(0);
  const [target, setTarget] = useState(-1),
    [target2, setTarget2] = useState(-1),
    [take, setTake] = useState([0, 0, 0, 0, 0]);
  useEffect(() => {
    setCollapsed(false);
    setOpen(false);
    setTarget(-1);
    setTarget2(-1);
    setTake([0, 0, 0, 0, 0]);
  }, [room.id, helper?.id, x.sequence, pending?.kind, pending?.player]);
  if (!x.helperRules) return null;
  const hand = (player?.resources || [0, 0, 0, 0, 0]).slice(0, 5);
  const can = explorerCanRespond(room),
    response = can && pending?.player === room.you;
  const active = can && !pending && actions.length > 0;
  const count = take.reduce((n, v) => n + v, 0);
  const send = (a: Record<string, unknown>) => {
    if (!busy && can) void act({ ...a, prompt: x.sequence });
  };
  const use = (a: Record<string, unknown>) =>
    send({ type: "catan_helper", ...a });
  const badge = (id: number) => {
    const r = rules.find((r) => r.id === id);
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
  const resource = (label: string, value: number, set: (n: number) => void) => (
    <label>
      {label}
      <select value={value} onChange={(e) => set(Number(e.target.value))}>
        {names.map((n, i) => (
          <option key={i} value={i}>
            {n}
          </option>
        ))}
      </select>
    </label>
  );
  const opponent = (label: string, value: number, set: (n: number) => void) => (
    <label>
      {label}
      <select value={value} onChange={(e) => set(Number(e.target.value))}>
        <option value={-1}>不选择</option>
        {g.players.map((p, i) =>
          i !== room.you && !p.eliminated ? (
            <option key={i} value={i}>
              {room.seats[i]?.name}
            </option>
          ) : null,
        )}
      </select>
    </label>
  );
  const label = (a: ExplorerAction) => {
    if (a.type === "catan_helper_choice") {
      if (a.choice === "flip") return "保留并翻至月面";
      if (a.choice === "skip")
        return pending?.kind === "leader"
          ? "没有普通资源，完成使用"
          : "本次不使用";
      return `领取${names[a.color ?? 0]}`;
    }
    if (helper?.id === 7) return `查看${room.seats[a.target!]?.name}的普通资源`;
    if (helper?.id === 10) return "驱逐海盗并领取 1 金币";
    if (helper?.id === 11) return `领取${names[a.color!]}`;
    if (helper?.id === 12) return `${names[a.color!]}换${names[a.target!]}`;
    return "使用助手";
  };
  const offerOK =
    target >= 0 &&
    target !== target2 &&
    offer !== color &&
    (target2 < 0 || offer2 !== color) &&
    hand.every(
      (n, i) => n >= Number(offer === i) + Number(target2 >= 0 && offer2 === i),
    );
  return (
    <section className="catan-helpers explorer-helpers">
      <header>
        <strong>Helpers · 探索者助手</strong>
        <button className="subtle" onClick={() => setOpen(!open)}>
          {open ? "收起" : "查看 / 使用"}
        </button>
      </header>
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
          </b>
          <span>{helper?.moon ? "☾ 月面" : "☀ 太阳面"}</span>
          <small>
            {x.setup
              ? "开局完成后可用"
              : player?.helperReady
                ? "本回合尚未使用"
                : "下一回合可用"}
          </small>
        </p>
      ) : (
        <p>以各玩家的助手为准。</p>
      )}
      {open && (
        <div className="helper-details">
          <p>{rule?.description}</p>
          <small>
            本站探索者适配。太阳面使用后翻面或交换；月面使用后必须交换。
          </small>
          {active && (
            <fieldset disabled={busy} className="helper-use">
              {helper?.id === 1 ? (
                <>
                  {resource("索取资源", color, setColor)}
                  {opponent("第一位对手", target, setTarget)}
                  {resource("给第一位", offer, setOffer)}
                  {opponent("第二位对手（可选）", target2, setTarget2)}
                  {resource("给第二位", offer2, setOffer2)}
                  <button
                    disabled={!offerOK}
                    onClick={() =>
                      use({
                        color,
                        targets: target2 < 0 ? [target] : [target, target2],
                        cards: target2 < 0 ? [offer] : [offer, offer2],
                      })
                    }
                  >
                    确认强制交易
                  </button>
                </>
              ) : helper?.id === 9 ? (
                <>
                  {resource("给出的资源（二比一）", color, setColor)}
                  <ResourcePicker
                    label="从银行换取"
                    values={take}
                    onChange={setTake}
                    limits={g.bank
                      .slice(0, 5)
                      .map((n, i) => (i === color ? 0 : n))}
                    assets={assets}
                    disabled={busy}
                  />
                  <button
                    disabled={
                      !count ||
                      take[color] > 0 ||
                      count * 2 > hand[color] ||
                      take.some((n, i) => n > g.bank[i])
                    }
                    onClick={() =>
                      use({
                        give: hand.map((_, i) => (i === color ? count * 2 : 0)),
                        take,
                      })
                    }
                  >
                    支付 {count * 2} 张{names[color]}并交易
                  </button>
                </>
              ) : [2, 4, 6, 8].includes(helper?.id || 0) ? (
                <button
                  onClick={() => {
                    setOpen(false);
                    onMap();
                  }}
                >
                  在地图选择
                  {helper?.id === 4 ? "原道路与新位置" : "建设位置与支付方式"}
                </button>
              ) : (
                actions.map((a) => (
                  <button key={explorerActionKey(a)} onClick={() => send(a)}>
                    {label(a)}
                  </button>
                ))
              )}
            </fieldset>
          )}
          {[3, 5].includes(helper?.id || 0) && (
            <p>生产结算符合条件时会自动提示选择资源及翻换助手。</p>
          )}
          <details>
            <summary>公共展示区 · {g.helperDisplay?.length || 0} 位</summary>
            <div className="helper-card-grid">
              {g.helperDisplay?.map((id) => (
                <article key={id}>{badge(id)}</article>
              ))}
            </div>
          </details>
          <details>
            <summary>各玩家的助手</summary>
            {g.players.map((p, i) => (
              <p key={i}>
                {room.seats[i]?.name}：
                {rules.find((r) => r.id === p.helper?.id)?.name || "无"}{" "}
                {p.helper && (p.helper.moon ? "☾" : "☀")}
              </p>
            ))}
          </details>
        </div>
      )}
      {pending && (
        <small>
          {room.seats[pending.player]?.name}正在
          {pending.kind === "exchange" ? "翻面或交换助手" : "选择助手奖励"}。
        </small>
      )}
      {response && (
        <section
          className={`helper-response ${collapsed ? "collapsed" : ""}`}
          role="dialog"
          aria-modal="false"
          aria-label="探索者助手选择"
        >
          <header>
            <strong>{rule?.name} · 助手选择</strong>
            <button className="subtle" onClick={() => setCollapsed(!collapsed)}>
              {collapsed ? "展开选择" : "收起看地图"}
            </button>
          </header>
          {!collapsed && (
            <div className="helper-response-body">
              <p>
                {pending.kind === "exchange"
                  ? helper?.moon
                    ? "月面已使用，请交换助手。"
                    : "翻至月面保留一次，或交换助手。"
                  : pending.kind === "leader"
                    ? "只有你能看到对方的普通资源，选择领取一张。"
                    : "选择领取一张普通资源。"}
              </p>
              {pending.kind === "leader" && pending.resources && (
                <p>
                  {pending.resources
                    .slice(0, 5)
                    .map((n, i) => `${names[i]} ${n}`)
                    .join(" · ")}
                </p>
              )}
              <div className="helper-resources">
                {actions
                  .filter((a) => a.choice !== "exchange")
                  .map((a) => (
                    <button
                      key={explorerActionKey(a)}
                      disabled={busy}
                      onClick={() => send(a)}
                    >
                      {label(a)}
                    </button>
                  ))}
              </div>
              <div className="helper-card-grid">
                {actions
                  .filter((a) => a.choice === "exchange")
                  .map((a) => (
                    <button
                      key={explorerActionKey(a)}
                      disabled={busy}
                      onClick={() => send(a)}
                    >
                      {a.choice === "exchange" ? badge(a.card!) : label(a)}
                    </button>
                  ))}
              </div>
            </div>
          )}
        </section>
      )}
    </section>
  );
}
