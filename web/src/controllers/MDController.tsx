import { keyLabel, type ControllerProfile } from "./input";
import {
  useControllerPointers,
  type ControllerSink,
} from "./pointer-controller";
import background from "./assets/md-controller.svg";
import "./md-controller.css";
import "./touch-overlay.css";

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
  ac: "A+C",
  abc: "A+B+C",
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
  ac: "A+C",
  abc: "A+B+C",
};
const combos: Record<string, readonly string[]> = { ac: ["a", "c"], abc: ["a", "b", "c"] };
const bridges = [{ name: "ab", buttons: ["a", "b"] }, { name: "bc", buttons: ["b", "c"] }];

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
  const { active, pointers, press, end, lost, direction, down, surface } =
    useControllerPointers({ profile, mode, disabled, sink });
  const control = (button: string, dpad = false) => (
    <button
      key={button}
      type="button"
      data-controller-button={button}
      data-controller-combo={combos[button]?.join(" ")}
      className={`md-control md-${button}${combos[button] ? " md-combo" : ""}${(combos[button] ?? [button]).every(name => active.has(name) || activeButtons?.has(name)) ? " is-pressed" : ""}${recording === button ? " recording" : ""}`}
      aria-label={
        mode === "configure" ? `修改 ${labels[button]} 按键` : labels[button]
      }
      aria-pressed={mode === "play" ? (combos[button] ?? [button]).every(name => active.has(name)) : undefined}
      disabled={disabled}
      onClick={mode === "configure" ? () => onRemap?.(button) : undefined}
      onPointerDown={
        !dpad && mode === "play" ? (e) => down(e, combos[button] ?? [button]) : undefined
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
      ref={surface}
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
          {mode === "play" && [["up-left", "↖"], ["up-right", "↗"], ["down-left", "↙"], ["down-right", "↘"]].map(([name, glyph]) => <span key={name} className={`controller-diagonal controller-${name}`} aria-hidden="true">{glyph}</span>)}
        </div>
        {["mode", "start", "x", "y", "z", "a", "b", "c"].map((button) =>
          control(button),
        )}
        {mode === "play" && <>
          {["ac", "abc"].map(button => control(button))}
          {bridges.map(({ name, buttons }) => {
            return <span
              key={name}
              className={`controller-combo-bridge md-bridge-${name}${buttons.every(button => active.has(button)) ? " is-pressed" : ""}`}
              data-controller-combo={buttons.join(" ")}
              data-controller-bridge
              data-touch-protected
              aria-hidden="true"
              aria-disabled={disabled}
              onPointerDown={event => down(event, buttons)}
              onPointerUp={event => end(event, false)}
              onPointerCancel={event => end(event, true)}
              onLostPointerCapture={lost}
            />;
          })}
        </>}
      </div>
    </div>
  );
}
