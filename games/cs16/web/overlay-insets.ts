import { useLayoutEffect, type RefObject } from 'react';

export function useOverlayInsets(root: RefObject<HTMLElement | null>) {
  useLayoutEffect(() => {
    const board = root.current;
    if (!board) return;
    const surface = board.closest<HTMLElement>('[data-game-fullscreen-surface]');
    let pending = 0;
    const registered = new Set<Element>();
    const set = (key: string, value: number) => {
      const text = `${Math.max(0, Math.ceil(value))}px`;
      if (board.style.getPropertyValue(key) !== text) board.style.setProperty(key, text);
    };
    const measure = () => {
      pending = 0;
      const rect = board.getBoundingClientRect();
      const toolbar = surface?.querySelector<HTMLElement>('.game-fullscreen-toolbar');
      const chat = surface?.querySelector<HTMLElement>('.chat-toggle') ?? document.querySelector<HTMLElement>('.chat-toggle');
      const top = toolbar?.getBoundingClientRect();
      const bubble = chat?.getBoundingClientRect();
      set('--cs16-toolbar-bottom', top && top.width && top.bottom > rect.top && top.top < rect.bottom ? top.bottom - rect.top + 8 : 0);
      set('--cs16-chat-right', bubble && bubble.width && bubble.bottom > rect.bottom - 140 && bubble.top < rect.bottom && bubble.right > rect.left + rect.width / 2 && bubble.left < rect.right ? rect.right - bubble.left + 8 : 0);
      for (const element of [toolbar, chat]) if (element && !registered.has(element)) {
        registered.add(element);
        resize.observe(element);
        mutation.observe(element, { attributes: true, attributeFilter: ['class', 'style'] });
      }
    };
    const schedule = () => { if (!pending) pending = requestAnimationFrame(measure); };
    const resize = new ResizeObserver(schedule);
    const mutation = new MutationObserver(schedule);
    const descendants = new MutationObserver(schedule);
    resize.observe(board);
    if (surface) {
      mutation.observe(surface, { attributes: true, childList: true, attributeFilter: ['data-room-fullscreen', 'style', 'class'] });
      descendants.observe(surface, { childList: true, subtree: true });
    }
    window.addEventListener('resize', schedule);
    window.addEventListener('scroll', schedule, true);
    document.addEventListener('fullscreenchange', schedule);
    measure();
    return () => { cancelAnimationFrame(pending); resize.disconnect(); mutation.disconnect(); descendants.disconnect(); window.removeEventListener('resize', schedule); window.removeEventListener('scroll', schedule, true); document.removeEventListener('fullscreenchange', schedule); };
  }, [root]);
}
