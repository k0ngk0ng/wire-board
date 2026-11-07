import { useRef, useState } from "react";
import { RefreshCw } from "lucide-react";
export function RefreshButton({
  onRefresh,
  disabled = false,
  className = "outline admin-list-refresh",
  label = "刷新",
}: {
  onRefresh: () => Promise<unknown>;
  disabled?: boolean;
  className?: string;
  label?: string;
}) {
  const [pending, setPending] = useState(false);
  const active = useRef(false);
  return (
    <button
      type="button"
      className={className}
      disabled={disabled || pending}
      aria-busy={pending}
      onClick={async () => {
        if (active.current) return;
        active.current = true;
        setPending(true);
        try {
          await onRefresh();
        } finally {
          active.current = false;
          setPending(false);
        }
      }}
    >
      <RefreshCw size={15} className={pending ? "refresh-spinning" : ""} />
      {pending ? "正在刷新…" : label}
    </button>
  );
}
