# 连续输入与恢复回归

失败范围见 `recovery-failures.md`，RLE 合并的针对性测试见 `network-stream.test.mjs`。

## 运行

前提：已安装项目依赖、Go 和 agent-browser，`.local/web-public/` 已按项目构建流程准备幽游白书素材和浏览器运行时。所有临时目录和浏览器 profile 均位于工作区内。

1. 执行 `npm --prefix web run build`，完成后再启动测试服务。运行测试期间不要重建前端或编辑游戏源码。
2. 执行 `node games/yuyuhakusho/tests/recovery-server.mjs "$PWD/.local/web-public"`，记录输出的本地 URL。该服务启动独立的认证、SQLite 和房间实例，并使用源码界面，以便记录真实 NetworkClient 恢复原因。
3. 在另一终端执行 `RUN=run node games/yuyuhakusho/tests/recovery-browser.mjs <本地 URL>`。默认是森林场地、浦饭对三名户愚吕弟；设置 `ARENA=2` 可检查灵界，`CPU=1` 可取消默认的两倍 CPU 限速。
4. 检查 `.local/yuyuhakusho/recovery-loop/results/browser-run.json` 的 `passed`，并查看对应截图。按 Ctrl+C 停止测试服务，服务会清理自己的数据库、编译文件和缓存。

脚本使用 Chromium 原生五点触控，反复同时按下、松开方向、A、B、C 和 A+C，检查服务端实际应用序号与恢复原因。WebSocket 每方向设置 60 毫秒交付延迟，游戏消息按 150 毫秒成批交给界面；这是人工设置的交付条件，没有把它标记为实测 RTT。

校正绘图检查从真实鉴权检查点创建独立 Go 场景，保留权威摘要校验，验证同次刷新只构造最终可见像素。恢复布局检查比较正常、恢复中和恢复后的画布及控制器几何。

源码夹具替换了代码资源清单，不能代替发行包验证。此次另保留正式构建页面的原生多点触控、服务端已应用输入、帧间隔和断流恢复记录。浏览器模拟结果不代表已完成 iPhone 真机复测。
