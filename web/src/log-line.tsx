import { cityChinese } from "./cities";
import type { Seat } from "./types";

const colors: Record<string, string> = {
  城市: "#b8845b",
  道路: "#bb9e63",
  田地: "#72953d",
  修道院: "#b84d42",
  纸张: "#417e3c",
  布料: "#86983e",
  钱币: "#637d99",
  木材: "#286540",
  砖块: "#bb633c",
  羊毛: "#8cac48",
  粮食: "#dbad43",
  矿石: "#7a8390",
  "祖母绿（绿）": "#218357",
  "钻石（白）": "#f7f2dd",
  "蓝宝石（蓝）": "#2974bb",
  "缟玛瑙（黑）": "#343c43",
  "红宝石（红）": "#c94440",
  黄金: "#dcac38",
  紫色列车牌: "#a35e91",
  白色列车牌: "#eee6d1",
  蓝色列车牌: "#357bac",
  黄色列车牌: "#e3bc38",
  橙色列车牌: "#dd8540",
  黑色列车牌: "#39434a",
  红色列车牌: "#c84b43",
  绿色列车牌: "#568757",
  万能牌: "#e4ba55",
};
const escape = (value: string) => value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const pattern = new RegExp(
  `(玩家 \\d+|(?:${Object.keys(colors).map(escape).join("|")})(?:×\\d+)?|${Object.keys(
    cityChinese,
  )
    .sort((a, b) => b.length - a.length)
    .map(escape)
    .join("|")})`,
  "g",
);

// Parse the stored public text first; a player's chosen name is always literal text.
export function LogLine({
  line,
  seats,
  seatColors,
}: {
  line: string;
  seats: Seat[];
  seatColors?: string[];
}) {
  return (
    <>
      {line.split(pattern).map((part, i) => {
        const player = /^玩家 (\d+)$/.exec(part);
        if (player)
          return (
            <strong key={i} className={seatColors ? "log-color" : undefined}>
              {seatColors && (
                <i
                  aria-hidden="true"
                  style={{ backgroundColor: seatColors[Number(player[1]) - 1] }}
                />
              )}
              {seats[Number(player[1]) - 1]?.name || part}
            </strong>
          );
        const label = part.replace(/×\d+$/, "");
        if (colors[label])
          return (
            <span className="log-color" key={i}>
              <i
                aria-hidden="true"
                className={label === "万能牌" ? "wild" : ""}
                style={{ backgroundColor: colors[label] }}
              />
              {part}
            </span>
          );
        return cityChinese[part] || part;
      })}
    </>
  );
}
