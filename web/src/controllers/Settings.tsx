import { useEffect, useRef, useState } from "react";
import { Modal, api } from "../ui/board-host";
import {
  actionsForKey,
  keyLabel,
  type ControllerProfile,
  type ControllerSettingsData,
} from "./input";
import { FCController } from "./FCController";
import { GamepadSettings } from "./GamepadSettings";
import "./settings.css";

export function ControllerSettings({
  userId,
  gameId,
  availableGames,
  onClose,
}: {
  userId: string;
  gameId?: string;
  availableGames?: string[];
  onClose: () => void;
}) {
  const [data, setData] = useState<ControllerSettingsData>();
  const [modelID, setModelID] = useState(gameId ? `game:${gameId}` : "");
  const [drafts, setDrafts] = useState<Record<string, ControllerProfile>>({});
  const [source, setSource] = useState("keyboard"),
    [search, setSearch] = useState("");
  const [waiting, setWaiting] = useState(""),
    [appendKey, setAppendKey] = useState(false),
    [gamepadRecording, setGamepadRecording] = useState(false);
  const [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false);
  const mounted = useRef(true);
  const choices = (value: ControllerSettingsData) => {
    const games = value.models.filter((m) => m.gameId);
    return games.length
      ? games.filter(
          (m) =>
            !availableGames ||
            availableGames.includes(m.gameId!) ||
            m.gameId === gameId,
        )
      : value.models;
  };
  useEffect(() => {
    mounted.current = true;
    let alive = true;
    void api("/controllers")
      .then((result: ControllerSettingsData) => {
        if (!alive) return;
        setData(result);
        setDrafts(structuredClone(result.profiles));
        const models = choices(result);
        setModelID((current) =>
          models.some((m) => m.id === current)
            ? current
            : (models[0]?.id ?? ""),
        );
      })
      .catch((e: Error) => {
        if (alive) setError(e.message);
      });
    return () => {
      alive = false;
      mounted.current = false;
    };
  }, []);
  const models = data ? choices(data) : [];
  const filtered = models.filter((m) =>
    `${m.name} ${m.gameId ?? m.id}`
      .toLowerCase()
      .includes(search.trim().toLowerCase()),
  );
  const model = data?.models.find((m) => m.id === modelID),
    draft = drafts[modelID];
  const classic = (model?.layout ?? model?.id) === "classic-2";
  useEffect(() => {
    if (source !== "keyboard" || !waiting || !draft) return;
    const capture = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopImmediatePropagation();
      if (event.repeat || event.isComposing) return;
      if (event.code === "Escape") {
        setWaiting("");
        setError("");
        return;
      }
      if (
        actionsForKey(draft, event.code).some((action) => action !== waiting)
      ) {
        setError("每个按键只能对应一个操作，请换一个按键。");
        return;
      }
      if (
        !/^(Key[A-Z]|Digit[0-9]|Numpad[0-9]|Arrow(Up|Down|Left|Right)|Space|Enter|Tab|Backspace|Delete|Home|End|PageUp|PageDown|Shift(Left|Right)|Control(Left|Right)|Alt(Left|Right)|Bracket(Left|Right)|Semicolon|Quote|Comma|Period|Slash|Backslash|Minus|Equal|Backquote|F([1-9]|1[0-2]))$/.test(
          event.code,
        )
      ) {
        setError("这个按键暂不支持，请选择其他按键。");
        return;
      }
      if (appendKey && actionsForKey(draft, event.code).includes(waiting)) {
        setError("这个按键已经绑定到当前操作。");
        return;
      }
      const alternate = (draft.keyboardAlt?.[waiting] ?? []).filter(
        (code) => code !== event.code,
      );
      if (appendKey && draft.keyboard[waiting] && alternate.length >= 4) {
        setError("每个操作最多添加 4 个替代按键。");
        return;
      }
      const next = {
        ...draft,
        keyboard: { ...draft.keyboard },
        keyboardAlt: { ...draft.keyboardAlt },
      };
      if (appendKey && draft.keyboard[waiting])
        next.keyboardAlt[waiting] = [...alternate, event.code];
      else {
        next.keyboard[waiting] = event.code;
        next.keyboardAlt[waiting] = alternate;
      }
      setDrafts((all) => ({ ...all, [modelID]: next }));
      setWaiting("");
      setError("");
      setNotice("");
    };
    window.addEventListener("keydown", capture, true);
    return () => window.removeEventListener("keydown", capture, true);
  }, [source, waiting, draft, modelID, appendKey]);
  const begin = (action: string, append = false) => {
    setWaiting(action);
    setAppendKey(append);
    setError("");
    setNotice("");
  };
  const save = async () => {
    if (!draft || busy) return;
    setBusy(true);
    setWaiting("");
    setError("");
    setNotice("");
    try {
      const saved: ControllerProfile = await api(`/controllers/${modelID}`, {
        ...draft,
        keyboardAlt: draft.keyboardAlt ?? {},
        gamepads: draft.gamepads ?? {},
      });
      if (!mounted.current) return;
      setDrafts((all) => ({ ...all, [modelID]: saved }));
      window.dispatchEvent(
        new CustomEvent("controller-settings-changed", {
          detail: { userId, model: modelID, profile: saved },
        }),
      );
      setNotice("控制器设置已保存。");
    } catch (e) {
      if (mounted.current) setError((e as Error).message);
    } finally {
      if (mounted.current) setBusy(false);
    }
  };
  return (
    <Modal
      title="控制器设置"
      onClose={onClose}
      className="controller-settings"
      dismissible={!busy}
    >
      {error && (
        <p className="controller-error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="controller-notice" role="status">
          {notice}
        </p>
      )}
      {!data && !error && <p>正在读取控制器设置…</p>}
      {data && (
        <>
          <input
            type="search"
            aria-label="搜索游戏控制方案"
            placeholder="搜索游戏"
            value={search}
            disabled={busy}
            onChange={(e) => {
              setSearch(e.target.value);
              setWaiting("");
            }}
          />
          <label className="controller-model">
            {models.some((m) => m.gameId) ? "游戏" : "操作方案"}
            <select
              data-controller-model
              aria-label="游戏／操作方案"
              value={search ? "" : modelID}
              disabled={busy || !filtered.length}
              onChange={(e) => {
                setModelID(e.target.value);
                setSearch("");
                setWaiting("");
                setError("");
                setNotice("");
              }}
            >
              {search && (
                <option value="" disabled>
                  选择游戏
                </option>
              )}
              {filtered.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
            </select>
          </label>
          {!filtered.length && (
            <p className="controller-help" role="status">
              没有匹配的游戏。
              {search && (
                <button
                  type="button"
                  className="outline"
                  onClick={() => setSearch("")}
                >
                  清除搜索
                </button>
              )}
            </p>
          )}
        </>
      )}
      {model && draft && !search && (
        <>
          <label className="controller-model">
            配置输入
            <select
              aria-label="输入来源"
              value={source}
              disabled={busy}
              onChange={(e) => {
                setSource(e.target.value);
                setWaiting("");
                setError("");
                setNotice("");
              }}
            >
              <option value="keyboard">键盘</option>
              <option value="gamepad">实体手柄</option>
            </select>
          </label>
          {source === "keyboard" ? (
            <>
              <p className="controller-help" aria-live="polite">
                {waiting
                  ? `请按下「${model.buttons.find((b) => b.id === waiting)?.label}」的新按键，Esc 取消。`
                  : "点击要修改的操作，再按下键盘按键。"}
              </p>
              {classic && (
                <FCController
                  profile={draft}
                  mode="configure"
                  recording={waiting}
                  disabled={busy}
                  onRemap={begin}
                  bindingLabels={Object.fromEntries(
                    model.buttons.map((b) => [
                      b.id,
                      (keyLabel(draft.keyboard[b.id]) || "—") +
                        (draft.keyboardAlt?.[b.id]?.length ? "+" : ""),
                    ]),
                  )}
                />
              )}
              <details
                className="controller-map-details"
                open={!classic}
                key={modelID + "-keyboard"}
              >
                <summary>查看按键绑定</summary>
                <div className="gamepad-bindings">
                  {model.buttons.map((button) => (
                    <div className="gamepad-binding" key={button.id}>
                      <span>{button.label}</span>
                      <button
                        type="button"
                        className={`controller-key ${waiting === button.id ? "recording" : ""}`}
                        aria-label={`修改 ${button.label} 按键`}
                        disabled={busy}
                        onClick={() => begin(button.id)}
                      >
                        {waiting === button.id
                          ? "请按键…"
                          : [
                              draft.keyboard[button.id],
                              ...(draft.keyboardAlt?.[button.id] ?? []),
                            ]
                              .filter(Boolean)
                              .map(keyLabel)
                              .join(" / ") || "未绑定"}
                      </button>
                      <button
                        type="button"
                        className="gamepad-binding-action"
                        aria-label={`添加 ${button.label} 替代按键`}
                        disabled={busy}
                        onClick={() => begin(button.id, true)}
                      >
                        添加
                      </button>
                      <button
                        type="button"
                        className="gamepad-binding-action"
                        aria-label={`清除 ${button.label} 按键`}
                        disabled={busy}
                        onClick={() => {
                          setWaiting("");
                          setDrafts((all) => ({
                            ...all,
                            [modelID]: {
                              ...draft,
                              keyboard: { ...draft.keyboard, [button.id]: "" },
                              keyboardAlt: {
                                ...draft.keyboardAlt,
                                [button.id]: [],
                              },
                            },
                          }));
                          setNotice("");
                        }}
                      >
                        清除
                      </button>
                    </div>
                  ))}
                </div>
              </details>
            </>
          ) : (
            <GamepadSettings
              key={`${userId}:${modelID}`}
              model={model}
              profile={draft}
              busy={busy}
              onRecording={setGamepadRecording}
              onError={setError}
              onChange={(next) => {
                setDrafts((all) => ({ ...all, [modelID]: next }));
                setError("");
                setNotice("");
              }}
            />
          )}
          {(model.buttons.some((b) => b.id === "turboA" || b.id === "turboB") ||
            Object.keys(model.turboTargets ?? {}).length > 0) && (
            <label className="controller-model controller-turbo">
              连发速度
              <select
                aria-label="连发速度"
                value={draft.turboHz}
                disabled={busy}
                onChange={(e) => {
                  setDrafts((all) => ({
                    ...all,
                    [modelID]: { ...draft, turboHz: Number(e.target.value) },
                  }));
                  setNotice("");
                }}
              >
                {Array.from({ length: 14 }, (_, i) => i + 2).map((hz) => (
                  <option key={hz} value={hz}>
                    每秒 {hz} 次
                  </option>
                ))}
              </select>
            </label>
          )}
          <div className="controller-actions">
            {source === "keyboard" ? (
              <button
                className="secondary"
                disabled={busy}
                onClick={() => {
                  setWaiting("");
                  setError("");
                  setNotice("");
                  setDrafts((all) => ({
                    ...all,
                    [modelID]: {
                      ...draft,
                      keyboard: structuredClone(model.defaults.keyboard),
                      keyboardAlt: structuredClone(
                        model.defaults.keyboardAlt ?? {},
                      ),
                      turboHz: model.defaults.turboHz,
                    },
                  }));
                }}
              >
                恢复默认键位
              </button>
            ) : (
              <span />
            )}
            <button
              className="primary"
              data-controller-save
              disabled={busy || !!waiting || gamepadRecording}
              onClick={() => void save()}
            >
              {busy ? "保存中…" : "保存设置"}
            </button>
          </div>
        </>
      )}
    </Modal>
  );
}
