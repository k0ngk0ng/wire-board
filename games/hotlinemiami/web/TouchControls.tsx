import { useEffect, useRef, useState } from "react";
import type { PointerEvent } from "react";

type Stick = { x: number; y: number };

function ControlStick({ kind, disabled, change, native }: { kind: "move" | "aim"; disabled: boolean; change: (value: Stick) => void; native: (event: globalThis.PointerEvent) => void }) {
  const pointer = useRef<number | null>(null);
  const [stick, setStick] = useState<Stick>({ x: 0, y: 0 });
  useEffect(() => {
    if (!disabled) return;
    pointer.current = null;
    setStick({ x: 0, y: 0 });
    change({ x: 0, y: 0 });
  }, [disabled]);
  function update(event: PointerEvent<HTMLDivElement>) {
    const rect = event.currentTarget.getBoundingClientRect();
    const radius = Math.min(rect.width, rect.height) * 0.38;
    let x = Math.round((event.clientX - rect.left - rect.width / 2) * 1000 / radius);
    let y = Math.round((event.clientY - rect.top - rect.height / 2) * 1000 / radius);
    const length = Math.trunc(Math.sqrt(x * x + y * y));
    if (length > 1000) {
      x = Math.trunc(x * 1000 / length);
      y = Math.trunc(y * 1000 / length);
    }
    if (length < 120) { x = 0; y = 0; }
    setStick({ x, y });
    change({ x, y });
    native(event.nativeEvent);
  }
  function release(event: PointerEvent<HTMLDivElement>) {
    if (pointer.current !== event.pointerId) return;
    pointer.current = null;
    setStick({ x: 0, y: 0 });
    change({ x: 0, y: 0 });
    native(event.nativeEvent);
  }
  return <div className={`hm-stick hm-stick-${kind}`} role="group" aria-label={kind === "move" ? "移动摇杆" : "瞄准与攻击摇杆"} data-hotlinemiami-stick={kind} aria-disabled={disabled}
    onPointerDown={event => {
      if (disabled || pointer.current !== null) return;
      event.preventDefault();
      pointer.current = event.pointerId;
      event.currentTarget.setPointerCapture(event.pointerId);
      update(event);
    }}
    onPointerMove={event => { if (pointer.current === event.pointerId) { event.preventDefault(); update(event); } }}
    onPointerUp={release} onPointerCancel={release} onLostPointerCapture={release}>
    <span className="hm-stick-orbit" />
    <span className="hm-stick-thumb" style={{ transform: `translate(${stick.x * 0.025}px, ${stick.y * 0.025}px)` }} />
    <span className="hm-stick-name">{kind === "move" ? "移动" : "瞄准 · 攻击"}</span>
  </div>;
}

export function TouchControls({ disabled, move, aim, edge, native }: { disabled: boolean; move: (stick: Stick) => void; aim: (stick: Stick) => void; edge: (action: "throw" | "pickup" | "finish" | "interact") => void; native: (event: globalThis.PointerEvent) => void }) {
  return <div className="hm-touch-controls" aria-label="触屏操作">
    <ControlStick kind="move" disabled={disabled} change={move} native={native} />
    <div className="hm-touch-actions">
      <button type="button" disabled={disabled} aria-label="拾取武器" onPointerDown={event => { event.preventDefault(); native(event.nativeEvent); edge("pickup"); }}>拾取</button>
      <button type="button" disabled={disabled} aria-label="投掷武器" onPointerDown={event => { event.preventDefault(); native(event.nativeEvent); edge("throw"); }}>投掷</button>
      <button type="button" disabled={disabled} aria-label="处决倒地对手" onPointerDown={event => { event.preventDefault(); native(event.nativeEvent); edge("finish"); }}>处决</button>
      <button type="button" disabled={disabled} aria-label="开关门" onPointerDown={event => { event.preventDefault(); native(event.nativeEvent); edge("interact"); }}>开门</button>
    </div>
    <ControlStick kind="aim" disabled={disabled} change={aim} native={native} />
  </div>;
}
