# 扑克牌桌稳定布局验收

斗地主、掼蛋和升级统一固定牌桌网格、各席位出牌区与操作区。出牌区始终占位，长牌组横向滚动；展开的牌桌信息在中央区域滚动。手牌区按玩法最大容量预留，数量减少或清空不会缩短页面，最后一行不再随数量变化重新居中。多行手牌按牌宽的 0.9 倍作为行距上下叠放（牌面高度仍为牌宽的 1.4 倍），保留点数与花色；后行选中上提也不能覆盖前行角标。掼蛋牌型选择固定在操作按钮后方，不挤动出牌按钮。

## 验收范围与结果

- `scripts/poker/check-layout.py` 使用生产 Board 和实际样式，提供标明用途的界面状态样例，不把它们当作自然完整对局。覆盖三款游戏、1280／390／320px、有牌／无牌、长牌组、不出、等待、阶段切换及所有席位视角；102 个状态切换通过逐帧几何检查，容差为 0.75px。
- 9 组实际鼠标出牌、原生触屏多选与溢出内容检查通过；另有 9 组手牌重叠与逐行角标点击检查通过。长牌组支持鼠标横向滚动、键盘和触屏滑动；检查每张牌的角标可滚动到可见范围。
- `POKER_HINT_LAYOUT=1 scripts/poker/check-hints.py` 在真实 HTTP 房间中使用已有的明确牌型夹具，验证斗地主、掼蛋的提示、等待、下一轮、刷新和实际出牌。10 个截图检查点通过，记录实际对局中的区域坐标范围；真人加载门禁由各席位确认。
- 共享扑克反馈和语音的 7 项前端回归通过，生产构建与模块边界检查通过。没有修改服务端规则或运行其他游戏的完整对局矩阵。
- CI 范围预检仅选择 `doudizhu`、`guandan`、`shengji`，三者 `backend` 均为 `false`。

产物在 `.local/poker-layout/`：`result.json`、`live/result.json`、`ci-scope.json`、桌面／手机截图及验证日志。`before.json` 保留修改前的测量摘要，原布局 102 个场景中有 98 个发生超出容差的位移，最大约 658px。临时 Vite 入口、浏览器 profile 和专用临时缓存在脚本结束时清理；失败截图和中间格式化日志不保留。

## 重复执行

```sh
. scripts/env.sh
python3 -B scripts/poker/check-layout.py
POKER_HINT_LAYOUT=1 python3 -B scripts/poker/check-hints.py
node --experimental-strip-types --test web/tests/poker-feedback.test.mjs web/tests/poker-speech.test.mjs
python3 -B scripts/check_game_boundaries.py
```

两份浏览器验收顺序运行。第一份只启动本地前端样例，第二份构建生产前端并启动既有的 HTTP 测试服务；均使用 agent-browser 独立会话。
