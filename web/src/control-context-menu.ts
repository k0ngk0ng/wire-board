// Long presses on controls are game input, not copy-menu requests. Keep this
// separate from pointer handlers so holding, dragging and releasing still run.
const controls =
  'button,[role="button"],[data-game-controls],input[type="button"],input[type="submit"],input[type="reset"],input[type="image"]';
const editors =
  'input:not([type="button"]):not([type="submit"]):not([type="reset"]):not([type="image"]),textarea,select,[contenteditable]:not([contenteditable="false"])';

// Opt in only controls already driven by pointerdown/up. Preventing touchstart
// on ordinary click buttons would suppress their compatibility click in WebKit.
const heldControls = "[data-game-hold-control]";
export function installControlContextMenuGuard(root: Document): () => void {
  const heldTouches = new Set<number>();
  const elements = (event: Event) =>
    event
      .composedPath()
      .filter((node): node is Element => node instanceof Element);
  const isEditor = (event: Event) =>
    elements(event).some((element) => element.closest(editors));
  const onContextMenu = (event: Event) => {
    if (isEditor(event)) return;
    if (
      heldTouches.size ||
      elements(event).some((element) => element.closest(controls))
    )
      event.preventDefault();
  };
  const onTouchStart = (event: TouchEvent) => {
    if (isEditor(event)) return;
    let protectedTouch = false;
    for (const touch of Array.from(event.changedTouches)) {
      const target = touch.target;
      if (
        target instanceof Element &&
        target.closest(heldControls) &&
        !target.closest(editors)
      ) {
        heldTouches.add(touch.identifier);
        protectedTouch = true;
      }
    }
    if (protectedTouch) {
      onSelection();
      event.preventDefault();
    }
  };
  const onTouchMove = (event: TouchEvent) => {
    if (
      !isEditor(event) &&
      Array.from(event.changedTouches).some((touch) =>
        heldTouches.has(touch.identifier),
      )
    )
      event.preventDefault();
  };
  const onTouchEnd = (event: TouchEvent) => {
    for (const touch of Array.from(event.changedTouches))
      heldTouches.delete(touch.identifier);
  };
  const clear = () => heldTouches.clear();
  const onHidden = () => {
    if (root.visibilityState === "hidden") clear();
  };
  const onSelection = () => {
    if (!heldTouches.size || root.activeElement?.closest(editors)) return;
    const selection = root.getSelection();
    if (selection && !selection.isCollapsed) selection.removeAllRanges();
  };
  const guarded = { capture: true, passive: false };
  root.addEventListener("contextmenu", onContextMenu, guarded);
  root.addEventListener("selectstart", onContextMenu, guarded);
  root.addEventListener("touchstart", onTouchStart, guarded);
  root.addEventListener("touchmove", onTouchMove, guarded);
  root.addEventListener("touchend", onTouchEnd, true);
  root.addEventListener("touchcancel", onTouchEnd, true);
  root.addEventListener("selectionchange", onSelection);
  root.addEventListener("visibilitychange", onHidden);
  root.defaultView?.addEventListener("blur", clear);
  return () => {
    clear();
    root.removeEventListener("contextmenu", onContextMenu, true);
    root.removeEventListener("selectstart", onContextMenu, true);
    root.removeEventListener("touchstart", onTouchStart, true);
    root.removeEventListener("touchmove", onTouchMove, true);
    root.removeEventListener("touchend", onTouchEnd, true);
    root.removeEventListener("touchcancel", onTouchEnd, true);
    root.removeEventListener("selectionchange", onSelection);
    root.removeEventListener("visibilitychange", onHidden);
    root.defaultView?.removeEventListener("blur", clear);
  };
}
