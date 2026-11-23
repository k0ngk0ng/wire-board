import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  useImperativeHandle,
  type Ref,
  type ReactNode,
} from "react";
import { Maximize, Minimize } from "lucide-react";
import "./room-fullscreen.css";
import "./game-control-guard.css";
import { GameOverlayRoot } from "./game-overlay-root";
import { protectGameControls } from "./game-control-guard";
import { restoreGameFocusOnBackground } from "./game-focus";
import { placeFullscreenOverlay } from "./fullscreen-overlay";

type Mode = "native" | "page" | "";
type FullscreenDocument = Document & {
  webkitFullscreenElement?: Element;
  webkitExitFullscreen?: () => Promise<void> | void;
};
type FullscreenElement = HTMLElement & {
  webkitRequestFullscreen?: () => Promise<void> | void;
};

function useRoomFullscreen(
  scope: string | undefined,
  root: FullscreenElement | null,
) {
  const [mode, setMode] = useState<Mode>("");
  const [pending, setPending] = useState(false);
  const control = useRef({ generation: 0, wanted: false, mode: "" as Mode });
  const scroll = useRef({ x: 0, y: 0 });
  const trigger = useRef<HTMLElement | null>(null);
  const doc = document as FullscreenDocument;
  const nativeElement = () =>
    doc.fullscreenElement ?? doc.webkitFullscreenElement;
  const exitNative = async () => {
    if (!root || nativeElement() !== root) return;
    try {
      if (doc.exitFullscreen) await doc.exitFullscreen();
      else await doc.webkitExitFullscreen?.();
    } catch {
    }
  };
  const update = (next: Mode) => {
    control.current.mode = next;
    setMode(next);
  };
  const exit = () => {
    control.current.wanted = false;
    control.current.generation++;
    setPending(false);
    update("");
    void exitNative();
  };

  useEffect(() => {
    const change = () => {
      if (root && nativeElement() === root && control.current.wanted)
        update("native");
      else if (control.current.mode === "native") exit();
    };
    const key = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || !control.current.wanted) return;
      event.preventDefault();
      exit();
    };
    document.addEventListener("fullscreenchange", change);
    document.addEventListener("webkitfullscreenchange", change);
    document.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("fullscreenchange", change);
      document.removeEventListener("webkitfullscreenchange", change);
      document.removeEventListener("keydown", key, true);
      exit();
    };
  }, [scope, root]);

  useLayoutEffect(() => {
    if (!mode || !root) return;
    const siblings: [HTMLElement, boolean][] = [];
    for (
      let node: HTMLElement | null = root;
      node?.parentElement;
      node = node.parentElement
    ) {
      for (const sibling of node.parentElement.children) {
        if (sibling !== node && sibling instanceof HTMLElement) {
          siblings.push([sibling, sibling.inert]);
          sibling.inert = true;
        }
      }
    }
    const overflow = document.documentElement.style.overflow;
    document.documentElement.style.overflow = "hidden";
    root.scrollTop = 0;
    const gameFocus = root.querySelector<HTMLElement>("[data-game-focus]") ??
      root.querySelector<HTMLElement>("[data-fullscreen-board]");
    gameFocus?.focus({ preventScroll: true });
    return () => {
      siblings.forEach(([element, inert]) => {
        element.inert = inert;
      });
      document.documentElement.style.overflow = overflow;
      const focusTarget = trigger.current?.isConnected
        ? trigger.current
        : root.querySelector<HTMLElement>("[data-fullscreen-board]");
      focusTarget?.focus({ preventScroll: true });
      window.scrollTo(scroll.current.x, scroll.current.y);
    };
  }, [mode, root]);

  const enter = async () => {
    if (control.current.wanted || !scope || !root) return;
    trigger.current = document.activeElement as HTMLElement | null;
    scroll.current = { x: window.scrollX, y: window.scrollY };
    control.current.wanted = true;
    const generation = ++control.current.generation;
    setPending(true);
    update("page");
    try {
      if (root.requestFullscreen) await root.requestFullscreen();
      else if (root.webkitRequestFullscreen)
        await root.webkitRequestFullscreen();
      if (generation !== control.current.generation) {
        await exitNative();
        return;
      }
      update(nativeElement() === root ? "native" : "page");
    } catch {
      if (generation === control.current.generation) update("page");
    } finally {
      if (generation === control.current.generation) setPending(false);
    }
  };
  const toggle = () => control.current.wanted ? exit() : void enter();
  return { active: !!mode, mode, pending, toggle, enter };
}

function FullscreenButton({
  active,
  pending,
  toggle,
}: ReturnType<typeof useRoomFullscreen>) {
  return (
    <button
      className="room-fullscreen-toggle"
      data-fullscreen-toggle
      aria-label={active ? "退出全屏" : "全屏"}
      aria-pressed={active}
      disabled={pending}
      title={active ? "退出全屏（Esc）" : "全屏显示游戏"}
      onClick={() => void toggle()}
    >
      {active ? <Minimize size={16} /> : <Maximize size={16} />}
    </button>
  );
}

export type GameFullscreenHandle = { enter: () => void };

export function GameFullscreen({
  ref,
  scope,
  enabled,
  autoEnter = false,
  children,
  result,
  status,
  toolbarActions,
  toolbarOverlay,
  preserveWhileLoading = false,
  onReadyChange,
  onChatTargetChange,
}: {
  ref?: Ref<GameFullscreenHandle>;
  scope: string;
  enabled: boolean;
  autoEnter?: boolean;
  children: ReactNode;
  result?: ReactNode;
  status?: ReactNode;
  toolbarActions?: ReactNode;
  toolbarOverlay?: boolean;
  preserveWhileLoading?: boolean;
  onReadyChange: (ready: boolean) => void;
  onChatTargetChange?: (target: HTMLElement | null) => void;
}) {
  const [root, setRoot] = useState<HTMLDivElement | null>(null);
  const board = useRef<HTMLDivElement>(null);
  const toolbar = useRef<HTMLDivElement>(null);
  const statusRoot = useRef<HTMLDivElement>(null);
  const autoStarted = useRef("");
  const overlay = toolbarOverlay !== false;
  const fullscreen = useRoomFullscreen(enabled || preserveWhileLoading || autoEnter ? scope : undefined, root);
  useEffect(() => {
    if (!autoEnter || !root || autoStarted.current === scope || !window.matchMedia("(pointer: coarse)").matches) return;
    autoStarted.current = scope;
    void fullscreen.enter();
    return () => { autoStarted.current = ""; };
  }, [autoEnter, root, scope]);
  useEffect(() => {
    if (root) return protectGameControls(root);
  }, [root]);
  useEffect(() => {
    if (root && enabled) return restoreGameFocusOnBackground(root);
  }, [root, enabled]);
  useLayoutEffect(() => {
    onChatTargetChange?.(fullscreen.active ? root : null);
    return () => onChatTargetChange?.(null);
  }, [fullscreen.active, root, onChatTargetChange]);
  useImperativeHandle(ref, () => ({
    enter: () => {
      if (enabled) void fullscreen.enter();
    },
  }));
  useEffect(() => {
    onReadyChange(enabled && !!root);
    return () => onReadyChange(false);
  }, [enabled, root, onReadyChange]);
  useLayoutEffect(() => {
    const container = board.current;
    const actions = toolbar.current;
    if (!container || !root || !actions || !fullscreen.active || !overlay) return;
    let frame = 0;
    const rectangle = (element: Element) => {
      const value = element.getBoundingClientRect();
      return { left: Math.round(value.left * 1000), top: Math.round(value.top * 1000), right: Math.round(value.right * 1000), bottom: Math.round(value.bottom * 1000) };
    };
    const position = (element: HTMLElement, obstacles: ReturnType<typeof rectangle>[], edge: 'left' | 'right') => {
      for (const property of ['left', 'top', 'right']) element.style.removeProperty(property);
      const style = getComputedStyle(element);
      const safeTop = Math.round(parseFloat(style.top) * 1000);
      const safeEdge = Math.round(parseFloat(edge === 'right' ? style.right : style.left) * 1000);
      const inset = Math.max(8000, Number.isSafeInteger(safeTop) ? safeTop : 8000, Number.isSafeInteger(safeEdge) ? safeEdge : 8000);
      const box = element.getBoundingClientRect();
      const size = { width: Math.round(box.width * 1000), height: Math.round(box.height * 1000) };
      if (size.width <= 0 || size.height <= 0) return undefined;
      const bounds = rectangle(root);
      const placed = placeFullscreenOverlay({ bounds, size, obstacles, edge, inset });
      element.style.left = (placed.left - bounds.left) / 1000 + 'px';
      element.style.top = (placed.top - bounds.top) / 1000 + 'px';
      element.style.right = 'auto';
      element.dataset.fullscreenOccluded = placed.overlap ? 'true' : 'false';
      return { left: placed.left, top: placed.top, right: placed.left + size.width, bottom: placed.top + size.height };
    };
    const measure = () => {
      frame = 0;
      const obstacles = [...container.querySelectorAll<HTMLElement>('button,input,select,[role="button"],[data-fullscreen-protected],[data-chat-protected],[data-touch-protected]')].filter(element => {
        if (element.closest('[data-controller-group="board"],[data-controller-group="grid"]') && !element.matches('[data-fullscreen-protected],[data-chat-protected],[data-touch-protected]')) return false;
        return element.getClientRects().length > 0 && getComputedStyle(element).visibility !== 'hidden';
      }).map(rectangle).filter(rect => rect.right > rect.left && rect.bottom > rect.top);
      const actionBox = position(actions, obstacles, 'right');
      root.style.setProperty('--game-fullscreen-toolbar-width', actionBox ? (actionBox.right - actionBox.left) / 1000 + 'px' : '0px');
      const status = statusRoot.current;
      const statusBox = status ? position(status, actionBox ? [...obstacles, actionBox] : obstacles, 'left') : undefined;
      root.style.setProperty('--game-fullscreen-status-width', status ? status.getBoundingClientRect().width + 'px' : '0px');
      root.style.setProperty('--game-fullscreen-status-offset', statusBox ? (statusBox.right - rectangle(root).left + 8000) / 1000 + 'px' : '0px');
      root.style.setProperty('--game-fullscreen-toolbar-offset', actionBox ? (rectangle(root).right - actionBox.left + 8000) / 1000 + 'px' : '0px');
      const bottom = Math.max(actionBox?.bottom ?? 0, statusBox?.bottom ?? 0) - rectangle(root).top + 8000;
      root.style.setProperty('--game-fullscreen-toolbar-bottom', bottom / 1000 + 'px');
    };
    const schedule = () => { if (!frame) frame = requestAnimationFrame(measure); };
    const resize = new ResizeObserver(schedule);
    resize.observe(container);
    resize.observe(actions);
    if (statusRoot.current) resize.observe(statusRoot.current);
    if (container.firstElementChild) resize.observe(container.firstElementChild);
    const changes = new MutationObserver(schedule);
    changes.observe(container, { childList: true, subtree: true, characterData: true });
    window.addEventListener('resize', schedule);
    measure();
    return () => {
      cancelAnimationFrame(frame);
      resize.disconnect();
      changes.disconnect();
      window.removeEventListener('resize', schedule);
      for (const property of ['--game-fullscreen-toolbar-width', '--game-fullscreen-status-width', '--game-fullscreen-toolbar-bottom', '--game-fullscreen-status-offset', '--game-fullscreen-toolbar-offset']) root.style.removeProperty(property);
    };
  }, [fullscreen.active, root, overlay]);
  return (
    <div
      ref={setRoot}
      className="game-fullscreen-surface"
      data-game-fullscreen-surface
      data-room-fullscreen={fullscreen.mode || undefined}
      data-fullscreen-toolbar-overlay={overlay || undefined}
      data-fullscreen-toolbar-owned={toolbarOverlay === true || undefined}
      onKeyDown={(event) => {
        if (event.key !== "F11" || (!enabled && !fullscreen.active)) return;
        event.preventDefault();
        if (!event.repeat && !fullscreen.pending) void fullscreen.toggle();
      }}
    >
      <GameOverlayRoot.Provider value={root}>
        {fullscreen.active && overlay && status && <div ref={statusRoot} className="game-fullscreen-status" data-game-controls>{status}</div>}
        {fullscreen.active && (
          <div
            ref={toolbar}
            className="game-fullscreen-toolbar"
            data-game-controls
          >
            {!overlay && status && (
              <div ref={statusRoot} className="game-fullscreen-status">{status}</div>
            )}
            {toolbarActions && (
              <div className="game-fullscreen-actions">{toolbarActions}</div>
            )}
            <FullscreenButton {...fullscreen} />
          </div>
        )}
        <div
          ref={board}
          className="game-fullscreen-board"
          data-game-controls
          data-fullscreen-board
          tabIndex={-1}
        >
          {children}
        </div>
        {result}
      </GameOverlayRoot.Provider>
    </div>
  );
}
