import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ButtonHTMLAttributes,
} from "react";
import {
  createHoldActionController,
  type HoldActionCallbacks,
} from "./hold-action-controller";

export type HoldActionOptions = HoldActionCallbacks & {
  enabled: boolean;
  // Include the room/session, action epoch and selected province. Changing any
  // of those invalidates a held gesture rather than redirecting it elsewhere.
  identity: string;
  delayMS?: number;
};
type Handlers = Pick<
  ButtonHTMLAttributes<HTMLButtonElement>,
  | "onPointerDown"
  | "onPointerMove"
  | "onPointerUp"
  | "onPointerCancel"
  | "onPointerLeave"
  | "onLostPointerCapture"
  | "onClick"
  | "onKeyDown"
  | "onKeyUp"
  | "onBlur"
  | "onContextMenu"
>;

/**
 * Spread `handlers` onto the actual button instead of adding another onClick.
 * Use aria-keyshortcuts="Shift+Enter" for the hold equivalent. `holding` may
 * drive a small progress indicator; the hook adds no UI or game action rules.
 */
export function useHoldAction(options: HoldActionOptions) {
  const latest = useRef(options);
  latest.current = options;
  const mounted = useRef(false);
  const captured = useRef<{ element: HTMLButtonElement; id: number } | null>(
    null,
  );
  const globalPointers = useRef(new Set<number>());
  const [holding, setHolding] = useState(false);
  const [controller] = useState(() =>
    createHoldActionController({
      getCallbacks: () => latest.current,
      onHoldingChange: (value) => {
        if (mounted.current) setHolding(value);
      },
      delayMS: options.delayMS,
    }),
  );
  const releaseCapture = useCallback((id?: number) => {
    const capture = captured.current;
    if (!capture || (id !== undefined && id !== capture.id)) return;
    captured.current = null;
    try {
      if (capture.element.hasPointerCapture(capture.id))
        capture.element.releasePointerCapture(capture.id);
    } catch {
      /* The element may have been removed during a confirmed action. */
    }
  }, []);
  const cancelInputs = useCallback(() => {
    controller.resetInputs();
    globalPointers.current.clear();
    releaseCapture();
  }, [controller, releaseCapture]);

  useLayoutEffect(() => {
    mounted.current = true;
    controller.activate();
    controller.configure(latest.current);
    setHolding(false);
    return () => {
      mounted.current = false;
      controller.dispose();
      globalPointers.current.clear();
      releaseCapture();
    };
  }, [controller, releaseCapture]);
  useLayoutEffect(() => {
    // In particular, re-enabling after a fast network round trip must not erase
    // suppression of the click belonging to a hold which already executed.
    controller.configure(options);
  }, [controller, options.enabled, options.identity, options.delayMS]);

  useEffect(() => {
    const down = (event: PointerEvent) => {
      globalPointers.current.add(event.pointerId);
      controller.additionalPointer(event.pointerId);
    };
    const up = (event: PointerEvent) => {
      globalPointers.current.delete(event.pointerId);
      if (event.type === "pointercancel")
        controller.cancelPointer(event.pointerId);
      const capture = captured.current;
      const rect = capture?.element.getBoundingClientRect();
      const inside =
        !rect ||
        (event.clientX >= rect.left &&
          event.clientX <= rect.right &&
          event.clientY >= rect.top &&
          event.clientY <= rect.bottom);
      controller.pointerUp(
        event.pointerId,
        event.clientX,
        event.clientY,
        inside,
      );
      releaseCapture(event.pointerId);
    };
    const keyUp = (event: KeyboardEvent) =>
      controller.keyUp(event.key === "Spacebar" ? " " : event.key);
    const visibility = () => {
      if (document.hidden) cancelInputs();
    };
    // Capture observes a second finger on another button as well as a release
    // outside this button. The game/map can still own its normal input path.
    window.addEventListener("pointerdown", down, true);
    window.addEventListener("pointerup", up, true);
    window.addEventListener("pointercancel", up, true);
    window.addEventListener("keyup", keyUp, true);
    window.addEventListener("blur", cancelInputs);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      window.removeEventListener("pointerdown", down, true);
      window.removeEventListener("pointerup", up, true);
      window.removeEventListener("pointercancel", up, true);
      window.removeEventListener("keyup", keyUp, true);
      window.removeEventListener("blur", cancelInputs);
      document.removeEventListener("visibilitychange", visibility);
      cancelInputs();
    };
  }, [controller, cancelInputs, releaseCapture]);

  const handlers = useMemo<Handlers>(
    () => ({
      onPointerDown(event) {
        if (
          controller.pointerDown(
            event.pointerId,
            event.clientX,
            event.clientY,
            event.isPrimary,
            event.button,
            globalPointers.current.size > 1,
          )
        ) {
          captured.current = {
            element: event.currentTarget,
            id: event.pointerId,
          };
          try {
            event.currentTarget.setPointerCapture(event.pointerId);
          } catch {
            /* Pointer leave/up guards remain usable without capture. */
          }
        }
      },
      onPointerMove(event) {
        const r = event.currentTarget.getBoundingClientRect();
        controller.pointerMove(
          event.pointerId,
          event.clientX,
          event.clientY,
          event.clientX >= r.left &&
            event.clientX <= r.right &&
            event.clientY >= r.top &&
            event.clientY <= r.bottom,
        );
      },
      onPointerUp(event) {
        const r = event.currentTarget.getBoundingClientRect();
        controller.pointerUp(
          event.pointerId,
          event.clientX,
          event.clientY,
          event.clientX >= r.left &&
            event.clientX <= r.right &&
            event.clientY >= r.top &&
            event.clientY <= r.bottom,
        );
        releaseCapture(event.pointerId);
      },
      onPointerCancel(event) {
        controller.cancelPointer(event.pointerId);
        controller.pointerUp(event.pointerId);
        releaseCapture(event.pointerId);
      },
      onPointerLeave(event) {
        const r = event.currentTarget.getBoundingClientRect();
        if (
          event.clientX < r.left ||
          event.clientX > r.right ||
          event.clientY < r.top ||
          event.clientY > r.bottom
        )
          controller.cancelPointer(event.pointerId);
      },
      onLostPointerCapture(event) {
        // Ignore the SVG child's implicit capture handing over to this button.
        if (
          event.target === event.currentTarget &&
          !event.currentTarget.hasPointerCapture(event.pointerId)
        )
          controller.cancelPointer(event.pointerId);
      },
      onClick(event) {
        event.preventDefault();
        event.stopPropagation();
        controller.click(event.detail);
      },
      onKeyDown(event) {
        if (event.key === "Escape") {
          cancelInputs();
          event.preventDefault();
        } else if (
          controller.keyDown(
            event.key === "Spacebar" ? " " : event.key,
            event.shiftKey,
            event.repeat,
          )
        ) {
          event.preventDefault();
        }
      },
      onKeyUp(event) {
        controller.keyUp(event.key === "Spacebar" ? " " : event.key);
      },
      onBlur() {
        cancelInputs();
      },
      onContextMenu(event) {
        event.preventDefault();
      },
    }),
    [controller, cancelInputs, releaseCapture],
  );
  return { handlers, holding };
}
