import { useContext, useEffect, useRef } from "react";
import { GameOverlayRoot } from "../ui/game-overlay-root";
import { api } from "../ui/board-host";
import {
  KeyboardController,
  actionsForKey,
  type ControllerProfile,
  type ControllerSettingsData,
} from "./input";
import { PhysicalController, mergeLogicalButtons } from "./gamepad";
import { ControllerNavigation } from "./navigation";
import "./navigation.css";

export function NavigationController({
  gameId,
  userId,
  enabled,
}: {
  gameId: string;
  userId: string;
  enabled: boolean;
}) {
  const root = useContext(GameOverlayRoot);
  const allowed = useRef(enabled);
  allowed.current = enabled;
  useEffect(() => {
    if (!root || !userId) return;
    let alive = true,
      frame = 0,
      revision = -1;
    let keyboard: KeyboardController | undefined,
      physical: PhysicalController | undefined;
    let profile: ControllerProfile | undefined;
    const defaultKeys = new Set<string>();
    const navigation = new ControllerNavigation(root);
    const modelID = `game:${gameId}`;
    const clear = () => {
      keyboard?.clear();
      physical?.clear();
      navigation.clear();
    };
    const context = () => {
      const dialogs = [
        ...document.querySelectorAll<HTMLElement>(
          '[aria-modal="true"],dialog[open]',
        ),
      ].filter((e) => e.getClientRects().length && !e.closest("[inert]"));
      const active = document.activeElement;
      return (
        allowed.current &&
        !document.hidden &&
        document.hasFocus() &&
        !dialogs.some((d) => !root.contains(d)) &&
        !(
          active instanceof Element &&
          active.closest(
            'input:not([type="checkbox"]):not([type="radio"]):not([type="number"]):not([type="range"]),textarea,[contenteditable]:not([contenteditable="false"])',
          )
        )
      );
    };
    const apply = (next: ControllerProfile) => {
      if (!alive || next.revision < revision) return;
      revision = next.revision;
      profile = next;
      keyboard?.configure(next);
      physical?.configure(next);
      navigation.clear();
    };
    const changed = (event: Event) => {
      const change = (event as CustomEvent).detail;
      if (change?.userId === userId && change.model === modelID)
        apply(change.profile);
    };
    const down = (event: KeyboardEvent) => {
      if (
        !profile ||
        !context() ||
        event.isComposing ||
        ((event.ctrlKey || event.altKey || event.metaKey) &&
          !/^(Control|Alt|Meta)/.test(event.code))
      )
        return;
      const target = event.target;
      if (
        target instanceof Element &&
        target.closest('input[type="number"],input[type="range"]')
      )
        return;
      if (
        target instanceof Element &&
        target !== document.body &&
        !root.contains(target)
      )
        return;
      if (
        !actionsForKey(profile, event.code).length &&
        !defaultKeys.has(event.code)
      )
        return;
      event.preventDefault();
      event.stopImmediatePropagation();
      keyboard?.press(event.code, performance.now(), event.repeat);
    };
    const up = (event: KeyboardEvent) => keyboard?.release(event.code);
    const tick = (at: number) => {
      if (!alive) return;
      if (context()) {
        if (
          document.activeElement?.matches(
            'input[type="number"],input[type="range"]',
          ) ||
          (document.activeElement !== document.body &&
            !root.contains(document.activeElement))
        )
          keyboard?.clear();
        navigation.sample(
          mergeLogicalButtons(
            keyboard?.sample(at) ?? [],
            physical?.sample(at, true, root) ?? [],
          ),
          at,
        );
        root.dataset.controllerNavigation = gameId;
      } else clear();
      frame = requestAnimationFrame(tick);
    };
    window.addEventListener("controller-settings-changed", changed);
    void api(`/controllers?game=${encodeURIComponent(gameId)}`)
      .then((settings: ControllerSettingsData) => {
        const model = settings.models.find((m) => m.id === modelID),
          saved = settings.profiles[modelID];
        if (!alive || model?.layout !== "navigation" || !saved) return;
        for (const key of Object.values(model.defaults.keyboard))
          defaultKeys.add(key);
        for (const keys of Object.values(model.defaults.keyboardAlt ?? {}))
          for (const key of keys) defaultKeys.add(key);
        if (!profile || saved.revision >= revision) apply(saved);
        keyboard = new KeyboardController(profile!, {});
        physical = new PhysicalController(profile!, model);
        window.addEventListener("keydown", down, true);
        window.addEventListener("keyup", up, true);
        window.addEventListener("blur", clear);
        document.addEventListener("visibilitychange", clear);
        frame = requestAnimationFrame(tick);
      })
      .catch(() => {});
    return () => {
      alive = false;
      cancelAnimationFrame(frame);
      clear();
      physical?.dispose();
      navigation.dispose();
      if (root.dataset.controllerNavigation === gameId)
        delete root.dataset.controllerNavigation;
      window.removeEventListener("controller-settings-changed", changed);
      window.removeEventListener("keydown", down, true);
      window.removeEventListener("keyup", up, true);
      window.removeEventListener("blur", clear);
      document.removeEventListener("visibilitychange", clear);
    };
  }, [root, gameId, userId]);
  return null;
}
