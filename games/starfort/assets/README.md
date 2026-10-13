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
