import type { CatanRuleContext } from "./catan-rule-context";

export function CatanRiversTransportRules({
  info,
}: {
  info: CatanRuleContext;
}) {
  if (!info.rivers || !info.transport) return null;
  return (
    <section className="catan-combination-rules">
      <h4>河流＋运输{info.citiesKnights ? "＋城市与骑士" : ""}</h4>
      <ul>
        <li>
          每人起始 3 金币；起始河岸村庄、城市和道路各领 1
          金币。之后沿河建村或道路领 1 金币，升级城市不重领。
        </li>
        <li>
          桥梁花费 1 木材＋2 砖块，每人最多 3 座，建成领 2
          金币。道路不能跨越桥位，免费道路不能改建桥梁。
        </li>
        <li>
          马车无桥跨河花 3 移动点；走自己的桥花 1 点；走对手的桥花 1
          点并付给对手 2 金币。道路蛮族额外增加 2 点。
        </li>
        <li>
          采石场产砖块、玻璃工坊产木材、城堡产羊毛；沼泽不生产。同一地块的 2／12
          均可生产，掷出 2／12 不重掷。
        </li>
        <li>
          金币共用一个余额；唯一最富者加 1
          分，最贫者不扣分。没有强盗与最长道路；自己回合达到 {info.target}{" "}
          分立即获胜。
        </li>
        {info.two && (
          <li>
            建桥后须为中立势力建桥，无合法桥位时改修道路。本站补充：中立桥梁收费
            2 金币，银行与对手各得 1 金币。
          </li>
        )}
        {info.players > 4 && (
          <li>
            本站五六人地图：37
            格、七处商品地块、三条河流与固定数字配置，使用配对回合。
          </li>
        )}
        {info.citiesKnights && (
          <li>
            本站三模块组合：保留海上蛮族、道路蛮族与进步牌；随机地形用一块麦田替换森林。可付
            5 金币免除一次城市劫掠；外交拆河岸道路需归还 1
            金币，不能拆桥。炼金术可选 2／12。
          </li>
        )}
      </ul>
    </section>
  );
}
