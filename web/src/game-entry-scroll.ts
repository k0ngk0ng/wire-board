// Enter the playable surface once after initial layout settles. A user gesture
// takes precedence; ordinary snapshots and subsequent resizes never re-scroll.
export function revealGameOnEntry(element: HTMLElement) {
  let finished = false;
  let settle = 0;
  let deadline = 0;
  let frame = 0;
  const events = ["pointerdown", "touchstart", "wheel", "keydown"] as const;
  const cleanup = () => {
    if (finished) return;
    finished = true;
    window.clearTimeout(settle);
    window.clearTimeout(deadline);
    cancelAnimationFrame(frame);
    observer.disconnect();
    for (const event of events) window.removeEventListener(event, cleanup, true);
  };
  const reveal = () => {
    if (finished) return;
    if (element.isConnected && !document.querySelector('[aria-modal="true"]')) {
      const top = element.getBoundingClientRect().top + window.scrollY;
      window.scrollTo({ top: Math.max(0, top - 8), behavior: "instant" });
    }
    cleanup();
  };
  const schedule = () => {
    if (finished) return;
    window.clearTimeout(settle);
    settle = window.setTimeout(reveal, 160);
  };
  const observer = new ResizeObserver(schedule);
  observer.observe(element);
  const page = element.closest(".room-page");
  if (page) observer.observe(page);
  for (const event of events)
    window.addEventListener(event, cleanup, { capture: true, passive: true });
  frame = requestAnimationFrame(schedule);
  deadline = window.setTimeout(reveal, 1200);
  return cleanup;
}
