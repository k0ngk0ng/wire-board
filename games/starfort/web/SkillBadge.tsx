import { useEffect, useId, useRef, useState } from "react";
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
  const [open, setOpen] = useState(false),
    [pinned, setPinned] = useState(false),
    [elapsed, setElapsed] = useState(0);
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
      if (!root.current?.contains(e.target as Node)) {
        setOpen(false);
        setPinned(false);
      }
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        setPinned(false);
      }
    };
    document.addEventListener("pointerdown", outside);
    window.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("pointerdown", outside);
      window.removeEventListener("keydown", key, true);
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
        onPointerEnter={(e) => {
          if (e.pointerType === "mouse") setOpen(true);
        }}
        onPointerLeave={() => {
          if (!pinned) setOpen(false);
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => {
          if (!pinned) setOpen(false);
        }}
        onClick={() => {
          setPinned(!pinned);
          setOpen(!pinned);
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
      {open && (
        <div
          id={tipID}
          role="tooltip"
          className="sf-skill-tooltip"
          data-sf-skill-tooltip
        >
          <strong>{passive.name}</strong>
          <p>{passive.description}</p>
          {passive.cooldownMS > 0 && (
            <small>冷却 {passive.cooldownMS / 1000} 秒</small>
          )}
        </div>
      )}
    </div>
  );
}
