import { useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import type { Event } from "./types";
import type { Point } from "./camera";

const verbs: Record<string, string> = {
  move: "行军",
  recruit: "征兵",
  build: "建造",
  disband: "解散",
  tax: "调整税率",
};
export function ActionPlayback({
  event,
  name,
  color,
  own,
  provinceName,
  now,
}: {
  event: Event;
  name: string;
  color: string;
  own: boolean;
  provinceName: (id?: string) => string;
  now: number;
}) {
  const applied = !!event.applied || !!event.after;
  const progress = Math.max(
    0,
    Math.min(1, (now - event.at) / Math.max(1, event.until - event.at)),
  );
  const where =
    event.type === "move"
      ? `${provinceName(event.from)} → ${provinceName(event.to)}`
      : event.province
        ? provinceName(event.province)
        : "国政";
  return (
    <div
      className="cq-playback"
      role="status"
      aria-live="polite"
      aria-atomic="true"
      data-cq-playback
      data-event-id={event.id}
      data-seat={event.seat}
      data-type={event.type}
      data-state={applied ? "result" : "running"}
      data-from={event.from}
      data-to={event.to}
      data-province={event.province}
      style={{ "--cq-action-color": color } as CSSProperties}
    >
      <div className="cq-playback-heading">
        <b title={name}>
          <i aria-hidden="true" />
          {own ? "我方" : "对方"} · {name}
        </b>
        <strong>
          {event.skipped
            ? "未执行"
            : applied
              ? event.text
              : (verbs[event.type] ?? event.type)}
        </strong>
      </div>
      <p>
        {where}
        {event.skipped
          ? ` · ${event.text}`
          : event.amount > 0
            ? ` · ${event.amount.toLocaleString("zh-CN")} 兵`
            : event.amount < 0
              ? " · 兵力未知"
              : ""}
      </p>
      <div className="cq-playback-progress" aria-hidden="true">
        <span
          key={`${event.id}:${applied}`}
          style={
            {
              "--cq-step-progress": progress,
              animationDuration: `${Math.max(1, event.until - now)}ms`,
            } as CSSProperties
          }
        />
      </div>
    </div>
  );
}

function MarchingArmy({
  event,
  from,
  to,
  scale,
  color,
  army,
  now,
  animate,
}: {
  event: Event;
  from: Point;
  to: Point;
  scale: number;
  color: string;
  army: string;
  now: number;
  animate: boolean;
}) {
  const motion = useRef<SVGAnimateMotionElement>(null);
  const [anchor] = useState(() => ({ server: now, local: performance.now() }));
  const started = anchor.server;
  const progress = Math.max(
    0,
    Math.min(
      1,
      ((animate ? started : now) - event.at) /
        Math.max(1, event.until - event.at),
    ),
  );
  const x = from.x + (to.x - from.x) * progress,
    y = from.y + (to.y - from.y) * progress;
  useLayoutEffect(() => {
    if (!animate || !motion.current) return;
    const current =
      anchor.server + Math.max(0, performance.now() - anchor.local);
    const progress = Math.max(
      0,
      Math.min(1, (current - event.at) / Math.max(1, event.until - event.at)),
    );
    motion.current.setAttribute(
      "path",
      `M${from.x + (to.x - from.x) * progress} ${from.y + (to.y - from.y) * progress}L${to.x} ${to.y}`,
    );
    motion.current.setAttribute(
      "dur",
      `${Math.max(1, event.until - current)}ms`,
    );
    motion.current.beginElement();
  }, [event.id, animate]);
  return (
    <g
      data-cq-moving={event.id}
      transform={animate ? undefined : `translate(${x} ${y})`}
    >
      {animate && (
        <animateMotion
          ref={motion}
          begin="indefinite"
          path={`M${x} ${y}L${to.x} ${to.y}`}
          dur={`${Math.max(1, event.until - started)}ms`}
          fill="freeze"
        />
      )}
      <g transform={`scale(${1 / scale})`} className="cq-action-army">
        <circle r="13" fill="#fff4d3" stroke={color} strokeWidth="2" />
        <use
          href={army}
          x="-10"
          y="-11"
          width="20"
          height="20"
          style={{ color }}
        />
        <text y="28" textAnchor="middle">
          {event.amount < 0 ? "?" : event.amount.toLocaleString("zh-CN")}
        </text>
      </g>
    </g>
  );
}
export function ActionTrail({
  event,
  from,
  to,
  scale,
  color,
  army,
  arrow,
  now,
  animate,
}: {
  event: Event;
  from?: Point;
  to: Point;
  scale: number;
  color: string;
  army: string;
  arrow: string;
  now: number;
  animate: boolean;
}) {
  if (event.skipped) return null;
  const applied = !!event.applied || !!event.after;
  return (
    <g data-cq-event={event.id} pointerEvents="none">
      {from && event.type === "move" && (
        <>
          <path
            d={`M${from.x} ${from.y}L${to.x} ${to.y}`}
            fill="none"
            stroke="#fff5d6"
            strokeWidth="7"
            vectorEffect="non-scaling-stroke"
          />
          <path
            d={`M${from.x} ${from.y}L${to.x} ${to.y}`}
            fill="none"
            stroke={color}
            strokeWidth="3.5"
            markerEnd={arrow}
            vectorEffect="non-scaling-stroke"
          />
        </>
      )}
      <g transform={`translate(${to.x} ${to.y}) scale(${1 / scale})`}>
        <circle
          className="cq-action-pulse"
          r="23"
          fill="none"
          stroke="#fff5d6"
          strokeWidth="6"
        />
        <circle r="23" fill="none" stroke={color} strokeWidth="3" />
        {event.type !== "move" && event.amount > 0 && (
          <text className="cq-action-change" y="-43" textAnchor="middle">
            {event.type === "disband" ? "−" : "+"}
            {event.amount.toLocaleString("zh-CN")}
          </text>
        )}
      </g>
      {from && event.type === "move" && !applied && (
        <MarchingArmy
          key={event.id}
          event={event}
          from={from}
          to={to}
          scale={scale}
          color={color}
          army={army}
          now={now}
          animate={animate}
        />
      )}
    </g>
  );
}
