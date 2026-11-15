import { useLayoutEffect, useState } from "react";
import type { RefObject } from "react";
import { toolbarLayout } from "./toolbar-layout";

export function useCompactToolbar(root: RefObject<HTMLElement | null>) {
  const [visible, setVisible] = useState(false), [open, setOpen] = useState(false);
  const close = (focus = false) => {
    setOpen(false);
    if (focus) root.current?.querySelector<HTMLButtonElement>("[data-sf-toolbar-more]")?.focus({ preventScroll: true });
  };
  useLayoutEffect(() => {
    const bar = root.current?.querySelector<HTMLElement>(".sf-topbar");
    const hud = bar?.querySelector<HTMLElement>(".sf-hud");
    if (!bar || !hud) return;
    const media = matchMedia("(max-width: 600px)");
    const update = () => {
      const buttons = [...bar.querySelectorAll<HTMLButtonElement>(".sf-control-buttons > button, .sf-field-actions > button")];
      for (const button of buttons) { button.removeAttribute("data-sf-overflow"); button.style.removeProperty("--sf-overflow-index"); }
      if (!media.matches) { setVisible(false); setOpen(false); return; }
      const keys = buttons.map((button, i) => {
        for (const key of ["ready", "resume", "tasks", "info"])
          if (button.hasAttribute(key === "tasks" || key === "info" ? "data-sf-open-" + key : "data-sf-" + key)) return key;
        return button.classList.contains("sf-select-pad") ? "build" : "other-" + i;
      });
      const layout = toolbarLayout(bar.clientWidth, hud.getBoundingClientRect().width, keys);
      for (const [i, key] of layout.overflow.entries()) {
        const button = buttons[keys.indexOf(key)];
        button.setAttribute("data-sf-overflow", ""); button.style.setProperty("--sf-overflow-index", String(i));
      }
      setVisible(layout.overflow.length > 0);
      if (!layout.overflow.length) setOpen(false);
    };
    const observer = new ResizeObserver(update), changes = new MutationObserver(update);
    observer.observe(bar); observer.observe(hud); changes.observe(bar, { childList: true, subtree: true });
    media.addEventListener("change", update); update();
    const outside = (event: PointerEvent) => { if (!bar.contains(event.target as Node)) setOpen(false); };
    const chosen = (event: MouseEvent) => { if ((event.target as Element).closest("[data-sf-overflow]")) setOpen(false); };
    document.addEventListener("pointerdown", outside, true); bar.addEventListener("click", chosen, true);
    return () => { observer.disconnect(); changes.disconnect(); media.removeEventListener("change", update); document.removeEventListener("pointerdown", outside, true); bar.removeEventListener("click", chosen, true); };
  });
  useLayoutEffect(() => {
    if (open && document.activeElement?.hasAttribute("data-sf-toolbar-more"))
      root.current?.querySelector<HTMLButtonElement>("[data-sf-overflow]:not([disabled])")?.focus({ preventScroll: true });
  }, [open, root]);
  return { visible, open, toggle: () => setOpen((value) => !value), close };
}
