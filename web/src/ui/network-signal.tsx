import { networkQuality, type NetworkInfo } from "../network-quality";
import "./network-signal.css";
export function NetworkSignal({
  info,
  connected,
  name = "",
}: {
  info?: NetworkInfo;
  connected?: boolean;
  name?: string;
}) {
  const quality = networkQuality(
    connected === undefined
      ? info
      : { connected, rttMs: connected ? (info?.rttMs ?? null) : null },
  );
  const label = `${name ? name + "：" : ""}${quality.label}${(info?.connections ?? 0) > 1 ? "，显示多端连接中的较高延迟" : ""}`;
  return (
    <span
      className={`network-signal network-${quality.tone}`}
      data-network-bars={quality.bars}
      title={label}
      aria-label={label}
      role="img"
    >
      <svg width="20" height="17" viewBox="0 0 20 17" aria-hidden="true">
        {[4, 7, 11, 15].map((height, i) => (
          <rect
            key={height}
            x={i * 5}
            y={16 - height}
            width="3.5"
            height={height}
            rx=".7"
            className={i < quality.bars ? "lit" : "unlit"}
          />
        ))}
      </svg>
      <span className="network-delay">{quality.text}</span>
    </span>
  );
}
