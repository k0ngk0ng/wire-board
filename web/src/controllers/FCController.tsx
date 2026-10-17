import { useEffect, useRef, useState, type PointerEvent } from "react";
import { keyLabel, type ControllerProfile } from "./input";
import background from "./assets/fc-controller.svg";
import "./fc-controller.css";

export type FCControllerSink = {
  pressPointer(id: number, buttons: readonly string[]): void;
  releasePointer(id: number, cancelled: boolean): void;
};
export type FCControllerProps = {
  profile: ControllerProfile;
  mode: "play" | "configure";
  recording?: string;
  disabled?: boolean;
  onRemap?: (button: string) => void;
  sink?: FCControllerSink;
};
const labels: Record<string, string> = {
  up: "上",
  down: "下",
  left: "左",
  right: "右",
  select: "选择（Select）",
  start: "开始（Start）",
  b: "B",
  a: "A",
  turboB: "Turbo B",
  turboA: "Turbo A",
};
const playLabels: Record<string, string> = {
  up: "↑",
  down: "↓",
  left: "←",
  right: "→",
  select: "SELECT",
  start: "START",
  b: "B",
  a: "A",
  turboB: "Turbo B",
  turboA: "Turbo A",
};
type OwnedPointer = { buttons: readonly string[]; sink: FCControllerSink };

/** FC faceplate with paired upper turbo controls. Input and turbo timing belong to the platform sink. */
export function FCController({
  profile,
  mode,
  recording = "",
  disabled = false,
  onRemap,
  sink,
}: FCControllerProps) {
  const pointers = useRef(new Map<number, OwnedPointer>());
  const [active, setActive] = useState<Set<string>>(new Set());
  const current = useRef({ sink, disabled, mode });
  current.current = { sink, disabled, mode };
  const signature = JSON.stringify([
    profile.revision,
    profile.keyboard,
    profile.turboHz,
  ]);
  const refresh = () =>
    setActive(
      new Set([...pointers.current.values()].flatMap((p) => [...p.buttons])),
    );
  const release = (id: number, cancelled: boolean) => {
    const p = pointers.current.get(id);
    if (!p) return;
    pointers.current.delete(id);
    p.sink.releasePointer(id, cancelled);
    refresh();
  };
  const cancelAll = () => {
    for (const [id, p] of pointers.current) p.sink.releasePointer(id, true);
    pointers.current.clear();
    setActive(new Set());
  };
  useEffect(() => {
    cancelAll();
    // Only a real profile/mode/disabled change resets sources, not a fresh sink
    // object supplied by the game's normal rendering cycle.
  }, [signature, disabled, mode]);
  useEffect(() => {
    const blur = () => cancelAll();
    const visibility = () => {
      if (document.hidden) cancelAll();
    };
    window.addEventListener("blur", blur);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      window.removeEventListener("blur", blur);
      document.removeEventListener("visibilitychange", visibility);
      for (const [id, p] of pointers.current) p.sink.releasePointer(id, true);
      pointers.current.clear();
    };
  }, []);
  const press = (id: number, buttons: readonly string[]) => {
    const c = current.current;
    if (c.disabled || c.mode !== "play" || !c.sink) return;
    const previous = pointers.current.get(id);
    if (previous && previous.buttons.join(",") === buttons.join(",")) return;
    const owner = previous?.sink ?? c.sink;
    pointers.current.set(id, { buttons: [...buttons], sink: owner });
    owner.pressPointer(id, buttons);
    refresh();
  };
  const end = (event: PointerEvent<HTMLElement>, cancelled: boolean) => {
    release(event.pointerId, cancelled);
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
  };
  const lost = (event: PointerEvent<HTMLElement>) => {
    if (event.target === event.currentTarget) release(event.pointerId, true);
  };
  const direction = (event: PointerEvent<HTMLElement>): string[] => {
    const r = event.currentTarget.getBoundingClientRect();
    const x = ((event.clientX - r.left) / r.width) * 2 - 1;
    const y = ((event.clientY - r.top) / r.height) * 2 - 1;
    if (
      Math.abs(x) > 1 ||
      Math.abs(y) > 1 ||
      Math.max(Math.abs(x), Math.abs(y)) < 0.22
    )
      return [];
    const buttons: string[] = [];
    if (Math.abs(y) >= Math.abs(x) * 0.5) buttons.push(y < 0 ? "up" : "down");
    if (Math.abs(x) >= Math.abs(y) * 0.5)
      buttons.push(x < 0 ? "left" : "right");
    return buttons;
  };
  const down = (
    event: PointerEvent<HTMLElement>,
    buttons: readonly string[],
  ) => {
    if (event.button !== 0 || disabled || mode !== "play" || !sink) return;
    event.preventDefault(); // Keep the game's keyboard focus while pointing.
    event.currentTarget.setPointerCapture(event.pointerId);
    press(event.pointerId, buttons);
  };
  const control = (button: string, dpad = false) => {
    const waiting = recording === button;
    const keyboard = keyLabel(profile.keyboard[button] ?? "");
    return (
      <button
        key={button}
        type="button"
        data-controller-button={button}
        className={`fc-control fc-${button}${active.has(button) ? " is-pressed" : ""}${waiting ? " recording" : ""}${mode === "configure" ? " controller-key" : ""}`}
        aria-label={
          mode === "configure" ? `修改 ${labels[button]} 按键` : labels[button]
        }
        aria-pressed={mode === "play" ? active.has(button) : undefined}
        disabled={disabled}
        title={
          mode === "configure"
            ? `${labels[button]} · ${keyboard}`
            : labels[button]
        }
        onClick={mode === "configure" ? () => onRemap?.(button) : undefined}
        onPointerDown={
          !dpad && mode === "play" ? (e) => down(e, [button]) : undefined
        }
        onPointerUp={
          !dpad && mode === "play" ? (e) => end(e, false) : undefined
        }
        onPointerCancel={
          !dpad && mode === "play" ? (e) => end(e, true) : undefined
        }
        onLostPointerCapture={!dpad && mode === "play" ? lost : undefined}
      >
        {mode === "configure" ? (
          <kbd>{waiting ? "…" : keyboard}</kbd>
        ) : (
          <span
            className={`fc-play-name${dpad ? " fc-play-direction" : " fc-logical-sr-only"}`}
          >
            {playLabels[button]}
          </span>
        )}
      </button>
    );
  };
  return (
    <div
      className={`fc-controller fc-${mode}${disabled ? " is-disabled" : ""}`}
      data-controller-layout="fc"
    >
      <div className="fc-faceplate">
        <img
          className="fc-background"
          src={background}
          alt="红白机手柄：左十字方向键，中间 Select 与 Start，右侧上排 Turbo B、Turbo A，下排 B、A"
          draggable={false}
        />
        <div
          className="fc-dpad"
          data-controller-dpad
          role="group"
          aria-label="方向键"
          onPointerDown={
            mode === "play" ? (e) => down(e, direction(e)) : undefined
          }
          onPointerMove={
            mode === "play"
              ? (e) => {
                  if (pointers.current.has(e.pointerId)) {
                    e.preventDefault();
                    press(e.pointerId, direction(e));
                  }
                }
              : undefined
          }
          onPointerUp={mode === "play" ? (e) => end(e, false) : undefined}
          onPointerCancel={mode === "play" ? (e) => end(e, true) : undefined}
          onLostPointerCapture={mode === "play" ? lost : undefined}
        >
          {["up", "down", "left", "right"].map((b) => control(b, true))}
        </div>
        {["select", "start", "b", "a", "turboB", "turboA"].map((b) =>
          control(b),
        )}
      </div>
    </div>
  );
}
