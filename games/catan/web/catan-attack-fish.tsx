import type { AttackSelection } from "./catan-attack-state";

export function AttackFishPayment({
  pick,
  onSelect,
  tokens,
  cost,
  available,
  disabled = false,
}: {
  pick: AttackSelection;
  onSelect: (pick: AttackSelection) => void;
  tokens: { id: number; fish: number }[];
  cost: number;
  available: boolean;
  disabled?: boolean;
}) {
  const paid = tokens
    .filter((t) => pick.tokens?.includes(t.id))
    .reduce((n, t) => n + t.fish, 0);
  return (
    <div className="attack-fish-payment">
      <label>
        <input
          type="checkbox"
          checked={!!pick.fish}
          disabled={disabled || (!pick.fish && !available)}
          onChange={(e) =>
            onSelect({
              ...pick,
              fish: e.target.checked,
              wheat: false,
              target: null,
              tokens: [],
            })
          }
        />
        支付 {cost} 鱼，最多 5 步
      </label>
      {pick.fish && (
        <>
          <div className="attack-picks" aria-label="选择骑士移动的鱼筹码">
            {tokens.map((t) => (
              <button
                key={t.id}
                disabled={disabled}
                className={pick.tokens?.includes(t.id) ? "selected" : ""}
                aria-pressed={!!pick.tokens?.includes(t.id)}
                onClick={() =>
                  onSelect({
                    ...pick,
                    tokens: pick.tokens?.includes(t.id)
                      ? pick.tokens.filter((id) => id !== t.id)
                      : [...(pick.tokens || []), t.id],
                  })
                }
              >
                {t.fish} 鱼
              </button>
            ))}
          </div>
          <small>
            已选 {paid} 鱼／费用 {cost}{" "}
            鱼，多付不找零。加入计划后预留，结束移动时扣除。
          </small>
        </>
      )}
    </div>
  );
}
