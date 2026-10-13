# 战场交互与战斗反馈验收

实现前列出失败场景：

- 主战场、小地图不能用鼠标连续拖动，移出边界后拖动卡住，拖动误触建造；普通滚轮只滚页面，缩放锚点漂移。
- 有缩放功能的其他游戏遗漏滚轮或鼠标拖动，强手棋放大后无法平移。
- 准备时间在手机上不显著；选择期限误标成开战期限；暂停仍倒数；时间到自动开战却没有提前说明。
- 扩展选项拉长页面；弹窗关闭后无法重开；每个快照强制重开；投票完成不关闭；观众可以投票。
- 320px 手机上主要战场被次要信息挤出，金币或波次不可见；弹窗溢出或无法滚动。
- 手机空建造点的可见圆圈覆盖道路；缩小圆圈同时使触摸区域难点；建成后仍保留空点圆圈／编号，或售出后未恢复空点；建成后只能点底部标记，点塔身无效；拖动塔身误打开升级；镜头移动／缩放后命中区偏离塔的实际绘制范围。
- 子弹飞行时先掉血／消失／播放命中特效；同一敌人连续受击、护盾、范围伤害、击杀和治疗顺序混乱；刷新重播旧战斗。

验证：实际 Chromium 原生鼠标输入、DOM 操作、生产计时与 HTTP 房间。保存命令日志、截图、计算布局和 result.json。动画时间轴的隔离验证覆盖上述快照顺序与重连边界，先编写测试再实现。

## 可重复执行的验证

```sh
. scripts/env.sh
npm --prefix web run build
npm --prefix web test
WIRE_BOARD_STARFORT_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B games/starfort/tests/accept_interaction_ui.py --skip-build
WIRE_BOARD_STARFORT_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B games/starfort/tests/accept_world_ui.py --skip-build
WIRE_BOARD_MONOPOLY_ASSETS=https://cdn.ichenj.com/bga/v20261002-1 python3 -B games/monopoly/tests/accept_ui.py --skip-full
python3 -B scripts/accept_game_modules.py
```

浏览器脚本须串行运行。交互验收产物位于 `.local/starfort-interaction/browser/`：`result.json` 记录通过项与源码校验和，`native-impact-timing.json` 记录实际 Canvas 绘制中的子弹与血条时序，`tower-ink-and-hitbox.json` 比对塔身绘制与点击范围，`primary-viewport-*.png` 记录主要战场的实际视口。输入使用自有 agent-browser 会话；滚轮通过该会话的 CDP 在明确的视口坐标发送，避免 CLI 滚轮位置与鼠标移动位置不一致。双指分支使用合成 PointerEvents，不作为真机触摸验证。
