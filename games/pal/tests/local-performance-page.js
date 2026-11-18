(() => {
  let armed,
    started,
    baseline,
    frames = [],
    frame = 0,
    area = [0, 0, 160, 150];
  const pixels = () => {
    const canvas = document.querySelector("[data-pal-board] canvas");
    if (!canvas) return "";
    const data = canvas.getContext("2d").getImageData(...area).data;
    let hash = 2166136261;
    for (let i = 0; i < data.length; i += 4)
      hash = Math.imul(
        hash ^ (data[i] | (data[i + 1] << 8) | (data[i + 2] << 16)),
        16777619,
      );
    return hash >>> 0;
  };
  const api = {
    done: null,
    events: [],
    debug: () => ({
      armed,
      started,
      baseline,
      frames: frames.slice(-6),
      hash: pixels(),
    }),
    arm(spec) {
      armed = spec;
      area = spec.area ?? [0, 0, 160, 150];
      started = undefined;
      baseline = pixels();
      frames = [];
      api.done = null;
    },
  };
  window.palLocalPerformance = api;
  const input = (event) => {
    api.events.push({
      type: event.type,
      code: event.code,
      key: event.key,
      trusted: event.isTrusted,
    });
    if (armed?.trigger && event.type !== armed.trigger) return;
    if (
      !armed ||
      started ||
      !event.isTrusted ||
      (event.type === "keydown" && event.code !== armed.code)
    )
      return;
    started = {
      at: performance.now(),
      wall: performance.timeOrigin + performance.now(),
      trusted: event.isTrusted,
      type: event.type,
      code: event.code,
    };
  };
  window.addEventListener("keydown", input, true);
  window.addEventListener("pointerdown", input, true);
  window.addEventListener("pointerup", input, true);
  const tick = (at) => {
    if (started && armed) {
      const board = document.querySelector("[data-pal-board]"),
        hash = pixels();
      frames.push({
        at,
        hash,
        phase: board?.dataset.palPhase,
        kind: board?.dataset.palMenuKind,
      });
      const matches =
        (armed.battleMenu === undefined ||
          Number(board?.dataset.palBattleMenu) === armed.battleMenu) &&
        (armed.battleState === undefined ||
          Number(board?.dataset.palBattleState) === armed.battleState) &&
        (armed.battleSelected === undefined ||
          Number(board?.dataset.palBattleSelected) === armed.battleSelected) &&
        (armed.dialogueFrom === undefined ||
          board?.dataset.palDialogId !== armed.dialogueFrom);
      if (
        matches &&
        hash !== baseline &&
        (armed.phase === "*" || board?.dataset.palPhase === armed.phase) &&
        Number(board?.dataset.palMenuKind ?? 0) === armed.kind &&
        (armed.selected === undefined ||
          Number(board?.dataset.palMenuSelected) === armed.selected)
      ) {
        api.done = {
          ...started,
          responseMS: at - started.at,
          pixelWall: performance.timeOrigin + at,
          target: armed,
          frames,
        };
        armed = undefined;
      }
    }
    frame = requestAnimationFrame(tick);
  };
  frame = requestAnimationFrame(tick);
  window.addEventListener("pagehide", () => cancelAnimationFrame(frame));
})();
