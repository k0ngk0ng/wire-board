import { useEffect, useRef, type RefObject } from "react";
import { api } from "../ui/board-host";
import type { ControllerProfile, ControllerSettingsData } from "./input";
import { GameController, type GameControllerContext } from "./game-controller";
import "./navigation.css";

export function useGameController(
  options: GameControllerContext & {
    gameId: string;
    userId: string;
    scope: string;
    root: RefObject<HTMLElement | null>;
    error: (message: string) => void;
  },
) {
  const latest = useRef(options);
  latest.current = options;
  const controller = useRef<GameController | null>(null);
  useEffect(() => {
    if (!options.userId || !options.root.current) return;
    const root =
      options.root.current.closest<HTMLElement>("[data-fullscreen-board]") ??
      options.root.current;
    let alive = true,
      changedProfile: ControllerProfile | undefined;
    const modelID = `game:${options.gameId}`;
    const change = (event: Event) => {
      const detail = (event as CustomEvent).detail;
      if (detail?.userId !== options.userId || detail.model !== modelID) return;
      if (changedProfile && changedProfile.revision > detail.profile.revision)
        return;
      changedProfile = detail.profile;
      controller.current?.configure(detail.profile);
    };
    window.addEventListener("controller-settings-changed", change);
    void api(`/controllers?game=${encodeURIComponent(options.gameId)}`)
      .then((settings: ControllerSettingsData) => {
        if (!alive) return;
        const model = settings.models.find((m) => m.id === modelID),
          fetched = settings.profiles[modelID];
        if (!model || !fetched) throw Error("未找到当前游戏的控制器设置");
        const profile =
          changedProfile && changedProfile.revision >= fetched.revision
            ? changedProfile
            : fetched;
        changedProfile = profile;
        controller.current = new GameController(root, profile, model, {
          enabled: () => latest.current.enabled(),
          menu: () => latest.current.menu?.() ?? null,
          menuActions: options.menuActions,
          changed: () => latest.current.changed(),
          keyboardTarget: (target) =>
            latest.current.keyboardTarget?.(target) ?? true,
        });
        root.dataset.controllerInput = options.gameId;
        latest.current.changed();
      })
      .catch(() => {
        if (alive)
          latest.current.error("控制器设置暂时无法读取，可继续使用触屏操作。");
      });
    return () => {
      alive = false;
      window.removeEventListener("controller-settings-changed", change);
      controller.current?.dispose();
      controller.current = null;
      if (root.dataset.controllerInput === options.gameId)
        delete root.dataset.controllerInput;
    };
  }, [options.gameId, options.userId, options.scope]);
  return controller;
}
