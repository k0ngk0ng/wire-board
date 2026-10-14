# 扑克牌素材

默认「传统英式」采用 Byron Knoll 绘制的 54 张牌面，另外提供 Nicu 插画和 Jumbo Index 大字四色。所有图片保留原件、来源路径、许可说明和 SHA-256；牌背与三个游戏封面由本项目绘制。

| 素材 | 作者与来源 | 许可 |
| --- | --- | --- |
| 传统英式 | [Byron Knoll / notpeter](https://github.com/notpeter/Vector-Playing-Cards) | Public domain / WTFPL |
| 轻松插画 | [Nicu / nicubunu](https://nicubunu.ro/graphics:playingcards) | Public domain / CC0 |
| 大字四色 | [Saul / saulspatz](https://opengameart.org/content/jumbo-index-playing-cards) | CC0-1.0 |
| 纸牌音效 | [Kenney Casino Audio](https://kenney.nl/assets/casino-audio) | CC0-1.0 |
| 普通话报牌 | [hexgrad/Kokoro-82M-v1.1-zh](https://huggingface.co/hexgrad/Kokoro-82M-v1.1-zh)，通用声音 `zf_001` | Apache-2.0 |
| 炸弹爆破、统一角标与点数花色 | wire-board contributors | 项目原创 |
| 三款封面、三款牌背 | wire-board contributors | 项目原创 |

`source/` 保存实际采用的原图和许可证明；`source-manifest.json` 保存原件来源，`manifest.json` 同时记录发布文件与校验和。双副牌的第二张同图牌使用独立的实体 ID，显示时映射到同一图片；换肤不改变牌值、牌序或选牌。

## 重新制作与核验

从仓库根目录执行，缓存和临时图片均留在 `.local/`：

```sh
. scripts/env.sh
node scripts/poker/render-large-index.cjs
# 安装 kokoro、misaki[zh]、soundfile、scipy 后，使用工作区内的 Python 环境：
.local/poker-fixes/tts/bin/python -B scripts/poker/generate-speech.py
python3 -B scripts/poker/verify_oss.py
```

源包的下载地址、哈希与许可证据见 `docs/research/playing-card-asset-sources.json`。准备脚本只读取具名压缩包成员；Nicu 包中的 `../svg` 不能直接解压到工作目录。

渲染使用 `.local/starfort/rasterizer/node_modules/@resvg/resvg-js` 与 `cwebp`。当前牌面为 `web/public/poker/v4/{skin}/`，64 段普通话语音为 `v3/audio/`，封面为 `v2/covers/`，牌背和纸牌声仍为 `v1/`。OSS 前缀为 `bga/v20261002-1/poker/`，子目录与本地一致。已发布对象不覆盖，后续改图或改音频须新增版本。旧版渲染脚本用于历史溯源，不用于重做当前素材清单。

牌面在生成时将统一角标、原创花色与原始人物图组合为整张图片，所有数字和字母使用 Arial 64／700，包括 10 和 Joker。牌面不叠加中文主牌、级牌、配牌角标。浏览器优先使用 OSS，失败时依次回退到本地图片与文字牌面；封面回退到各游戏的 SVG 组件。

语音使用 Apache-2.0 的 Kokoro 中文模型，版本固定为 `01e7505bd6a7a2ac4975463114c3a7650a9f7218`；模型、声音和依赖版本的校验记录见 `manifest.json`。原始 WAV、文案和生成器均入库。炸弹／王炸语音叠加本项目合成的爆破声；没有复用商业游戏录音。模型说明与许可证保存在 `source/licenses/`。

音效通过平台 `BoardProps.audio.playWithFallback` 播放，同一声音在 OSS 失败时才尝试本地源，避免把回退地址当播放列表重复报牌。统一静音、离桌清队列和设备媒体音量均由平台处理。
