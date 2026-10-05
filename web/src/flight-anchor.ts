type Bounds = { left: number; top: number; right: number; bottom: number };

// Keep the real center: clamping an offscreen seat onto the viewport edge can
// make a flight appear to belong to a different, visible player.
export function flightCenter(rect: Bounds, clips: Bounds[]) {
  const x = (rect.left + rect.right) / 2;
  const y = (rect.top + rect.bottom) / 2;
  if (
    rect.right <= rect.left ||
    rect.bottom <= rect.top ||
    !Number.isFinite(x) ||
    !Number.isFinite(y) ||
    clips.some(
      (clip) =>
        x < clip.left || x >= clip.right || y < clip.top || y >= clip.bottom,
    )
  )
    return null;
  return { x, y };
}

export function visibleFlightAnchor(element: Element) {
  const view = window.visualViewport;
  const clips: Bounds[] = [
    {
      left: view?.offsetLeft ?? 0,
      top: view?.offsetTop ?? 0,
      right: view
        ? view.offsetLeft + view.width
        : document.documentElement.clientWidth,
      bottom: view
        ? view.offsetTop + view.height
        : document.documentElement.clientHeight,
    },
  ];
  for (
    let parent = element.parentElement;
    parent;
    parent = parent.parentElement
  ) {
    const style = getComputedStyle(parent);
    const clipX = /^(auto|scroll|hidden|clip)$/.test(style.overflowX);
    const clipY = /^(auto|scroll|hidden|clip)$/.test(style.overflowY);
    if (!clipX && !clipY) continue;
    const rect = parent.getBoundingClientRect();
    const left = rect.left + parent.clientLeft;
    const top = rect.top + parent.clientTop;
    clips.push({
      left: clipX ? left : -Infinity,
      right: clipX ? left + parent.clientWidth : Infinity,
      top: clipY ? top : -Infinity,
      bottom: clipY ? top + parent.clientHeight : Infinity,
    });
  }
  return flightCenter(element.getBoundingClientRect(), clips);
}
