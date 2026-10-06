import type { CSSProperties, ReactNode } from "react";
import type { Card } from "./types";
import {
  gemBonus,
  gemCardDescription,
  gemCardName,
  gemNames,
} from "./splendor-orient-state";
import "./splendor-orient.css";

export function OrientCard({
  card,
  assets,
  onClick,
  selected,
  affordable,
  renderGem,
  renderCost,
}: {
  card: Card;
  assets: string;
  onClick?: () => void;
  selected?: boolean;
  affordable?: boolean;
  renderGem: (color: number) => ReactNode;
  renderCost: (cost: number[]) => ReactNode;
}) {
  const effect = (name: string, label: string) =>
    assets ? (
      <img src={`${assets}/splendor/expansions/${name}.webp`} alt={label} />
    ) : (
      <span>{label}</span>
    );
  return (
    <button
      className={`dev-card orient-card ${selected ? "selected" : ""} ${affordable ? "affordable" : ""}`}
      data-card-id={card.id}
      onClick={onClick}
      aria-label={`${affordable ? "可购买，" : ""}${gemCardName(card)}，${card.points}分。${gemCardDescription(card)}费用 ${card.cost
        .map((n, i) => (n ? `${gemNames[i]}${n}` : ""))
        .filter(Boolean)
        .join("，")}`}
      style={
        {
          "--cost-columns": Math.min(2, card.cost.filter(Boolean).length),
        } as CSSProperties
      }
    >
      <div className="card-top">
        <strong>{card.points}</strong>
        <span className="orient-bonus">
          {gemBonus(card) > 0 && (
            <>
              {renderGem(card.color)}
              {gemBonus(card) > 1 && <b>×{gemBonus(card)}</b>}
            </>
          )}
          {card.orient === "gold" && effect("orient-gold", "两枚虚拟黄金")}
        </span>
      </div>
      <div className="orient-card-effects">
        {(card.orient === "copy" || card.orient === "copy_cascade") &&
          effect("orient-copy", "复制奖励")}
        {card.orient === "copy_cascade" &&
          effect("orient-free-1", "免费取得一级卡")}
        {card.orient === "cascade" && effect("orient-free-2", "免费取得二级卡")}
        {card.orient === "double" && <span>双奖励</span>}
        {card.orient === "gold" && <span>黄金卡</span>}
        {card.orient === "sacrifice" && (
          <span className="orient-sacrifice">
            弃置{renderGem(card.sacrificeColor ?? 0)}
            <b>×2 张</b>
          </span>
        )}
      </div>
      {card.orient !== "sacrifice" && renderCost(card.cost)}
      <span className="orient-card-tier">
        东方 · {["Ⅰ", "Ⅱ", "Ⅲ"][card.tier - 1]}
      </span>
    </button>
  );
}
