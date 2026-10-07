import { visibleFlightBounds, type Bounds } from "./flight-anchor.ts";

type Scroll = { scrollLeft: number; scrollTop: number };

// Cards/nobles disappear before React's layout effects run. Remember their
// content coordinates relative to the persistent hand/nobles area, so later
// document and nested scrolling still move the original departure point.
export function relativeFlightBounds(
  rect: Bounds,
  reference: Bounds,
  scroll: Scroll,
): Bounds {
  return {
    left: rect.left - reference.left + scroll.scrollLeft,
    right: rect.right - reference.left + scroll.scrollLeft,
    top: rect.top - reference.top + scroll.scrollTop,
    bottom: rect.bottom - reference.top + scroll.scrollTop,
  };
}

export function projectFlightBounds(
  relative: Bounds,
  reference: Bounds,
  scroll: Scroll,
): Bounds {
  return {
    left: relative.left + reference.left - scroll.scrollLeft,
    right: relative.right + reference.left - scroll.scrollLeft,
    top: relative.top + reference.top - scroll.scrollTop,
    bottom: relative.bottom + reference.top - scroll.scrollTop,
  };
}

export type CardOrigin = {
  reference: HTMLElement;
  bounds: Bounds;
  viewportWidth: number;
};

// A confirmed card in a fixed panel disappears or moves when its effect
// resolves. Keep its actual screen position, independent of document scroll.
export type FixedCardOrigin = {
  fixed: true;
  bounds: Bounds;
  viewportWidth: number;
  viewportHeight: number;
};

export function captureFixedCardOrigin(element: HTMLElement): FixedCardOrigin {
  const rect = element.getBoundingClientRect();
  return {
    fixed: true,
    bounds: {
      left: rect.left,
      right: rect.right,
      top: rect.top,
      bottom: rect.bottom,
    },
    viewportWidth: document.documentElement.clientWidth,
    viewportHeight: document.documentElement.clientHeight,
  };
}

export function captureCardOrigin(
  element: HTMLElement,
  reference: HTMLElement,
): CardOrigin {
  return {
    reference,
    bounds: relativeFlightBounds(
      element.getBoundingClientRect(),
      reference.getBoundingClientRect(),
      reference,
    ),
    viewportWidth: document.documentElement.clientWidth,
  };
}

export function resolveCardOrigin(origin: CardOrigin | FixedCardOrigin) {
  if ("fixed" in origin) {
    if (
      origin.viewportWidth !== document.documentElement.clientWidth ||
      origin.viewportHeight !== document.documentElement.clientHeight
    )
      return null;
    return visibleFlightBounds(origin.bounds, null);
  }
  const ref = origin.reference;
  // A removed card has no new position after a responsive reflow. Hiding its
  // short remaining flight is safer than borrowing another card's new slot.
  if (
    !ref.isConnected ||
    origin.viewportWidth !== document.documentElement.clientWidth
  )
    return null;
  const rect = ref.getBoundingClientRect();
  if (!rect.width || !rect.height) return null;
  return visibleFlightBounds(
    projectFlightBounds(origin.bounds, rect, ref),
    ref,
  );
}
