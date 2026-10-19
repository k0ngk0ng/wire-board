# 疯狂坦克（fortressblue）验收记录

日期：2026-10-19。实现来源：已审核许愿「复刻一个疯狂坦克的游戏，最多 5v5」。参考游戏仅作为玩法灵感；本机不能取得/运行参考游戏，未使用其商业图片、音频或名称，素材全部原创。

## 自动化范围

```sh
go test -count=1 ./games/fortressblue/server
go test -count=1 ./games/fortressblue/tests
go test -count=1 ./games/fortressblue/... ./games
go vet ./games/fortressblue/...
npm --prefix web run build
npm --prefix web test -- fortressblue
python3 -B scripts/check_game_boundaries.py
python3 -B scripts/fortressblue/verify-assets.py --dist
python3 -B scripts/fortressblue/check-browser.py
```

结果：

- 服务端规则测试通过：2/4/6/8/10 人配置、地图对称与出生台地、红蓝交替顺序、选角超时自动补齐、移动燃料/碰撞、错误 epoch/turn/席位/角度/力度/特殊弹拒绝、风力确定性、爆炸伤害与地形破坏、坠落/沉没、队灭胜负、同时全灭与时间到同生命和局、突然死亡、View 控制权限、坏存档拒绝。
- 模块 E2E：2／4／6／8／10 人全电脑对局分别在地图 hills／canyon／islands／snow／volcano 完成；校验 `Team=true`、胜方全队 Winners、Ranks/Scores 和结果合法性；无死循环。
- HTTP/SQLite E2E：新游戏默认下架；管理员上架；10 人房间、9 名电脑、选坦克、真实回合、错误操作/观众拒绝；服务重启后地图、地形、生命、回合、epoch 和最后炮弹保持一致；2v2 随机地图和 4 人房间可正常选角/移动；独立低生命夹具验证团队结果、历史记录与结算字段。
- 前端 3 项纯控制/元数据测试通过；构建通过；23 款游戏边界检查通过。
- 原创音频 10 个 WAV 通过 SHA-256、44.1 kHz/16-bit/单声道、时长与非静音/削波校验，且本地和 `web/dist` 副本一致。

## 真实浏览器验收

`python3 -B scripts/fortressblue/check-browser.py` 启动临时生产式服务，真实注册/登录、创建 10 人房、添加 9 名电脑、开始选坦克，共 18 项通过：

- 创建后进入共享等待/对局框架；共享玩家栏含 10 个席位。
- 5 张坦克卡可选，卡片和标题对比度通过；点击「重装」后进入 playing，Canvas 战场可见。
- 游戏内倒计时显示 0:19，不是错误归零；风力显示正常。
- 计算样式对比度：标题、队伍标题、坦克行、回合提示、开火按钮均 ≥ 4.5:1。
- 点击向左移动后服务器返回的座位 x 坐标确实变小。
- 设置角度 55°、力度 72 后开火，服务器返回 shot id，炮弹动画结束；刷新后 `animating=0`，不重播旧动作。
- 320×844：无横向溢出，Canvas 实际宽度 ≥300px，10 行坦克状态和开火按钮存在，截图保留。
- 截图与结果：`.local/fortressblue/browser/<run>/desktop-select.png`、`desktop-board.png`、`desktop-after-shot.png`、`mobile-320.png`、`result.json`。本次运行：`.local/fortressblue/browser/c80adab0/result.json`（18/18）。

## 已知边界

- 浏览器时间盒只验证到一发炮弹和刷新恢复；完整队灭/和局结果由模块 E2E 与 HTTP 房间生命周期覆盖。平台结果浮层为共享组件，本游戏不单独复制。
- 10 人共享玩家栏在手机上会占据较高区域，战场需向下滚动；游戏内队伍 HUD、坦克血条和全部 10 席状态仍完整可见，没有横向裁切。共享玩家栏未用游戏 CSS 隐藏或重排。
- 音效为原创合成，通过平台 `BoardProps.audio` 播放；没有使用参考游戏的原声音乐/音效，也没有在游戏内创建播放器或 `AudioContext`。
- 新游戏保持默认下架，部署后需管理员在后台显式上架；不自动上架。
