import { useEffect, useState } from "react";
import { classicGaugeTicks, classicGaugeMode } from "./classic-gauges";

export function classicGaugeArc(start: number, end = 130): string {
  const point = (angle: number) => {
    const radians = angle * Math.PI / 180;
    return [80 + Math.sin(radians) * 66, 80 - Math.cos(radians) * 66];
  };
  const a = point(start), b = point(end);
  return `M ${a[0]} ${a[1]} A 66 66 0 ${end-start > 180 ? 1 : 0} 1 ${b[0]} ${b[1]}`;
}

function Dial({ rpm }: { rpm: boolean }) {
  const ticks = classicGaugeTicks(rpm ? 10000 : 400, rpm ? 1000 : 50, rpm ? 250 : 10);
  return (
    <svg viewBox="0 0 160 160" aria-hidden="true" data-racing-dial={rpm ? "rpm" : "speed"}>
      <circle className="racing-dial-rim" cx="80" cy="80" r="77" />
      <circle className="racing-dial-face" cx="80" cy="80" r="72" />
      {rpm && <path className="racing-dial-redline" data-racing-redline d={classicGaugeArc(39)} />}
      {ticks.map(tick => (
        <g key={tick.value}>
          <path className={tick.major ? "racing-dial-major" : "racing-dial-minor"}
            d={`M 80 ${tick.major ? 17 : 20} V ${tick.major ? 27 : 25}`} transform={`rotate(${tick.angle} 80 80)`} />
          {tick.major && <text className="racing-dial-number"
            x={80+Math.sin(tick.angle*Math.PI/180)*48} y={80-Math.cos(tick.angle*Math.PI/180)*48}>
            {rpm ? tick.value/1000 : tick.value}
          </text>}
        </g>
      ))}
      <text className="racing-dial-unit" x="80" y="112">{rpm ? "RPM ×1000" : "km/h"}</text>
      <text className="racing-dial-readout" x="80" y="131" {...(rpm ? { "data-racing-rpm": true } : { "data-racing-speed": true })}>0</text>
      <g className="racing-dial-needle" {...(rpm ? { "data-racing-rpm-needle": true } : { "data-racing-speed-needle": true })} transform="rotate(-130 80 80)">
        <path d="M 77 91 L 79 22 L 81 22 L 83 91 Z" />
      </g>
      <circle className="racing-dial-hub" cx="80" cy="80" r="6" />
    </svg>
  );
}

export function ClassicGauges({ hidden }: { hidden: boolean }) {
  const currentMode = () => classicGaugeMode(window.innerWidth,window.innerHeight);
  const [mode,setMode] = useState(currentMode);
  useEffect(() => {
    const resize = () => setMode(currentMode());
    window.addEventListener("resize",resize);
    resize();
    return () => window.removeEventListener("resize",resize);
  },[]);
  return (
    <div className="racing-gauges" data-mode={mode} hidden={hidden} role="group" aria-label="速度 0 千米每小时，转速 0 转每分钟，挡位 1">
      {mode === "analog" ? <>
        <div className="racing-gauges-analog"><Dial rpm={false} /><Dial rpm /></div>
        <strong className="racing-gauges-gear" data-racing-gear>1</strong>
      </> : <>
        <span className="racing-compact-speed"><span data-racing-speed>0</span><small>km/h</small></span>
        <span className="racing-compact-rpm"><span data-racing-rpm>0</span><small>rpm</small></span>
        <strong className="racing-gauges-gear" data-racing-gear>1</strong>
      </>}
    </div>
  );
}
