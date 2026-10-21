# 疯狂坦克原生动画图集 v3

v3 增加坦克动画，保留 v2 地图、音频、头像及其他已发布媒体。

## 生成与核验

```sh
python3 -B scripts/fortressblue/prepare-animation-assets.py
python3 -B scripts/fortressblue/verify-animation-assets.py
```

生成器沿用仓库内的原版解码依赖，路径为 `.local/fortressblue-original/tools/py`。输入为官方网页客户端 `Download/char01.i16` 至 `char14.i16`，本地副本位于 `.local/fortressblue-original/downloaded/`。

主图块实际朝左，另一个图块朝右；图集保持主图块在偶数项、另一图块在奇数项的原始顺序。`animations.json` 的 `left` 指向偶数项，`right` 指向奇数项。方向修正只更新元数据及坡面帧选择，已发布图集像素、矩形、锚点及 SHA-256 保持不变。

每个文件有 296 条记录。每条记录含 RGB565/LH5 图像、原始锚点及可选的另一个朝向图像。只有一个朝向的记录复用原图，并在 `sharedFacing` 标明，不自行镜像。

输出包括：

- `public/fortressblue/v3/tanks/`：14 辆坦克的无损 WebP 图集，边长均不超过 2048 像素。
- `../data/animations.json`：全部原始帧的图集矩形、锚点、方向及动作选择。
- `manifest-v3.json`：各输出文件的 SHA-256、字节数和不可变 URL 路径。
- `.local/fortressblue/assets-v3-evidence/verification.json`：逐帧像素与原表检查结果。
- `.local/fortressblue/assets-v3-evidence/action-samples.png`：每辆坦克的动作中间帧及锚点样本。

图集没有裁切、缩放、重着色或镜像。`pixelScale=1`，地图原始像素与坦克原始像素使用同一世界坐标比例。移动端可通过镜头缩放保证可辨识性，无需单独放大坦克。绘制位置为 `(worldX-ax,worldY-ay)`。

## 动作与证据边界

`animation-sequences.json` 保存原生动作分组。分组由原版 WASM 的 TSV 加载器 `f2354`、角色构造函数 `f1580`、状态设置器 `f398`、绘制函数 `f3321` 和状态推进函数 `f3303` 确认，覆盖全部 14 辆坦克。来源、原文件 SHA-256、表地址、文件偏移、函数摘要与 REA Evidence ID 保存在 `evidence/animation-native-evidence.json`。完整解释见 [原版分析](../../../docs/research/fortressblue-refactor/native-findings.md)。

普通弹与特殊弹都使用 Fire 的 10 帧；Fire 在第 5 帧发射炮弹，结束后进入 AFire。原编号 80–89 是 Damage，90–99 是 Item，死亡使用 Die 的 226–275（含每帧 5 个坡度），随后使用 Dead 的 276–295。水平姿态只选择每组坡度的中间帧，不把坡度变体连播。 `slopeVariants` 保留原生表中的 1 或 5。五坡度动作可在水平帧的 `sourceIndex` 上加 −2…2；双朝向交错图集的帧数组索引相应加两倍偏移，朝向保持不变。原图接触表显示右向第 0 帧右低、第 4 帧右高；因此画布纵坐标向下为正、世界坡度取右地面减左地面时，原生坡度符号还需取反，并乘角色朝向。该像素方向核对图由来源验证脚本生成。Drop 为第 225 帧。此前视觉推测的映射已被这些原生证据替换。

原版只确认每两次更新推进一帧，精确更新频率仍未知。本项目选用 30 Hz 更新时钟，每两次更新推进一帧，标为 `timingEvidence: reconstructed-30hz`。Fire 总时长 667 ms，第 5 帧对应出膛延迟 334 ms；AFire 为 333 ms、Die 为 667 ms、Dead 为 1333 ms。这些毫秒值是重构参数，不是原版实测值。

`tanks.json` 的 `original` 保存原版 `Asset/character.txt` 的每列与每值，包括重复的 `Character` 列名、`MinAngle`、`Energy`、`Range`、双弹 `Power` 和 `Round`。`Round` 的游戏含义尚未证明，不据此推导延迟。

第 13 辆坦克的原表属性与旧大厅 JSON 不一致，本次保存并应用战斗客户端原表。每个坦克还记录来源文件路径和 SHA-256。

## 本地与发布状态

v3 图集共约 1.2 MiB，随源码保存，确保干净检出包含本地副本。2026-10-21 已将 14 张图集上传至 `https://cdn.ichenj.com/bga/v20261002-1/fortressblue/v3/`，逐对象核对 OSS 和 CDN 的 SHA-256。前端沿用 CDN 失败后回退同源路径的加载方式。正式生产版本以 GitHub Release 记录为准。

`manifest-v3.json` 与已发布 v2 清单分开。CDN 上传使用不可变目录及禁止覆盖请求，按清单核验，不覆盖旧对象。

原素材版权属于原权利方。本次沿用会话中已授权的原版资源提取范围，记录来源、转换与 SHA-256，不将原版图片标为原创。

## 标准检查与历史生成器

`python3 -B scripts/fortressblue/verify-assets.py --dist` 同时检查 v2 媒体和 v3 图集、当前坦克原参数、原生动作分组及构建目录中的资源。v3 检查只读取已提交的证据与媒体，不依赖 `.local` 原包、Pillow 或解码库；可独立运行 `python3 -B scripts/fortressblue/verify-v3-assets.py --dist`。

旧 `prepare-original-assets.py` 在存在已发布 v2 清单或当前 v3 数据时拒绝覆盖。恢复 v2 文件使用 `fetch-assets.py`。`verify-original-source.py` 仅在仓库内临时目录重建旧媒体并比较 v2 清单 SHA，不写入当前坦克、道具或地图规则数据；结束后清理临时重建目录。

## 选择坦克预览

选车卡片直接使用 v3 图集的彩色 `idle.right[0]` 原生帧，保留当前方向元数据，不镜像、不重着色。旧 v2 `*-portrait.webp` 来自 `Asset/tank.i16`，前 13 张图像本身含蓝色调和英文标题，不能用于表达普通可选状态。旧对象保持原样，不覆盖其内容或 SHA-256。

预览沿用 CDN 失败转同源的加载顺序；v3 均不可用时使用已有 v2 `*-idle.webp` 彩色战场帧。全部媒体失败时显示无编号的可见坦克占位，中文名称、参数和选中边框仍可用。桌面和手机均使用一致的画幅及居中比例，不通过透明度或 CSS 滤镜改变原始色彩。重复验收入口见 `tests/PICKER_COLOR_SCENARIOS.md`。

`picker-frames.json` 仅记录 idle 帧的透明边界和原图 SHA-256，让包括透明留白较多的萨满在内的 14 辆车保持相近的预览尺寸；不修改原始锚点或战场绘制。可用 `python3 -B scripts/fortressblue/prepare-picker-frames.py --check` 逐像素重算并核对，去掉 `--check` 可重新生成元数据。验收同时要求 320px 下每辆车实际不透明区域的宽、高均至少 30px。
