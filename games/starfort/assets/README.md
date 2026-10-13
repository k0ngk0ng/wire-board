# 星垒素材

素材采用原创 2D 卡通矢量插画：圆润轮廓、低饱和林地配色、暖色石材，以及用于区分防御塔的绿色、橙色、冰蓝和紫色。源文件与制作脚本保留在本目录，透明 WebP 图集供 Canvas 使用。声音由数学波形合成，不使用第三方采样或生成模型。

## 制作前列出的失败场景

- SVG 缺失、不能解析、引用外部资源，或导出遗漏某种塔、等级、敌人姿态。
- 位图不能解码、尺寸错误、透明区域丢失、有效图像全透明、画面溢出边界，或图集裁片与单张素材不一致。
- 同一敌人的姿态完全相同，塔升级只有文件名变化，素材无法表达动画或成长。
- WAV 不能解码、通道或采样率错误、静音、削波、异常时长，或随机合成不可重复。
- 清单文件遗漏、字节数或 SHA-256 不匹配，发布时拿到旧素材。
- 地图铺满道路导致无法跟随服务端路线；背景应只包含草地和边缘装饰。

## 可重复执行

在仓库根目录执行，缓存与输出留在工作区内：

```sh
. scripts/env.sh
python3 -B -m venv .local/starfort/art-env
.local/starfort/art-env/bin/python -B -m pip install --cache-dir .local/pip -r games/starfort/assets/requirements.txt
npm install --prefix .local/starfort/rasterizer --no-package-lock --no-save @resvg/resvg-js@2.6.2
.local/starfort/art-env/bin/python -B games/starfort/assets/generate.py
.local/starfort/art-env/bin/python -B games/starfort/assets/validate.py --report .local/starfort/assets-validation.json
```

依赖为 Pillow 和 resvg。栅格化不使用系统字体，保留透明通道与 SVG 渐变。`src/` 保存 SVG，`public/starfort/v1/` 保存随包与 OSS 共用的成品，`manifest.json` 保存原创声明和校验和。图集索引 `atlas.json` 的 `frames` 以无扩展名的素材名为键，值为 `{x,y,w,h}`；透明边缘包含在尺寸内。塔、敌人与基地使用底部居中锚点；弹道、特效与金币使用中心锚点。地图为 960 × 640，路径与建造点由游戏按服务端坐标绘制。

## 世界扩展素材（world-v2）

`world-v2/` 独立保存扩展素材的源文件、生成脚本、验证脚本与清单；成品位于 `public/starfort/world-v2/`。原有 `v1` 素材与发布记录保持不变。新增 40 张原创 SVG 对应 40 张 WebP、3 个合成音效、1 张图集和 1 份图集索引，共 45 个成品文件。

| 素材 | 命名与尺寸 | 用途 |
| --- | --- | --- |
| 地表纹理 | `tile-{forest,canyon,river,snow,ruins,marsh}`，256 × 256 | 按世界坐标重复铺设；不把原始小地图拉伸为大地图 |
| 路线插画 | `choice-{forest,canyon,river,snow,ruins,marsh}`，256 × 160 | 分支选择卡片；森林、峡谷与河流接入当前地图，其余供后续地形使用 |
| 场景物件 | `prop-{tree,pine,rock,bridge,highground,watchtower,mine,cart,gate,ruins,reeds}`，128 × 128 | 地形、瞭望塔、矿场和补给车；`watchtower` 对应规则中的 `beacon`，`cart` 对应 `convoy` |
| 功能敌人 | `enemy-{healer,banner,splitter,engineer}-{0,1,2}`，128 × 128 | 治疗者、旗手、分裂怪与工兵各 3 个姿态 |
| 事件图标 | `event-{fog,frost,surge,star}`，64 × 64 | 波次事件提示 |
| 首领光环 | `effect-boss-aura`，128 × 128 | 与原有首领图像叠加，表现阶段变化 |
| 音效 | `audio/{expand,event,objective}.wav` | 地图扩展、事件提示、支线或成长奖励；单声道 22,050 Hz、16 位 PCM |

透明物件和敌人使用底部居中锚点，图标与光环使用中心锚点。图集为 1024 × 1372，索引继续采用 `{frames: {name: {x, y, w, h}}, meta: {image, width, height}}`。前端应读取索引，不自行推算裁片位置。

安装前文的 Pillow 与 resvg 后，在仓库根目录生成并验证：

```sh
. scripts/env.sh
.local/starfort/art-env/bin/python -B games/starfort/assets/world-v2/generate.py
.local/starfort/art-env/bin/python -B games/starfort/assets/world-v2/validate.py --reproduce --report .local/starfort/world-assets/validation.json
```

验证覆盖独立解码、透明边缘、地表拼接边缘、图集逐像素裁片、姿态差异、音效格式和幅度、清单校验和，以及重新生成后的逐文件一致性。联系表输出到 `.local/starfort/world-assets/contact-sheet.png`，用于人工核对画风与轮廓。素材上传 OSS 时使用独立的 `starfort/world-v2/` 路径，并核对 `world-v2/manifest.json`；声音统一接入平台播放器。
