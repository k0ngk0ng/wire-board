import type { Room } from "./types";
import { catanBaseLayoutName } from "./catan-base-layout";
import "./catan-scenarios.css";

export function CatanBasePicker({
  room,
  disabled,
  command,
}: {
  room: Room;
  disabled: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
}) {
  const setup = room.catanBaseConfiguration;
  const layouts = room.catanBaseLayouts;
  if (
    !setup ||
    room.catanCitiesKnights ||
    !layouts?.length ||
    room.catanSeafarers ||
    room.catanNewWorldMap
  )
    return null;
  return (
    <fieldset
      className="catan-helper-options catan-scenario-options"
      disabled={disabled}
    >
      <legend>基础地图布局</legend>
      {layouts.length > 1 ? (
        <label>
          地图布局
          <select
            value={setup.layout}
            onChange={(e) =>
              command("catan_base_configuration", {
                catanBaseConfiguration: { layout: e.target.value },
              })
            }
          >
            {layouts.map((layout) => (
              <option key={layout} value={layout}>
                {catanBaseLayoutName(layout)}
              </option>
            ))}
          </select>
        </label>
      ) : (
        <p className="catan-scenario-layout">
          {catanBaseLayoutName(setup.layout)}
        </p>
      )}
      <p>
        {setup.layout === "fixed"
          ? "使用规则书的固定地图，随机分配颜色，预放两座村庄和两条道路并领取起始资源，直接开始掷骰。五人局保留剩余颜色的两座中立村庄，不放它们的道路。"
          : "随机安排地形，开局后由玩家依次选择村庄和道路的位置。"}
      </p>
      <small>
        更换布局后需要重新准备。固定新手布局适用于五至六人；关闭该扩充后会切换为随机布局。
      </small>
    </fieldset>
  );
}
