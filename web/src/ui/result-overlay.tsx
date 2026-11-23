import { useState, type ReactNode } from "react";
import { ArrowLeft, Crown, Eye, RotateCcw } from "lucide-react";
import { Modal } from "./board-host";
import "./room-actions.css";

export function ResultOverlay({
  children,
  busy,
  onReturn,
  onRestart,
  waitingForHost,
}: {
  children: ReactNode;
  busy: boolean;
  onReturn?: () => void;
  onRestart?: () => void;
  waitingForHost: boolean;
}) {
  const [open, setOpen] = useState(true);
  return open ? (
    <Modal
      title="本局结果"
      className="match-result-modal"
      dismissible={false}
      initialFocus=".result-review"
      onClose={() => setOpen(false)}
    >
      {children}
      {waitingForHost && (
        <p className="result-waiting" role="status">
          等待房主开启下一局
        </p>
      )}
      <div className="result-actions">
        <button
          type="button"
          className="outline result-review"
          onClick={() => setOpen(false)}
        >
          <Eye size={17} />
          查看最终画面
        </button>
        {onReturn && (
          <button
            type="button"
            className="primary result-return"
            disabled={busy}
            onClick={onReturn}
          >
            <ArrowLeft size={17} />
            返回房间
          </button>
        )}
        {onRestart && (
          <button type="button" className="primary result-restart" disabled={busy} onClick={onRestart}>
            <RotateCcw size={17} />
            再来一局
          </button>
        )}
      </div>
    </Modal>
  ) : (
    <button
      type="button"
      className="result-reopen"
      aria-haspopup="dialog"
      onClick={() => setOpen(true)}
    >
      <Crown size={18} />
      查看结果
    </button>
  );
}
