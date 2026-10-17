(() => {
  const row = document.querySelector(".nobles-row");
  const rowRect = row?.getBoundingClientRect();
  const nobleSources = new Map(
    Array.from(row?.querySelectorAll("[data-noble-id]") || []).map((e) => {
      const r = e.getBoundingClientRect();
      return [
        e.dataset.nobleId,
        {
          x: r.left + r.width / 2 - rowRect.left,
          y: r.top + r.height / 2 - rowRect.top,
        },
      ];
    }),
  );
  const result = {
    samples: 0,
    visible: 0,
    hidden: 0,
    errors: [],
    frames: [],
    scrolls: [],
    players: [],
    directions: [],
    card: 0,
    visibleCardActions: [],
    motion: [],
    feedbackFrames: 0,
    reducedMotion: matchMedia("(prefers-reduced-motion: reduce)").matches,
  };
  window.flightResult = result;
  let start = performance.now(),
    last = start,
    moved = false,
    settle = 0;
  function point(e) {
    if (!e) return null;
    const r = e.getBoundingClientRect(),
      x = r.left + r.width / 2,
      y = r.top + r.height / 2;
    if (
      !r.width ||
      !r.height ||
      x < 0 ||
      x >= innerWidth ||
      y < 0 ||
      y >= innerHeight
    )
      return null;
    for (let p = e.parentElement; p; p = p.parentElement) {
      const s = getComputedStyle(p),
        b = p.getBoundingClientRect();
      if (
        /auto|scroll|hidden|clip/.test(s.overflowX) &&
        (x < b.left + p.clientLeft ||
          x >= b.left + p.clientLeft + p.clientWidth)
      )
        return null;
      if (
        /auto|scroll|hidden|clip/.test(s.overflowY) &&
        (y < b.top + p.clientTop || y >= b.top + p.clientTop + p.clientHeight)
      )
        return null;
    }
    return { x, y };
  }
  function sample(now) {
    result.frames.push(now - last);
    last = now;
    const nodes = [
      ...document.querySelectorAll("[data-token-flight], [data-card-flight]"),
    ].filter((node) =>
      node.hasAttribute("data-card-flight")
        ? Number(node.dataset.cardFlight) > (window.flightAfter?.card || 0)
        : parseInt(node.dataset.tokenFlight, 10) >
          (window.flightAfter?.token || 0),
    );
    if (nodes.length && !moved) {
      moved = true;
      setTimeout(() => {
        const strip = document.querySelector(".players-strip");
        const before = strip.scrollLeft;
        strip.scrollLeft += window.flightScrollDelta || -35;
        window.scrollBy(0, 15);
        settle = 3;
        result.scrolls.push({
          before,
          after: strip.scrollLeft,
          document: scrollY,
        });
      }, 100);
    }
    if (settle > 0) settle--;
    else
      for (const n of nodes) {
        const card = n.hasAttribute("data-card-flight");
        const e = card ? n.querySelector(".splendor-card-flight") : n;
        const feedback = card ? n.querySelector('.splendor-card-arrival') : e;
        if (feedback) {
          const css = getComputedStyle(feedback);
          if (css.visibility === 'visible' && Number(css.opacity) > 0 && feedback.getBoundingClientRect().width > 0)
            result.feedbackFrames++;
          if (card && result.motion.length < 160)
            result.motion.push({kind: 'arrival', name: css.animationName, visibility:css.visibility,
              opacity:css.opacity, transform:css.transform, animations:feedback.getAnimations().map(a=>({state:a.playState,time:a.currentTime}))});
        }
        if (e && result.motion.length < 160) {
          const css = getComputedStyle(e);
          result.motion.push({
            kind: card ? "card" : "token",
            name: css.animationName,
            visibility: css.visibility,
            opacity: css.opacity,
            transform: css.transform,
            animations: e
              .getAnimations()
              .map((a) => ({ state: a.playState, time: a.currentTime })),
          });
        }
        const player = Number(n.dataset.flightPlayer),
          color = Number(n.dataset.flightColor);
        const avatarTarget = point(
          document.querySelector(`[data-player-seat="${player}"] .avatar`),
        );
        const recipient = n
          .closest(".splendor-card-animation,.splendor-token-animation")
          ?.querySelector(`[data-flight-recipient="${player}"]`);
        const target = avatarTarget || point(recipient);
        if (
          !avatarTarget &&
          recipient &&
          recipient.dataset.recipientName !== window.flightSeats?.[player]
        )
          result.errors.push({ wrongRecipient: true, player });
        result.players.push(player);
        if (card) {
          result.card++;
          let source = window.flightCardSource
            ? point(document.querySelector(window.flightCardSource))
            : null;
          if (window.flightFixedSource) {
            const r = window.flightFixedSource;
            source = { x: r.left + r.width / 2, y: r.top + r.height / 2 };
          }
          if (n.dataset.flightAction === "noble") {
            const id = e.querySelector("[data-noble-id]")?.dataset.nobleId;
            const saved = nobleSources.get(id),
              ref = row?.getBoundingClientRect();
            source =
              saved && ref
                ? { x: saved.x + ref.left, y: saved.y + ref.top }
                : null;
          }
          if (source && target && e.style.visibility !== "visible")
            result.errors.push({ card: true, unexpectedHidden: true, player });
          if (e.style.visibility === "visible") {
            result.visible++;
            result.visibleCardActions.push(n.dataset.flightAction);
            const tx =
              parseFloat(e.style.left) +
              parseFloat(e.style.width) / 2 +
              parseFloat(e.style.getPropertyValue("--flight-x"));
            const ty =
              parseFloat(e.style.top) +
              parseFloat(e.style.height) / 2 +
              parseFloat(e.style.getPropertyValue("--flight-y"));
            const x = parseFloat(e.style.left) + parseFloat(e.style.width) / 2,
              y = parseFloat(e.style.top) + parseFloat(e.style.height) / 2;
            if (
              ![x, y, tx, ty].every(Number.isFinite) ||
              !source ||
              Math.abs(x - source.x) > 1 ||
              Math.abs(y - source.y) > 1
            )
              result.errors.push({ card: true, source, x, y });
            if (
              !target ||
              Math.abs(tx - target.x) > 1 ||
              Math.abs(ty - target.y) > 1
            )
              result.errors.push({ card: true, player, tx, ty, target });
          }
          continue;
        }
        result.samples++;
        const bank = point(
          document.querySelector(`[data-bank-color="${color}"] .token`),
        );
        const back = n.dataset.flightDirection === "to-bank";
        result.directions.push(n.dataset.flightDirection);
        const from = back ? target : bank,
          to = back ? bank : target;
        if (e.style.visibility !== "visible") {
          result.hidden++;
          if (from && to)
            result.errors.push({ unexpectedHidden: true, player, color });
          continue;
        }
        result.visible++;
        const x = parseFloat(e.style.left) + 19,
          y = parseFloat(e.style.top) + 19;
        const tx = x + parseFloat(e.style.getPropertyValue("--token-x")),
          ty = y + parseFloat(e.style.getPropertyValue("--token-y"));
        if (
          ![x, y, tx, ty].every(Number.isFinite) ||
          !from ||
          !to ||
          Math.abs(x - from.x) > 1 ||
          Math.abs(y - from.y) > 1 ||
          Math.abs(tx - to.x) > 1 ||
          Math.abs(ty - to.y) > 1
        )
          result.errors.push({ player, color, from, to, x, y, tx, ty });
      }
    if (now - start < 1350) requestAnimationFrame(sample);
    else {
      result.players = [...new Set(result.players)];
      result.directions = [...new Set(result.directions)];
      result.visibleCardActions = [...new Set(result.visibleCardActions)];
      result.done = true;
    }
  }
  requestAnimationFrame(sample);
  return true;
})();
