# 扑克牌素材

默认「传统英式」采用 Byron Knoll 绘制的 54 张牌面，另外提供 Nicu 插画和 Jumbo Index 大字四色。所有图片保留原件、来源路径、许可说明和 SHA-256；牌背与三个游戏封面由本项目绘制。

| 素材 | 作者与来源 | 许可 |
| --- | --- | --- |
| 传统英式 | [Byron Knoll / notpeter](https://github.com/notpeter/Vector-Playing-Cards) | Public domain / WTFPL |
| 轻松插画 | [Nicu / nicubunu](https://nicubunu.ro/graphics:playingcards) | Public domain / CC0 |
| 大字四色 | [Saul / saulspatz](https://opengameart.org/content/jumbo-index-playing-cards) | CC0-1.0 |
| 纸牌音效 | [Kenney Casino Audio](https://kenney.nl/assets/casino-audio) | CC0-1.0 |
| 三款封面、三款牌背 | wire-board contributors | 项目原创 |

`source/` 保存实际采用的原图和许可证明；`source-manifest.json` 保存原件来源，`manifest.json` 同时记录发布文件与校验和。双副牌的第二张同图牌使用独立的实体 ID，显示时映射到同一图片；换肤不改变牌值、牌序或选牌。

## 重新制作与核验

从仓库根目录执行，缓存和临时图片均留在 `.local/`：

```sh
. scripts/env.sh
python3 -B scripts/poker/prepare_sources.py
node scripts/poker/render.cjs
node scripts/poker/render-covers.cjs
python3 -B scripts/poker/verify_oss.py
```

源包的下载地址、哈希与许可证据见 `docs/research/playing-card-asset-sources.json`。准备脚本只读取具名压缩包成员；Nicu 包中的 `../svg` 不能直接解压到工作目录。

渲染使用 `.local/starfort/rasterizer/node_modules/@resvg/resvg-js` 与 `cwebp`。成品在 `web/public/poker/v1/`，按原有公共素材发布流程收入前端产物。OSS 对象前缀为 `bga/v20261002-1/poker/v1/`；若更换已发布素材须新增不可变版本目录，不覆盖现有对象。浏览器优先使用平台 `assets` 地址，失败时回退到随应用发布的本地图片；封面回退到各游戏的 SVG 组件。

音效使用平台 `BoardProps.audio`，遵循统一静音与设备媒体音量。首版使用洗牌、滑牌、落牌、收牌音效，不使用来源不明的商业游戏语音。
