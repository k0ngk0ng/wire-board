import { useEffect, useRef, useState } from "react";
import { FCController } from "./FCController";
import {
  bindingKey,
  bindingLabel,
  defaultGamepadProfile,
  gamepadKey,
  readGamepads,
  type GamepadSnapshot,
} from "./gamepad";
import type {
  ControllerModel,
  ControllerProfile,
  GamepadBinding,
  GamepadProfile,
} from "./input";

function controls(
  pad: GamepadSnapshot,
  threshold = 0.65,
  buttonThreshold = threshold,
): GamepadBinding[] {
  return [
    ...pad.buttons.flatMap((v, index): GamepadBinding[] =>
      v >= buttonThreshold ? [{ type: "button", index }] : [],
    ),
    ...pad.axes.flatMap((v, index): GamepadBinding[] =>
      Math.abs(v) >= threshold
        ? [{ type: "axis", index, direction: v > 0 ? 1 : -1 }]
        : [],
    ),
  ];
}
type Recording = {
  button: string;
  append: boolean;
  ignored: Set<string>;
  device: string;
};

export function GamepadSettings({
  model,
  profile,
  busy,
  onChange,
  onRecording,
  onError,
}: {
  model: ControllerModel;
  profile: ControllerProfile;
  busy: boolean;
  onChange: (profile: ControllerProfile) => void;
  onRecording: (recording: boolean) => void;
  onError: (message: string) => void;
}) {
  const [pads, setPads] = useState<GamepadSnapshot[]>([]);
  const [apiError, setAPIError] = useState("");
  const [waiting, setWaiting] = useState("");
  const capture = useRef<Recording | null>(null);
  const current = useRef({
    model,
    profile,
    busy,
    onChange,
    onRecording,
    onError,
  });
  current.current = { model, profile, busy, onChange, onRecording, onError };
  const automatic =
    pads.find(
      (p) =>
        p.mapping === "standard" ||
        Object.values(profile.gamepads?.[gamepadKey(p)]?.bindings ?? {}).some(
          (bindings) => bindings.length > 0,
        ),
    ) ?? pads[0];
  const selected =
    profile.gamepadDevice ||
    (automatic
      ? gamepadKey(automatic)
      : (Object.keys(profile.gamepads ?? {})[0] ?? ""));
  const pad = pads.find((p) => gamepadKey(p) === selected);
  const standard = selected.startsWith("standard:");
  const mapping =
    profile.gamepads?.[selected] ??
    defaultGamepadProfile(
      { mapping: standard ? "standard" : "" },
      model.buttons.map((b) => b.id),
      model.standardGamepad,
    );
  const finish = () => {
    capture.current = null;
    setWaiting("");
    current.current.onRecording(false);
  };
  const saveMapping = (key: string, value: GamepadProfile) => {
    const p = current.current;
    p.onChange({
      ...p.profile,
      gamepads: { ...p.profile.gamepads, [key]: value },
    });
  };
  useEffect(() => {
    let frame = 0,
      signature = "",
      live = true;
    const poll = () => {
      if (!live) return;
      const state = readGamepads();
      const next = JSON.stringify(state);
      if (next !== signature) {
        signature = next;
        setPads(state.pads);
        setAPIError(state.error ?? "");
      }
      const recording = capture.current;
      if (recording) {
        const p = current.current;
        const device = state.pads.find(
          (d) => gamepadKey(d) === recording.device,
        );
        if (!device || p.busy || document.hidden) {
          finish();
        } else {
          const released = new Set(controls(device, 0.3).map(bindingKey));
          for (const key of recording.ignored)
            if (!released.has(key)) recording.ignored.delete(key);
          const fresh = controls(device).find(
            (binding) => !recording.ignored.has(bindingKey(binding)),
          );
          if (fresh) {
            recording.ignored.add(bindingKey(fresh));
            const map =
              p.profile.gamepads?.[recording.device] ??
              defaultGamepadProfile(
                device,
                p.model.buttons.map((b) => b.id),
                p.model.standardGamepad,
              );
            const conflict = Object.entries(map.bindings).find(
              ([logical, bindings]) =>
                logical !== recording.button &&
                bindings.some((b) => bindingKey(b) === bindingKey(fresh)),
            );
            if (fresh.index >= (fresh.type === "button" ? 64 : 16))
              p.onError("此输入编号超出支持范围，请选择其他按钮或轴。");
            else if (conflict)
              p.onError(
                `此输入已用于「${p.model.buttons.find((b) => b.id === conflict[0])?.label ?? conflict[0]}」，请先清除原绑定或选择其他输入。`,
              );
            else {
              const previous = recording.append
                ? (map.bindings[recording.button] ?? [])
                : [];
              if (previous.some((b) => bindingKey(b) === bindingKey(fresh))) {
                p.onError("这个输入已经绑定到当前操作。");
              } else if (previous.length >= 4) {
                p.onError("每个操作最多添加 4 个手柄输入。");
                finish();
              } else {
                saveMapping(recording.device, {
                  ...map,
                  bindings: {
                    ...map.bindings,
                    [recording.button]: [...previous, fresh],
                  },
                });
                p.onError("");
                finish();
              }
            }
          }
        }
      }
      frame = requestAnimationFrame(poll);
    };
    const escape = (event: KeyboardEvent) => {
      if (capture.current && event.code === "Escape") {
        event.preventDefault();
        event.stopImmediatePropagation();
        finish();
        current.current.onError("");
      }
    };
    frame = requestAnimationFrame(poll);
    window.addEventListener("keydown", escape, true);
    return () => {
      live = false;
      cancelAnimationFrame(frame);
      window.removeEventListener("keydown", escape, true);
      current.current.onRecording(false);
    };
  }, []);
  const begin = (button: string, append = false) => {
    if (!pad || busy) return;
    capture.current = {
      button,
      append,
      ignored: new Set(controls(pad, 0.3).map(bindingKey)),
      device: selected,
    };
    setWaiting(button);
    onRecording(true);
    onError("");
  };
  const devices = new Map(
    Object.keys(profile.gamepads ?? {}).map((key) => [
      key,
      key.slice(key.indexOf(":") + 1),
    ]),
  );
  for (const p of pads) devices.set(gamepadKey(p), p.id);
  if (
    profile.gamepadDevice &&
    profile.gamepadDevice !== "none" &&
    !devices.has(profile.gamepadDevice)
  )
    devices.set(
      profile.gamepadDevice,
      profile.gamepadDevice.slice(profile.gamepadDevice.indexOf(":") + 1),
    );
  const rawBindings = pad
    ? controls(pad, mapping.deadzone, model.continuous?.length ? 0.05 : 0.5)
    : [];
  const digitalRaw = new Set(
    (pad ? controls(pad, mapping.deadzone, 0.5) : []).map(bindingKey),
  );
  const continuousBindings = new Set(
    (model.continuous ?? []).flatMap((name) =>
      (mapping.bindings[name] ?? []).map(bindingKey),
    ),
  );
  const raw = new Set(rawBindings.map(bindingKey));
  const active = new Set(
    Object.entries(mapping.bindings)
      .filter(([name, bindings]) =>
        bindings.some((b) =>
          (model.continuous?.includes(name) ? raw : digitalRaw).has(
            bindingKey(b),
          ),
        ),
      )
      .map(([name]) => name),
  );
  const labels = Object.fromEntries(
    model.buttons.map(({ id }) => {
      const bindings = mapping.bindings[id] ?? [],
        first = bindings[0];
      const short = [
        "下",
        "右",
        "左",
        "上",
        "L1",
        "R1",
        "L2",
        "R2",
        "Sel",
        "Sta",
        "L3",
        "R3",
        "↑",
        "↓",
        "←",
        "→",
      ];
      const label = !first
        ? "—"
        : first.type === "button"
          ? (standard && short[first.index]) || `#${first.index + 1}`
          : `轴${first.index + 1}${first.direction < 0 ? "−" : "+"}`;
      return [id, label + (bindings.length > 1 ? "+" : "")];
    }),
  );
  return (
    <section className="gamepad-settings" aria-label="实体手柄配置">
      <label className="controller-model">
        使用手柄
        <select
          aria-label="使用手柄"
          value={profile.gamepadDevice ?? ""}
          disabled={busy}
          onChange={(event) => {
            finish();
            onError("");
            onChange({
              ...profile,
              gamepads: profile.gamepads ?? {},
              gamepadDevice: event.target.value,
            });
          }}
        >
          <option value="">自动选择可用手柄</option>
          <option value="none">不使用实体手柄</option>
          {[...devices].map(([key, name]) => (
            <option key={key} value={key}>
              {name}
              {pads.some((p) => gamepadKey(p) === key) ? "" : "（未连接）"}
            </option>
          ))}
        </select>
      </label>
      {apiError ? (
        <p className="controller-error" role="status">
          {apiError}
        </p>
      ) : (
        <p className="gamepad-connection" data-gamepad-connection role="status">
          {pad
            ? `${pad.id} · ${standard ? "标准映射" : "自定义映射"}`
            : selected === "none"
              ? "实体手柄输入已停用。"
              : selected
                ? `${selected.slice(selected.indexOf(":") + 1)} · 未连接`
                : "连接 USB／蓝牙手柄后，按一下手柄按钮以便浏览器识别。"}
        </p>
      )}
      {selected && selected !== "none" && (
        <>
          <p className="controller-help" aria-live="polite">
            {waiting
              ? `请操作「${model.buttons.find((b) => b.id === waiting)?.label}」的新按钮或摇杆方向；先松开已按住的输入，Esc 取消。`
              : standard ||
                  Object.values(mapping.bindings).some(
                    (bindings) => bindings.length > 0,
                  )
                ? "点按要修改的操作，再按手柄按钮或推动摇杆。"
                : "此手柄没有标准映射，请设置需要使用的操作。"}
          </p>
          {(model.layout ?? model.id) === "classic-2" && (
            <FCController
              profile={profile}
              mode="configure"
              bindingLabels={labels}
              activeButtons={active}
              recording={waiting}
              disabled={busy || !pad}
              onRemap={begin}
            />
          )}
          <output className="gamepad-live" data-gamepad-live>
            {rawBindings.length
              ? `输入：${rawBindings.map((binding) => bindingLabel(binding, standard) + (continuousBindings.has(bindingKey(binding)) ? ` ${Math.round(Math.abs(binding.type === "button" ? pad!.buttons[binding.index] : pad!.axes[binding.index]) * 100)}%` : "")).join("、")}`
              : "未按键"}
          </output>
          <details
            className="controller-map-details gamepad-map-details"
            open={(model.layout ?? model.id) !== "classic-2"}
          >
            <summary>查看手柄绑定明细</summary>
            <div className="gamepad-bindings">
              {model.buttons.map((button) => (
                <div
                  className="gamepad-binding"
                  key={button.id}
                  data-gamepad-binding={button.id}
                >
                  <span>{button.label}</span>
                  <button
                    className={`controller-key ${waiting === button.id ? "recording" : ""}`}
                    disabled={busy || !pad}
                    aria-label={`修改手柄 ${button.label}`}
                    onClick={() => begin(button.id)}
                  >
                    {waiting === button.id
                      ? "等待输入…"
                      : (mapping.bindings[button.id] ?? [])
                          .map((b) => bindingLabel(b, standard))
                          .join(" / ") || "未绑定"}
                  </button>
                  <button
                    className="gamepad-binding-action"
                    disabled={
                      busy ||
                      !pad ||
                      (mapping.bindings[button.id]?.length ?? 0) >= 4
                    }
                    aria-label={`添加手柄 ${button.label} 绑定`}
                    onClick={() => begin(button.id, true)}
                  >
                    添加
                  </button>
                  <button
                    className="gamepad-binding-action"
                    disabled={busy || !mapping.bindings[button.id]?.length}
                    aria-label={`清除手柄 ${button.label} 绑定`}
                    onClick={() => {
                      finish();
                      const bindings = { ...mapping.bindings };
                      delete bindings[button.id];
                      saveMapping(selected, { ...mapping, bindings });
                      onError("");
                    }}
                  >
                    清除
                  </button>
                </div>
              ))}
            </div>
          </details>
          <label className="controller-model">
            摇杆死区
            <select
              aria-label="摇杆死区"
              value={Math.round(mapping.deadzone * 100)}
              disabled={busy}
              onChange={(e) => {
                finish();
                saveMapping(selected, {
                  ...mapping,
                  deadzone: Number(e.target.value) / 100,
                });
              }}
            >
              {Array.from({ length: 15 }, (_, i) => i * 5 + 10).map((value) => (
                <option key={value} value={value}>
                  {value}%
                </option>
              ))}
            </select>
          </label>
          <button
            className="outline"
            disabled={busy}
            onClick={() => {
              finish();
              saveMapping(
                selected,
                defaultGamepadProfile(
                  { mapping: standard ? "standard" : "" },
                  model.buttons.map((b) => b.id),
                  model.standardGamepad,
                ),
              );
              onError("");
            }}
          >
            {standard ? "恢复此手柄默认映射" : "清空此手柄映射"}
          </button>
        </>
      )}
    </section>
  );
}
