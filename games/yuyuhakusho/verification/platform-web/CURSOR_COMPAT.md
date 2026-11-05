# 逐连接输入窗口兼容验收

此修订基于已提交的 `40237232`。前端、网络 RPC 与本地时间轴保持原样，仅补充协议兼容测试。生产文件的 16 项 SHA-256 与上一轮 [冻结清单](final-manifest.json) 全部一致；新结果绑定在 [兼容清单](cursor-compat-manifest.json)。

`resync` 表示客户端必须拥有对应的有效权威状态。若同 epoch、同帧的 confirmed 已通过规则与呈现双摘要验证，该要求已满足；重复标记不强制再次下载。新帧缺少必要输入、真实缺帧或摘要不符时，现有重放校验会失败，`NetworkClient` 通过单个在途 `/sync` 请求恢复原始 checkpoint 字符串。不能用预测状态满足该要求，也不能仅因窗口为空就忽略新的权威帧。

实际 Go/Wasm 的 [场景验收](cursor-compat-scene.json) 覆盖：仅新帧、最多 6 帧冗余、同帧空窗口、已验证 resync 的幂等、新帧空窗口、缺帧及双摘要不符。正常路径逐帧校验，失败路径保持原权威状态不变。

[实际 Board 验收](cursor-gap-singleflight/result.json) 连续注入 3 个缺输入窗口的真实模块视图，并将真实 checkpoint 请求延迟 450 ms。客户端全程只发送 1 次 `/sync`，在同 epoch 从帧 251 恢复并推进至 298；阶段保持 `playing`，游戏 `loaded` 次数仍为 1，没有加载错误。测试只省略输入窗口并延迟恢复请求，不替换游戏规则、像素或 checkpoint。

此处的模块宿主验收不包含认证和 WebSocket 写入游标；后两者由服务端 owner 与 Root 真实平台验收覆盖。既有 12 帧预测、64 项输入历史、120 帧恢复环及本地 RAF 时间轴均未修改。

```sh
web/node_modules/.bin/esbuild games/yuyuhakusho/web/network-scene.ts --bundle --format=esm --platform=node --outfile=.local/md-yuyuhakusho/platform-web/network-scene-review.mjs
node scripts/yuyuhakusho/check_platform_cursor_compat.mjs
python3 scripts/yuyuhakusho/check_platform_board.py --label cursor-gap-recheck --cursor-review
```
