# 联机画面与控制器接入验收

`Board.tsx` 已接入真实房间协议、个人控制器设置、可靠输入批次、原 Go/Wasm 引擎及 Canvas 绘制。当前是开发中的对局组件，尚未加入正式游戏注册表；本报告不代表完整多人比赛、全部球场或发布验收完成。

## 状态和画面

服务端在每个成功的视频 tick 后保留输入记录，最多 120 帧。记录包括逻辑按钮、取消后的先前按住状态、待消费短按和真实托管标志；不包含客户端给出的物理结果。`StepRecordedVideo` 仍调用原 `StepVideo`。浏览器通过 Wasm 的 `replay` 操作还原两次快照之间的每一帧，比分、AI、动作和物理都由同一套 Go 代码计算。

逐帧重放有事务边界：缺帧、倒序或失败不会提交前半批状态。当前快照覆盖重放锚点，吸收最后一帧之后的明确取消或接管。观战按 60Hz 视频时间线播放，保留中间画面；缺失超过历史范围的数据、页面长时间挂起或新比赛会直接同步最新权威状态，不虚构离线动作。

本席操作先在 Go/Wasm 中预测，最多领先最近权威帧 12 帧，超过 250ms 无新快照停止预测。新快照到达后，以服务端状态为起点重放本席仍领先的输入。JavaScript 仅安排输入和播放时序，没有第二套篮球物理或 AI。预测声音不播放，预测结果不提交服务端。声音只消费确认事件，并等到对应画面帧再发给平台音频端口；重连不补播旧事件。

浏览器对局组件复用公共 FC 手柄。游玩仅显示原始按钮名；设置弹窗阻止游戏输入，保存改键立即生效。输入实例按房间、比赛、席位和用户绑定，换账号先销毁旧输入源，新实例准备完成前不能操作。取消触屏 A 时，独立键盘方向继续保持，不产生第二次方向边沿。

## 浏览器策略与资源

生产 CSP 允许同源 Wasm 编译，保持 JavaScript 脚本仅同源。素材请求只额外允许配置中的 OSS 源，不启用 JavaScript 的 `unsafe-eval`。运行时代码固定从同源 `/basketball/v1/runtime/` 加载；图片及 JSON 从配置素材源下的 `/basketball/v1/graphics/` 加载。Wasm 已接入网页与应用打包，见 [构建说明](../build/README.md)；素材发布仍待完成，不能只修改注册表就发布。

## 可重复验收

从仓库根目录执行：

```sh
source scripts/env.sh
go test -race ./games/basketball/server -count=1 -v
python3 scripts/basketball/accept_network_scene.py
BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowserNetworkBoard$' -count=1 -v
python3 scripts/basketball/accept_wasm_runtime.py
go test ./internal/server -run '^TestAssetURLValidationAndCSP$' -count=1 -v
web/node_modules/.bin/tsc -p web/tsconfig.json --noEmit
```

浏览器验收使用工作区现有原图提取产物，所有浏览器配置和结果写入 `.local/basketball/`。普通服务端命令不会自动启动浏览器；显式设置 `BASKETBALL_BROWSER_E2E=1` 才运行真实网页测试。

本轮结果：

- 服务端 18 项竞态检查通过，独立浏览器入口在普通命令中显式跳过。短按、取消、托管切换后逐帧重放的完整 `RuntimeState` 与服务端一致；输入历史有界并可恢复。
- `network-scene/result.json` 的 24 项浏览器检查通过：真实 Wasm／Canvas、连续观战帧、确认声音时序、预测与校正、缺帧恢复、页面挂起、新比赛、结果停车及会话释放。声音排程和结果边界使用明确标注的专用场景，不冒充自然完整比赛。
- `network-board/result.json` 的 14 项真实 HTTP／浏览器检查通过：原生键盘驱动权威角色、START、延迟响应下的短按、设置阻断、保存改键、两个账号的独立键位和席位、原生触屏取消、观战及 320px 布局。页面使用实际生产 CSP。此验收使用静音的音频端口替身，不证明媒体播放音质或音乐恢复。
- 原有 Native／Wasm 协议对照、针对性 CSP 检查及 TypeScript 编译通过。

主要记录：

| 范围 | 产物 |
| --- | --- |
| 服务端竞态 | `.local/basketball/engine/network-presentation-race.log` |
| 逐帧画面与校正 | `.local/basketball/network-scene/result.json`、`desktop.png`、`mobile-320.png` |
| 真实房间页面 | `.local/basketball/network-board/result.json`、`desktop.png`、`mobile-320.png`、`spectator-mobile.png` |
| 原协议回归 | `.local/basketball/wasm-runtime/result.json` |
| CSP | `.local/basketball/engine/network-csp-after.log` |
| 编译 | `.local/basketball/engine/network-typecheck.log` |

实现前及修复前失败记录包括 `video-history-before.log`、`network-scene-before.log`、`network-scene-resume-before.log`、`network-scene-result-before.log`、`network-board-before.log`、`network-board-profile-before.log`、`network-board-csp-before.log` 和 `network-csp-before.log`，均在 `.local/basketball/engine/`。真实失败暴露的账号切换就绪状态、生产 CSP、挂起恢复和结果停车问题已修复；未删除失败记录。

## 尚未完成

正式游戏目录、封面／创建选项和 OSS 素材发布仍待接入。多人同时操作的完整比赛、服务端重启后的浏览器恢复及实际窄屏平台外框还需验收。持续音乐的服务端时间线、原命令和位置恢复已接入，新增证据与限制见 [音乐恢复](AUDIO_SCENE_STATE.md)；其跨设备整局混音仍未验收。当前服务器只提供纽约配置；Court 0 的引擎接线已独立通过，尚未加入服务器规则选项。其他原场地、战役和未恢复动作仍保留各自明确边界，不以本报告替代原版完整复刻验收。
