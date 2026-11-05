# 真实平台主机接入

《魔强统一战》已注册到本地平台的 `games.Modules()`，新增房间默认可见并允许创建。五项真实 HTTP／认证／SQLite／时钟／WebSocket E2E 与竞态检查通过，完整结果绑定在 [platform-host/validation.json](../../../games/yuyuhakusho/verification/platform-host/validation.json)。尚未上传资源、发布或部署。

## 已验证范围

- 两名真人通过公共房间流程开始，逐席位验证 12-bit MD 按键、序号和 epoch；观战者与非法输入被拒绝。
- 公共 `load_begin`／`load_ready` 门槛先于游戏初始化门槛；任一真人未完成时保持帧 0。全部就绪后时钟推进，恢复请求不重建门槛。
- 无输入后释放按键，保持真人控制；只有平台主动托管才开启电脑。取消托管和 SQLite 重启保持同一 epoch、局面与递增帧号。
- `/sync` 提供原样字符串 checkpoint，保留超过 JavaScript 安全整数的呈现位；普通更新不发送世界、呈现缓存或完整存档。
- 实际认证 WebSocket 的相同 nonce 输入重试只接受一次；连续发布紧凑游戏消息，恢复数据与普通消息分离。
- 一个真实 TCP WebSocket 观战者停止读取期间，快玩家请求与权威帧继续推进。本项没有直接探测内核写阻塞，不能扩大为所有背压条件已覆盖。

五份可读房间轨迹与最终竞态日志一并保留。首轮缺少注册的真实失败见 `platform-host-registration-red.jsonl`；公共门槛测试的首个失败来自测试工具在创建房间前调用 `Restart()`，按测试准备错误另存，未通过修改产品逻辑消除。

## 正式页面与双加载门槛

正常 React 应用的首次真实浏览器验收发现，游戏在公共加载门槛解除前发送 `loaded`，服务器拒绝后阻断公共 `onReady`，双方无法开赛。帧 0 的过期时间检查还触发重复恢复与无输入的 release。原始截图和完整消息观察保留在 [play02](../../../games/yuyuhakusho/verification/platform-app-browser/play02/result.json)。

修复把画面初始化、公共加载确认和游戏 `loaded` 按顺序衔接，等待阶段不发送游戏输入，不把静止的加载状态当作断线。实际 Module／Wasm 浏览器夹具已覆盖保持公共门槛 2.2 秒、只恢复一次画面、门槛解除后单次游戏就绪，以及暂停与恢复的本地预测。正式认证页面与 TCP 网络干扰仍按独立浏览器结果验收，不以夹具或 ACK 耗时替代首次画布响应。

正式认证页面的 [performance-network02](../../../games/yuyuhakusho/verification/platform-app-browser/performance-network02/result.json) 已完成创建、双人加载、原生键盘、保存后改键、虚拟手柄混合输入、全屏／ESC、320px 原生触摸和真实连接关闭后恢复。两端正常网络可见更新为 59.96／59.97 次／秒，帧间隔 p95 为 29.0／23.8 ms；先前约 8.8 帧／秒的性能失败保留。正常 HTTP 的 Resource Timing 约 3.2～4.2 ms，主线程观察值另外记录；不再把旧版同步 Wasm 阻塞后的 334 ms 回调耗时当作纯连接 RTT。

该版本单端 TCP 写入延迟 120 ms 时，快端维持正常显示，但慢端约 115 ms 才产生一次新画面，本地按键到像素约 130 ms，耗尽了有界预测提前量。后续对照篮球改为独立显示步进时钟，确认后重放至旧本地目标，保留 12 帧预测上限。最终 [native-clock-network04](../../../games/yuyuhakusho/verification/platform-app-browser/native-clock-network04/result.json) 正常双人 59.94／59.93 次可见更新／秒，慢端在同样的 120 ms TCP 写入延迟下为 55.91 次／秒、帧间隔 p95 为 31.7 ms；慢端按键到像素为 27.8 ms，ACK 为 226.4 ms。完整改键、触屏、全屏与真实断开恢复流程也通过。

## 复跑

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/platformtest

python3 -B scripts/yuyuhakusho/build_platform_app.py
python3 -B scripts/yuyuhakusho/check_platform_app.py --label <新的工作区结果名>
```

浏览器脚本使用完整的本地 Vite 构建、真实组合根与两条独立 TCP 入口；第二条入口只延迟写入或关闭连接，不改写应用消息。构建和数据库均为本地验收产物，不是正式服务器包。虚拟 Gamepad API 的检查不代表实体型号兼容性。
