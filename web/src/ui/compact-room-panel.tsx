import { useEffect, useState, type ReactNode } from "react";
import { Modal } from "./board-host";

// Platform-owned compact room chrome. Games never hide shared controls;
// on narrow poker tables the same controls stay available in labelled dialogs.
export function CompactRoomPanel({
  enabled,
  label,
  name,
  children,
}: {
  enabled: boolean;
  label: string;
  name: string;
  children: ReactNode;
}) {
  const [mobile, setMobile] = useState(
    () => matchMedia("(max-width: 600px)").matches,
  );
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const query = matchMedia("(max-width: 600px)");
    const update = () => {
      setMobile(query.matches);
      setOpen(false);
    };
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);
  if (!enabled || !mobile) return <>{children}</>;
  return (
    <>
      <button
        className={`compact-room-button compact-room-${name}`}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        {label}
      </button>
      {open && (
        <Modal
          title={label}
          onClose={() => setOpen(false)}
          className="compact-room-dialog"
        >
          <div
            onClick={(e) => {
              if ((e.target as HTMLElement).closest("button")) setOpen(false);
            }}
          >
            {children}
          </div>
        </Modal>
      )}
    </>
  );
}
