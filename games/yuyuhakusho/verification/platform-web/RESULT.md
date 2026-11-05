# 魔强统一战平台前端验收

游戏前端的功能与性能修订已通过，当前时间轴版本冻结于 [最终清单](final-manifest.json)，旧功能基线保留在 [性能修订前的功能清单](functional-before-performance-manifest.json)。Root 的 [正式双真人验收](../platform-app-browser/native-clock-network04/result.json) 覆盖实际认证、SQLite、WebSocket、快慢连接并存与恢复；正式发布仍由 Root 单独收口。

性能修订将 RPC 分叉与重放的 JSON 复制改为已校验的内存深拷贝；无消费者请求像素的权威追赶帧只执行同一 canonical tick，实际显示的最后一帧仍使用回调时刻的绘图请求。预测帧不再计算未被消费的摘要，确认帧的规则与呈现双摘要继续校验。[100 帧对照](replay-pixels-performance-v1.json) 的完整保存、双摘要及下一显示帧像素与原路径一致。

本地时间轴沿用独立 RAF 节奏，校正后只追赶原本已经显示的预测目标，新输入排在该目标之后。[延迟权威集成](timeline-native-clock-verified.json) 和 [重校正输入边界](timeline-rebase-input-verified.json) 使用实际 Go/Wasm，仍保持 12 帧上限。Root 正式平台的正常两端分别为 59.94／59.93 fps，95 分位为 21.6／28.2 ms；每次 TCP 写入延迟 120 ms 后，慢端为 55.91 fps、95 分位 31.7 ms。慢端原生按键至首次像素变化为 27.8 ms，输入 ACK 为 226.4 ms；另一快端原生响应为 23.6 ms。预算保持至少 50 次可见更新／秒、95 分位不超过 40 ms。

## 已验证结果

| 范围 | 结果与证据 |
| --- | --- |
| 输入与资源边界 | [8 项针对性测试](unit-final.log) 通过，包含收到确认与已应用确认分离、12 位输入、空取消不发释放、加载／终局撤销不发非法释放。 |
| 实际 Board | [组合浏览器验收](final-all-boundaries/result.json) 通过：原生键盘在 500 ms 服务器输入延迟前改变 Go 预测姿态与 Canvas 像素；公共全屏及 ESC；320 px 无横向溢出，手柄位于画面下方。 |
| 两层加载门槛 | 同一验收中，实际 Preview 完成并通知平台就绪后，公共门槛继续保持 2.2 秒，游戏 action 为零、checkpoint 请求仅 1 次；门槛解除后只发送一次游戏 `loaded` 并推进规则。原始失败见 [实际模块 red](public-loading-red/failure.json) 和 Root 的正式平台 `play02` 记录。 |
| 暂停与恢复 | [原始 red](pause-resume-review-red/failure.json) 复现等待确认才能恢复；修复后，500 ms 输入延迟期间，首暂停及恢复均先改变本地 Go 状态，权威阶段尚未变化。观测轮询耗时不等同于完整原生事件延迟。 |
| 实体输入接口 | 通过明确标记的虚拟 standard Gamepad API 设备驱动真实游戏，验证服务端应用、松开、断开保留键盘、重连必须中立、合法清除主键及替代绑定。未使用实体硬件，不声明任何型号兼容。 |
| 原生触屏 | 320 px 下，多指方向和 X 高位输入、斜向滑动、单指松开、取消保留键盘均通过；原始触屏事件和服务端按键结果见组合验收。 |
| Wasm 适配器 | [实际 Go/Wasm 检查](wasm-validation-first.json) 通过：错误像素目标不推进状态、Preview 分离、12 位输入、非法 buffer 后运行时存活、原始 JSON 字节身份、16 会话上限与释放。 |
| 预测与终局 | [实际场景复核](scene-review.json) 确认预测最多 12 帧，错误摘要不改权威状态，result 不推进；终局恢复保持帧 3952，RGBA 与独立终局参考逐字节一致。 |
| 普通平台构建 | [构建检查](app-build-final.json) 通过：首页不带 Board／网络／Wasm 实现，13 个固定资源路径的压缩大小和摘要一致，运行时清单与 Wasm 一致。 |
| 正式平台与慢端预算 | Root 的 `native-clock-network04` 全流程通过：正常两端及 120 ms 写延迟慢端都通过同一预算，同 epoch 断线恢复、保存改键、虚拟手柄混合断开、320 px 原生 C 触摸及全屏 ESC 均通过。 |

桌面和窄屏截图分别为 [desktop.png](final-all-boundaries/desktop.png) 和 [mobile-320.png](final-all-boundaries/mobile-320.png)。输入 ACK、权威应用时间、可见画面间隔及首个本地姿态／像素变化记录在组合验收 JSON 中；模块测试宿主不代替真实平台网络性能结论。

## 重复执行

所有命令在当前 worktree 根目录执行，临时构建和缓存均位于 `.local/`。

```sh
node --experimental-strip-types --test games/yuyuhakusho/tests/platform-web.test.mjs
web/node_modules/.bin/tsc --noEmit -p web/tsconfig.json
python3 scripts/yuyuhakusho/check_platform_board.py --label recheck --controls --pause-review --loading-review
node scripts/yuyuhakusho/check_platform_wasm.mjs wasm-recheck
web/node_modules/.bin/esbuild games/yuyuhakusho/web/network-scene.ts --bundle --format=esm --platform=node --outfile=.local/md-yuyuhakusho/platform-web/network-scene-review.mjs
node scripts/yuyuhakusho/check_platform_scene.mjs
node scripts/yuyuhakusho/check_platform_replay_pixels.mjs replay-pixels-recheck
node scripts/yuyuhakusho/check_platform_timeline.mjs timeline-recheck
node scripts/yuyuhakusho/check_platform_timeline.mjs timeline-rebase-recheck --rebase
python3 scripts/yuyuhakusho/build_platform_app.py
```

普通 React 平台构建输出到 `.local/md-yuyuhakusho/platform-app/dist`。真实宿主使用 `server.New`、`ClientLoading: true`、已注册模块及 `os.DirFS(dist)` 服务此目录，沿用实际认证和 WebSocket。构建提供 `/game-load-manifest.json`、13 个资源清单条目、资源描述符及 `/yuyuhakusho/v1/runtime/{build.json,wasm_exec.js,client.wasm}`。

此构建只用于本地验收。正式 CDN 不可变发布、客户端载荷剥离和最终服务端包检查仍须按项目发布流程执行。Board 保留平台音频接口；本前端验收不把尚未接入的原版混音计为完成。

已按 [清理记录](cleanup-final.json) 删除本子任务已关闭的两个浏览器临时 profile 及其会话元数据，保留旧结果、真实 red、最终证据与 Root 仍在使用的本地构建。后续按连接游标减少重复输入窗口属于新的独立修订，不包含在本轮时间轴冻结中。
