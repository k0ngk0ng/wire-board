import { useEffect, useId, useRef } from "react";
import { cueTip } from "./cue-input";
export function CueTip({
  top,
  side,
  disabled,
  onChange,
}: {
  top: number;
  side: number;
  disabled: boolean;
  onChange: (p: { top: number; side: number }) => void;
}) {
  const pointer = useRef<number | null>(null),
    id = useId();
  useEffect(() => {
    if (disabled) pointer.current = null;
  }, [disabled]);
  const select = (event: React.PointerEvent<HTMLButtonElement>) => {
    const r = event.currentTarget.getBoundingClientRect(),
      radius = r.width * 0.34;
    onChange(
      cueTip(
        (event.clientX - r.left - r.width / 2) / radius,
        (r.top + r.height / 2 - event.clientY) / radius,
      ),
    );
  };
  const label = `${top > 0.04 ? "高杆" : top < -0.04 ? "低杆" : "中杆"} · ${side > 0.04 ? "右旋" : side < -0.04 ? "左旋" : "无侧旋"}`;
  return (
    <div className="snooker-tip-control">
      <button
        type="button"
        className="snooker-tip-ball"
        disabled={disabled}
        data-top={top}
        data-side={side}
        aria-label={`白球击点：${label}；方向键调整，Home 恢复中杆`}
        onPointerDown={(e) => {
          if (disabled || e.button !== 0) return;
          pointer.current = e.pointerId;
          e.currentTarget.setPointerCapture(e.pointerId);
          e.currentTarget.focus({ preventScroll: true });
          select(e);
        }}
        onPointerMove={(e) => {
          if (!disabled && pointer.current === e.pointerId) select(e);
        }}
        onPointerUp={(e) => {
          if (pointer.current !== e.pointerId) return;
          pointer.current = null;
          if (e.currentTarget.hasPointerCapture(e.pointerId))
            e.currentTarget.releasePointerCapture(e.pointerId);
        }}
        onPointerCancel={() => (pointer.current = null)}
        onLostPointerCapture={() => (pointer.current = null)}
        onKeyDown={(e) => {
          const delta = 0.05;
          let next = { top, side };
          switch (e.key) {
            case "ArrowUp":
              next.top += delta;
              break;
            case "ArrowDown":
              next.top -= delta;
              break;
            case "ArrowLeft":
              next.side -= delta;
              break;
            case "ArrowRight":
              next.side += delta;
              break;
            case "Home":
              next = { top: 0, side: 0 };
              break;
            default:
              return;
          }
          e.preventDefault();
          onChange(cueTip(next.side, next.top));
        }}
      >
        <svg viewBox="0 0 160 160" aria-hidden="true">
          <defs>
            <radialGradient id={id} cx="35%" cy="28%">
              <stop offset="0" stopColor="#fff" />
              <stop offset=".75" stopColor="#f2f1e7" />
              <stop offset="1" stopColor="#c1c6b9" />
            </radialGradient>
          </defs>
          <circle
            cx="80"
            cy="80"
            r="54.4"
            fill={`url(#${id})`}
            stroke="#778c7e"
            strokeWidth="1.5"
          />
          <path
            d="M80 30v100M30 80h100"
            stroke="#7e8e82"
            strokeDasharray="3 4"
          />
          <circle
            cx="80"
            cy="80"
            r="49"
            fill="none"
            stroke="#9da797"
            strokeDasharray="2 5"
          />
          <g fill="#294a39" fontSize="12" textAnchor="middle">
            <text x="80" y="17">
              高杆
            </text>
            <text x="80" y="153">
              低杆
            </text>
            <text x="12" y="84">
              左
            </text>
            <text x="148" y="84">
              右
            </text>
          </g>
          <circle
            cx={80 + side * 54.4}
            cy={80 - top * 54.4}
            r="6"
            fill="#b84332"
            stroke="#fff"
            strokeWidth="2"
          />
        </svg>
      </button>
      <p>
        <strong>{label}</strong>
        <span>点击或拖动白球上的红点</span>
      </p>
      <button
        type="button"
        disabled={disabled}
        onClick={() => onChange({ top: 0, side: 0 })}
      >
        恢复中杆
      </button>
    </div>
  );
}
