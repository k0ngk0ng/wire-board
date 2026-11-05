# 浏览器入口先行失败范围

本清单与 `scripts/yuyuhakusho/check_runtimebridge_schema2.py` 先于本地 Wasm／页面迁移。

- 页面仍按 JS 对象传输资源，丢失原始换行，存档仍为 schema 1；不验证 gzip／raw SHA 或预加载全部场地。
- `load-v2` 缺少七段 `Uint8Array`、类型错误或精确字节错误，却清空正在运行的会话。
- checkpoint 经 JS parse／stringify 后接受舍入的 `MaxUint64`，或原样传递仍丢失精度；恢复失败修改当前会话。
- 十进制字符串帧号超过 `2^53` 后变为 Number 或不能准确递增；存档同时保留新旧呈现 owner。
- 加载预览推进权威规则／帧，错误的 RGBA 目标修改状态，预览未提供可见的 F+1 画面。
- 原样恢复后 30 帧世界／槽／HUD／像素轨迹不一致；PC 原生键盘、320px 三指触摸或隐藏释放失效。
- 当前地图缺少 aux，或损坏的压缩资源仍允许启动。

验收产物位于 `games/yuyuhakusho/verification/client-bridge-schema2/`。测试只启动本地 loopback，不发布资源。
