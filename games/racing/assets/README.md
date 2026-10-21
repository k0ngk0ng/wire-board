# 极品飞车素材

保留的音效和赛道地图图集目录为 `public/racing/v1/`，公共素材基地址沿用 `https://cdn.ichenj.com/bga/v20261002-1`。客户端先请求 OSS／CDN，同版本路径提供本地回退。发布脚本禁止覆盖不同内容的已有对象；修改已发布素材须新增版本目录。

## 三维重制

主画面不再使用俯视车辆图集。六款风格化车身、环境和新封面分别由 `web/car-definitions.ts`、`vehicle-model.ts`、`chase-renderer.ts`、`chase-fallback.ts`、`scene-theme.ts` 和 `Cover.tsx` 原创绘制。车型名称用于选择外观，未导入厂商模型或《极品飞车》游戏素材。旧图集仅保留给可选赛道地图。

Three.js 0.180.0 使用 MIT 许可，全文保存在 `public/racing/v2/three-license.txt`。`manifest-v2.json` 绑定公开署名文件，`public/racing/v2/attribution.json` 记录代码来源与 SHA-256，并链接沿用音效的署名。原始 v1 对象保持不变。使用 `verify-assets.py --remake --dist --oss` 验证两版素材；`publish-assets.py --remake` 仅发布新的 v2 署名文件。

## 来源与许可

| 用途 | 作者与来源 | 许可 | 处理 |
| --- | --- | --- | --- |
| 车辆、地形与物件 | [Kenney Racing Pack](https://kenney.nl/assets/racing-pack) | CC0 1.0 | 从 SVG 裁切并以 4 倍分辨率导出车辆，统一六种配色；保留五种车身，场景适度降低饱和度 |
| 烟尘、火花 | [Smoke Particles](https://kenney.nl/assets/smoke-particles)、[Particle Pack](https://kenney.nl/assets/particle-pack)，Kenney | CC0 1.0 | 透明图片缩放、火花着色、图集打包 |
| 引擎循环 | [racing car engine sound loops](https://opengameart.org/content/racing-car-engine-sound-loops)，domasx2 | CC0 1.0 | 单声道 44.1 kHz、去除直流偏移、峰值归一化、3 ms 循环边缘淡化、MP3 编码 |
| 漂移轮胎循环 | [Car tire squeal skid loop](https://opengameart.org/content/car-tire-squeal-skid-loop)，audible-edge（Tom Haigh），qubodup 制作循环 | **CC BY 3.0** | 单声道 44.1 kHz、去除直流偏移、峰值归一化、3 ms 循环边缘淡化、MP3 编码 |
| 轻重碰撞 | [Kenney Impact Sounds](https://kenney.nl/assets/impact-sounds) | CC0 1.0 | 单声道、音量调整、MP3 编码 |
| 封面、倒计时、发车、过线与结束提示 | wire-board contributors 原创 | 项目原创 | SVG 绘制；离线加法合成短音 |

漂移音效原始来源为 [Tom Haigh 的录音](https://freesound.org/people/audible-edge/sounds/71739/)，许可链接为 [CC BY 3.0](https://creativecommons.org/licenses/by/3.0/)。作者、素材链接、许可及修改说明保存在公开的 `attribution.json`；游戏素材信息也须提供这些说明，不能省略署名。

原文件及许可证据保存在 `source/`；摘要、作者与来源见 `source-manifest.json`。成品摘要、路径、生成方式及来源关系见 `manifest.json`。外站页面只保留为非公开的 `.html.txt` 证据，不复制到 `public/`。

## 前端契约

- `cars.webp`：1152 × 1600；每格 192 × 320，6 列席位颜色 × 5 行车身，车头朝上、中心锚点。实际汽车从 SVG 导出，不放大原 PNG。尺寸为统一显示单元，不决定物理碰撞尺寸或性能；通过颜色和本车轮廓辨识，不叠加席位数字。
- `scenery.webp`：1024 × 1280；19 个元素的实际裁切矩形在 `atlas.json`。地形可平铺，装饰按道路几何摆放。素材自身不包含路径、检查点或物理规则。
- `cover.webp`：1040 × 640，源文件 `source/cover.svg`；`art.ts` 同时导出 `coverSVG` 供封面回退。
- 音频：`engine.mp3`、`tires.mp3` 为循环音；`collision-light.mp3`、`collision-heavy.mp3`、`countdown.mp3`、`start.mp3`、`lap.mp3`、`finish.mp3` 为单次音。全部通过平台音效接口播放，游戏不得创建播放器或音频上下文。

## 重建与验证

需要 Python Pillow、FFmpeg 和 `@resvg/resvg-js` 2.6.2。所有工作缓存留在仓库目录；首次导入从研究阶段保存的原 ZIP 提取，已经提交的 `source/` 足以重建成品。

```sh
source scripts/env.sh
npm install --prefix .local/racing/rasterizer --package-lock=false --no-audit --no-fund @resvg/resvg-js@2.6.2
.local/board-art-env/bin/python scripts/racing/prepare-assets.py
python3 scripts/racing/verify-assets.py --decode-audio
python3 scripts/racing/publish-assets.py
python3 scripts/racing/verify-assets.py --dist --oss
```

制作前失败场景和验收边界见 [VALIDATION.md](VALIDATION.md)。接触表在 `.local/racing/assets/`，验收 JSON 在 `.local/racing/verification/`。素材检查不代替实际浏览器中的车辆可辨识性、碰撞动画、音效事件和统一静音验证。
