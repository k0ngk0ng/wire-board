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
import { GameOverlayRoot } from "./game-overlay-root";

type Mode = "native" | "page" | "";
type FullscreenDocument = Document & {
  webkitFullscreenElement?: Element;
  webkitExitFullscreen?: () => Promise<void> | void;
};
type FullscreenElement = HTMLElement & {
  webkitRequestFullscreen?: () => Promise<void> | void;
};

// Only the game surface enters fullscreen; the surrounding room stays mounted.
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
      // A simultaneous browser exit can invalidate the request.
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
      // Games also use Escape to cancel a held shot or gesture. Let their
      // handlers release input while the platform restores the page layout.
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
    root
      .querySelector<HTMLElement>("[data-fullscreen-board]")
      ?.focus({ preventScroll: true });
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

  const toggle = async () => {
    if (control.current.wanted) {
      exit();
      return;
    }
    if (!scope || !root) return;
    trigger.current = document.activeElement as HTMLElement | null;
    scroll.current = { x: window.scrollX, y: window.scrollY };
    control.current.wanted = true;
    const generation = ++control.current.generation;
    setPending(true);
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
  return { active: !!mode, mode, pending, toggle };
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
  children,
  result,
  status,
  onReadyChange,
  onChatTargetChange,
}: {
  ref?: Ref<GameFullscreenHandle>;
  scope: string;
  enabled: boolean;
  children: ReactNode;
  result?: ReactNode;
  status?: ReactNode;
  onReadyChange: (ready: boolean) => void;
  onChatTargetChange?: (target: HTMLElement | null) => void;
}) {
  const [root, setRoot] = useState<HTMLDivElement | null>(null);
  const board = useRef<HTMLDivElement>(null);
  const [rightInset, setRightInset] = useState(0);
  const fullscreen = useRoomFullscreen(enabled ? scope : undefined, root);
  useLayoutEffect(() => {
    onChatTargetChange?.(fullscreen.active ? root : null);
    return () => onChatTargetChange?.(null);
  }, [fullscreen.active, root, onChatTargetChange]);
  useImperativeHandle(ref, () => ({
    enter: () => {
      if (!fullscreen.active) void fullscreen.toggle();
    },
  }));
  useEffect(() => {
    onReadyChange(enabled && !!root);
    return () => onReadyChange(false);
  }, [enabled, root, onReadyChange]);
  useLayoutEffect(() => {
    const container = board.current;
    if (!container) return;
    const measure = () => {
      const target = container.firstElementChild;
      if (!target) return;
      const outer = container.getBoundingClientRect(),
        inner = target.getBoundingClientRect();
      setRightInset(Math.max(0, outer.right - inner.right));
    };
    const resize = new ResizeObserver(measure);
    resize.observe(container);
    const observeBoard = () => {
      if (container.firstElementChild)
        resize.observe(container.firstElementChild);
      measure();
    };
    const changes = new MutationObserver(observeBoard);
    changes.observe(container, { childList: true });
    observeBoard();
    return () => {
      resize.disconnect();
      changes.disconnect();
    };
  }, []);
  return (
    <div
      ref={setRoot}
      className="game-fullscreen-surface"
      data-game-fullscreen-surface
      data-room-fullscreen={fullscreen.mode || undefined}
    >
      <GameOverlayRoot.Provider value={root}>
        {fullscreen.active && (
          <div
            className="game-fullscreen-toolbar"
            style={{ paddingRight: rightInset }}
          >
            {fullscreen.active && status && (
              <div className="game-fullscreen-status">{status}</div>
            )}
            <FullscreenButton {...fullscreen} />
          </div>
        )}
        <div
          ref={board}
          className="game-fullscreen-board"
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
