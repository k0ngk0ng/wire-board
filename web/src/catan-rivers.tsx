import { useEffect, useState } from "react";
import { Coins } from "lucide-react";
import type { CatanState, Room } from "./types";
import { CatanResource } from "./catan-resources";
import { catanCardNames } from "./catan-cards";
import {
  catanPieceColors,
  catanColorIndex,
  catanSeatColor,
} from "./catan-player-colors";
import { catanCoinReason } from "./catan-rivers-layout";
import "./catan-rivers.css";

type Act = (a: Record<string, unknown>) => Promise<unknown>;

export function CatanCoins({
  count,
  assets,
}: {
  count: number;
  assets: string;
}) {
  return (
    <span className="catan-coins">
      {assets ? (
        <img src={`${assets}/catan/rivers/coin-1-v1.webp`} alt="" />
      ) : (
        <Coins size={17} />
      )}
      <b>{count}</b> 金币
    </span>
  );
}

export function CatanRiverSeat({
  game,
  seat,
  assets,
}: {
  game: CatanState;
  seat: number;
  assets: string;
}) {
  const r = game.rivers;
  if (!r) return null;
  const poor = (r.poor || []).includes(seat),
    richest = r.richest === seat;
  return (
    <span className="catan-river-seat">
      <CatanCoins count={r.gold[seat]} assets={assets} />
      {(["richest", "poor"] as const)
        .filter((status) => (status === "poor" ? poor : richest))
        .map((status) => (
          <span key={status} className={`catan-wealth ${status}${r.poorPenalty === 0 && status === "poor" ? " no-penalty" : ""}`}>
            {assets && (
              <img
                src={`${assets}/catan/rivers/${status === "poor" ? "poor" : "wealthiest"}-v1.webp`}
                alt=""
              />
            )}
            {status === "poor"
              ? r.poorPenalty === 0
                ? "最贫 · 不扣分"
                : "最贫 −2分"
              : "最富 +1分"}
          </span>
        ))}
    </span>
  );
}

// Render inside the existing route transform, whose x-axis follows the edge.
export function CatanBridge({
  game,
  seat,
  assets,
}: {
  game: CatanState;
  seat: number;
  assets: string;
}) {
  return assets ? (
    <image
      href={`${assets}/catan/rivers/bridge-${catanPieceColors[catanColorIndex(game, seat)]}-v1.webp`}
      x={-29}
      y={-17}
      width={58}
      height={30}
      preserveAspectRatio="xMidYMid meet"
    />
  ) : (
    <path
      d="M-28 7V-5L-16-14H16L28-5V7H17L10-3H-10L-17 7Z"
      fill={catanSeatColor(game, seat)}
      stroke="#604b35"
      strokeWidth={2}
    />
  );
}

export function CatanRiverStart({
  room,
  busy,
  act,
  selectedTile,
  clearTile,
}: {
  room: Room;
  busy: boolean;
  act: Act;
  selectedTile: number | null;
  clearTile: () => void;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const game = room.game!,
    g = game.catan!;
  useEffect(
    () => setCollapsed(false),
    [room.id, game.phase, selectedTile, room.you],
  );
  if (
    !g.rivers ||
    game.phase !== "catan_rivers_start" ||
    game.finished ||
    room.status !== "playing"
  )
    return null;
  const mine =
    !room.spectating &&
    room.you === game.turn &&
    !g.players[room.you]?.eliminated;
  return (
    <section className="catan-gold-choice" aria-label="选择河流强盗起点">
      <header>
        <strong>
          {mine
            ? "选择强盗起点"
            : `${room.seats[game.turn].name} 正在选择强盗起点`}
        </strong>
        <button
          aria-expanded={!collapsed}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "展开" : "收起"}
        </button>
      </header>
      {!collapsed && (
        <div className="catan-gold-body">
          <p>
            先手在两处沼泽中选择一处，再开始放置建筑与道路。
            {g.citiesKnights &&
              "首次蛮族进攻前强盗不会入场；起始河岸城市也领1金币。"}
            限时120秒，超时自动选择；可收起面板查看地图。
          </p>
          {mine && (
            <>
              <p>
                {selectedTile === null
                  ? "点击亮起的沼泽，再确认起点。"
                  : `已选择沼泽 #${selectedTile + 1}。`}
              </p>
              <div className="cloth-confirm-actions">
                {selectedTile !== null && (
                  <button disabled={busy} onClick={clearTile}>
                    重选位置
                  </button>
                )}
                <button
                  className="primary"
                  disabled={
                    busy ||
                    selectedTile === null ||
                    !g.legal.robber?.includes(selectedTile)
                  }
                  onClick={async () => {
                    await act({
                      type: "catan_rivers_start",
                      tile: selectedTile,
                    });
                    clearTile();
                  }}
                >
                  确认起点
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </section>
  );
}

export function CatanRiverBank({
  room,
  busy,
  act,
  assets,
}: {
  room: Room;
  busy: boolean;
  act: Act;
  assets: string;
}) {
  const g = room.game!.catan!,
    r = g.rivers;
  const [choice, setChoice] = useState<{ color: number; buy: boolean } | null>(
    null,
  );
  useEffect(
    () => setChoice(null),
    [
      room.id,
      room.you,
      room.spectating,
      room.game!.turn,
      room.game!.round,
      room.game!.phase,
    ],
  );
  if (!r) return null;
  const mine =
    room.status === "playing" &&
    !room.game!.finished &&
    !room.spectating &&
    room.you === room.game!.turn &&
    room.game!.phase === "catan_turn" &&
    !g.players[room.you]?.eliminated;
  return (
    <details className="catan-river-bank">
      <summary>
        <Coins size={18} /> 金币兑换 <span>银行 {r.bank}</span>
      </summary>
      <p>
        2金币买1张普通资源，每次行动最多买2张。出售按自己的兑换比例，每组获得1金币。
        {g.citiesKnights && "商品也可出售，但不能用金币购买。"}
        {r.goldRule === "ledger" &&
          "本站补充规则：金币用完仍照常记账发放，归还银行的金币优先复用。"}
      </p>
      {r.map.numberRecipe && (
        <p className="small">
          本站数字配置：固定数列沿逆时针螺旋摆放，跳过沼泽；数字数量不变，未采用2025实体字母背面的对应表。
        </p>
      )}
      {mine ? (
        <>
          <p>
            <CatanCoins count={r.gold[room.you]} assets={assets} /> ·
            本次还可购买 {Math.max(0, 2 - r.bought)} 张
          </p>
          <div className="catan-coin-options">
            {g.bank.slice(0, g.citiesKnights ? 8 : 5).map((count, color) => (
              <div key={color}>
                <CatanResource color={color} assets={assets} small />
                <span>库存 {count}</span>
                {(color < 5 ? [true, false] : [false]).map((buy) => (
                  <button
                    key={String(buy)}
                    className={
                      choice?.color === color && choice.buy === buy
                        ? "selected"
                        : ""
                    }
                    title={catanCoinReason(g, room.you, color, buy)}
                    aria-label={
                      buy
                        ? `用2金币购买1张${catanCardNames[color]}`
                        : `出售${g.players[room.you].rates[color]}张${catanCardNames[color]}换1金币`
                    }
                    disabled={
                      busy || !!catanCoinReason(g, room.you, color, buy)
                    }
                    onClick={() => setChoice({ color, buy })}
                  >
                    {buy
                      ? "2金买1"
                      : `${g.players[room.you].rates[color]}换1金`}
                  </button>
                ))}
              </div>
            ))}
          </div>
          {choice && (
            <div className="catan-coin-confirm" aria-label="确认金币兑换">
              <strong>
                {choice.buy
                  ? `支付2金币，领取1张${catanCardNames[choice.color]}`
                  : `支付${g.players[room.you].rates[choice.color]}张${catanCardNames[choice.color]}，领取1金币`}
              </strong>
              <button disabled={busy} onClick={() => setChoice(null)}>
                取消
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  !!catanCoinReason(g, room.you, choice.color, choice.buy)
                }
                onClick={async () => {
                  await act({
                    type: choice.buy ? "catan_coin_buy" : "catan_coin_sell",
                    color: choice.color,
                  });
                  setChoice(null);
                }}
              >
                确认兑换
              </button>
            </div>
          )}
        </>
      ) : (
        <p>在自己的行动阶段可以兑换金币。</p>
      )}
      <p>
        唯一最富者加1分，并列最富无人加分；所有并列最贫者各扣2分。金币不计入资源手牌，也不受强盗或7点弃牌影响。
      </p>
    </details>
  );
}

export function CatanGoldTradePicker({
  give,
  take,
  giveLimit,
  takeLimit,
  onChange,
  disabled,
}: {
  give: number;
  take: number;
  giveLimit: number;
  takeLimit: number;
  onChange: (give: number, take: number) => void;
  disabled: boolean;
}) {
  return (
    <fieldset className="catan-gold-trade" disabled={disabled}>
      <legend>玩家交易 · 金币</legend>
      {([true, false] as const).map((sending) => {
        const value = sending ? give : take,
          limit = sending ? giveLimit : takeLimit;
        const change = (next: number) =>
          onChange(sending ? next : 0, sending ? 0 : next);
        const label = sending ? "给出金币" : "索取金币";
        return (
          <div key={label}>
            <span>{label}</span>
            <div className="catan-stepper">
              <button
                aria-label={`减少${label}`}
                disabled={disabled || value <= 0}
                onClick={() => change(Math.max(0, value - 1))}
              >
                −
              </button>
              <b>{value}</b>
              <button
                aria-label={`增加${label}`}
                disabled={disabled || value >= limit}
                onClick={() => change(Math.min(limit, value + 1))}
              >
                +
              </button>
            </div>
          </div>
        );
      })}
      <small>
        可与资源混合交易，同一提议只能给出或索取金币。金币买卖请使用“金币兑换”。
      </small>
    </fieldset>
  );
}
