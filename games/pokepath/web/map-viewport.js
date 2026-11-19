const canvasProperties = ["position", "display", "left", "top", "right", "bottom", "width", "height", "max-width", "max-height", "transform", "transform-origin", "margin"];
const wrapperProperties = ["overflow", "touch-action", "overscroll-behavior", "--scroll-pct", "--scroll-pct-h", "--scroll-pct-v"];
const buttons = "[data-map-zoom-in],[data-map-zoom-out],[data-map-fit],[data-map-reset]";

export function attachMapViewport({ canvas, wrapper, onChange, minimumScale, maximumScale = 4, dragThreshold = 7, initialMode = "native" } = {}) {
  if (!canvas || !wrapper || canvas.parentElement !== wrapper || typeof canvas.getBoundingClientRect !== "function") throw Error("Map viewport requires the original canvas and its parent");
  if (!Number.isFinite(maximumScale) || maximumScale <= 0 || !Number.isFinite(dragThreshold) || dragThreshold < 0) throw Error("Invalid map viewport bounds");
  if (!["native", "fit", "fill"].includes(initialMode)) throw Error("Invalid initial map view");
  const view = canvas.ownerDocument.defaultView;
  const pointers = new Map();
  const cancelled = new Set();
  const savedCanvas = canvasProperties.map(name => [name, canvas.style.getPropertyValue(name), canvas.style.getPropertyPriority(name)]);
  const savedWrapper = wrapperProperties.map(name => [name, wrapper.style.getPropertyValue(name), wrapper.style.getPropertyPriority(name)]);
  let width = canvas.width;
  let height = canvas.height;
  let scale = 1;
  let fitted = false;
  let filled = false;
  let viewportWidth = wrapper.clientWidth;
  let viewportHeight = wrapper.clientHeight;
  let marginX = 0;
  let marginY = 0;
  let destroyed = false;
  let pinch;
  let suppressClick;
  let mutation;
  let resize;

  const setStyle = (element, name, value) => {
    if (element.style.getPropertyValue(name) !== value || element.style.getPropertyPriority(name) !== "important") element.style.setProperty(name, value, "important");
  };
  const fitScale = () => Math.min(wrapper.clientWidth / Math.max(1, width), wrapper.clientHeight / Math.max(1, height), maximumScale);
  const fillScale = () => Math.min(Math.max(wrapper.clientWidth / Math.max(1, width), wrapper.clientHeight / Math.max(1, height)), maximumScale);
  const minScale = () => Math.max(0.02, Math.min(maximumScale, Math.max(fitScale(), Number.isFinite(minimumScale) ? minimumScale : 0)));
  const clampScale = value => Math.max(minScale(), Math.min(maximumScale, value));
  const parentScale = () => {
    const rect = wrapper.getBoundingClientRect();
    return { x: rect.width / Math.max(1, wrapper.offsetWidth), y: rect.height / Math.max(1, wrapper.offsetHeight) };
  };
  const center = () => {
    const rect = wrapper.getBoundingClientRect();
    return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
  };
  const logicalPoint = (clientX, clientY) => {
    const rect = canvas.getBoundingClientRect();
    return { x: (clientX - rect.left) * width / Math.max(0.0001, rect.width), y: (clientY - rect.top) * height / Math.max(0.0001, rect.height) };
  };
  const pointToClient = (x, y) => {
    const rect = canvas.getBoundingClientRect();
    return { x: rect.left + x * rect.width / Math.max(1, width), y: rect.top + y * rect.height / Math.max(1, height) };
  };
  const state = () => ({ scale, width, height, fitted, minimumScale: minScale(), maximumScale, scrollLeft: wrapper.scrollLeft, scrollTop: wrapper.scrollTop, maxScrollLeft: Math.max(0, wrapper.scrollWidth - wrapper.clientWidth), maxScrollTop: Math.max(0, wrapper.scrollHeight - wrapper.clientHeight), pointers: pointers.size });
  const notify = () => { onChange?.(state()); };
  const constrain = () => {
    wrapper.scrollLeft = Math.max(0, Math.min(wrapper.scrollLeft, wrapper.scrollWidth - wrapper.clientWidth));
    wrapper.scrollTop = Math.max(0, Math.min(wrapper.scrollTop, wrapper.scrollHeight - wrapper.clientHeight));
    wrapper.style.setProperty("--scroll-pct", String(wrapper.scrollLeft / Math.max(1, wrapper.scrollWidth - wrapper.clientWidth)));
    wrapper.style.setProperty("--scroll-pct-h", String(wrapper.scrollLeft / Math.max(1, wrapper.scrollWidth - wrapper.clientWidth)));
    wrapper.style.setProperty("--scroll-pct-v", String(wrapper.scrollTop / Math.max(1, wrapper.scrollHeight - wrapper.clientHeight)));
  };
  const apply = () => {
    setStyle(wrapper, "overflow", "auto");
    setStyle(wrapper, "touch-action", "none");
    setStyle(wrapper, "overscroll-behavior", "contain");
    viewportWidth = wrapper.clientWidth;
    viewportHeight = wrapper.clientHeight;
    marginX = Math.max(0, (viewportWidth - width * scale) / 2);
    marginY = Math.max(0, (viewportHeight - height * scale) / 2);
    const properties = { position: "relative", display: "block", left: "0px", top: "0px", right: "auto", bottom: "auto", width: `${width * scale}px`, height: `${height * scale}px`, "max-width": "none", "max-height": "none", transform: "none", "transform-origin": "0px 0px", margin: `${marginY}px ${marginX}px` };
    for (const [name, value] of Object.entries(properties)) setStyle(canvas, name, value);
    constrain();
  };
  const zoomTo = (value, point = center(), anchor = logicalPoint(point.x, point.y)) => {
    if (destroyed || !Number.isFinite(value)) return;
    scale = clampScale(value);
    apply();
    const physical = pointToClient(anchor.x, anchor.y);
    const outer = parentScale();
    wrapper.scrollLeft += (physical.x - point.x) / Math.max(0.0001, outer.x);
    wrapper.scrollTop += (physical.y - point.y) / Math.max(0.0001, outer.y);
    constrain();
    notify();
  };
  const zoomBy = (factor, point) => {
    if (!Number.isFinite(factor) || factor <= 0) return;
    fitted = false;
    filled = false;
    zoomTo(scale * factor, point);
  };
  const reset = () => {
    fitted = false;
    filled = false;
    zoomTo(1, center(), { x: width / 2, y: height / 2 });
  };
  const fit = () => {
    fitted = true;
    filled = false;
    zoomTo(fitScale(), center(), { x: width / 2, y: height / 2 });
  };
  const fill = () => {
    fitted = false;
    filled = true;
    zoomTo(fillScale(), center(), { x: width / 2, y: height / 2 });
  };
  const release = id => {
    try { if (wrapper.hasPointerCapture?.(id)) wrapper.releasePointerCapture(id); } catch {}
  };
  const cancel = () => {
    for (const id of pointers.keys()) {
      cancelled.add(id);
      release(id);
    }
    pointers.clear();
    pinch = undefined;
    suppressClick = undefined;
    notify();
  };
  const update = () => {
    if (destroyed) return;
    const anchor = { x: (wrapper.scrollLeft + viewportWidth / 2 - marginX) / scale, y: (wrapper.scrollTop + viewportHeight / 2 - marginY) / scale };
    if (canvas.width !== width || canvas.height !== height) {
      width = canvas.width;
      height = canvas.height;
      cancel();
      if (initialMode === "fill") fill();
      else if (initialMode === "fit") fit();
      else reset();
    } else {
      if (viewportWidth !== wrapper.clientWidth || viewportHeight !== wrapper.clientHeight) cancel();
      if (fitted) fit();
      else if (filled) fill();
      else zoomTo(scale, center(), anchor);
    }
  };
  const consume = (event, prevent = true) => {
    if (prevent && event.cancelable) event.preventDefault();
    event.stopImmediatePropagation?.();
    return true;
  };
  const inside = event => event.target === wrapper || wrapper.contains(event.target);
  const distance = (a, b) => Math.hypot(a.x - b.x, a.y - b.y);
  const midpoint = (a, b) => ({ x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 });
  const beginPinch = () => {
    const [first, second] = [...pointers.values()];
    const point = midpoint(first, second);
    pinch = { distance: Math.max(1, distance(first, second)), scale, anchor: logicalPoint(point.x, point.y) };
    for (const pointer of pointers.values()) {
      pointer.moved = true;
      try { wrapper.setPointerCapture(pointer.id); } catch {}
    }
  };
  const handleEvent = event => {
    if (destroyed) return false;
    const button = event.target?.closest?.(buttons);
    if (button) {
      if (event.type === "click") {
        if (button.hasAttribute("data-map-zoom-in")) zoomBy(1.25);
        else if (button.hasAttribute("data-map-zoom-out")) zoomBy(0.8);
        else if (button.hasAttribute("data-map-fit")) fit();
        else reset();
      }
      return consume(event, event.type === "click");
    }
    if (event.type === "wheel") {
      if (!inside(event)) return false;
      const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? wrapper.clientHeight : 1);
      zoomBy(Math.exp(Math.max(-0.7, Math.min(0.7, -delta * 0.0015))), { x: event.clientX, y: event.clientY });
      return consume(event);
    }
    if (event.type === "click") {
      if (suppressClick && event.timeStamp - suppressClick.at < 500 && Math.hypot(event.clientX - suppressClick.x, event.clientY - suppressClick.y) < 25) {
        suppressClick = undefined;
        return consume(event);
      }
      return false;
    }
    if (event.type === "pointerdown") {
      if (!inside(event) || event.button !== 0 && event.pointerType !== "touch") return false;
      cancelled.delete(event.pointerId);
      suppressClick = undefined;
      pointers.set(event.pointerId, { id: event.pointerId, x: event.clientX, y: event.clientY, startX: event.clientX, startY: event.clientY, left: wrapper.scrollLeft, top: wrapper.scrollTop, moved: false });
      if (pointers.size === 2) beginPinch();
      notify();
      return consume(event, pointers.size > 1);
    }
    if (cancelled.has(event.pointerId)) {
      if (event.type === "pointerup" || event.type === "pointercancel") {
        if (event.type === "pointerup") suppressClick = { x: event.clientX, y: event.clientY, at: event.timeStamp };
        cancelled.delete(event.pointerId);
      }
      if (["pointermove", "pointerup", "pointercancel", "lostpointercapture"].includes(event.type)) return consume(event);
    }
    const pointer = pointers.get(event.pointerId);
    if (!pointer) return false;
    if (event.type === "pointermove") {
      pointer.x = event.clientX;
      pointer.y = event.clientY;
      if (pointers.size >= 2 && pinch) {
        const [first, second] = [...pointers.values()];
        fitted = false;
        filled = false;
        zoomTo(pinch.scale * distance(first, second) / pinch.distance, midpoint(first, second), pinch.anchor);
        return consume(event);
      }
      if (!pointer.moved && Math.hypot(pointer.x - pointer.startX, pointer.y - pointer.startY) < dragThreshold) return false;
      pointer.moved = true;
      fitted = false;
      filled = false;
      try { wrapper.setPointerCapture(pointer.id); } catch {}
      const outer = parentScale();
      wrapper.scrollLeft = pointer.left + (pointer.startX - pointer.x) / Math.max(0.0001, outer.x);
      wrapper.scrollTop = pointer.top + (pointer.startY - pointer.y) / Math.max(0.0001, outer.y);
      constrain();
      notify();
      return consume(event);
    }
    if (event.type === "pointerup" || event.type === "pointercancel" || event.type === "lostpointercapture") {
      const moved = pointer.moved || Boolean(pinch);
      if (moved) suppressClick = { x: event.clientX, y: event.clientY, at: event.timeStamp };
      release(event.pointerId);
      pointers.delete(event.pointerId);
      pinch = undefined;
      if (pointers.size === 1) {
        const remaining = [...pointers.values()][0];
        Object.assign(remaining, { startX: remaining.x, startY: remaining.y, left: wrapper.scrollLeft, top: wrapper.scrollTop, moved: true });
      }
      notify();
      return consume(event, moved || event.type === "pointercancel");
    }
    return false;
  };

  wrapper.addEventListener("scroll", constrain, { passive: true });
  view.addEventListener("blur", cancel);
  view.addEventListener("pagehide", cancel);
  view.addEventListener("resize", update);
  if (view.MutationObserver) {
    mutation = new view.MutationObserver(update);
    mutation.observe(canvas, { attributes: true, attributeFilter: ["width", "height", "style"] });
  }
  if (view.ResizeObserver) {
    resize = new view.ResizeObserver(update);
    resize.observe(wrapper);
  }
  if (initialMode === "fill") fill();
  else if (initialMode === "fit") fit();
  else { scale = clampScale(1); apply(); notify(); }

  return {
    handleEvent,
    zoomBy,
    fit,
    fill,
    reset,
    update,
    logicalPoint,
    pointToClient,
    cancel,
    get state() { return state(); },
    destroy() {
      if (destroyed) return;
      cancel();
      destroyed = true;
      cancelled.clear();
      mutation?.disconnect();
      resize?.disconnect();
      wrapper.removeEventListener("scroll", constrain);
      view.removeEventListener("blur", cancel);
      view.removeEventListener("pagehide", cancel);
      view.removeEventListener("resize", update);
      for (const [name, value, priority] of savedCanvas) {
        if (value) canvas.style.setProperty(name, value, priority);
        else canvas.style.removeProperty(name);
      }
      for (const [name, value, priority] of savedWrapper) {
        if (value) wrapper.style.setProperty(name, value, priority);
        else wrapper.style.removeProperty(name);
      }
    }
  };
}
