# 热血篮球输入同步优化

用户最新决定：保留现有 Go/Wasm 引擎，优先解决同步数据过大和重复恢复／序列化问题，暂不移植整套 JS 规则。客户端继续本地计算；日常传输权威帧输入、必要事件和校验值，不传整套内部状态或服务器绘制帧。完整状态仅在进入、重连、历史缺口、配置修订及校验偏差时按需获取。

旧实现基线：游戏视图平均 54,596 字节，按 10 Hz 约 545,960 字节/秒；其中完整 runtime 约 38 KB，120 帧历史约 14 KB。对比泡泡堂约 2,081 字节/视图。以上不含公共包裹层、压缩和浏览器耗时，不能当成按键上传量或 FPS。

## 实现前失败场景

- 相同输入无法合并、帧号断续被默许；压缩丢失短按锁存、取消、自动控制归属；非法长度／位掩码造成无限分配或部分重演。
- 连续快照仍包含完整 runtime 或全部长历史；完整页面元数据更新再次附带全量运行状态。
- 启动快照与其帧号／版本不原子；旁观者无法初始化或未授权用户能读取其他房间的恢复数据。
- 取消按键和接管发生在视频帧之间，导致错误校验或每次松手都拉取全量状态；中场配置变更未触发恢复。
- 重复、旧版本和乱序消息导致重复模拟、旧声音重播、分数错位；历史缺口没有显式恢复，或恢复请求无限排队。
- JS/Wasm 每次推进仍返回完整内部状态；预测分叉经由 JS 全量来回序列化，或共享可变状态污染权威会话。
- 优化只降低流量而仍有长帧、操作等待 ACK、反复跳回；需要实际浏览器测量而不是仅通过逻辑测试。

验证保留原规则与 AI 的确定性，对压缩前后的输入逐帧比较；使用真实 HTTP/WebSocket 的身份、旁观和恢复流程。浏览器记录 PC/320px、正常网络与 300ms RTT 下的流量、帧时间、输入响应和校正情况。只验证篮球及受影响的平台契约。

## 同步边界

- 常规快照发送最近 24 帧的压缩输入、帧号、物理按键锁存、自动控制归属、声音事件和状态摘要。连续相同输入合并；重复或重叠窗口只推进尚未确认的帧。
- 完整状态通过鉴权后的 `/api/rooms/{id}/sync` 获取。房间、版本、视图和检查点来自同一状态边界。普通 `/api/state` 和 WebSocket 推送均不携带完整运行状态。
- 本地使用 Wasm 会话推进权威输入，再在独立的原生会话中预测操作；分叉不再通过 JS 导出、导入整个状态。日常返回值仅包含当前可见画面所需的数据。
- 检查点下载期间最多保留 64 个增量更新，恢复后接续输入并显示最新画面，不补播旧声音。确实无法接续时最多尝试 3 次，失败显示可重试错误。
- 普通移动的画面修正使用短暂平滑；命中、场景变化及大幅位移立即采用新画面。平滑只改变显示副本，不修改规则、AI 或服务器结果。

## 浏览器测量

2026-10-23，本地真实 Go WebSocket 服务、生产 `StreamInputs` 和 React Board、当前 Go/Wasm 构建。外层页面和音频端口使用测试夹具。字节数为压缩前的 WebSocket 文本，不是网络线路上的总字节数。

| 项目 | 结果 |
| --- | --- |
| 游戏视图平均大小 | 912 字节，旧版为 54,596 字节，减少约 98.3% |
| 300 ms RTT，普通移动下行 | 约 16,038 字节/秒，包含游戏消息公共字段 |
| 300 ms RTT，CPU 降速 4 倍，普通移动下行 | 约 15,191 字节/秒 |
| 同上，连续方向、A/B、组合键和 Turbo 操作 | 约 20,963 字节/秒，采样约 9.9 秒 |
| 同上，动画帧间隔 P95 / 本地移动首次响应 | 16.8 ms / 98.1 ms |
| 同上，普通移动同帧瞬间校正 | 0 次 |
| 同上，完整状态请求 | 初始化 1 次；连续操作期间无新增；断线重连新增 1 次 |
| 检查点响应延迟 1 秒 | 25 项交互、席位切换及恢复检查通过 |

移动响应发生在服务器确认该按键之前。CPU 降速和 320px 触屏模拟属于桌面 Chromium 验证，不代表已经覆盖所有实体手机。短时流量和帧间隔采样也不能保证所有球场、设备和网络都没有卡顿。

## 复现与产物

从仓库根目录执行；需已安装 Go、Node、前端依赖与 agent-browser，并能读取已发布素材。浏览器脚本复用已验证的素材缓存。

```sh
source scripts/env.sh
export AGENT_BROWSER_SOCKET_DIR="$PWD/.local/cs"
node --experimental-strip-types --test games/basketball/tests/input-sync-scene.test.mjs games/basketball/tests/reconciliation.test.mjs
BASKETBALL_BROWSER_CPU_RATE=4 BASKETBALL_INPUT_SYNC_E2E=1 BASKETBALL_BROWSER_RTT_MS=300 BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowserNetworkBoard$' -count=1
BASKETBALL_CHECKPOINT_DELAY_MS=1000 BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowserNetworkBoard$' -count=1
BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowser(Loading|PlatformRooms)$' -count=1
python3 -B scripts/basketball/accept_network_scene.py
```

验收输出在 `.local/basketball/`：`input-sync-rtt300-cpu4/result.json` 保存消息大小、时间戳和实际画布绘制记录，同目录保存桌面及手机截图；`network-board-checkpoint1000/result.json` 保存慢恢复交互结果。生命周期检查覆盖中场、配置修改、结算、继续比赛、声音顺序及会话释放。完整原版对局和竞态检查由篮球专属 CI 执行，平台恢复接口由 `TestModuleSyncSnapshotE2E` 验证鉴权、隔离和只读行为。
