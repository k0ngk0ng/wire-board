import { Fragment, useEffect, useState } from "react";
import type { CSSProperties } from "react";
import {
  Building2,
  Flag,
  House,
  Landmark,
  LockKeyhole,
  TrainFront,
  Zap,
} from "lucide-react";
import type { BoardProps } from "../../../web/src/game-api/module";
import { useMapControls } from "../../../web/src/ui/map-controls";
import { Modal } from "../../../web/src/ui/board-host";
import { useMotion } from "./useMotion";
import { squarePoint } from "./motion";
import { Sound } from "./Sound";
import { PropertyDialog } from "./PropertyDialog";
import { TradePanel } from "./TradePanel";
import type { View } from "./types";
import {
  Art,
  colors,
  Dice,
  GameButton,
  playerName,
  seatStyle,
  seconds,
  useServerTime,
} from "./ui";

type Tab = "action" | "assets" | "trade" | "cards";
function grid(id: number) {
  if (id <= 10) return { gridRow: 11, gridColumn: 11 - id };
  if (id <= 20) return { gridRow: 21 - id, gridColumn: 1 };
  if (id <= 30) return { gridRow: 1, gridColumn: id - 19 };
  return { gridRow: id - 29, gridColumn: 11 };
}
function Symbol({ kind }: { kind: string }) {
  if (kind === "station") return <TrainFront />;
  if (kind === "utility") return <Zap />;
  if (kind === "go") return <Flag />;
  if (kind === "jail" || kind === "visiting") return <LockKeyhole />;
  if (kind === "chance") return <b className="mono-question">?</b>;
  if (kind === "chest") return <Landmark />;
  if (kind === "parking") return <b>P</b>;
  return <b>₥</b>;
}

export function Board({
  room,
  data,
  busy: hostBusy,
  act,
  assets,
  sound,
  audio,
  clock,
}: BoardProps<View>) {
  const motion = useMotion(room.id, data, clock.receivedAt);
  const busy = hostBusy || room.status !== "playing" || motion.active;
  const isClosed = room.status === "closed";
  const mapControls = useMapControls({ aspect: 1, minMobileWidth: 0 });
  const zoomed = mapControls.zoom > 1;
  const [tab, setTab] = useState<Tab>("action");
  const [selected, setSelected] = useState<number | null>(null);
  const [assetOwner, setAssetOwner] = useState(
    room.you >= 0 ? room.you : data.turn,
  );
  const [cardDeck, setCardDeck] = useState("chance");
  const [bid, setBid] = useState(10),
    [bidTarget, setBidTarget] = useState(0);
  const [confirmBankrupt, setConfirmBankrupt] = useState(false);
  const now = useServerTime(clock);
  const c = data.controls,
    me = data.players[room.you];
  const name = (p: number) => playerName(room, p);
  const send = (type: string, extra: Record<string, unknown> = {}) =>
    act({ type, window: data.window, ...extra });
  const inspect = (id: number) => setSelected(id);
  useEffect(() => setBid(c.minBid || 10), [data.window]);
  useEffect(
    () => setBid((value) => Math.max(value, c.minBid || 10)),
    [c.minBid],
  );
  useEffect(() => {
    if (!c.bidTargets.includes(bidTarget)) setBidTarget(c.bidTargets[0] ?? 0);
  }, [c.bidTargets.join(), bidTarget]);
  useEffect(() => {
    if (me?.bankrupt) setConfirmBankrupt(false);
  }, [me?.bankrupt]);
  const incoming = data.offers.filter((o) => o.to === room.you).length;
  const turnName = name(data.turn);
  const isFinished = !!room.game?.finished;
  const auction = data.auction;
  const phaseText = isClosed
    ? "牌桌已结束"
    : isFinished
      ? "本局结束"
      : data.phase === "auction"
        ? "公开竞价"
        : data.phase === "debt"
          ? "筹资还款"
          : data.phase === "mortgage_transfer"
            ? "接收抵押地产"
            : data.phase === "purchase"
              ? "选择购买或拍卖"
              : data.phase === "end"
                ? "回合结束前"
                : "准备掷骰";

  return (
    <section
      className="monopoly-board"
      data-monopoly-board
      data-mono-motion={motion.stage}
    >
      <Sound
        room={room}
        data={data}
        enabled={sound}
        assets={assets}
        audio={audio}
      />
      <div className="mono-workspace">
        <div className="mono-board-region">
          <div className="mono-map-tools">
            <span>棋盘概览</span>
            <button
              type="button"
              aria-pressed={zoomed}
              onClick={() => mapControls.zoomAt(zoomed ? 1 : 2)}
            >
              {zoomed ? "适应屏幕" : "放大棋盘"}
            </button>
          </div>
          <div
            className={`mono-board-scroll ${zoomed ? "zoomed" : ""}`}
            ref={mapControls.viewport}
            {...mapControls.handlers}
          >
            <div style={mapControls.canvasStyle}>
              <div
                data-controller-group="board"
                className="mono-map"
                aria-label="强手棋棋盘"
                style={mapControls.mapStyle}
              >
                <div className="mono-center">
                  <Art
                    base={assets}
                    name="logo"
                    className="mono-logo-image"
                    fallback={<div className="mono-logo">MONOPOLY</div>}
                  />
                  <p className="mono-tagline">强手棋 · 经典地产交易</p>
                  <Dice
                    values={motion.dice}
                    rolling={motion.stage === "roll"}
                  />
                  <p className="mono-center-turn">
                    <span
                      className="mono-turn-dot"
                      style={{ background: colors[data.turn % colors.length] }}
                    />
                    {isFinished ? "本局已结束" : `${turnName}的回合`}
                  </p>
                  {data.lastCard && (
                    <button
                      className="mono-last-card"
                      type="button"
                      onClick={() => {
                        setCardDeck(data.lastCard!.deck);
                        setTab("cards");
                      }}
                    >
                      <small>
                        最近事件 ·{" "}
                        {data.lastCard.deck === "chance" ? "机会" : "公共基金"}
                      </small>
                      <strong>{data.lastCard.title}</strong>
                      <span>{data.lastCard.text}</span>
                    </button>
                  )}
                  <div className="mono-bank-stock">
                    <span>
                      <Art base={assets} name="house" fallback={<House />} />
                      {data.houses}
                    </span>
                    <span>
                      <Art
                        base={assets}
                        name="hotel"
                        fallback={<Building2 />}
                      />
                      {data.hotels}
                    </span>
                    <small>银行剩余建筑</small>
                  </div>
                </div>
                {data.board.map((space) => {
                  const p = data.properties[space.id];
                  return (
                    <button
                      type="button"
                      data-mono-square={space.id}
                      key={space.id}
                      onClick={() => inspect(space.id)}
                      className={`mono-square ${space.price ? "property" : "special"} ${space.id % 10 === 0 ? "corner" : ""} ${p.mortgaged ? "mortgaged" : ""}`}
                      style={
                        {
                          ...grid(space.id),
                          "--group": space.color,
                          "--owner":
                            p.owner >= 0
                              ? colors[p.owner % colors.length]
                              : "transparent",
                        } as CSSProperties
                      }
                      aria-label={`${space.name}${space.price ? `，价格 ${space.price}，${name(p.owner)}持有` : ""}`}
                    >
                      {space.kind === "street" ? (
                        <span className="mono-color-bar" />
                      ) : (
                        <span className="mono-square-symbol">
                          <Art
                            base={assets}
                            name={`symbol-${space.id === 28 ? "water" : space.kind}`}
                            fallback={<Symbol kind={space.kind} />}
                          />
                        </span>
                      )}
                      <span className="mono-space-name">
                        <span className="mono-full-name">{space.name}</span>
                        <span className="mono-short-name">
                          {space.name.slice(0, 2)}
                        </span>
                      </span>
                      {space.price > 0 && (
                        <span className="mono-space-price">
                          {p.mortgaged ? "抵押" : `M${space.price}`}
                        </span>
                      )}
                      {p.buildings > 0 && (
                        <span
                          className="mono-buildings"
                          aria-label={
                            p.buildings === 5 ? "旅馆" : `${p.buildings} 座房屋`
                          }
                        >
                          {p.buildings === 5 ? (
                            <Art
                              base={assets}
                              name="hotel"
                              fallback={<Building2 />}
                            />
                          ) : (
                            <>
                              <Art
                                base={assets}
                                name="house"
                                fallback={<House />}
                              />
                              <b>{p.buildings}</b>
                            </>
                          )}
                        </span>
                      )}
                    </button>
                  );
                })}
                <div className="mono-piece-layer" aria-hidden="true">
                  {data.players.map((player, i) => {
                    if (player.bankrupt && motion.actor !== i) return null;
                    const pos = motion.positions[i] ?? player.position,
                      point = squarePoint(pos);
                    const same = data.players
                      .map((p, j) =>
                        !p.bankrupt &&
                        (motion.positions[j] ?? p.position) === pos
                          ? j
                          : -1,
                      )
                      .filter((j) => j >= 0);
                    const index = Math.max(0, same.indexOf(i));
                    const cols = Math.min(3, same.length),
                      rows = Math.ceil(same.length / 3);
                    const dx =
                      same.length > 1
                        ? ((index % cols) - (cols - 1) / 2) * 18
                        : 0;
                    const dy =
                      same.length > 1
                        ? (Math.floor(index / 3) - (rows - 1) / 2) * 20
                        : 0;
                    const inset = same.length > 1 ? 36 : 23;
                    return (
                      <Fragment key={i}>
                        <span
                          data-mono-pawn={i}
                          data-position={pos}
                          className={`mono-pawn ${motion.actor === i ? "moving" : ""} ${room.you === i ? "yours" : ""}`}
                          style={{
                            ...seatStyle(i),
                            left: `calc(clamp(${inset}px, ${point.x}%, calc(100% - ${inset}px)) + ${dx}px)`,
                            top: `calc(clamp(${inset}px, ${point.y}%, calc(100% - ${inset}px)) + ${dy}px)`,
                            zIndex: motion.actor === i ? 20 : 5 + i,
                          }}
                          title={`${name(i)} · ${data.board[pos].name}`}
                        >
                          <Art
                            base={assets}
                            name={`token-${i + 1}`}
                            fallback={<span>{i + 1}</span>}
                          />
                        </span>
                        <b
                          className="mono-pawn-number"
                          style={{
                            left: `calc(clamp(${inset}px, ${point.x}%, calc(100% - ${inset}px)) + ${dx}px)`,
                            top: `calc(clamp(${inset}px, ${point.y}%, calc(100% - ${inset}px)) + ${dy}px)`,
                          }}
                        >
                          {i + 1}
                        </b>
                      </Fragment>
                    );
                  })}
                </div>
              </div>
            </div>
          </div>
          <p className="mono-map-help">
            滚轮缩放，拖动棋盘；点选格子查看地产证。
            <span>绿色房屋 · 红色旅馆</span>
          </p>
        </div>

        <aside className="mono-panel">
          <nav
            data-controller-group="tabs"
            className="mono-tabs"
            aria-label="牌桌面板"
          >
            {(
              [
                ["action", "行动"],
                ["assets", "资产"],
                ["trade", "交易"],
                ["cards", "卡牌"],
              ] as [Tab, string][]
            ).map(([id, label]) => (
              <button
                key={id}
                type="button"
                data-mono-tab={id}
                aria-pressed={tab === id}
                className={tab === id ? "active" : ""}
                onClick={() => setTab(id)}
              >
                {label}
                {id === "trade" && incoming > 0 && <b>{incoming}</b>}
              </button>
            ))}
          </nav>
          <div data-controller-group="actions" className="mono-panel-content">
            {tab === "action" && (
              <>
                <header className="mono-phase-heading">
                  <small>{phaseText}</small>
                  <h2>
                    {isClosed
                      ? "本局已关闭"
                      : isFinished
                        ? "本局已结束"
                        : me?.bankrupt
                          ? "你已出局"
                          : room.spectating
                            ? "观战中"
                            : c.roll ||
                                c.end ||
                                c.auction ||
                                data.debt?.debtor === room.you
                              ? "轮到你了"
                              : `${turnName}的回合`}
                  </h2>
                  {!!room.turnDeadline &&
                    !isFinished &&
                    !isClosed &&
                    data.phase !== "auction" && (
                      <span className="mono-clock">
                        {seconds(room.turnDeadline, now)} 秒
                      </span>
                    )}
                </header>
                {isClosed ? (
                  <p>
                    房主已结束本局。可以继续查看局面，或在牌桌上方再开一局。
                  </p>
                ) : (
                  <>
                    {me?.bankrupt && (
                      <p>本局已出局，可以继续观战或通过顶部按钮离开房间。</p>
                    )}
                    {isFinished && (
                      <p className="mono-victory">
                        {room.game?.winners.map(name).join("、")} 获胜！
                      </p>
                    )}
                    {data.phase === "roll" && c.roll && (
                      <div className="mono-action-stack">
                        {me.jailed && (
                          <p>
                            你在监狱。可尝试双骰相同出狱，也可付费或使用免狱卡。
                          </p>
                        )}
                        <GameButton
                          primary
                          disabled={busy}
                          action="roll"
                          onClick={() => void send("roll")}
                        >
                          {me.jailed ? "尝试掷双骰" : "掷骰"}
                        </GameButton>
                        {me.jailed && (
                          <>
                            <GameButton
                              disabled={busy || !c.jailPay}
                              onClick={() => void send("jail_pay")}
                            >
                              支付 50 出狱
                            </GameButton>
                            {c.jailCards.map((card) => (
                              <GameButton
                                key={card}
                                disabled={busy}
                                onClick={() =>
                                  void send("jail_card", { choice: card })
                                }
                              >
                                使用{card === "chance" ? "机会" : "公共基金"}
                                免狱卡
                              </GameButton>
                            ))}
                          </>
                        )}
                        {data.doubles > 0 && !me.jailed && (
                          <p className="mono-help">
                            双骰相同，再行动一次。连续第三次相同会直接入狱。
                          </p>
                        )}
                      </div>
                    )}
                    {data.phase === "purchase" && (
                      <div data-mono-purchase className="mono-action-stack">
                        <button
                          type="button"
                          className="mono-landing-card"
                          onClick={() =>
                            inspect(data.players[data.turn].position)
                          }
                          style={{
                            borderTopColor:
                              data.board[data.players[data.turn].position]
                                .color,
                          }}
                        >
                          <small>无主地产</small>
                          <strong>
                            {data.board[data.players[data.turn].position].name}
                          </strong>
                          <span>
                            M{" "}
                            {data.board[data.players[data.turn].position].price}
                          </span>
                        </button>
                        {c.auction ? (
                          <>
                            <GameButton
                              primary
                              disabled={busy || !c.buy}
                              onClick={() => void send("buy")}
                            >
                              购买 ·{" "}
                              {
                                data.board[data.players[data.turn].position]
                                  .price
                              }
                            </GameButton>
                            <GameButton
                              disabled={busy}
                              action="auction"
                              onClick={() => void send("auction")}
                            >
                              转入拍卖
                            </GameButton>
                            {!c.buy && (
                              <p className="mono-help">
                                现金不足，可先管理资产筹资，或将地产转入拍卖。
                              </p>
                            )}
                          </>
                        ) : (
                          <p>等待{turnName}选择购买或拍卖。</p>
                        )}
                      </div>
                    )}
                    {auction && (
                      <div data-mono-auction className="mono-auction">
                        <p className="mono-auction-title">
                          {auction.kind === "house"
                            ? "最后一座房屋"
                            : auction.kind === "hotel"
                              ? "最后一座旅馆"
                              : data.board[auction.property].name}
                        </p>
                        <div className="mono-bid-display">
                          <span>当前最高价</span>
                          <strong data-mono-high-bid>M {auction.bid}</strong>
                          <small>
                            {auction.bidder >= 0
                              ? name(auction.bidder)
                              : "等待首个报价"}
                          </small>
                        </div>
                        <p className="mono-help">
                          剩余 {seconds(auction.expires, now)} 秒 ·
                          新报价可延长应答时间
                        </p>
                        {c.bid && (
                          <form
                            onSubmit={(e) => {
                              e.preventDefault();
                              void send("bid", {
                                amount: bid,
                                property: bidTarget,
                              });
                            }}
                          >
                            {auction.kind !== "property" && (
                              <label>
                                建造位置
                                <select
                                  aria-label="竞购建筑位置"
                                  value={bidTarget}
                                  onChange={(e) =>
                                    setBidTarget(Number(e.target.value))
                                  }
                                >
                                  {c.bidTargets.map((id) => (
                                    <option value={id} key={id}>
                                      {data.board[id].name}
                                    </option>
                                  ))}
                                </select>
                              </label>
                            )}
                            <label>
                              出价金额
                              <input
                                type="number"
                                aria-label="出价金额"
                                min={c.minBid}
                                max={me.cash}
                                step="1"
                                value={bid}
                                onChange={(e) => setBid(Number(e.target.value))}
                              />
                            </label>
                            <button
                              className="mono-button primary mono-game-action"
                              data-controller-action="bid"
                              type="submit"
                              disabled={
                                busy ||
                                !Number.isInteger(bid) ||
                                bid < c.minBid ||
                                bid > me.cash
                              }
                            >
                              确认出价
                            </button>
                          </form>
                        )}
                        {c.auctionPass && (
                          <GameButton
                            disabled={busy}
                            onClick={() => void send("auction_pass")}
                          >
                            退出本场竞价
                          </GameButton>
                        )}
                        {auction.bidder === room.you ? (
                          <p className="mono-help">
                            你暂时领先，报价资金已保留。
                          </p>
                        ) : (
                          !c.bid && (
                            <p className="mono-help">
                              {c.auctionPass
                                ? "现金不足，可在资产面板筹资后出价。"
                                : "可在此查看竞价进度，无需回应。"}
                            </p>
                          )
                        )}
                      </div>
                    )}
                    {data.debt && (
                      <div data-mono-debt className="mono-debt">
                        <h3>
                          {name(data.debt.debtor)}需支付 {data.debt.amount}
                        </h3>
                        <p>
                          {data.debt.reason} · 收款方：
                          {name(data.debt.creditor)}
                        </p>
                        <strong>
                          还差{" "}
                          {Math.max(
                            0,
                            data.debt.amount -
                              data.players[data.debt.debtor].cash,
                          )}
                        </strong>
                        {data.debt.debtor === room.you ? (
                          <>
                            <p>
                              可出售建筑、抵押地产或通过交易筹资，资金足够后自动付款。
                            </p>
                            <button
                              type="button"
                              className="mono-button"
                              onClick={() => {
                                setAssetOwner(room.you);
                                setTab("assets");
                              }}
                            >
                              管理资产
                            </button>
                            <button
                              type="button"
                              className="mono-button"
                              onClick={() => setTab("trade")}
                            >
                              发起交易
                            </button>
                            {c.bankrupt && (
                              <GameButton
                                danger
                                disabled={busy}
                                onClick={() => setConfirmBankrupt(true)}
                              >
                                宣告破产
                              </GameButton>
                            )}
                          </>
                        ) : (
                          <p>等待对方筹资，其他玩家无需确认。</p>
                        )}
                      </div>
                    )}
                    {data.transfer && (
                      <div className="mono-transfer">
                        <h3>接收抵押地产</h3>
                        <p>
                          {name(data.transfer.player)}接收
                          {data.board[data.transfer.property].name}
                        </p>
                        {data.transfer.player === room.you ? (
                          <>
                            <p>可先付 10% 利息保留抵押，也可立即赎回。</p>
                            <GameButton
                              disabled={busy}
                              onClick={() =>
                                void send("accept_mortgage", { choice: "keep" })
                              }
                            >
                              保留抵押 ·{" "}
                              {Math.ceil(
                                data.board[data.transfer.property].price / 20,
                              )}
                            </GameButton>
                            <GameButton
                              disabled={
                                busy ||
                                me.cash <
                                  data.board[data.transfer.property].price / 2 +
                                    Math.ceil(
                                      data.board[data.transfer.property].price /
                                        20,
                                    )
                              }
                              onClick={() =>
                                void send("accept_mortgage", {
                                  choice: "redeem",
                                })
                              }
                            >
                              立即赎回 ·{" "}
                              {data.board[data.transfer.property].price / 2 +
                                Math.ceil(
                                  data.board[data.transfer.property].price / 20,
                                )}
                            </GameButton>
                          </>
                        ) : (
                          <p>等待接收者处理抵押费用。</p>
                        )}
                      </div>
                    )}
                    {c.end && (
                      <div className="mono-action-stack">
                        <p>可继续管理资产或交易，也可以交给下一位玩家。</p>
                        <GameButton
                          primary
                          disabled={busy}
                          action="end"
                          onClick={() => void send("end")}
                        >
                          结束回合
                        </GameButton>
                      </div>
                    )}
                    {!c.roll &&
                      !c.end &&
                      !c.auction &&
                      !data.debt &&
                      !data.transfer &&
                      !auction &&
                      !isFinished &&
                      !me?.bankrupt && (
                        <p>
                          等待当前玩家行动。可点选棋盘查看地产
                          {room.spectating ? "。" : "，或管理自己的资产。"}
                        </p>
                      )}
                    {incoming > 0 && (
                      <button
                        className="mono-incoming"
                        type="button"
                        onClick={() => setTab("trade")}
                      >
                        收到 {incoming} 份交易报价 · 查看
                      </button>
                    )}
                  </>
                )}
              </>
            )}
            {tab === "assets" && (
              <div className="mono-assets">
                <label>
                  查看资产
                  <select
                    aria-label="查看谁的资产"
                    value={assetOwner}
                    onChange={(e) => setAssetOwner(Number(e.target.value))}
                  >
                    {data.players.map((_, i) => (
                      <option key={i} value={i}>
                        {name(i)}
                      </option>
                    ))}
                  </select>
                </label>
                <p className="mono-help">
                  现金 {data.players[assetOwner]?.cash} ·
                  点选地产查看租金、建造与抵押。
                </p>
                <div className="mono-asset-list">
                  {data.board
                    .filter((s) => data.properties[s.id].owner === assetOwner)
                    .map((space) => (
                      <button
                        type="button"
                        data-mono-asset={space.id}
                        key={space.id}
                        onClick={() => inspect(space.id)}
                      >
                        <i style={{ background: space.color }} />
                        <span>
                          <strong>{space.name}</strong>
                          <small>
                            {data.properties[space.id].mortgaged
                              ? "已抵押"
                              : data.properties[space.id].buildings === 5
                                ? "旅馆"
                                : data.properties[space.id].buildings > 0
                                  ? `${data.properties[space.id].buildings} 座房屋`
                                  : "未建设"}
                          </small>
                        </span>
                        <b>›</b>
                      </button>
                    ))}
                </div>
                {!data.properties.some((p) => p.owner === assetOwner) && (
                  <p>尚未持有地产。</p>
                )}
              </div>
            )}
            {tab === "trade" && (
              <TradePanel
                data={data}
                room={room}
                busy={busy}
                send={send}
                now={now}
              />
            )}
            {tab === "cards" && (
              <div className="mono-cards">
                <label>
                  卡组
                  <select
                    aria-label="查看卡组"
                    value={cardDeck}
                    onChange={(e) => setCardDeck(e.target.value)}
                  >
                    <option value="chance">机会 · 16 张</option>
                    <option value="chest">公共基金 · 16 张</option>
                  </select>
                </label>
                <p className="mono-help">
                  以下是卡牌内容，不表示牌堆顺序。免狱卡使用后放回对应牌堆。
                </p>
                {data.cards[cardDeck].map((card) => (
                  <article
                    key={card.id}
                    className={`mono-card ${data.lastCard?.deck === cardDeck && data.lastCard.id === card.id ? "recent" : ""}`}
                  >
                    <Art
                      base={assets}
                      name={`symbol-${cardDeck}`}
                      className="mono-card-art"
                    />
                    <h4>{card.title}</h4>
                    <p>{card.text}</p>
                  </article>
                ))}
              </div>
            )}
          </div>
        </aside>
      </div>
      {selected !== null && (
        <PropertyDialog
          id={selected}
          data={data}
          room={room}
          busy={busy}
          send={send}
          onClose={() => setSelected(null)}
        />
      )}
      {confirmBankrupt && (
        <Modal
          title="确认破产"
          className="mono-modal"
          onClose={() => setConfirmBankrupt(false)}
        >
          <div data-mono-bankrupt-confirm>
            <p>
              破产后将退出本局，现金、地产和免狱卡按债权人规则结算。仍可交易筹资时，也可以返回继续协商。
            </p>
            <div className="mono-button-row">
              <button
                type="button"
                className="mono-button"
                onClick={() => setConfirmBankrupt(false)}
              >
                继续筹资
              </button>
              <GameButton
                danger
                disabled={busy || !c.bankrupt}
                onClick={() => void send("bankrupt")}
              >
                确认破产
              </GameButton>
            </div>
          </div>
        </Modal>
      )}
    </section>
  );
}
