import { useEffect, useRef } from "react";
import {
  DAS_DELAY,
  DAS_INTERVAL,
  SOFT_DROP_DELAY,
  SOFT_DROP_INTERVAL,
  type InputKind,
} from "./input";

type Props = {
  disabled: boolean;
  onInput: (kind: InputKind) => void;
};

export function Controls({ disabled, onInput }: Props) {
  const timers = useRef<{
    delay?: ReturnType<typeof setTimeout>;
    repeat?: ReturnType<typeof setInterval>;
  }>({});

  const stop = () => {
    if (timers.current.delay) clearTimeout(timers.current.delay);
    if (timers.current.repeat) clearInterval(timers.current.repeat);
    timers.current = {};
  };

  useEffect(() => stop, []);

  const start = (kind: InputKind, repeat: boolean) => {
    if (disabled) return;
    stop();
    onInput(kind);
    if (!repeat) return;
    const delay = kind === "softDrop" ? SOFT_DROP_DELAY : DAS_DELAY;
    const interval = kind === "softDrop" ? SOFT_DROP_INTERVAL : DAS_INTERVAL;
    timers.current.delay = setTimeout(() => {
      timers.current.repeat = setInterval(() => {
        if (!disabled) onInput(kind);
      }, interval);
    }, delay);
  };

  const button = (
    kind: InputKind,
    label: string,
    glyph: string,
    repeat = false,
    className = "",
  ) => (
    <button
      type="button"
      className={`spf2t-key ${className}`}
      aria-label={label}
      disabled={disabled}
      onPointerDown={(event) => {
        event.preventDefault();
        event.currentTarget.setPointerCapture?.(event.pointerId);
        start(kind, repeat);
      }}
      onPointerUp={stop}
      onPointerCancel={stop}
      onLostPointerCapture={stop}
      onContextMenu={(event) => event.preventDefault()}
    >
      <span aria-hidden="true">{glyph}</span>
      <small>{label}</small>
    </button>
  );

  return (
    <div className="spf2t-controls" role="group" aria-label="街霸方块操作">
      {button("left", "左移", "←", true)}
      {button("right", "右移", "→", true)}
      {button("rotateCCW", "逆时针", "↺")}
      {button("rotateCW", "顺时针", "↻")}
      {button("softDrop", "软降", "↓", true)}
      {button("hardDrop", "硬降", "⇓", false, "is-wide")}
    </div>
  );
}
