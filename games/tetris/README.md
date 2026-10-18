# 俄罗斯方块

实时落块对战：1 人房间是单人练习，2～4 人房间是对战。生存赛通过消行向对手发送垃圾行，最后存活者获胜；40 行冲刺各自清理 40 行，先完成者获胜（冲刺模式不发送垃圾行）。所有人数与模式均支持电脑玩家和真人席位托管。

## 规则与操作

- 内部棋盘 10×22（顶部 2 行隐藏出生区），SRS 旋转与墙踢，7-bag，暂存每块一次。
- 等级 = 1 + 已消行 / 10；每 10 行提升一级，重力间隔最低 100ms。
- 单／双／三／四行分别 100／300／500／800 分 × 等级；软降 +1/格，硬降 +2/格。
- 生存赛攻击表 0／1／2／4 行；垃圾行在目标锁块后从底部升起，消行可抵消待落垃圾。
- 键盘：←／→ 移动，↑／X 顺旋，Z／Ctrl 逆旋，↓ 软降，空格硬降，C／Shift 暂存。
- 手机：使用棋盘下方的按钮；按住移动或软降连续触发。
- 对局上限 10 分钟；超时或主动托管由电脑接管本席。刷新、重连与恢复存档不会重洗队列或重置期限。

## 服务端

`games/tetris/server` 直接实现 `gameapi.Module`：`Realtime: true`，`OnTimer` 每 100ms 推进重力与锁定，`SystemDeadline` 驱动实时帧；`BotAction`/`Reclaim` 使用与真人相同的 `inputs` 批次。`View` 只向本人返回队列、暂存、ghost 与序号，对手和观众只看到公开棋盘与统计。

活动方块的移动、旋转和落点有一个仅用于显示的预测层（`web/prediction.ts`）；权威棋盘、消行、垃圾行、分数与胜负始终由服务器决定。

## 测试与验收

```sh
. scripts/env.sh
go test -count=1 ./games/tetris/server ./games/tetris/tests
npm --prefix web test -- tetris
python3 -B scripts/tetris/verify-assets.py --dist
python3 -B scripts/tetris/check-browser.py
python3 -B scripts/ci/scope.py run --game tetris --backend
```

浏览器验收需要已构建的 `web/dist` 和本机 `agent-browser`；结果保存在 `.local/tetris/browser/`，包含 1280／390／320px 截图、计算样式对比度、键盘与原生触屏操作、托管、刷新、单人冲刺结算与免费重开检查。规则与完整电脑对局证据在 `.local/tetris/module/`，HTTP 证据在 `.local/poker/tetris/http/`（复用公共测试夹具的固定目录）。

## 素材

封面是 `assets/art.ts` 中的原创手绘式 SVG；音效由 `scripts/tetris/prepare-audio.mjs` 离线合成，清单与 SHA-256 见 `assets/manifest.json`，说明见 `assets/README.md`。音效通过平台 `BoardProps.audio` 播放，本地回退随前端构建发布。

## 上架

新游戏注册后默认下架。部署或发布不会自动上架；管理员需在「管理后台 → 游戏上架」确认后，玩家大厅和创建列表才会显示。单人练习使用 `Result.Unrated` 保存成绩但不写积分流水。
