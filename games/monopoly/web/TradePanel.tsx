import { useEffect, useState } from "react";
import type { GameRoom } from "../../../web/src/game-api/module";
import type { Bundle, Send, View } from "./types";
import { GameButton, playerName, seconds } from "./ui";

const blank = (): Bundle => ({
  cash: 0,
  properties: [],
  jailCards: [],
  redeem: [],
});
function toggle<T>(values: T[] | null | undefined, value: T) {
  return values?.includes(value)
    ? values.filter((v) => v !== value)
    : [...(values || []), value];
}
function bankFee(bundle: Bundle, data: View) {
  return (bundle.properties || []).reduce(
    (sum, id) =>
      data.properties[id].mortgaged
        ? sum +
          Math.ceil(data.board[id].price / 20) +
          (bundle.redeem?.includes(id) ? data.board[id].price / 2 : 0)
        : sum,
    0,
  );
}

function BundleEditor({
  side,
  owner,
  bundle,
  update,
  data,
  disabled,
}: {
  side: "give" | "want";
  owner: number;
  bundle: Bundle;
  update: (b: Bundle) => void;
  data: View;
  disabled: boolean;
}) {
  const title = side === "give" ? "我付出" : "我获得";
  const improved = (id: number) =>
    data.properties.some(
      (p, i) =>
        p.buildings > 0 &&
        (i === id ||
          (data.board[id].kind === "street" &&
            data.board[i].group === data.board[id].group)),
    );
  const properties = data.board.filter(
    (s) => data.properties[s.id].owner === owner,
  );
  return (
    <fieldset
      data-trade-side={side}
      className="mono-bundle"
      disabled={disabled}
    >
      <legend>{title}</legend>
      <label className="mono-inline-field">
        现金
        <input
          type="number"
          min="0"
          step="1"
          aria-label={`${title}的现金`}
          value={bundle.cash || 0}
          onChange={(e) =>
            update({ ...bundle, cash: Math.max(0, Number(e.target.value)) })
          }
        />
      </label>
      <div className="mono-property-picks">
        {properties.map((space) => (
          <div className="mono-pick" key={space.id}>
            <label>
              <input
                type="checkbox"
                data-property={space.id}
                checked={!!bundle.properties?.includes(space.id)}
                disabled={disabled || improved(space.id)}
                onChange={() =>
                  update({
                    ...bundle,
                    properties: toggle(bundle.properties, space.id),
                    redeem: (bundle.redeem || []).filter(
                      (id) => id !== space.id,
                    ),
                  })
                }
              />
              <span>
                <i style={{ background: space.color }} />
                {space.name}
                {data.properties[space.id].mortgaged ? "（已抵押）" : ""}
                {improved(space.id) ? "（需先卖房）" : ""}
              </span>
            </label>
            {data.properties[space.id].mortgaged &&
              bundle.properties?.includes(space.id) && (
                <label className="mono-redeem-pick">
                  <input
                    type="checkbox"
                    checked={!!bundle.redeem?.includes(space.id)}
                    onChange={() =>
                      update({
                        ...bundle,
                        redeem: toggle(bundle.redeem, space.id),
                      })
                    }
                  />
                  接收时立即赎回
                </label>
              )}
          </div>
        ))}
      </div>
      {!properties.length && <p className="mono-help">没有持有的地产</p>}
      {data.players[owner]?.jailCards?.map((card) => (
        <label className="mono-pick" key={card}>
          <input
            type="checkbox"
            checked={!!bundle.jailCards?.includes(card)}
            onChange={() =>
              update({ ...bundle, jailCards: toggle(bundle.jailCards, card) })
            }
          />
          {card === "chance" ? "机会" : "公共基金"}免狱卡
        </label>
      ))}
    </fieldset>
  );
}

function Description({ bundle, data }: { bundle: Bundle; data: View }) {
  const words = [
    ...((bundle.cash || 0) > 0 ? [`现金 ${bundle.cash}`] : []),
    ...(bundle.properties || []).map(
      (id) =>
        data.board[id].name +
        (bundle.redeem?.includes(id)
          ? "（立即赎回）"
          : data.properties[id].mortgaged
            ? "（保留抵押）"
            : ""),
    ),
    ...(bundle.jailCards || []).map(
      (card) => `${card === "chance" ? "机会" : "公共基金"}免狱卡`,
    ),
  ];
  return <span>{words.join("、") || "无"}</span>;
}

export function TradePanel({
  data,
  room,
  busy,
  send,
  now,
}: {
  data: View;
  room: GameRoom;
  busy: boolean;
  send: Send;
  now: number;
}) {
  const choices = data.players
    .map((p, i) => ({ p, i }))
    .filter(({ p, i }) => !p.bankrupt && i !== room.you);
  const [target, setTarget] = useState(choices[0]?.i ?? 0);
  const [give, setGive] = useState<Bundle>(blank),
    [want, setWant] = useState<Bundle>(blank);
  const ownership = data.properties
    .map((p) => `${p.owner}:${p.mortgaged}:${p.buildings}`)
    .join();
  useEffect(() => {
    const clean = (b: Bundle, owner: number): Bundle => {
      const properties = (b.properties || []).filter(
        (id) =>
          data.properties[id]?.owner === owner &&
          !data.properties.some(
            (p, i) =>
              p.buildings > 0 &&
              (i === id ||
                (data.board[id].kind === "street" &&
                  data.board[i].group === data.board[id].group)),
          ),
      );
      return {
        ...b,
        properties,
        redeem: (b.redeem || []).filter(
          (id) => properties.includes(id) && data.properties[id].mortgaged,
        ),
        jailCards: (b.jailCards || []).filter((card) =>
          data.players[owner]?.jailCards.includes(card),
        ),
      };
    };
    setGive((b) => clean(b, room.you));
    setWant((b) => clean(b, target));
  }, [
    ownership,
    data.players.map((p) => p.jailCards.join()).join("|"),
    room.you,
    target,
  ]);
  useEffect(() => {
    if (!choices.some((c) => c.i === target)) {
      setTarget(choices[0]?.i ?? 0);
      setWant(blank());
    }
  }, [data.players.map((p) => p.bankrupt).join(), target]);
  const mayCompose =
    !room.spectating &&
    room.you >= 0 &&
    !data.players[room.you]?.bankrupt &&
    data.phase !== "finished";
  const meaningful =
    (give.properties?.length || 0) +
      (want.properties?.length || 0) +
      (give.jailCards?.length || 0) +
      (want.jailCards?.length || 0) >
    0;
  return (
    <div className="mono-trade-panel">
      <h3>玩家交易</h3>
      <p className="mono-help">
        报价仅双方可见。接受时一次性交割资产、现金与抵押费用。
      </p>
      {data.offers.map((offer) => (
        <article data-mono-offer key={offer.id} className="mono-offer">
          <header>
            <strong>
              {playerName(room, offer.from)} → {playerName(room, offer.to)}
            </strong>
            <span>{seconds(offer.expires, now)} 秒</span>
          </header>
          <p>
            <b>付出：</b>
            <Description bundle={offer.give} data={data} />
          </p>
          <p>
            <b>获得：</b>
            <Description bundle={offer.want} data={data} />
          </p>
          <p className="mono-help">
            银行费用：{playerName(room, offer.from)} {offer.fromFee}；
            {playerName(room, offer.to)} {offer.toFee}
          </p>
          {!offer.valid && (
            <p role="status" className="mono-warning">
              {offer.reason}
            </p>
          )}
          <div className="mono-button-row">
            {offer.from === room.you ? (
              <GameButton
                disabled={busy || !data.controls.active}
                onClick={() => void send("trade_cancel", { tradeId: offer.id })}
              >
                撤回报价
              </GameButton>
            ) : (
              <>
                <GameButton
                  primary
                  disabled={
                    busy ||
                    !offer.valid ||
                    !data.controls.trade ||
                    seconds(offer.expires, now) === 0
                  }
                  onClick={() =>
                    void send("trade_accept", { tradeId: offer.id })
                  }
                >
                  接受交易
                </GameButton>
                <GameButton
                  disabled={busy || !data.controls.active}
                  onClick={() =>
                    void send("trade_reject", { tradeId: offer.id })
                  }
                >
                  拒绝
                </GameButton>
              </>
            )}
          </div>
        </article>
      ))}
      {mayCompose ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void send("trade_offer", { offer: { target, give, want } });
          }}
        >
          <label>
            交易对象
            <select
              aria-label="交易对象"
              value={target}
              onChange={(e) => {
                setTarget(Number(e.target.value));
                setWant(blank());
              }}
              disabled={busy || !data.controls.trade}
            >
              {choices.map(({ i }) => (
                <option key={i} value={i}>
                  {playerName(room, i)}
                </option>
              ))}
            </select>
          </label>
          <div className="mono-bundles">
            <BundleEditor
              side="give"
              owner={room.you}
              bundle={give}
              update={setGive}
              data={data}
              disabled={busy || !data.controls.trade}
            />
            <BundleEditor
              side="want"
              owner={target}
              bundle={want}
              update={setWant}
              data={data}
              disabled={busy || !data.controls.trade}
            />
          </div>
          <p className="mono-help">
            你另付银行 {bankFee(want, data)}；对方另付银行 {bankFee(give, data)}
            。不立即赎回时，仍须支付 10% 受让利息。
          </p>
          <button
            className="mono-button primary mono-game-action"
            type="submit"
            disabled={busy || !data.controls.trade || !meaningful}
          >
            发送报价
          </button>
          {data.offers.some((o) => o.from === room.you) && (
            <p className="mono-help">发送新报价会替换你之前发出的报价。</p>
          )}
        </form>
      ) : (
        <p>可在这里查看与你有关的交易。观众无法查看玩家的私人报价。</p>
      )}
    </div>
  );
}
