# 魂斗罗运行层与平台房间验收

原生 Go 服务端负责推进规则，浏览器 Go/Wasm 只重放服务端接受的输入。每个状态须通过同源 Go 编码的摘要校验，才进入画面和音频队列。正式房间界面已接入平台加载屏障、控制器设置、FC 手柄、托管入口、全屏容器及 `BoardProps.audio`；本项没有注册生产游戏、上传 OSS 或发布应用。

本次验收构建的规则 ID 为 `b6082df8a8b596541dda3e4898ff4ce9a0a0fc681a7cab144439bb307a0cbbee`，正式前端加载清单 SHA-256 为 `c395d7ed8a9b2debbfc020afe6531030e5d718cf80b757ab2f658de864036ca1`。这些值标识本次实际运行的产物，不代表后续规则源码自动继承验收。

后续生产初始化准备已把规则身份与实际运行产物身份分开：当前资源目录使用 `RuntimeBuildID`，它是清单精确字节的摘要。下方保留本轮旧命名的历史记录；当前构建和恢复契约见[生产初始化与构建准备](../../../games/contra/build/README.md)，不能用旧规则 ID 替代新运行产物 ID。

## 状态与资源边界

- `rulesBuildID` 由规则源码及嵌入数据的排序摘要生成，并同时注入原生程序和 Wasm。检查点绑定 epoch、规则代码、稳定关卡 bundle ID 和状态摘要。规则帧与呈现修订分别计数，暂停呈现、恢复请求及终局提交各有连续 revision。
- 普通房间快照携带最近 120 个紧凑重放记录；缺口通过真实 `/api/rooms/{id}/sync` 恢复。旧检查点不得倒退已验证状态；后台返回、过长积压和网络断粮后的音频恢复均使用当前已验证状态。
- 本席位输入、释放和换关确认共用一个递增序号与发送链。观战、托管和加载屏障期间不发送操作心跳；当前托管归属读取平台席位标志。屏障解除后只发送新的物理输入，取消托管不重开对局。
- 清单中的 URL 绑定 `/contra/runtime-${rulesBuildID}/`。Wasm、Go 加载器、关卡数据和图形页校验实际 SHA-256；加载器另使用浏览器 SRI。Wasm 包检查确认八份关卡 JSON 及各自独有的 256 字节片段均未被嵌入。
- 全部可达图形清单包含 100 页，首关实际请求 26 页和 `stage-1.json`，未预取第二关数据。重连和换关还检查当前 Applied Patterns，允许前一关过场画面在边界暂时保留。声音只在事件需要时加载。

## 实际呈现与交互

服务端每 50 毫秒发送三个独立的 60 Hz 规则步骤。旧前端在一次刷新中画完三个状态，真实浏览器对照中 32 次可见更新全部跨三个 revision。现实现使用两帧起始缓冲，每次 RAF 只绘制一个已验证状态；没有预测输入或在前端决定物理结果。

音频与画面在同一呈现回调中消费。公共玩家栏及游戏内「备用生命」使用已绘制状态，并读取引擎在画面提交边界锁存的公开生命值。公共结算浮层等待本局终局状态实际呈现；服务端结算和积分不因此延后。超过 12 帧的积压、超过 250 毫秒的显示停顿或断粮会转到最新已验证状态并校准音频，不补播旧效果。

| 测量 | PC | 320px 手机 |
| --- | ---: | ---: |
| 连续可见样本 | 391 | 91 |
| revision 每次递增 1 的比例 | 100% | 100% |
| 可见帧间隔中位数 | 16.7 ms | 16.7 ms |
| 画面尺寸 | 600 × 562.5 px | 304 × 285 px |
| 实测 HUD 最低文字对比度 | 12.99:1 | 12.99:1 |
| 接收的 WebSocket 原始数据 | 约 784 KB/s | 约 785 KB/s |
| 输入 ACK 延迟中位数 | 4.82 ms | 6.35 ms |

ACK 延迟只衡量请求到确认，不是输入到画面的总延迟。以上是本机环回结果；较高的原始快照流量仍需公网和并发验收，不能据此宣称公网体验已通过。

真实房间覆盖双人首次加载屏障、原生键盘／鼠标与多指触摸移动射击和跳跃、自然失命、逐 RAF 公共／内部生命一致、暂停与恢复、PC／手机全屏及退出、观战、托管切换、超过 120 条记录的真实检查点恢复。控制器设置覆盖手机弹窗输入隔离、B 键改为 H、另一账户仍为 J、保存后不重挂载 Canvas，以及刷新后同 epoch、序号和配置恢复。

额外的字节代理只延迟真实 HTTP／WebSocket 返回数据，不改写消息或注入游戏状态。650 毫秒暂停传输后，客户端恢复呈现并产生九次原生媒体定位事件。检查点响应在页面隐藏后到达时，没有创建音轨；重新显示后持续音轨恢复，速率为素材时钟与 60 Hz 权威时钟的比值。完整音频边界另见 `audio-integration-review-verification.json`。

## 证据与复验

汇总及文件摘要：[`evidence/platform-runtime-verification.json`](evidence/platform-runtime-verification.json)。完整结果、旧行为对照、先红后绿记录、实际执行脚本和截图保存在 [`evidence/platform-runtime/`](evidence/platform-runtime/)；已对 PC、手机、设置模式和自然失命截图进行目检。

验证已保存的结果：

```sh
python3 -B scripts/contra/verify_platform_runtime.py \
  docs/research/nes-contra/evidence/platform-runtime/room-e2e.json.gz
```

重建和重新运行时，从仓库根目录执行。Python 环境需已安装 Pillow；本次使用工作区内既有的 `.local/board-art-env/bin/python`。输出目录必须是新的工作区内路径。

```sh
source scripts/env.sh
.local/board-art-env/bin/python scripts/contra/browser_build.py
npm --prefix web run build
CONTRA_PLATFORM_BROWSER=1 \
CONTRA_PLATFORM_BROWSER_OUT="$PWD/.local/contra-platform/recheck" \
go test ./games/contra/server -run '^TestContraPlatformBrowserFixture$' \
  -count=1 -timeout 30m -v
```

在另一终端运行浏览器验收，结束后创建 fixture 中声明的 `stop` 文件关闭测试服务器：

```sh
source scripts/env.sh
python3 -B scripts/contra/platform_accept.py \
  --fixture .local/contra-platform/recheck/fixture.json \
  --label recheck --presentation --network-boundaries
```

运行层七项 race、魂斗罗前端 56 项和公共音频 92 项检查通过。此证据覆盖首关的平台与呈现边界，不替代八关自然战役、电脑完整对局、实体 iOS、设备音频输出或长时间并发验收。
