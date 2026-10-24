import { useEffect } from "react";
import {
  defineGame,
  type OptionsProps,
} from "../../../web/src/game-api/module";
import catalog from "./catalog";
import { Board } from "./Board";
import type { Options, View } from "./types";
function RulesOptions({
  value,
  onChange,
  capacity,
  disabled,
}: OptionsProps<Options>) {
  useEffect(() => {
    if (value.teams.length !== capacity)
      onChange({
        ...value,
        teams: Array.from({ length: capacity }, (_, i) => i % 2),
      });
  }, [capacity]);
  return (
    <div className="tt-options">
      <label>
        赛制
        <select
          aria-label="乒乓球赛制"
          value={value.bestOf}
          disabled={disabled}
          onChange={(e) =>
            onChange({ ...value, bestOf: +e.target.value as 1 | 3 })
          }
        >
          <option value={1}>单局决胜</option>
          <option value={3}>三局两胜</option>
        </select>
      </label>
      <label>
        场馆
        <select
          aria-label="乒乓球场馆"
          value={value.court}
          disabled={disabled}
          onChange={(e) =>
            onChange({ ...value, court: e.target.value as Options["court"] })
          }
        >
          <option value="club">现代球馆</option>
          <option value="arena">街机赛场</option>
          <option value="heritage">复古体育馆</option>
        </select>
      </label>
      <label>
        电脑水平
        <select
          aria-label="乒乓球电脑水平"
          value={value.difficulty}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...value,
              difficulty: e.target.value as Options["difficulty"],
            })
          }
        >
          <option value="easy">轻松</option>
          <option value="normal">标准</option>
          <option value="hard">进阶</option>
        </select>
      </label>
      {capacity === 4 && (
        <label>
          双打分队
          <select
            aria-label="乒乓球双打分队"
            value={value.teams.join(",")}
            disabled={disabled}
            onChange={(e) =>
              onChange({
                ...value,
                teams: e.target.value.split(",").map(Number),
              })
            }
          >
            <option value="0,1,0,1">座位 1、3 对座位 2、4</option>
            <option value="0,0,1,1">座位 1、2 对座位 3、4</option>
            <option value="0,1,1,0">座位 1、4 对座位 2、3</option>
          </select>
        </label>
      )}
    </div>
  );
}
export default defineGame<View, Options>({
  ...catalog,
  asyncBoard: true,
  Board,
  Options: RulesOptions,
  defaultOptions: {
    bestOf: 1,
    teams: [0, 1],
    difficulty: "normal",
    court: "club",
  },
  PlayerSummary: ({ data, seat }) => (
    <div className="tt-public-score">
      <strong>
        {data.options.teams[seat] === 0 ? "A" : "B"} 队 ·{" "}
        {data.score[data.options.teams[seat]]} 分
      </strong>
      <span>
        已赢 {data.gamesWon[data.options.teams[seat]]} 局
        {data.expected === seat &&
        ["serve", "serve_toss", "rally"].includes(data.phase)
          ? " · 当前击球者"
          : ""}
      </span>
    </div>
  ),
  Rules: () => (
    <div>
      <p>
        每局 11 分，领先 2 分获胜。每 2 分交换发球，10
        平后每分换发球。双打须斜线发球、同队交替击球；决胜局首次到 5
        分交换场地和接发次序。
      </p>
      <p>
        球拍跟随手指或鼠标左右移动，贴近来球后划动：向前抽球、向后削球、斜划变线、短点挡球。划动速度影响力度，高球时快速抽球可扣杀。来球落台后击球，不需要移动身体。发球限时
        15 秒，超时对方得 1 分。托管通过平台入口手动开启或取消。
      </p>
      <p>
        键盘沿用控制器设置：A 抽球，B 削球，Select／Start
        挡球，左右方向移动球拍并选择落点，上下方向选择力度。默认 K／J 对应
        A／B。浏览器失焦或切后台会取消当前手势。
      </p>
    </div>
  ),
});
