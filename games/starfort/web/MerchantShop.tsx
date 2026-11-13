import { useState } from "react";
import type { View } from "./types";

export function MerchantShop({
  data,
  you,
  locked,
  send,
}: {
  data: View;
  you: number;
  locked: boolean;
  send: (type: string, extra?: Record<string, unknown>) => Promise<void>;
}) {
  const [amount, setAmount] = useState(1);
  const [damageTower, setDamageTower] = useState(0);
  const [rangeTower, setRangeTower] = useState(0);
  const quote = data.controls.shop;
  if (!quote) return null;
  const selected = Math.max(1, Math.min(amount, quote.repairMax));
  const own = data.towers.filter((t) => t.owner === you);
  const names = {
    arrow: "箭塔",
    cannon: "炮塔",
    frost: "冰塔",
    arcane: "法师塔",
  };
  return (
    <div data-sf-shop>
      <p>
        第 {quote.wave} 波商人已送达 · 各项全队额度 {quote.limit} 份 ·
        下一波开战后离店
      </p>
      <div className="sf-shop-grid">
        <section className="sf-shop-item">
          <h3>修复星核</h3>
          <p>
            ♥ {data.baseHP}/{data.baseMaxHP} · 剩余 {quote.repairRemaining} 点
          </p>
          <label>
            修复生命
            <input
              type="number"
              min={1}
              max={Math.max(1, quote.repairMax)}
              value={selected}
              disabled={locked || !quote.enabled || quote.repairMax < 1}
              onChange={(e) => setAmount(Number(e.target.value) || 1)}
            />
          </label>
          <button
            data-sf-shop-buy="repair"
            disabled={locked || !quote.enabled || quote.repairMax < 1}
            onClick={() =>
              void send("shop", { choice: "repair", amount: selected })
            }
          >
            修复 {selected} 点 · {selected * quote.unitCost} 金币
          </button>
        </section>
        {(["damage", "range"] as const).map((choice) => {
          const stored = choice === "damage" ? damageTower : rangeTower;
          const id = own.some((t) => t.id === stored)
            ? stored
            : (own[0]?.id ?? 0);
          const c = quote.towers.find((t) => t.towerId === id);
          const enabled =
            choice === "damage" ? c?.damageEnabled : c?.rangeEnabled;
          return (
            <section className="sf-shop-item" key={choice}>
              <h3>{choice === "damage" ? "攻击强化" : "射程强化"}</h3>
              <p>
                永久增加 5% · 剩余{" "}
                {choice === "damage"
                  ? quote.damageRemaining
                  : quote.rangeRemaining}{" "}
                座
              </p>
              <label>
                {choice === "damage" ? "攻击强化防御塔" : "射程强化防御塔"}
                <select
                  value={id}
                  disabled={locked || !quote.enabled || !own.length}
                  onChange={(e) =>
                    (choice === "damage" ? setDamageTower : setRangeTower)(
                      Number(e.target.value),
                    )
                  }
                >
                  {!own.length && <option value={0}>暂无自己的防御塔</option>}
                  {own.map((t) => (
                    <option key={t.id} value={t.id}>
                      {names[t.kind]} · {t.level} 级 · 建造点 {t.pad + 1}
                    </option>
                  ))}
                </select>
              </label>
              <button
                data-sf-shop-buy={choice}
                disabled={locked || !quote.enabled || !enabled}
                onClick={() => void send("shop", { choice, towerId: id })}
              >
                强化 · {quote.unitCost} 金币
              </button>
            </section>
          );
        })}
        <section className="sf-shop-item">
          <h3>{quote.invulnerability ? "星核无敌" : "星核庇护"}</h3>
          <p>
            {quote.invulnerability
              ? "使用后星核无敌 10 秒，可在波间或战斗中使用"
              : "免疫 1 次星陨巨像进入星核的扣血"}
          </p>
          <p>
            当前剩余{" "}
            {quote.invulnerability
              ? (data.invulnerabilityPacks ?? 0)
              : (data.world?.bossWards ?? 0)}{" "}
            份
          </p>
          <button
            data-sf-shop-buy="shield"
            disabled={locked || !quote.enabled || !quote.shieldEnabled}
            onClick={() => void send("shop", { choice: "shield" })}
          >
            购买 · 1000 金币
          </button>
        </section>
      </div>
    </div>
  );
}
