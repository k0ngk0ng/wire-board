import { useEffect, useRef, useState } from "react";
import type { PointerEvent } from "react";

type Stick = { x: number; y: number };

function stickAt(element: HTMLElement, clientX: number, clientY: number): Stick {
  const rect = element.getBoundingClientRect();
  const radius = Math.min(rect.width, rect.height) * 0.38;
  let x = Math.round((clientX - rect.left - rect.width / 2) * 1000 / radius);
  let y = Math.round((clientY - rect.top - rect.height / 2) * 1000 / radius);
  const length = Math.trunc(Math.sqrt(x * x + y * y));
  if (length > 1000) {
    x = Math.trunc(x * 1000 / length);
    y = Math.trunc(y * 1000 / length);
  }
  return length < 120 ? { x: 0, y: 0 } : { x, y };
}

function ControlStick({ kind, disabled, change, native }: { kind: "move" | "aim"; disabled: boolean; change: (value: Stick) => void; native: (event: Event) => void }) {
  const pointer = useRef<number | null>(null);
  const contact = useRef<number | null>(null);
  const surface = useRef<HTMLDivElement>(null);
  const current = useRef({ disabled, change, native });
  current.current = { disabled, change, native };
  const [stick, setStick] = useState<Stick>({ x: 0, y: 0 });
  function reset() {
    const id = pointer.current;
    pointer.current = null;
    contact.current = null;
    if (id !== null && surface.current?.hasPointerCapture(id)) surface.current.releasePointerCapture(id);
    setStick({ x: 0, y: 0 });
    current.current.change({ x: 0, y: 0 });
  }
  useEffect(() => {
    if (disabled) reset();
  }, [disabled]);
  useEffect(() => {
    const root = surface.current;
    if (!root) return;
    const hidden = () => { if (document.hidden) reset(); };
    const start = (event: TouchEvent) => {
      if (current.current.disabled || contact.current !== null) return;
      const point = Array.from(event.changedTouches).find(point => point.target instanceof Node && root.contains(point.target));
      if (!point) return;
      contact.current = point.identifier;
      const value = stickAt(root, point.clientX, point.clientY);
      setStick(value);
      current.current.native(event);
      current.current.change(value);
      if (event.cancelable) event.preventDefault();
    };
    const update = (event: TouchEvent) => {
      if (contact.current === null) return;
      const point = Array.from(event.touches).find(point => point.identifier === contact.current);
      if (point) {
        const value = stickAt(root, point.clientX, point.clientY);
        setStick(value);
        current.current.native(event);
        current.current.change(value);
      } else {
        current.current.native(event);
        reset();
      }
      if (event.cancelable) event.preventDefault();
    };
    const mode = matchMedia("(pointer: coarse)");
    root.addEventListener("touchstart", start, { passive: false });
    for (const type of ["touchmove", "touchend", "touchcancel"] as const) document.addEventListener(type, update, { capture: true, passive: false });
    mode.addEventListener("change", reset);
    window.addEventListener("blur", reset);
    window.addEventListener("resize", reset);
    window.addEventListener("orientationchange", reset);
    document.addEventListener("visibilitychange", hidden);
    return () => {
      root.removeEventListener("touchstart", start);
      for (const type of ["touchmove", "touchend", "touchcancel"] as const) document.removeEventListener(type, update, true);
      mode.removeEventListener("change", reset);
      window.removeEventListener("blur", reset);
      window.removeEventListener("resize", reset);
      window.removeEventListener("orientationchange", reset);
      document.removeEventListener("visibilitychange", hidden);
      reset();
    };
  }, []);
  function update(event: PointerEvent<HTMLDivElement>) {
    const value = stickAt(event.currentTarget, event.clientX, event.clientY);
    setStick(value);
    native(event.nativeEvent);
    change(value);
  }
  function release(event: PointerEvent<HTMLDivElement>) {
    if (pointer.current !== event.pointerId) return;
    native(event.nativeEvent);
    reset();
  }
  return <div ref={surface} className={`hm-stick hm-stick-${kind}`} role="group" aria-label={kind === "move" ? "移动摇杆" : "瞄准与攻击摇杆"} data-hotlinemiami-stick={kind} data-touch-control aria-disabled={disabled}
    onPointerDown={event => {
      if (event.pointerType === "touch" || disabled || pointer.current !== null) return;
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

export function TouchControls({ disabled, move, aim, edge, native }: { disabled: boolean; move: (stick: Stick) => void; aim: (stick: Stick) => void; edge: (action: "throw" | "pickup" | "finish" | "interact") => void; native: (event: Event) => void }) {
  return <div className="hm-touch-controls" aria-label="触屏操作">
    <ControlStick kind="move" disabled={disabled} change={move} native={native} />
    <div className="hm-touch-actions" data-touch-protected>
      <button type="button" disabled={disabled} aria-label="拾取武器" onPointerDown={event => { event.preventDefault(); native(event.nativeEvent); edge("pickup"); }}>拾取</button>
      <button type="button" disabled={disabled} aria-label="投掷武器" onPointerDown={event => { event.preventDefault(); native(event.nativeEvent); edge("throw"); }}>投掷</button>
      <button type="button" disabled={disabled} aria-label="处决倒地对手" onPointerDown={event => { event.preventDefault(); native(event.nativeEvent); edge("finish"); }}>处决</button>
      <button type="button" disabled={disabled} aria-label="开关门" onPointerDown={event => { event.preventDefault(); native(event.nativeEvent); edge("interact"); }}>开门</button>
    </div>
    <ControlStick kind="aim" disabled={disabled} change={aim} native={native} />
  </div>;
}
