// Long presses on controls are game input, not copy-menu requests. Keep this
// separate from pointer handlers so holding, dragging and releasing still run.
const controls =
  'button,[role="button"],[data-game-controls],input[type="button"],input[type="submit"],input[type="reset"],input[type="image"]';
const editors =
  'input:not([type="button"]):not([type="submit"]):not([type="reset"]):not([type="image"]),textarea,select,[contenteditable]:not([contenteditable="false"])';

export function installControlContextMenuGuard(root: Document): () => void {
  const onContextMenu = (event: Event) => {
    const elements = event
      .composedPath()
      .filter((node): node is Element => node instanceof Element);
    // Check editors first, including editors placed inside a controller panel.
    // composedPath also covers controls whose internals live in a shadow root.
    if (elements.some((element) => element.closest(editors))) return;
    if (elements.some((element) => element.closest(controls))) {
      event.preventDefault();
    }
  };
  root.addEventListener("contextmenu", onContextMenu, {
    capture: true,
    passive: false,
  });
  return () => root.removeEventListener("contextmenu", onContextMenu, true);
}
