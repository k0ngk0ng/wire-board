import { useId, useRef, useState } from "react";
import { generateResetPassword } from "./password";

export function ResetPasswordField({ disabled }: { disabled: boolean }) {
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const [value, setValue] = useState("");
  const [generated, setGenerated] = useState(false);
  const [error, setError] = useState("");
  return (
    <div className="admin-password-field">
      <label htmlFor={id}>新密码</label>
      <div className="admin-password-row">
        <input
          ref={input}
          id={id}
          name="password"
          type={generated ? "text" : "password"}
          required
          minLength={12}
          maxLength={72}
          disabled={disabled}
          autoComplete="new-password"
          autoCapitalize="none"
          spellCheck={false}
          placeholder="至少 12 字节，最长 72 字节"
          value={value}
          onChange={(event) => setValue(event.target.value)}
        />
        <button
          type="button"
          className="outline"
          disabled={disabled}
          onClick={() => {
            try {
              const password = generateResetPassword();
              setValue(password);
              setGenerated(true);
              setError("");
              // Select after React has applied the new value and visible type.
              requestAnimationFrame(() => {
                input.current?.focus();
                input.current?.select();
              });
            } catch {
              setError("无法生成随机密码，请手动输入");
            }
          }}
        >
          随机生成
        </button>
      </div>
      {error && (
        <span className="admin-message error" role="alert">
          {error}
        </span>
      )}
    </div>
  );
}
