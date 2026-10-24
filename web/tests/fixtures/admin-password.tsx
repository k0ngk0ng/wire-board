import React, { useState } from "react";
import { createRoot } from "react-dom/client";
import { ResetPasswordField } from "../../src/admin/password-field";
import "../../src/style.css";
import "../../src/admin.css";
function App() {
  const [busy, setBusy] = useState(false);
  return (
    <main className="admin-page">
      <div className="modal-backdrop">
        <div
          className="modal admin-confirm"
          role="dialog"
          aria-label="重置密码"
        >
          <div className="modal-head">
            <h2>重置密码 · 测试账号</h2>
          </div>
          <p>设置新密码后，原有登录会话全部失效。请私下把新密码告知玩家。</p>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              (window as any).submissions++;
              setBusy(true);
            }}
          >
            <ResetPasswordField disabled={busy} />
            <div className="admin-confirm-actions">
              <button
                type="button"
                className="outline"
                onClick={() => location.reload()}
              >
                取消
              </button>
              <button className="primary" disabled={busy}>
                确认操作
              </button>
            </div>
          </form>
        </div>
      </div>
    </main>
  );
}
(window as any).submissions = 0;
createRoot(document.getElementById("root")!).render(<App />);
