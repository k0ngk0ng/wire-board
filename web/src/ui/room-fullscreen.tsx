import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Maximize, Minimize } from "lucide-react";
import "./room-fullscreen.css";

type Mode = "native" | "page" | "";
type FullscreenDocument = Document & {
  webkitFullscreenElement?: Element;
  webkitExitFullscreen?: () => Promise<void> | void;
};
type FullscreenElement = HTMLElement & {
  webkitRequestFullscreen?: () => Promise<void> | void;
};

// Fullscreen the document so platform dialogs and game portals remain visible.
// The game stays mounted throughout entry, resize and exit.
export function useRoomFullscreen(scope: string | undefined) {
  const [mode, setMode] = useState<Mode>("");
  const [pending, setPending] = useState(false);
  const control = useRef({ generation: 0, wanted: false, mode: "" as Mode });
  const scroll = useRef({ x: 0, y: 0 });
  const doc = document as FullscreenDocument;
  const root = document.documentElement as FullscreenElement;
  const nativeElement = () =>
    doc.fullscreenElement ?? doc.webkitFullscreenElement;
  const exitNative = async () => {
    if (nativeElement() !== root) return;
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
      if (nativeElement() === root && control.current.wanted) update("native");
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
  }, [scope]);

  useLayoutEffect(() => {
    if (!mode) return;
    root.dataset.roomFullscreen = mode;
    window.scrollTo(0, 0);
    // Space belongs to the game after entry, not to the fullscreen button.
    document
      .querySelector<HTMLElement>(".game-main")
      ?.focus({ preventScroll: true });
    return () => {
      delete root.dataset.roomFullscreen;
      document
        .querySelector<HTMLElement>("[data-fullscreen-toggle]")
        ?.focus({ preventScroll: true });
      window.scrollTo(scroll.current.x, scroll.current.y);
    };
  }, [mode]);

  const toggle = async () => {
    if (control.current.wanted) {
      exit();
      return;
    }
    if (!scope) return;
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
  return { active: !!mode, pending, toggle };
}

export function FullscreenButton({
  active,
  pending,
  toggle,
}: ReturnType<typeof useRoomFullscreen>) {
  return (
    <button
      className="subtle room-fullscreen-toggle"
      data-fullscreen-toggle
      aria-label={active ? "退出全屏" : "全屏"}
      aria-pressed={active}
      disabled={pending}
      title={active ? "退出全屏（Esc）" : "全屏显示游戏"}
      onClick={() => void toggle()}
    >
      {active ? <Minimize size={16} /> : <Maximize size={16} />}
      <span>{active ? "退出全屏" : "全屏"}</span>
    </button>
  );
}
