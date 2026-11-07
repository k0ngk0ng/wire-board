import { useRef, useState, type FormEvent } from "react";
import { api } from "../ui/board-host";
import "./password.css";

export function PasswordForm({
  onCancel,
  onChanged,
}: {
  onCancel: () => void;
  onChanged: () => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const pending = useRef(false);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (pending.current) return;
    const form = event.currentTarget;
    const fields = new FormData(form);
    const currentPassword = String(fields.get("currentPassword") ?? "");
    const newPassword = String(fields.get("newPassword") ?? "");
    const confirmation = String(fields.get("confirmPassword") ?? "");
    const bytes = new TextEncoder().encode(newPassword).length;
    if (bytes < 8 || bytes > 72) {
      setError("新密码需 8–72 字节");
      return;
    }
    if (newPassword !== confirmation) {
      setError("两次输入的新密码不一致");
      return;
    }
    if (currentPassword === newPassword) {
      setError("新密码不能与原密码相同");
      return;
    }
    pending.current = true;
    setBusy(true);
    setError("");
    try {
      await api("/account/password", { currentPassword, newPassword });
      form.reset();
      await onChanged();
    } catch (failure) {
      setError(
        failure instanceof Error ? failure.message : "密码未修改，请重试",
      );
    } finally {
      pending.current = false;
      setBusy(false);
    }
  };
  return (
    <form
      className="account-password-form"
      data-password-form
      onSubmit={submit}
    >
      <fieldset disabled={busy}>
        <label>
          原密码
          <input
            name="currentPassword"
            type="password"
            autoComplete="current-password"
            required
            maxLength={72}
            autoFocus
          />
        </label>
        <label>
          新密码 <small>8–72 字节</small>
          <input
            name="newPassword"
            type="password"
            autoComplete="new-password"
            required
            maxLength={72}
          />
        </label>
        <label>
          确认新密码
          <input
            name="confirmPassword"
            type="password"
            autoComplete="new-password"
            required
            maxLength={72}
          />
        </label>
      </fieldset>
      <p>修改后，其他设备需要重新登录。</p>
      {error && (
        <p className="account-password-error" role="alert" data-password-error>
          {error}
        </p>
      )}
      <div className="modal-actions">
        <button
          type="button"
          className="outline"
          disabled={busy}
          onClick={onCancel}
        >
          取消
        </button>
        <button
          type="submit"
          className="primary"
          disabled={busy}
          data-password-submit
        >
          {busy ? "正在修改…" : "修改密码"}
        </button>
      </div>
    </form>
  );
}
