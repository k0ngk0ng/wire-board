import type { ButtonHTMLAttributes } from "react";
import "./room-actions.css";

export function RoomToolButton({
  label,
  children,
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { label: string }) {
  return (
    <button
      {...props}
      type="button"
      className={`subtle room-tool-button ${className}`}
      aria-label={label}
    >
      {children}
      <span className="room-tool-tooltip" aria-hidden="true">
        {label}
      </span>
    </button>
  );
}
