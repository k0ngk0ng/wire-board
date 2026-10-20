import type { PointerEvent as ReactPointerEvent } from "react";
import type { InputKind } from "./input";

export function Controls({
  disabled,
  autoPlay,
  onAction,
  onHoldStart,
  onHoldEnd,
}: {
  disabled: boolean;
  autoPlay: boolean;
  onAction: (kind: InputKind) => void;
  onHoldStart: (kind: InputKind) => void;
  onHoldEnd: () => void;
}) {
  const held = (kind: InputKind) => ({
    onPointerDown: (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (disabled || event.button !== 0) return;
      event.preventDefault();
      event.currentTarget.setPointerCapture(event.pointerId);
      onHoldStart(kind);
    },
    onPointerUp: () => onHoldEnd(),
    onPointerCancel: () => onHoldEnd(),
    onLostPointerCapture: () => onHoldEnd(),
  });
  const tap = (kind: InputKind) => () => {
    if (!disabled) onAction(kind);
  };
  return (
    <div
      className={`tetris-controls ${autoPlay ? "is-auto" : ""}`}
      data-game-controls
      aria-label={autoPlay ? "电脑托管中" : "俄罗斯方块操作"}
    >
      <div className="tetris-control-row">
        <button
          type="button"
          aria-label="暂存方块"
          disabled={disabled}
          onClick={tap("hold")}
        >
          暂存<small>C／Shift</small>
        </button>
        <button
          type="button"
          aria-label="逆时针旋转"
          disabled={disabled}
          onClick={tap("rotateCCW")}
        >
          逆旋<small>Z</small>
        </button>
        <button
          type="button"
          aria-label="顺时针旋转"
          disabled={disabled}
          onClick={tap("rotateCW")}
        >
          顺旋<small>↑／X</small>
        </button>
      </div>
      <div className="tetris-control-row">
        <button
          type="button"
          aria-label="向左移动"
          disabled={disabled}
          {...held("left")}
        >
          ←<small>左移</small>
        </button>
        <button
          type="button"
          aria-label="软降"
          disabled={disabled}
          {...held("softDrop")}
        >
          ↓<small>软降</small>
        </button>
        <button
          type="button"
          aria-label="向右移动"
          disabled={disabled}
          {...held("right")}
        >
          →<small>右移</small>
        </button>
      </div>
      <button
        type="button"
        className="tetris-drop"
        aria-label="硬降"
        disabled={disabled}
        onClick={tap("hardDrop")}
      >
        硬降<small>空格</small>
      </button>
      {autoPlay && (
        <p className="tetris-auto-note">电脑托管中，可在上方取消托管</p>
      )}
    </div>
  );
}
