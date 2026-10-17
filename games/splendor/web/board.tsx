import { Check, ChevronRight, Crown, Minus, Plus } from "lucide-react";
import React, { useContext, useEffect, useState } from "react";
import type { Card, Noble, Room } from "./types";
import type { Act } from "../../../web/src/types";
import { AssetsContext, Modal } from "../../../web/src/ui/board-host";
import { SplendorCardAnimation } from "./splendor-card-animation";
import { SplendorCities } from "./splendor-cities";
import { gemNobleEligible } from "./splendor-city-state";
import { SplendorExpansionBoard } from "./splendor-expansions";
import { SplendorMarket } from "./splendor-market";
import { extraNobleName, nobleArtwork } from "./splendor-nobles";
import { OrientCard } from "./splendor-orient-card";
import { gemCanBuy, gemNames } from "./splendor-orient-state";
import { SplendorPurchase } from "./splendor-purchase";
import { SplendorTokenAnimation } from "./splendor-token-animation";
export const gemDisplayOrder = [1, 2, 0, 4, 3, 5];
export const gemColors = [
  "#218357",
  "#f7f2dd",
  "#2974bb",
  "#343c43",
  "#c94440",
  "#dcac38",
];
export function Gemstone({
  color = 0,
  size = 23,
}: {
  color?: number;
  size?: number;
}) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" aria-hidden="true">
      <path
        d="M10 5h20l8 12-18 20L2 17Z"
        fill={gemColors[color]}
        stroke={color === 1 ? "#aaa389" : "#ffffff66"}
        strokeWidth="1.5"
      />
      <path
        d="M10 5l5 12 5 20 5-20 5-12M2 17h36M10 5l15 12L20 5l-5 12L30 5"
        fill="none"
        stroke={color === 1 ? "#b2ab94" : "#fff"}
        opacity=".45"
      />
    </svg>
  );
}
export function Cost({ cost }: { cost: number[] }) {
  return (
    <div className="costs">
      {cost.map(
        (n, i) =>
          n > 0 && (
            <span
              className={`cost color-${i}`}
              key={i}
              style={{
                background: gemColors[i],
                color: i === 1 ? "#46443a" : "white",
              }}
              title={gemNames[i]}
            >
              {n}
            </span>
          ),
      )}
    </div>
  );
}
export function DevCard({
  card,
  onClick,
  selected = false,
  affordable = false,
}: {
  card: Card;
  onClick?: () => void;
  selected?: boolean;
  affordable?: boolean;
}) {
  const assets = useContext(AssetsContext);
  if (card.orient)
    return (
      <OrientCard
        card={card}
        assets={assets}
        onClick={onClick}
        selected={selected}
        affordable={affordable}
        renderGem={(color) => <Gemstone color={color} size={26} />}
        renderCost={(cost) => <Cost cost={cost} />}
      />
    );
  return (
    <button
      className={`dev-card gem-${card.color} ${selected ? "selected" : ""} ${affordable ? "affordable" : ""}`}
      data-card-id={card.id}
      onClick={onClick}
      aria-label={`${affordable ? "可购买，" : ""}${gemNames[card.color]}发展卡，${card.points}分，费用 ${card.cost
        .map((n, i) => (n ? `${gemNames[i]}${n}` : ""))
        .filter(Boolean)
        .join("，")}`}
      style={
        {
          "--card-color": gemColors[card.color],
          "--cost-columns": Math.min(2, card.cost.filter((n) => n > 0).length),
          "--art-x": `${((card.tier - 1) * 2 + (card.id % 2)) * 20}%`,
          "--art-y": `${[3, 4, 0, 1, 2][card.color] * 20}%`,
        } as React.CSSProperties
      }
    >
      <div className="card-top">
        <strong>{card.points}</strong>
        <Gemstone color={card.color} size={31} />
      </div>
      <div className="card-art">
        <svg viewBox="0 0 160 85">
          <path
            d="M0 80L35 28l23 19 29-34 40 45 33-19v46H0Z"
            fill="currentColor"
            opacity=".35"
          />
          <path
            d="M48 76V40h13V29h14V12h13v17h14v11h13v36M43 77h78"
            fill="none"
            stroke="currentColor"
            opacity=".55"
            strokeWidth="3"
          />
          <path d="M66 77V54q16-18 31 0v23" fill="currentColor" opacity=".5" />
          <circle
            cx={23 + (card.id % 5) * 25}
            cy="20"
            r="10"
            fill="currentColor"
            opacity=".22"
          />
        </svg>
      </div>
      <Cost cost={card.cost} />
      <span className="card-id">
        {["Ⅰ", "Ⅱ", "Ⅲ"][card.tier - 1]} · {String(card.id).padStart(2, "0")}
      </span>
    </button>
  );
}
export function NobleCard({
  noble,
  eligible,
  onClick,
}: {
  noble: Noble;
  eligible: boolean;
  onClick: () => void;
}) {
  const assets = useContext(AssetsContext);
  return (
    <button
      className={`noble ${extraNobleName(noble.id) ? "extra-noble" : ""} ${eligible ? "eligible" : ""}`}
      data-noble-id={noble.id}
      style={nobleArtwork(noble.id, assets)}
      onClick={onClick}
      aria-label={`${extraNobleName(noble.id) || `贵族 ${noble.id}`}，3 分，需要 ${noble.cost
        .map((n, i) => (n ? `${gemNames[i]}发展卡${n}张` : ""))
        .filter(Boolean)
        .join("，")}`}
    >
      <span className="noble-points">3</span>
      <svg viewBox="0 0 70 65" aria-hidden="true">
        <path d="M14 65q1-25 21-25t22 25" fill="#637c71" />
        <ellipse cx="35" cy="26" rx="13" ry="17" fill="#d4b889" />
        <path d="M21 26Q10 5 35 6q24 1 14 24l-4-13-22 6" fill="#69583e" />
        <path d="m22 46 13 14 13-14-7-6-6 10-6-10" fill="#f5e7c4" />
        <path d="M18 10h33l-5-7-9 4-9-7-3 8Z" fill="#d7b35b" />
      </svg>
      <Cost cost={noble.cost} />
    </button>
  );
}
export function SplendorBoard({
  room,
  act,
  busy,
}: {
  room: Room;
  act: Act;
  busy: boolean;
}) {
  const assets = useContext(AssetsContext);
  const g = room.game!,
    s = g.splendor!,
    p = s.players[room.you] ?? {
      tokens: Array(6).fill(0),
      bonus: Array(5).fill(0),
      reserved: [],
      cards: [],
      nobles: [],
      score: 0,
    },
    mine = !room.spectating && g.turn === room.you && !g.finished;
  const canAfford = (card: Card) =>
    mine &&
    g.phase === "turn" &&
    room.status === "playing" &&
    (!s.strongholds?.[card.id] || s.strongholds[card.id].player === room.you) &&
    gemCanBuy(p, card);
  const [tokens, setTokens] = useState<number[]>(Array(6).fill(0));
  const [selected, setSelected] = useState<Card>();
  const [blindTier, setBlindTier] = useState<number>();
  const [conquestPurchase, setConquestPurchase] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(true);
  const [collection, setCollection] = useState(false);
  const selectCard = (card: Card, conquest = false) => {
    setConquestPurchase(conquest);
    setSelected(card);
  };
  useEffect(() => {
    setTokens(Array(6).fill(0));
    setSelected(undefined);
    setBlindTier(undefined);
  }, [room.version]);
  useEffect(() => {
    if (g.phase === "discard") setDiscardOpen(true);
  }, [g.phase, g.turn]);
  const taking = mine && g.phase === "turn",
    discard = mine && g.phase === "discard";
  return (
    <div className={`splendor-board ${s.options?.orient ? "has-orient" : ""}`}>
      <h2 className="sr-only">璀璨宝石游戏桌面</h2>
      {s.options?.cities ? (
        <SplendorCities
          room={room}
          assets={assets}
          renderGem={(color) => <Gemstone color={color} />}
        />
      ) : (
        <div className="nobles-row">
          <div className="board-section-label">
            <Crown size={18} />
            <span>
              贵族来访<small>满足条件 · 获得 3 分</small>
            </span>
          </div>
          <div className="nobles">
            {s.nobles.map((n) => (
              <NobleCard
                key={n.id}
                noble={n}
                eligible={gemNobleEligible(p, n)}
                onClick={() => {
                  if (mine && g.phase === "noble")
                    void act({ type: "noble", noble: n.id });
                }}
              />
            ))}
          </div>
        </div>
      )}
      <SplendorMarket
        assets={assets}
        room={room}
        busy={busy}
        onReserve={setBlindTier}
        renderCard={(c) => (
          <DevCard
            card={c}
            affordable={canAfford(c)}
            selected={selected?.id === c.id}
            onClick={() => selectCard(c)}
          />
        )}
      />
      <SplendorExpansionBoard
        onConquest={(card) => selectCard(card, true)}
        room={room}
        assets={assets}
        act={act}
        busy={busy}
        renderCard={(card, onClick) => (
          <DevCard card={card} onClick={onClick} />
        )}
        renderCost={(cost) => <Cost cost={cost} />}
        renderGem={(color) => <Gemstone color={color} />}
      />
      <div className="gem-bank">
        <div className="section-line">
          <h3>公共宝石</h3>
          <small>三种各一枚，或同色两枚（该堆至少四枚）</small>
        </div>
        <div className="token-row">
          {gemDisplayOrder.map((i) => {
            const n = s.bank[i];
            return (
              <button
                key={i}
                className={`token-button ${tokens[i] > 0 ? "chosen" : ""}`}
                data-bank-color={i}
                disabled={!taking || busy || i === 5 || n === 0}
                onClick={() =>
                  setTokens(
                    tokens.map((v, c) =>
                      c === i ? (v >= Math.min(2, n) ? 0 : v + 1) : v,
                    ),
                  )
                }
                aria-label={`拿取${gemNames[i]}，供应 ${n}，已选 ${tokens[i]}`}
              >
                <span
                  className={`token color-${i}`}
                  style={
                    {
                      "--gem": gemColors[i],
                      backgroundPosition: `${i * 20}% 0`,
                    } as React.CSSProperties
                  }
                >
                  <Gemstone color={i} size={31} />
                </span>
                <b>{n}</b>
                <small>{gemNames[i]}</small>
                {tokens[i] > 0 && (
                  <span className="token-picked">+{tokens[i]}</span>
                )}
              </button>
            );
          })}
        </div>
        <div
          className={`bank-actions ${taking ? "" : "splendor-action-placeholder"}`}
          aria-hidden={!taking}
          inert={!taking}
        >
          <span>已选择 {tokens.reduce((a, b) => a + b, 0)} 枚</span>
          <button
            className="subtle"
            disabled={!taking || busy}
            onClick={() => setTokens(Array(6).fill(0))}
          >
            重选
          </button>
          <button
            className="primary"
            disabled={!taking || busy || tokens.every((n) => n === 0)}
            onClick={() => void act({ type: "take", tokens })}
          >
            拿取宝石
            <Check size={16} />
          </button>
        </div>
      </div>
      {!room.spectating && (
        <div className="personal-area">
          <div className="section-line">
            <h3>你的商会</h3>
            <button
              className="subtle"
              onClick={() => setCollection(!collection)}
            >
              发展卡 {p.cards.length}
              {!s.options?.cities && ` · 贵族 ${p.nobles.length}`}
              <ChevronRight size={15} />
            </button>
          </div>
          <div className="splendor-wallet" aria-label="你的宝石与永久折扣">
            <div className="wallet-caption">
              <span>
                我的宝石 <small>永久折扣 / 手持筹码</small>
              </span>
              <b>{p.tokens.reduce((sum, n) => sum + n, 0)} / 10</b>
            </div>
            <div className="wallet-colors">
              {gemDisplayOrder.map((i) => (
                <div
                  key={i}
                  className={`wallet-gem color-${i}`}
                  style={
                    {
                      "--gem": gemColors[i],
                      "--ink": i === 1 || i === 5 ? "#332d20" : "#fff",
                    } as React.CSSProperties
                  }
                >
                  <Gemstone color={i} size={24} />
                  <div className="wallet-counts">
                    {i < 5 && (
                      <span
                        className={`wallet-discount ${p.bonus[i] ? "" : "empty"}`}
                        title={`${gemNames[i]}永久折扣 ${p.bonus[i]}`}
                      >
                        <span
                          aria-hidden="true"
                          className="discount-card-icon"
                        />
                        <b>{p.bonus[i]}</b>
                        <span className="sr-only">永久折扣</span>
                      </span>
                    )}
                    <span
                      className="wallet-held"
                      title={`${gemNames[i]}筹码 ${p.tokens[i]}`}
                    >
                      <strong>{p.tokens[i]}</strong>
                      <span
                        aria-hidden="true"
                        className="wallet-token"
                        style={{ backgroundPosition: `${i * 20}% 0` }}
                      />
                      <span className="sr-only">手持筹码</span>
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className="reserved">
            <span>
              预留卡 <b>{p.reserved?.length || 0}/3</b>
            </span>
            {p.reserved?.length ? (
              p.reserved.map((c) => (
                <DevCard
                  key={c.id}
                  card={c}
                  affordable={canAfford(c)}
                  selected={selected?.id === c.id}
                  onClick={() => selectCard(c)}
                />
              ))
            ) : (
              <p>预留心仪的发展卡，也能获得一枚黄金。</p>
            )}
          </div>
          {collection && (
            <div className="collection">
              {p.cards.map((c) => (
                <DevCard key={c.id} card={c} />
              ))}
              {p.nobles.map((n) => (
                <NobleCard key={n.id} noble={n} eligible onClick={() => {}} />
              ))}
            </div>
          )}
        </div>
      )}
      {mine && g.phase === "noble" && (
        <div className="action-prompt">
          多位贵族青睐你的商会，请点击一位符合条件的贵族。
        </div>
      )}
      {discard && (
        <section
          className={`discard-panel ${discardOpen ? "" : "is-collapsed"}`}
          aria-label="归还宝石"
        >
          <header>
            <h3>请归还 {p.tokens.reduce((a, b) => a + b, 0) - 10} 枚宝石</h3>
            <button
              className="subtle"
              aria-expanded={discardOpen}
              aria-controls="discard-controls"
              onClick={() => setDiscardOpen(!discardOpen)}
            >
              {discardOpen ? "查看牌桌" : "继续归还"}
            </button>
          </header>
          {discardOpen && (
            <div id="discard-controls">
              <p>回合结束时最多持有 10 枚，包括黄金。</p>
              <div className="discard-list">
                {p.tokens.map((n, i) => (
                  <div key={i}>
                    <Gemstone color={i} />
                    <span>
                      {gemNames[i]} · 持有 {n}
                    </span>
                    <button
                      className="icon-button"
                      aria-label={`减少归还${gemNames[i]}`}
                      disabled={tokens[i] === 0}
                      onClick={() =>
                        setTokens(tokens.map((v, c) => (c === i ? v - 1 : v)))
                      }
                    >
                      <Minus size={15} />
                    </button>
                    <b>{tokens[i]}</b>
                    <button
                      className="icon-button"
                      aria-label={`增加归还${gemNames[i]}`}
                      disabled={tokens[i] >= n}
                      onClick={() =>
                        setTokens(tokens.map((v, c) => (c === i ? v + 1 : v)))
                      }
                    >
                      <Plus size={15} />
                    </button>
                  </div>
                ))}
              </div>
              <button
                className="primary wide"
                disabled={
                  busy ||
                  tokens.reduce((a, b) => a + b, 0) !==
                    p.tokens.reduce((a, b) => a + b, 0) - 10
                }
                onClick={() => void act({ type: "discard", tokens })}
              >
                归还所选宝石
              </button>
            </div>
          )}
        </section>
      )}
      {blindTier !== undefined && taking && (
        <Modal
          title={`确认盲预留 · ${blindTier >= 3 ? "东方 " : ""}${(blindTier % 3) + 1} 级牌堆`}
          dismissible={!busy}
          onClose={() => setBlindTier(undefined)}
        >
          <p>
            {p.tradingPosts?.includes(2)
              ? s.remaining[blindTier] === 1
                ? "牌堆只剩一张发展卡，查看后预留这一张。"
                : `查看 ${blindTier >= 3 ? "东方 " : ""}${(blindTier % 3) + 1} 级牌堆顶部两张发展卡，选择一张预留，另一张放回牌堆底部。`
              : `随机预留一张 ${blindTier >= 3 ? "东方 " : ""}${(blindTier % 3) + 1} 级发展卡，确认前无法查看牌面。`}
            这会使用本回合的行动。
          </p>
          <p>
            {s.bank[5] > 0
              ? "同时获得 1 枚黄金。如果持有宝石超过 10 枚，需要归还多出的宝石。"
              : "公共供应区没有黄金，本次仅预留卡牌。"}
          </p>
          <div className="form-grid">
            <button
              className="outline"
              disabled={busy}
              onClick={() => setBlindTier(undefined)}
            >
              取消
            </button>
            <button
              className="primary"
              disabled={
                busy || !s.remaining[blindTier] || p.reserved!.length >= 3
              }
              onClick={() =>
                void act({
                  type: "reserve",
                  tier: (blindTier % 3) + 1,
                  choice: blindTier >= 3 ? "orient" : "",
                })
              }
            >
              确认预留
            </button>
          </div>
        </Modal>
      )}
      {selected && (
        <SplendorPurchase
          room={room}
          card={selected}
          conquest={conquestPurchase}
          act={act}
          busy={busy}
          onClose={() => setSelected(undefined)}
          renderCard={(card, onClick) => (
            <DevCard card={card} onClick={onClick} />
          )}
          renderCost={(cost) => <Cost cost={cost} />}
        />
      )}
      <button
        className={`pass-button ${taking ? "" : "splendor-action-placeholder"}`}
        aria-hidden={!taking}
        inert={!taking}
        disabled={!taking || busy}
        onClick={() => void act({ type: "pass" })}
      >
        无合法行动时跳过回合
      </button>
      <SplendorCardAnimation
        room={room}
        assets={assets}
        renderCard={(card) => <DevCard card={card} />}
        renderNoble={(noble) => (
          <NobleCard noble={noble} eligible={false} onClick={() => {}} />
        )}
      />
      <SplendorTokenAnimation
        room={room}
        assets={assets}
        colors={gemColors}
        renderGem={(color) => <Gemstone color={color} size={26} />}
      />
    </div>
  );
}
