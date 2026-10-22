import { useEffect, useState } from "react";
import {
  VersionMonitor,
  isEntryAsset,
  type ReleaseInfo,
} from "./version-update";

// Capture the entry document before React/game effects can add optional
// preload links or styles. Lazy game resources are not a new release.
const assets = Array.from(
  document.querySelectorAll<HTMLScriptElement | HTMLLinkElement>(
    "script[src],link[href]",
  ),
)
  .map(
    (element) =>
      element.getAttribute("src") ?? element.getAttribute("href") ?? "",
  )
  .filter(isEntryAsset);

export function VersionUpdateNotice() {
  const [update, setUpdate] = useState<ReleaseInfo | null>(null);
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => {
    const monitor = new VersionMonitor(
      assets,
      async (signal) => {
        const response = await fetch("/api/version", {
          cache: "no-store",
          credentials: "same-origin",
          signal,
        });
        if (!response.ok) throw new Error("Version unavailable");
        return response.json();
      },
      (info) => {
        setUpdate(info);
        setCollapsed(false);
      },
    );
    const check = () => {
      if (document.visibilityState === "visible") void monitor.check();
    };
    check();
    const timer = window.setInterval(check, 60000);
    document.addEventListener("visibilitychange", check);
    window.addEventListener("pageshow", check);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", check);
      window.removeEventListener("pageshow", check);
      monitor.stop();
    };
  }, []);
  if (!update) return null;
  if (collapsed)
    return (
      <button
        className="version-update-chip"
        onClick={() => setCollapsed(false)}
      >
        发现新版本 · 查看
      </button>
    );
  return (
    <aside
      className="version-update-notice"
      role="status"
      aria-live="polite"
      data-version-update={update.version}
    >
      <div>
        <strong>
          发现新版本 {update.version === "dev" ? "" : update.version}
        </strong>
        <p>刷新即可更新，可以等本局结束后再操作。</p>
      </div>
      <div className="version-update-actions">
        <button className="outline" onClick={() => setCollapsed(true)}>
          稍后
        </button>
        <button className="primary" onClick={() => window.location.reload()}>
          刷新更新
        </button>
      </div>
    </aside>
  );
}
