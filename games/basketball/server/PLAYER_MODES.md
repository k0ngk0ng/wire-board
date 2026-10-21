# 平台人数与分队验收

用户确认的装备规则：双人对战保留原版 Mode 128，不使用装备，也不增加可选装备规则。装备选择和属性预览只用于战役准备、战役中场及下一场准备。

本次先记录失败场景，再编写验收与实现。平台房间开放 1／2 个席位；单人、双人合作、双人对战由创建选项决定。球场始终有四个角色，未分配给席位的角色使用恢复的原版 AI。

## 失败场景

- 人数与模式不一致、未知模式、双人房间未满却能开始，或新房间仍要求四人。
- 对战第二席的输入错误地控制 A 队队友；请求伪造场上角色编号绕过平台身份校验；观众可以操作。
- 本地预测使用平台席位而非角色编号；确认序号读错槽位；切换用户后沿用旧控制器设置。
- 取消、重试、超时托管和取消托管作用于错误角色；固定电脑角色被关闭；中场第一席托管后无人能继续比赛。
- 重连和 SQLite 恢复丢失模式、分队、比赛进度、输入权限；旧四席开发存档无法恢复。
- 结果包含不存在的平台席位、合作失败被当作胜利、对战胜者映射错误；固定电脑不占平台席位却导致比赛计入竞技积分。
- 手机创建选项溢出；切换人数保留无效模式；等待房间无法看到分队；游戏内再次要求选择 1P／2P。

## 验收方式

真实 HTTP、身份校验、主机计时器与 SQLite 恢复覆盖三种模式；Go 边界验证结算与存档校验；实际 React Board + Go/Wasm 浏览器验收覆盖对战第二席的原生键盘／触屏和个人设置。开发用四席测试继续检查四个独立输入流，不在正式创建入口开放四席。

含固定电脑的比赛不计竞技积分。单人和合作使用平台合作结果；双人对战使用正常队伍结果，不借合作标记规避计分。

## 已完成的验证

- 真实 HTTP 覆盖三种模式的创建、人数校验、观众拒绝、对战第二席输入、托管／接管及 SQLite 重启恢复。
- 实时游戏空闲不自动托管。第一席、第二席均由平台显式托管请求切换原版 AI；取消托管不改动固定电脑队友。
- 第一席托管、第二席仍为真人时，暂停可按原控制器 0 路径继续。
- 独立联机 Board 完成 23 个浏览器检查，覆盖原生键盘／多指触屏、个人键位、模式表单和观战权限。
- 构建后的完整平台页面完成 17 个浏览器检查：三种模式的封面选择、标准创建弹窗及等待房间，双人加入、准备、开始，公共队伍得分与手机／PC 手柄分区。
- 合作和对战各运行两次完整上下半场，Native 与 Wasm 的恢复轨迹摘要一致。合作配置为 `[0,1,-1,-1]`，22,365 视频帧，比分 0–51；对战配置为 `[0,-1,2,-1]`，22,473 视频帧，比分 24–3。真人角色在此完整对局门禁中不操作；这些结果不能代替真人网络完整比赛验收。
- 平台契约、积分边界、控制器账号隔离、篮球服务端竞态检查、TypeScript／Vite 构建通过。没有运行其他游戏完整对局。

比赛画面与手柄使用独立区域：PC 与手机均在画面下方居中显示。手柄使用 Turbo B／Turbo A 标注；输入 ID、默认 U／I、频率和按下／松开逻辑不变。设置与游戏复用同一底图和按键组件，手机截图及原生鼠标／触屏验收保存在 `.local/fc-controller/browser/`。

## 复现与产物

```sh
source scripts/env.sh
go test -race ./games/basketball/server -count=1 -v
go test ./games/basketball/engine -run '^TestRuntime(Cooperative|Versus)CompleteMatchAndRestore$' -count=1 -v
npm --prefix web run build
BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowser(NetworkBoard|PlatformRooms)$' -count=1 -v
python3 scripts/accept_fc_controller.py
```

浏览器验收需要本地已恢复的球场素材。完整平台页面测试使用正式前端构建和实际 API，服务端仅在测试注册表装载篮球，图片由隔离的本地素材路由提供；不修改生产注册或上架设置。

- 模式与输入截图／报告：`.local/basketball/network-board/`。
- 标准平台创建、等待与比赛截图／报告：`.local/basketball/platform-rooms/`。
- HTTP／SQLite 记录：`.local/poker/basketball/http/TestBasketballHTTPPlayerModes-*.json`、`TestBasketballHTTPVersusTimeout.json`。
- 命令日志、阶段失败记录与最终结果：`.local/basketball/engine/player-modes-*.log`、`controller-placement-*.log`、`turbo-*.log`。
- 汇总与文件 SHA-256：`.local/basketball/platform-integration-summary.json`。

已加入正式服务端注册，素材已按不可变版本发布到 OSS。当前玩法范围、WebSocket 和最新验收见 [main 接入记录](../REALTIME_MAIN_INTEGRATION.md)；本文保留的早期样本不替代最终验证。
