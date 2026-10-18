# 俄罗斯方块验证记录

2026-10-18。新增独立模块 `games/tetris`，并在公共平台增加 `Result.Unrated`，用于单人练习保存成绩但不计积分。规则、动作协议和失败场景先写入 [games/tetris/CONTRACT.md](../games/tetris/CONTRACT.md)，实现后再完成本记录。

## 已通过

| 验证 | 结果 | 产物 |
| --- | --- | --- |
| 规则单测（7-bag、SRS 墙踢、硬降、消行、计分、等级、垃圾行、目标、排名） | 通过 | `go test ./games/tetris/server` |
| 生产 Module 完整电脑对局（生存／冲刺 × 1～4 人） | 8 个完整对局通过 | `.local/tetris/module/` |
| 电脑隐藏信息差分、输入不被修改、Reclaim 席位隔离 | 通过 | `go test ./games/tetris/tests` |
| 隐私、非法动作、恢复、托管、并发、重启 HTTP 验收 | 通过 | `.local/poker/tetris/http/` |
| 俄罗斯方块范围检查（前端、素材、Go、竞态） | `PASS tetris: 5 scoped checks` | `.local/ci/tetris/` |
| 公共平台范围检查（含 `TestRateMatchUnratedPractice`） | `PASS shared: 3 scoped checks` | `.local/ci/shared/` |
| 前端生产构建与 12 项前端单测 | 通过 | `npm --prefix web run build`、`npm --prefix web test -- tetris` |
| 真实浏览器验收（生产注册表、HTTP、SQLite、真实计时器） | 23 个检查点通过 | `.local/tetris/browser/result.json` |
| 素材清单、WAV 头、本地回退与 `web/dist` 字节 | 9 个对象通过 | `.local/tetris/assets-verify.json` |
| 全部游戏两步创建流程（20 款、1280／320px、40 个房间） | 通过 | `.local/create-flow/result.json` |
| 公共回合提示布局（14 款独立游戏 × 9 宽度） | 通过 | `.local/turn-layout/result.json` |
| 游戏目录／分类／搜索／320px（含 30 个临时目录项） | 通过 | `.local/game-filters/browser/result.json` |
| `go vet`、全仓编译、`gofmt`、游戏边界 | 通过 | 本地命令输出 |

## 浏览器验收覆盖

- 新游戏默认不在大厅；管理员接口上架后才出现，创建未上架游戏被拒绝。
- 创建弹窗第一步选择封面，第二步显示 1／2／3／4 人、生存／冲刺选项；1280、390、320px 截图与计算样式对比度。
- 等待房间添加 3 个电脑、准备、开局；4 人玩家栏与对手卡片的关键资源（存活、行数、分数、待落）在 320px 均可见，零值直出；页面无横向溢出。
- 真实键盘移动、顺时针旋转、硬降；原生触屏向右移动；操作后 `data-tetris-*` 与服务端分数／方块编号更新。
- 开启／取消托管立即禁用／恢复控制。
- 刷新后追上服务端事件序号，且不重播旧消行动画。
- 单人冲刺通过真实托管完成；结算浮层显示「练习结束」「练习对局不计积分」，`room.result.unrated=true`、`rated=false`，个人积分与计分场次不变；浮层关闭后可重新打开，「再来一局」返回等待房间。
- 单人练习和多人对局的阶段、倒计时上限、当前棋盘布局均使用公共宿主界面，没有游戏专属的结算横条或大厅布局。

## 素材与平台边界

- 封面为原创矢量 SVG；9 个 WAV 由 `scripts/tetris/prepare-audio.mjs` 确定性离线合成，`assets/manifest.json` 记录生成器与逐文件 SHA-256。当前未执行 OSS 上传，本地回退完整通过；部署时可按既有 `ASSETS_BASE_URL` 的 `tetris/v1/` 前缀补充上传。
- 所有音效经 `BoardProps.audio.playWithFallback` 播放；游戏未创建播放器、`AudioContext` 或静音开关。
- 新游戏注册后仍为默认下架；本记录和发布流程不自动改变上架设置。
- 未运行其他游戏的规则、完整对局或扩展组合矩阵；公共目录验收只检查创建、筛选和公共提示栏。

## 重复执行

```sh
. scripts/env.sh
npm --prefix web run build
npm --prefix web test -- tetris
go test -count=1 ./games/tetris/server ./games/tetris/tests
python3 -B scripts/tetris/verify-assets.py --dist
python3 -B scripts/tetris/check-browser.py
python3 -B scripts/ci/scope.py run --game tetris --backend
python3 -B scripts/ci/scope.py run --game shared
python3 -B scripts/accept_create_flow.py
python3 -B scripts/accept_turn_layout.py
python3 -B scripts/accept_game_filters.py
```

浏览器验收需要已安装 `agent-browser`；`check-browser.py` 自己启动真实服务夹具，结束后写入 `stop` 并退出。不要与其他前端构建并发运行。
