import {
  useControllerPointers,
  type ControllerSink,
} from "./pointer-controller";
import { keyLabel, type ControllerProfile } from "./input";
import background from "./assets/fc-controller.svg";
import "./fc-controller.css";

export type FCControllerSink = ControllerSink;
export type FCControllerProps = {
  profile: ControllerProfile;
  bindingLabels?: Record<string, string>;
  activeButtons?: ReadonlySet<string>;
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

/** FC faceplate with paired upper turbo controls. Input and turbo timing belong to the platform sink. */
export function FCController({
  profile,
  bindingLabels,
  activeButtons,
  mode,
  recording = "",
  disabled = false,
  onRemap,
  sink,
}: FCControllerProps) {
  const { active, pointers, press, end, lost, direction, down } =
    useControllerPointers({ profile, mode, disabled, sink });
  const control = (button: string, dpad = false) => {
    const waiting = recording === button;
    const keyboard =
      bindingLabels?.[button] ?? keyLabel(profile.keyboard[button] ?? "");
    const pressed = active.has(button) || !!activeButtons?.has(button);
    return (
      <button
        key={button}
        type="button"
        data-controller-button={button}
        className={`fc-control fc-${button}${pressed ? " is-pressed" : ""}${waiting ? " recording" : ""}${mode === "configure" ? " controller-key" : ""}`}
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
