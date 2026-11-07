import { useEffect, useRef, type RefObject } from "react";

// A game owns what cancel means for its current selection. Dialog dismissal
// remains with the navigation scope and must not clear the underlying board.
export function useControllerCancel(
  root: RefObject<HTMLElement | null>,
  cancel: () => void,
) {
  const latest = useRef(cancel);
  latest.current = cancel;
  useEffect(() => {
    const node = root.current;
    const handle = (event: Event) => {
      if (
        event.target instanceof Element &&
        event.target.closest('[role="dialog"],dialog')
      )
        return;
      event.preventDefault();
      event.stopPropagation();
      latest.current();
    };
    node?.addEventListener("controller-cancel", handle);
    return () => node?.removeEventListener("controller-cancel", handle);
  }, [root]);
}
