import { useEffect, useRef, useState } from "react";
import { Modal, api } from "../ui/board-host";
import {
  keyLabel,
  type ControllerProfile,
  type ControllerSettingsData,
} from "./input";
import { FCController } from "./FCController";
import "./settings.css";

export function ControllerSettings({
  userId,
  onClose,
}: {
  userId: string;
  onClose: () => void;
}) {
  const [data, setData] = useState<ControllerSettingsData>();
  const [modelID, setModelID] = useState("classic-2");
  const [drafts, setDrafts] = useState<Record<string, ControllerProfile>>({});
  const [waiting, setWaiting] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    let alive = true;
    void api("/controllers")
      .then((result: ControllerSettingsData) => {
        if (alive) {
          setData(result);
          setDrafts(structuredClone(result.profiles));
        }
      })
      .catch((e: Error) => {
        if (alive) setError(e.message);
      });
    return () => {
      alive = false;
      mounted.current = false;
    };
  }, []);
  const model = data?.models.find((m) => m.id === modelID);
  const draft = drafts[modelID];
  useEffect(() => {
    if (!waiting || !draft) return;
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
        Object.entries(draft.keyboard).some(
          ([button, code]) => button !== waiting && code === event.code,
        )
      ) {
        setError("每个按键只能对应一个操作，请换一个按键。");
        return;
      }
      if (
        !/^(Key[A-Z]|Digit[0-9]|Numpad[0-9]|Arrow(Up|Down|Left|Right)|Space|Enter|Tab|Backspace|Shift(Left|Right)|Control(Left|Right)|Alt(Left|Right)|Bracket(Left|Right)|Semicolon|Quote|Comma|Period|Slash|Backslash|Minus|Equal|Backquote|F([1-9]|1[0-2]))$/.test(
          event.code,
        )
      ) {
        setError("这个按键暂不支持，请选择其他按键。");
        return;
      }
      setDrafts((all) => ({
        ...all,
        [modelID]: {
          ...draft,
          keyboard: { ...draft.keyboard, [waiting]: event.code },
        },
      }));
      setWaiting("");
      setError("");
      setNotice("");
    };
    window.addEventListener("keydown", capture, true);
    return () => window.removeEventListener("keydown", capture, true);
  }, [waiting, draft, modelID]);
  const beginRemap = (button: string) => {
    setWaiting(button);
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
      const saved: ControllerProfile = await api(
        `/controllers/${modelID}`,
        draft,
      );
      if (!mounted.current) return;
      setDrafts((all) => ({ ...all, [modelID]: saved }));
      setData((old) =>
        old ? { ...old, profiles: { ...old.profiles, [modelID]: saved } } : old,
      );
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
      <p className="controller-intro">
        每种型号分别保存你的键位，登录后可继续使用。不会影响其他玩家。
      </p>
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
      {model && draft && (
        <>
          <label className="controller-model">
            控制器型号
            <select
              aria-label="控制器型号"
              value={modelID}
              disabled={busy}
              onChange={(e) => {
                setModelID(e.target.value);
                setWaiting("");
                setError("");
                setNotice("");
              }}
            >
              {data?.models.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name}
                </option>
              ))}
            </select>
          </label>
          <p className="controller-help" aria-live="polite">
            {waiting
              ? `请按下「${model.buttons.find((b) => b.id === waiting)?.label}」的新按键，Esc 取消。`
              : modelID === "classic-2"
                ? "点击手柄上的按键，再按下新的键盘按键。"
                : "点击要修改的按键，再按下新的键盘按键。"}
          </p>
          {modelID === "classic-2" ? (
            <>
              <FCController
                profile={draft}
                mode="configure"
                recording={waiting}
                disabled={busy}
                onRemap={beginRemap}
              />
              <details className="controller-map-details">
                <summary>查看键位明细</summary>
                <dl className="controller-map-readonly">
                  {model.buttons.map((button) => (
                    <div key={button.id}>
                      <dt>{button.label}</dt>
                      <dd>{keyLabel(draft.keyboard[button.id])}</dd>
                    </div>
                  ))}
                </dl>
              </details>
            </>
          ) : (
            <div className="controller-keys">
              {model.buttons.map((button) => (
                <div className="controller-key-row" key={button.id}>
                  <span>{button.label}</span>
                  <button
                    type="button"
                    className={`controller-key ${waiting === button.id ? "recording" : ""}`}
                    aria-label={`修改 ${button.label} 按键`}
                    disabled={busy}
                    onClick={() => beginRemap(button.id)}
                  >
                    {waiting === button.id
                      ? "请按键…"
                      : keyLabel(draft.keyboard[button.id])}
                  </button>
                </div>
              ))}
            </div>
          )}
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
          <div className="controller-actions">
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
                    ...structuredClone(model.defaults),
                    revision: draft.revision,
                  },
                }));
              }}
            >
              恢复此型号默认键位
            </button>
            <button
              className="primary"
              data-controller-save
              disabled={busy || !!waiting}
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
