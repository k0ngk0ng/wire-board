// Browser timers can fire slightly before a fractional performance.now()
// deadline. Keep waiting in that case instead of leaving playback locked until
// an unrelated snapshot happens to render the component again.
export function schedulePresentationWake(wakeAt: number, wake: () => void) {
  let cancelled = false;
  let timer: ReturnType<typeof setTimeout>;
  const check = () => {
    if (cancelled) return;
    const remaining = wakeAt - performance.now();
    if (remaining > 0) timer = setTimeout(check, Math.ceil(remaining));
    else {
      cancelled = true;
      wake();
    }
  };
  timer = setTimeout(check, Math.max(0, Math.ceil(wakeAt - performance.now())));
  return () => {
    cancelled = true;
    clearTimeout(timer);
  };
}
