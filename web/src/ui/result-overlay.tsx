import { useState, type ReactNode } from "react";
import { Crown } from "lucide-react";
import { Modal } from "./board-host";

// Mounted once per completed match. Ordinary snapshot refreshes must not
// reopen a result the player dismissed while reviewing the final board.
export function ResultOverlay({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(true);
  return open ? (
    <Modal
      title="本局结果"
      className="match-result-modal"
      onClose={() => setOpen(false)}
    >
      {children}
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
