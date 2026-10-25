export type NetworkInfo = {
  connections?: number;
  connected: boolean;
  rttMs: number | null;
  sampledAt?: number;
};
export function networkQuality(info?: NetworkInfo) {
  if (!info?.connected)
    return {
      bars: 0,
      tone: "offline",
      text: "重连中",
      label: "连接中断，正在重连",
    };
  if (
    info.rttMs === null ||
    info.rttMs === undefined ||
    !Number.isFinite(info.rttMs)
  )
    return {
      bars: 0,
      tone: "unknown",
      text: "测速中",
      label: "正在测量网络延迟",
    };
  const ms = Math.max(0, Math.round(info.rttMs)),
    bars = ms <= 80 ? 4 : ms <= 180 ? 3 : ms <= 350 ? 2 : 1;
  return {
    bars,
    tone: bars >= 3 ? "good" : bars === 2 ? "fair" : "poor",
    text: `${ms}ms`,
    label: `网络延迟 ${ms} 毫秒`,
  };
}
