import { X } from "lucide-react";
import React, { createContext, useEffect, useRef } from "react";
export const AssetsContext = createContext("");
export async function api(path: string, body?: unknown) {
  const res = await fetch("/api" + path, {
    method: body ? "POST" : "GET",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json();
  if (!res.ok)
    throw Object.assign(new Error(data.error || "请求失败"), {
      status: res.status,
    });
  return data;
}
export function Modal({
  title,
  children,
  onClose,
  dismissible = true,
  className = "",
}: {
  title: string;
  dismissible?: boolean;
  className?: string;
  children: React.ReactNode;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const prev = document.activeElement as HTMLElement;
    const el = ref.current;
    el?.querySelector<HTMLElement>("input,button,select")?.focus();
    function key(e: KeyboardEvent) {
      if (el?.closest("[inert]")) return;
      if (e.key === "Escape" && dismissible) {
        // Dismiss the overlay without invoking the browser's stop-loading
        // shortcut, which can interrupt requests while reviewing a result.
        e.preventDefault();
        onClose();
      }
      if (e.key === "Tab" && el) {
        const nodes = Array.from(
          el.querySelectorAll<HTMLElement>(
            'button:not(:disabled),input,select,[tabindex="0"]',
          ),
        ).filter(
          (node) =>
            node.getClientRects().length > 0 &&
            !node.matches(":disabled") &&
            !node.closest("[inert]") &&
            getComputedStyle(node).visibility !== "hidden",
        );
        if (!nodes.length) return;
        const first = nodes[0],
          last = nodes[nodes.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("keydown", key);
      prev?.focus();
    };
  }, []);
  return (
    <div
      className="modal-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget && dismissible) onClose();
      }}
    >
      <div
        ref={ref}
        className={`modal ${className}`}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <div className="modal-head">
          <h2>{title}</h2>
          {dismissible && (
            <button className="icon-button" onClick={onClose} aria-label="关闭">
              <X size={20} />
            </button>
          )}
        </div>
        {children}
      </div>
    </div>
  );
}
