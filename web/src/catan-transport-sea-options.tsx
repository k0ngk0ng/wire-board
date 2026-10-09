import type { Room } from "./types";

export const isCatanTransportSea = (scenario = "") =>
  ["transport-shores", "transport-desert"].includes(scenario);

export function CatanTransportSeaLayout({
  value,
  onChange,
  disabled = false,
}: {
  value: string;
  onChange: (layout: string) => void;
  disabled?: boolean;
}) {
  return (
    <fieldset className="catan-helper-options" disabled={disabled}>
      <legend>运输海图布局</legend>
      <label>
        地图布局
        <select value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="fixed">固定基础配方</option>
          <option value="variable">本站可变资源地图</option>
        </select>
      </label>
      <small>
        可变地图保留海陆、沙漠带、港口、货物地点和双数字格；分区重排普通资源与数字，红色数字不相邻。
      </small>
    </fieldset>
  );
}

export function CatanTransportSeaWaitingLayout({
  room,
  disabled,
  command,
}: {
  room: Room;
  disabled: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
}) {
  if (!isCatanTransportSea(room.catanScenario)) return null;
  const scenario =
    room.catanScenario === "transport-desert" ? "desert" : "shores";
  return (
    <CatanTransportSeaLayout
      disabled={disabled}
      value={room.catanTransportSea?.layout || "fixed"}
      onChange={(layout) =>
        command("catan_transport_sea", {
          catanTransportSea: { scenario, layout },
        })
      }
    />
  );
}
