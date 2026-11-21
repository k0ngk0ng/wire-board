const editorSelector = "input,textarea,select,[contenteditable]";
const buttonSelector = 'input:is([type="button"],[type="submit"],[type="reset"],[type="image"])';

function protectedTarget(target: EventTarget | null, root: HTMLElement) {
  const node = target as Node | null;
  if (!node || typeof node.nodeType !== "number" || !root.contains(node)) return false;
  const element = node.nodeType === 1 ? node as Element : node.parentElement;
  const controls = element?.closest("[data-game-controls]");
  if (!element || !controls || !root.contains(controls)) return false;
  const editor = element.closest(editorSelector);
  if (!editor || !controls.contains(editor) || editor.matches(buttonSelector)) return true;
  if (editor.matches("input,textarea,select")) return false;
  return editor.hasAttribute("contenteditable") && editor.getAttribute("contenteditable")?.toLowerCase() === "false";
}

export function protectGameControls(root: HTMLElement) {
  const document = root.ownerDocument;
  const preventNative = (event: Event) => {
    if (protectedTarget(event.target, root)) event.preventDefault();
  };
  const clearSelection = () => {
    const selection = document.getSelection();
    if (!selection?.rangeCount) return;
    if (protectedTarget(selection.anchorNode, root) || protectedTarget(selection.focusNode, root)) selection.removeAllRanges();
  };
  const beginControl = (event: Event) => {
    if (protectedTarget(event.target, root)) clearSelection();
  };
  root.addEventListener("selectstart", preventNative, true);
  root.addEventListener("contextmenu", preventNative, true);
  root.addEventListener("pointerdown", beginControl, true);
  document.addEventListener("selectionchange", clearSelection);
  clearSelection();
  return () => {
    root.removeEventListener("selectstart", preventNative, true);
    root.removeEventListener("contextmenu", preventNative, true);
    root.removeEventListener("pointerdown", beginControl, true);
    document.removeEventListener("selectionchange", clearSelection);
  };
}
