import { createContext, useContext, type ReactNode } from "react";
import { createPortal } from "react-dom";

// Game-owned dialogs and animation layers stay inside the fullscreen surface.
export const GameOverlayRoot = createContext<HTMLElement | null>(null);
export function useGameOverlayRoot() {
  return useContext(GameOverlayRoot) ?? document.body;
}

// Action errors originate in the host, but must remain visible while playing.
export function GameFeedback({ children }: { children: ReactNode }) {
  const target = document.querySelector<HTMLElement>("[data-room-fullscreen]");
  return createPortal(children, target ?? document.body);
}
