export function restoreGameFocusOnBackground(surface: HTMLElement) {
  const focus = (event: MouseEvent) => {
    if (
      !event.isTrusted ||
      event.defaultPrevented ||
      event.button !== 0 ||
      event.altKey ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      document.hidden ||
      !surface.isConnected ||
      document.getSelection()?.isCollapsed === false
    )
      return;
    const target = event.target;
    if (!(target instanceof Element) || !target.isConnected) return;
    if (
      target.closest(
        'button,a[href],input,textarea,select,summary,label,[contenteditable]:not([contenteditable="false"]),[role="button"],[role="link"],[role="textbox"],[role="combobox"],[role="dialog"]',
      )
    )
      return;
    if (
      [...document.querySelectorAll('dialog[open],[aria-modal="true"]')].some(
        (dialog) =>
          dialog.getClientRects().length > 0 && !dialog.closest("[inert]"),
      )
    )
      return;
    const game = surface.querySelector<HTMLElement>("[data-game-focus]");
    if (game && !game.contains(target)) game.focus({ preventScroll: true });
  };
  document.addEventListener("click", focus);
  return () => document.removeEventListener("click", focus);
}
