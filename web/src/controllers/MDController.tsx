import { keyLabel, type ControllerProfile } from "./input";
import {
  useControllerPointers,
  type ControllerSink,
} from "./pointer-controller";
import background from "./assets/md-controller.svg";
import "./md-controller.css";

export type MDControllerProps = {
  profile: ControllerProfile;
  bindingLabels?: Record<string, string>;
  activeButtons?: ReadonlySet<string>;
  mode: "play" | "configure";
  recording?: string;
  disabled?: boolean;
  onRemap?: (button: string) => void;
  sink?: ControllerSink;
};
const labels: Record<string, string> = {
  up: "上",
  down: "下",
  left: "左",
  right: "右",
  mode: "模式（Mode）",
  start: "开始（Start）",
  a: "A",
  b: "B",
  c: "C",
  x: "X",
  y: "Y",
  z: "Z",
};
const names: Record<string, string> = {
  up: "↑",
  down: "↓",
  left: "←",
  right: "→",
  mode: "MODE",
  start: "START",
  a: "A",
  b: "B",
  c: "C",
  x: "X",
  y: "Y",
  z: "Z",
};

/** MD six-button faceplate; shared by settings and games. */
export function MDController({
  profile,
  bindingLabels,
  activeButtons,
  mode,
  recording = "",
  disabled = false,
  onRemap,
  sink,
}: MDControllerProps) {
  const { active, pointers, press, end, lost, direction, down } =
    useControllerPointers({ profile, mode, disabled, sink });
  const control = (button: string, dpad = false) => (
    <button
      key={button}
      type="button"
      data-controller-button={button}
      className={`md-control md-${button}${active.has(button) || activeButtons?.has(button) ? " is-pressed" : ""}${recording === button ? " recording" : ""}`}
      aria-label={
        mode === "configure" ? `修改 ${labels[button]} 按键` : labels[button]
      }
      aria-pressed={mode === "play" ? active.has(button) : undefined}
      disabled={disabled}
      onClick={mode === "configure" ? () => onRemap?.(button) : undefined}
      onPointerDown={
        !dpad && mode === "play" ? (e) => down(e, [button]) : undefined
      }
      onPointerUp={!dpad && mode === "play" ? (e) => end(e, false) : undefined}
      onPointerCancel={
        !dpad && mode === "play" ? (e) => end(e, true) : undefined
      }
      onLostPointerCapture={!dpad && mode === "play" ? lost : undefined}
    >
      {mode === "configure" ? (
        <>
          <span className="md-config-name">{names[button]}</span>
          <kbd>
            {recording === button
              ? "…"
              : (bindingLabels?.[button] ??
                keyLabel(profile.keyboard[button] ?? ""))}
          </kbd>
        </>
      ) : (
        names[button]
      )}
    </button>
  );
  return (
    <div
      className={`md-controller md-${mode}${disabled ? " is-disabled" : ""}`}
      data-controller-layout="md"
    >
      <div className="md-faceplate">
        <img
          className="md-background"
          src={background}
          alt="MD 六键手柄：左侧方向键，中间 Mode 与 Start，右侧上排 X、Y、Z，下排 A、B、C"
          draggable={false}
        />
        <div
          className="md-dpad"
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
          {["up", "down", "left", "right"].map((button) =>
            control(button, true),
          )}
        </div>
        {["mode", "start", "x", "y", "z", "a", "b", "c"].map((button) =>
          control(button),
        )}
      </div>
    </div>
  );
}
