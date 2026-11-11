# NFS4 原版资源转换

本目录记录离线工具契约、依赖和可重复验收，不保存原始安装包、ISO、VIV 或 FRD 副本。输入保存在仓库内 `.local/racing-nfs-reference/`：试玩提供 P911、FM50、France；补充原盘按文件范围取得 LDIA、MCF1、CCOR、Coastal、UK、Park。原文件摘要与取得范围分别见调研清单和 `.local/racing-nfs-reference/additional/acquisition-manifest.json`。完整 ISO 未下载，不能声称已核验整张光盘的字节摘要。

发布候选资产位于 `games/racing/assets/public/racing/v3/`，不与旧 v1/v2 对象混写。它们是原版资源的派生产物，包含 EA 的美术与声音内容；工具许可证不改变原资源的权利归属。

| 车型 | 原名称 | 外观三角面 | 驾驶舱三角面 | GLB 字节 |
| --- | --- | ---: | ---: | ---: |
| p911 | Porsche 911 Turbo | 1127 | 1512 | 230816 |
| fm50 | Ferrari 550 Maranello | 995 | 1546 | 234364 |
| ldia | Lamborghini Diablo SV | 1077 | 1094 | 227588 |
| mcf1 | McLaren F1 GTR | 907 | 1529 | 233360 |
| ccor | Chevrolet Corvette | 1044 | 1207 | 228064 |

保留原顶点身份，包括 LDIA 中原有的零面积面。MCF1 仅有一套基础 CARP；其余四车各有 base/stage1/stage2/stage3，不能把缺失升级参数补成虚构版本。

五车 GLB 同时包含独立的 `exterior` 和 `cockpit` 节点。驾驶舱来自各车 VIV 的 `dash.fce` 与 `dash00.tga`，不是外车简化内饰或程序仪表台；每个原部件的位置、顶点、贴图 RGBA 与原三角面数量均单独验证。GLB 顶层 `cockpitCamera` 为原 `interior_camera` 经 X 反射并只加一次 `groundOffset` 后的眼位；`cockpitSource` 记录原成员大小和 SHA-256。驾驶舱模式显示本车原内舱，其他车辆保持完整外观，离开此视角恢复本车外观。日夜仪表面按原 unlit/lit 部件切换。

| 独立赛道 | 原 VROAD 点数 | 长度 | 高差 | 日景 GLB 字节 | 夜景 GLB 字节 |
| --- | ---: | ---: | ---: | ---: | ---: |
| france | 973 | 4487.36 m | 188.18 m | 5580064 | 5608836 |
| coastal | 1075 | 4955.41 m | 73.11 m | 7211276 | 7229172 |
| uk | 839 | 3867.08 m | 27.86 m | 4610616 | 4609816 |
| park | 939 | 4327.86 m | 69.50 m | 6238096 | 6237264 |

日夜是同一路线的场景变体，不计作两条独立赛道。资源清单含五车及驾驶舱、八种场景、路线、离线碰撞数据、天空、原车循环声与下述自制喇叭，共 65 项、54,631,266 字节。该数字包含所有变体和辅助规则数据，不能当作首场下载量；网页应按当前赛道、当前昼夜和选中车型加载。

坐标、对象和材质的约定见 [CONTRACT.md](CONTRACT.md)。重要细节：原版左手系先反射 X，再平移到非负 X/Z；GLB 与路线使用同一 origin。车头为 +Z，四个轮心节点独立，轮轴为局部 X。道路原始 3D 高程、左右宽度、墙面、对象和原贴图分辨率均保留。France 首点的铺装横断面经原碰撞面验证为左 6 米、右 12 米；墙距是另一项数据。

车身 TGA 的 alpha 是颜色标签，不能全部当作透明度。公开重建的区间说明为：223～230 对应 primary、160～182 对应 interior、96～118 对应 secondary、32～46 对应 driver；命中像素按原颜色组处理后设为不透明，0 继续透明、255 固定色不变。应在原像素上处理，避免 shader 插值导致分区串色。这来自公开重建的 `core/resources/car_texture.gd`，尚未作为原版运行行为直接测量。GLB 保留原 RGBA；渲染器需完成该处理。

每轨还保留原生循环对象动画、正确的 billboard 旋转中心及原纹理动画元数据。循环动画作为 glTF AnimationClip 输出，64 Hz、原帧间距；billboard、additive、flipbook/UV scroll 由网页渲染器应用。对象破坏动画需要碰撞事件驱动，少量未解释的原碰撞类型保存在 route 的 `unresolved`，不能静默当作实体墙。

原车声音仅提取与负载/滑行表关联的 27 个前方发动机短循环，排除后方空间混音及其余整库内容。每个 FLAC 都与原循环区间 PCM 逐字节相同；五车音频及完整原控制表合计 1,171,725 字节。RPM 表下标映射标为社区重建依据；连续音高倍率是数学近似，原固定点 LUT、随机微调与精确对数音量包络未移植。不能据此声称原版音频输出已被实机验证。

喇叭为单独标注的自制双音，**非极品飞车原声**。目前未能证实 H 到原 BNK patch 的映射，不将候选循环冒充原声。`scripts/racing/nfs-horn.py` 生成 350／440 Hz 加权正弦、22050 Hz 单声道 16 位 PCM，完整 0.2 秒周期无缝循环，再无损编码 FLAC。`audio/horn/horn.flac` 为 1,690 字节，连同 `horn.json` 共 2,283 字节，按 CC0-1.0 提供。元数据记录合成公式、循环范围及 FLAC／PCM 摘要；三项合成 E2E 校验精确 PCM、频率、循环接缝、无削波、格式、清单与重复字节一致。

从仓库根目录执行，依赖和缓存均留在仓库内：

```sh
. scripts/env.sh
export PIP_CACHE_DIR="$PWD/.local/racing-nfs-reference/asset-cache"
export PYTHONDONTWRITEBYTECODE=1
python3 -m venv .local/racing-nfs-reference/asset-venv
.local/racing-nfs-reference/asset-venv/bin/pip install -r games/racing/assets/source/nfs4/tool-requirements.txt

# out 必须是新目录，转换器不会覆盖已有产物。
.local/racing-nfs-reference/asset-venv/bin/python -B scripts/racing/nfs-convert.py \
  --game-root .local/racing-nfs-reference/additional/game-root \
  --out .local/racing-nfs-reference/rebuild-geometry \
  --cars P911 FM50 LDIA MCF1 CCOR --tracks France Coastal UK Park
.local/racing-nfs-reference/asset-venv/bin/python -B scripts/racing/nfs-audio.py \
  --game-root .local/racing-nfs-reference/additional/game-root \
  --out .local/racing-nfs-reference/rebuild-audio

PYTHONWARNINGS=ignore::ResourceWarning .local/racing-nfs-reference/asset-venv/bin/python -B scripts/racing/nfs-assets.e2e.py --extended
PYTHONWARNINGS=ignore::ResourceWarning .local/racing-nfs-reference/asset-venv/bin/python -B scripts/racing/nfs-cockpit.e2e.py
PYTHONWARNINGS=ignore::ResourceWarning .local/racing-nfs-reference/asset-venv/bin/python -B scripts/racing/nfs-audio.e2e.py
python3 -B scripts/racing/nfs-horn.e2e.py
```

音频工具另需 PATH 中的 FFmpeg/FFprobe；本次为 FFmpeg 9.0.1。未使用 Blender，也未安装新的客户端几何解码运行时。

验收结果见 [verification.json](verification.json)：六项真实资源 E2E、四项真实驾驶舱 E2E、三项真实音频 E2E 均通过；重复生成字节一致；驾驶舱加入后重新检查的 13 个 GLB 通过 Khronos `gltf-validator@2.0.0-dev.3.10`，错误、警告和提示均为 0。RGBA 图片与原始像素摘要匹配，无损 WebP 仅在整个 GLB 更小时采用；FLAC 解码后采样格式、帧数及 PCM 摘要均与对应原区间一致。这些离线验证不替代正式网页的画面、操控、加载和平台音频验证。
