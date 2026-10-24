import { useId } from "react";
import { Check, X } from "lucide-react";
import { troopAmount } from "./map-actions";

export function TroopAmount({
  mode,
  maximum,
  requested,
  enabled,
  onChange,
  onCancel,
  onConfirm,
}: {
  mode: "recruit" | "march";
  maximum: number;
  requested: number | null;
  enabled: boolean;
  onChange: (amount: number | null) => void;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const id = useId(),
    amount = troopAmount(maximum, requested);
  const label = mode === "recruit" ? "征兵人数" : "派兵人数";
  const disabled = !enabled || maximum <= 0;
  return (
    <section
      className="cq-amount-panel"
      data-cq-amount-panel={mode}
      aria-label={label}
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          event.stopPropagation();
          onCancel();
        } else if (
          event.key === "Enter" &&
          event.target instanceof HTMLInputElement &&
          mode === "recruit"
        ) {
          event.preventDefault();
          if (!disabled && amount > 0) onConfirm();
        }
      }}
    >
      <div className="cq-amount-heading">
        <label htmlFor={id}>{mode === "recruit" ? "征兵" : "派兵"}</label>
        <input
          id={id}
          data-cq-amount="number"
          aria-label={label}
          type="number"
          inputMode="numeric"
          min={1}
          max={maximum}
          step={1}
          value={amount || ""}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.currentTarget.value))}
        />
        <span className="cq-amount-limit">
          / {maximum.toLocaleString("zh-CN")}
        </span>
        <button
          aria-label={`${label}恢复一半`}
          aria-pressed={requested === null}
          disabled={disabled}
          onClick={() => onChange(null)}
        >
          一半
        </button>
        <button
          data-cq-action="cancel-amount"
          aria-label="关闭人数选择"
          onClick={onCancel}
        >
          <X size={17} />
        </button>
      </div>
      <div className="cq-amount-adjust">
        <input
          data-cq-amount="range"
          aria-label={`${label}滑块`}
          type="range"
          min={1}
          max={Math.max(1, maximum)}
          step={1}
          value={Math.max(1, amount)}
          disabled={disabled}
          onChange={(event) => onChange(Number(event.currentTarget.value))}
        />
        {mode === "recruit" && (
          <button
            data-cq-action="confirm-recruit"
            aria-label="确认征兵"
            disabled={disabled || amount <= 0}
            onClick={onConfirm}
          >
            <Check size={16} />
            <span>确认</span>
          </button>
        )}
      </div>
    </section>
  );
}
