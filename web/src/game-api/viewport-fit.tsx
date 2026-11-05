import { useLayoutEffect, useRef, type ReactNode } from "react";
import { viewportFitHeight } from "./viewport-fit-core";

/** Optional platform surface; games retain their own proportional layout. */
export function GameViewportFit({
  enabled,
  children,
}: {
  enabled: boolean;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const shell = ref.current?.closest<HTMLElement>(".room-page");
    if (!shell) return;
    shell.dataset.gameViewportFitShell = "true";
    return () => {
      delete shell.dataset.gameViewportFitShell;
    };
  }, []);
  useLayoutEffect(() => {
    const root = ref.current;
    if (!root || !enabled) return;
    let frame = 0,
      previousHeight: number | undefined,
      reason: "layout" | "scroll" = "layout",
      measures = 0;
    const surface = root.closest<HTMLElement>("[data-game-fullscreen-surface]");
    const external = new Set<Element>();
    const measure = () => {
      frame = 0;
      if (!root.isConnected || root.closest('[aria-hidden="true"],[inert]'))
        return;
      measures++;
      root.dataset.fitMeasurements = String(measures);
      if (surface?.hasAttribute("data-room-fullscreen")) {
        root.dataset.fitFullscreen = "true";
        root.dataset.fitActive = "true";
        root.style.setProperty("--game-viewport-height", "100%");
        previousHeight = undefined;
        return;
      }
      delete root.dataset.fitFullscreen;
      const rect = root.getBoundingClientRect(),
        child = root.firstElementChild?.getBoundingClientRect() ?? rect;
      const height = window.visualViewport?.height ?? window.innerHeight;
      let occlusionTop: number | undefined;
      // The platform chat is a fixed overlay, not part of the document stack.
      for (const overlay of document.querySelectorAll<HTMLElement>(
        ".table-chat",
      )) {
        if (overlay.closest("[inert]")) continue;
        const box = overlay.getBoundingClientRect(),
          style = getComputedStyle(overlay);
        if (
          style.visibility !== "hidden" &&
          style.display !== "none" &&
          (style.position === "fixed" || style.position === "sticky") &&
          box.width > 0 &&
          box.height > 0 &&
          box.right > child.left &&
          box.left < child.right &&
          box.top > rect.top
        )
          occlusionTop = Math.min(occlusionTop ?? height, box.top);
      }
      const allocated = viewportFitHeight({
        viewportHeight: height,
        documentTop: rect.top + window.scrollY,
        occlusionTop,
        bottomPadding: 8,
        previousHeight,
        reason,
      });
      reason = "layout";
      if (allocated === undefined) return;
      if (
        previousHeight === undefined ||
        Math.abs(previousHeight - allocated) > 0.5
      ) {
        previousHeight = allocated;
        root.style.setProperty("--game-viewport-height", allocated + "px");
      }
      root.dataset.fitActive = "true";
    };
    const schedule = (next: "layout" | "scroll" = "layout") => {
      if (next === "layout") reason = "layout";
      else if (!frame) reason = "scroll";
      if (!frame) frame = requestAnimationFrame(measure);
    };
    // Observe external boundaries above the game, including CSS-reordered panels.
    for (
      let node: Element | null = root;
      node?.parentElement;
      node = node.parentElement
    ) {
      for (const sibling of node.parentElement.children)
        if (sibling !== node) external.add(sibling);
    }
    for (const overlay of document.querySelectorAll(".table-chat"))
      external.add(overlay);
    const resize = new ResizeObserver(() => schedule());
    for (const node of external) resize.observe(node);
    let width = root.parentElement?.getBoundingClientRect().width;
    const parentResize = new ResizeObserver((entries) => {
      const next = entries[0]?.contentRect.width;
      if (next !== undefined && next !== width) {
        width = next;
        schedule();
      }
    });
    if (root.parentElement) parentResize.observe(root.parentElement);
    const mutation = new MutationObserver(() => schedule());
    if (surface)
      mutation.observe(surface, {
        attributes: true,
        attributeFilter: ["data-room-fullscreen"],
      });
    const staging = root.closest(".game-ready-surface");
    if (staging)
      mutation.observe(staging, {
        attributes: true,
        attributeFilter: ["class", "aria-hidden", "inert"],
      });
    const layout = () => schedule(),
      scroll = () => schedule("scroll");
    window.addEventListener("resize", layout);
    window.addEventListener("scroll", scroll, { passive: true });
    window.visualViewport?.addEventListener("resize", layout);
    window.visualViewport?.addEventListener("scroll", scroll);
    schedule();
    return () => {
      cancelAnimationFrame(frame);
      resize.disconnect();
      parentResize.disconnect();
      mutation.disconnect();
      window.removeEventListener("resize", layout);
      window.removeEventListener("scroll", scroll);
      window.visualViewport?.removeEventListener("resize", layout);
      window.visualViewport?.removeEventListener("scroll", scroll);
    };
  }, [enabled]);
  return (
    <div ref={ref} className="game-viewport-fit" data-game-viewport-fit>
      {children}
    </div>
  );
}
