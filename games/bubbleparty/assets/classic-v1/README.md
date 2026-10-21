# 2002 原始地图贴图与预览

由 `scripts/bubbleparty/extract-classic-maps.py` 从已绑定 SHA-256 的保留客户端提取。此目录是新增版本，不覆盖已有资源版本。

- `sprites/`：366 张完整 RGB565 RLE 解码图集，包含原始全部帧。
- `previews/`：112 张默认帧静态地图重建图，不是运行截图。
- 每个文件的 SHA-256、尺寸、源成员偏移、成员 SHA-256、帧矩形及未解释字段见 `games/bubbleparty/data/classic-2002.json`。
- 解析依据、已确认规则及未知项见 `docs/research/bubble-classic-maps/README.md`。

资源提取不改变原作权利归属；来源摘要不是许可证明。正式使用时按平台既有 OSS 不可变目录发布及校验流程处理。
