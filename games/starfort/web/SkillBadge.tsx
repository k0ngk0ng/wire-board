import { useEffect, useLayoutEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useGameOverlayRoot } from "../../../web/src/ui/game-overlay-root";
import type { Passive, TowerKind } from "./types";
import { skillProgress } from "./skill-progress";

export function SkillBadge({
  passive,
  kind,
  phase,
  elapsedMS,
}: {
  passive: Passive;
  kind: TowerKind;
  phase: string;
  elapsedMS: number;
}) {
  const root = useRef<HTMLDivElement>(null),
    tipID = useId();
  const overlayRoot = useGameOverlayRoot(),
    tip = useRef<HTMLDivElement>(null);
  const hovered = useRef(false),
    focused = useRef(false),
    pinnedRef = useRef(false),
    closeTimer = useRef<number | undefined>(undefined);
  const [placement, setPlacement] = useState<{
    left: number;
    top: number;
    width: number;
    maxHeight: number;
  }>();
  const clearClose = () => {
    if (closeTimer.current !== undefined)
      window.clearTimeout(closeTimer.current);
    closeTimer.current = undefined;
  };
  const show = () => {
    clearClose();
    setOpen(true);
  };
  const closeLater = () => {
    clearClose();
    closeTimer.current = window.setTimeout(() => {
      if (!hovered.current && !focused.current && !pinnedRef.current)
        setOpen(false);
    }, 100);
  };
  const dismiss = () => {
    clearClose();
    pinnedRef.current = false;
    setPinned(false);
    setOpen(false);
  };
  const [open, setOpen] = useState(false),
    [pinned, setPinned] = useState(false),
    [elapsed, setElapsed] = useState(0);
  useEffect(() => () => clearClose(), []);
  useLayoutEffect(() => {
    if (!open) {
      setPlacement(undefined);
      return;
    }
    let frame = 0;
    const position = () => {
      const button = root.current?.querySelector("button"),
        node = tip.current;
      if (!button || !node) return;
      const r = button.getBoundingClientRect(),
        viewport = window.visualViewport;
      const x = viewport?.offsetLeft ?? 0,
        y = viewport?.offsetTop ?? 0,
        w = viewport?.width ?? innerWidth,
        h = viewport?.height ?? innerHeight;
      const width = Math.min(280, Math.max(1, w - 16)),
        maxHeight = Math.max(48, h - 16),
        height = Math.min(node.getBoundingClientRect().height, maxHeight);
      const left = Math.round(
        Math.max(x + 8, Math.min(x + w - width - 8, r.left)),
      );
      const preferred =
        r.top - height - 8 >= y + 8 ? r.top - height - 8 : r.bottom + 8;
      const top = Math.round(
        Math.max(y + 8, Math.min(y + h - height - 8, preferred)),
      );
      setPlacement((old) =>
        old &&
        old.left === left &&
        old.top === top &&
        old.width === width &&
        old.maxHeight === maxHeight
          ? old
          : { left, top, width, maxHeight },
      );
      frame = requestAnimationFrame(position);
    };
    position();
    return () => cancelAnimationFrame(frame);
  }, [open, overlayRoot]);
  useEffect(() => {
    setElapsed(0);
    let frame = 0,
      started = performance.now(),
      last = 0;
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)");
    const tick = (now: number) => {
      if (document.hidden || phase !== "battle" || reduced.matches) return;
      if (now - last >= 80 || now - started >= passive.readyInMS) {
        setElapsed(now - started);
        last = now;
      }
      if (now - started < passive.readyInMS)
        frame = requestAnimationFrame(tick);
    };
    const visibility = () => {
      cancelAnimationFrame(frame);
      started = performance.now();
      setElapsed(0);
      if (!document.hidden) frame = requestAnimationFrame(tick);
    };
    if (phase === "battle" && passive.readyInMS > 0 && !passive.maxStacks)
      frame = requestAnimationFrame(tick);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      cancelAnimationFrame(frame);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, [phase, elapsedMS, passive.readyInMS, passive.maxStacks]);
  useEffect(() => {
    if (!open) return;
    const outside = (e: PointerEvent) => {
      if (
        !root.current?.contains(e.target as Node) &&
        !tip.current?.contains(e.target as Node)
      )
        dismiss();
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        dismiss();
      }
    };
    document.addEventListener("pointerdown", outside);
    window.addEventListener("keydown", key, true);
    // Native fullscreen can consume Escape before the page receives keydown.
    document.addEventListener("fullscreenchange", dismiss);
    document.addEventListener("webkitfullscreenchange", dismiss);
    return () => {
      document.removeEventListener("pointerdown", outside);
      window.removeEventListener("keydown", key, true);
      document.removeEventListener("fullscreenchange", dismiss);
      document.removeEventListener("webkitfullscreenchange", dismiss);
    };
  }, [open]);
  const state = skillProgress(passive, elapsed, phase === "battle");
  const label = passive.triggerChance
    ? `${passive.triggerChance}% 触发`
    : state.stacks
      ? `${passive.stacks}/${passive.maxStacks}`
      : state.ready
        ? "就绪"
        : `${(state.remainingMS / 1000).toFixed(1)} 秒`;
  return (
    <div className="sf-skill" ref={root} data-ready={state.ready}>
      <button
        className={`sf-skill-icon sf-skill-${kind}`}
        data-sf-skill
        aria-label={`${passive.name}，${label}，查看技能说明`}
        aria-describedby={open ? tipID : undefined}
        aria-expanded={open}
        aria-pressed={pinned}
        onPointerEnter={(e) => {
          if (e.pointerType === "mouse") {
            hovered.current = true;
            show();
          }
        }}
        onPointerLeave={() => {
          hovered.current = false;
          closeLater();
        }}
        onFocus={() => {
          focused.current = true;
          show();
        }}
        onBlur={() => {
          focused.current = false;
          closeLater();
        }}
        onClick={() => {
          const next = !pinnedRef.current;
          pinnedRef.current = next;
          setPinned(next);
          clearClose();
          setOpen(next);
        }}
      >
        <svg viewBox="0 0 48 48" aria-hidden="true">
          <circle className="sf-skill-track" cx="24" cy="24" r="20" />
          <circle
            className="sf-skill-progress"
            data-sf-skill-progress
            cx="24"
            cy="24"
            r="20"
            pathLength="100"
            strokeDasharray="100"
            strokeDashoffset={100 * (1 - state.progress)}
            transform="rotate(-90 24 24)"
          />
          <g
            className="sf-skill-glyph"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.4"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            {passive.variant === "volley" ? (
              <>
                <path d="M24 34V13m-4 4 4-4 4 4M17 32l-5-17m-2 6 2-6 5 3M31 32l5-17m-5 3 5-3 2 6" />
              </>
            ) : passive.variant === "heart" ? (
              <>
                <path d="M14 34 34 14m-8 0h8v8" />
                <path d="M24 29c-13-6-8-15 0-9 8-6 13 3 0 9Z" />
              </>
            ) : passive.variant === "ice-wall" ? (
              <>
                <path d="m12 32 2-14 5-4 3 18m-2 0 4-20 5 7 1 13m-1 0 3-12 4 3 1 9M11 34h27" />
              </>
            ) : kind === "arrow" ? (
              <>
                <path d="M15 33 32 16m-9 0h9v9M15 25l7 7m-9-3 6 6" />
              </>
            ) : kind === "cannon" ? (
              <>
                <ellipse cx="24" cy="30" rx="10" ry="4" />
                <path d="m18 25-3-6m15 6 3-6m-9 5v-9m-5 3-3-3m13 3 3-3" />
              </>
            ) : kind === "frost" ? (
              <>
                <path d="M24 12v24M14 18l20 12M14 30l20-12m-14-3 4 4 4-4m-8 18 4-4 4 4" />
              </>
            ) : (
              <>
                <circle cx="15" cy="28" r="4" />
                <circle cx="25" cy="17" r="4" />
                <circle cx="34" cy="30" r="4" />
                <path d="m18 25 4-5m6 0 4 6" />
              </>
            )}
          </g>
        </svg>
      </button>
      <span>
        <strong>{passive.name}</strong>
        <small data-sf-skill-status>{label}</small>
      </span>
      {open &&
        createPortal(
          <div
            ref={tip}
            id={tipID}
            role="tooltip"
            className="sf-skill-tooltip"
            data-sf-skill-tooltip
            style={{
              ...placement,
              visibility: placement ? "visible" : "hidden",
            }}
            onPointerEnter={(e) => {
              if (e.pointerType === "mouse") {
                hovered.current = true;
                show();
              }
            }}
            onPointerLeave={() => {
              hovered.current = false;
              closeLater();
            }}
          >
            <strong>{passive.name}</strong>
            <p>{passive.description}</p>
            {passive.cooldownMS > 0 && (
              <small>冷却 {passive.cooldownMS / 1000} 秒</small>
            )}
          </div>,
          overlayRoot,
        )}
    </div>
  );
}
