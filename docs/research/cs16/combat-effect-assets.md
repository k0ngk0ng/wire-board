# 血效与 HE 原始资源核对

本轮只恢复缺失的四种精灵，不改规则、渲染器或既有媒体正文。来源沿用固定的 `martinsam16/cstrike` ref `651d0d248ba04028b1b8167b156888b522bbcb73`；每个原始 SPR 和调色板的路径、URL、字节数、SHA-256、Git blob 位于 `games/cs16/assets/effects-sources.json`。第三方归档身份已记录，未建立发行方签名，也未运行原版游戏。

## 原始精灵

| 名称 | 原始字节 | 帧数／尺寸 | origin | 格式／混合 |
| --- | ---: | --- | --- | --- |
| `blood` | 3294 | 9 帧，16×16 | -8，8 | ALPHATEST 3，normal |
| `bloodspray` | 41970 | 10 帧，64×64 | -32，32 | ALPHATEST 3，normal |
| `fexplo` | 231810 | 30 帧，80×96 | -40，48 | ADDITIVE 1，additive |
| `eexplo` | 161310 | 25 帧，80×80 | -40，40 | ADDITIVE 1，additive |

四个原文件均为 IDSP v2，orientation 为 2。血效保留索引 255 的透明性，输出 alpha 为 0 或 255；其灰度像素由事件颜色调制。原 `cstrike/gfx/palette.lmp` 的 247 项是 RGB `(103,0,0)`，与公开参考的 `BLOOD_COLOR_RED=247` 对应。该色值作为 blood／bloodspray 的默认 `tint` 和 `paletteIndex` 登记，PNG 保留源灰度，可重复核对而无需把颜色烘焙进新图。

火球按原加色语义将黑底还原为透明并保留颜色强度。`fexplo` 最后一帧透明；`eexplo` 首帧和末帧透明。这些原帧保留在序列中，不能通过删除空帧改变时序。SPR 文件本身不保存播放 FPS，因此元数据不声称它自带帧率。

## 参考实现入口与边界

固定 ReGameDLL 参考源码的 `weapons.cpp:391` 起预载 bloodspray、blood、eexplo、fexplo。`util.cpp:1058` 的 `UTIL_BloodDrips` 发出 `TE_BLOODSPRITE`，包含两种精灵编号、血色和 3 至 16 的大小值；多人模式的 amount 计算和颜色条件仍须由规则／渲染对照核验。

`ggrenade.cpp:250` 起的 HE `Explode3` 发两层 `TE_EXPLOSION`：fexplo 位于原点 Z+20，scale 字节 25；eexplo 在 XY ±64、Z+30 至 35 的范围，scale 字节 30。两层消息的帧率字节均为 30，flags 为 NONE。C4 另使用多个 `TE_SPRITE`，不能把它的呈现路径混作 HE。

这里的代码入口是公开参考重实现，静态原始精灵及其格式由实际样本核验；不能把这些入口或 PNG 转换结果表述成已观察原版运行。实际血效锚点、血滴轨迹、动画计时、可见性、装甲抑制和伤害分别由对应实现及 E2E 验证。

## 音频差异

当前正式清单已包含 `weapons/hegrenade-1.wav`、`hegrenade-2.wav`、`explode3/4/5.wav`、`debris1/2/3.wav`、`sg_explode.wav` 和人物 flesh／helmet／kevlar 声。本轮没有下载、转码或上传新音频。

两个 hegrenade 原文件及 PCM 摘要相同，71778 帧、22050 Hz；explode3/4/5 分别是 28695、28574、28779 帧、22050 Hz，三条 PCM 不同。它们不是同一文件改名。参考 HE 路径发 NONE 标记的爆炸临时消息及一条 debris 声；NONE 和 NOSOUND 分别为 0 和 4。hegrenade 声只在该参考的武器预缓存处找到。原引擎临时消息的最终声音选择尚不以实际听音或运行回放作证。

## 客户端资源契约

`manifest.effects` 下保留原名 `blood`、`bloodspray`、`fexplo`、`eexplo`。每项提供：

- `atlas`：标准 file／bytes／sha256／mime 资源描述。
- `width`、`height`：PNG 实际尺寸，分别为 54×54、264×198、492×490、410×410。
- `frames`：按原顺序的 x、y、width、height 和原始 origin；x／y 是 PNG 左上坐标。
- `textureFormat`、`orientation`：原始整数；`blend` 为 normal 或 additive。
- `sourceFile`、`sourceBytes`、`sourceSHA256`：原文件身份。
- 血效另有 `tint`、`paletteIndex` 和 `paletteSourceSHA256`。

四张 PNG 总计 189245 字节。读取时保持 `flipY=false`；血效按 ALPHATEST 处理透明边，火球使用加色混合。渲染接线由客户端模块完成，素材转换器不决定命中或伤害。

## 复跑与结果

```sh
python3 -B scripts/cs16/fetch_sources.py --check
python3 -B scripts/cs16/build_combat_effects.py
python3 -B scripts/cs16/test_combat_effect_assets.py
```

转换 E2E 对照全部 74 个原帧的尺寸、origin、alpha 和逐像素 RGBA，验证 atlas 资源摘要、重复执行字节一致，以及其余清单字段和 Go 地图／命中规则摘要不变。来源恢复九项验证通过。

产物位于 `.local/cs16/verification/combat-effect-assets/`：`effects-e2e.json`、`frames.json`、`palette-evidence.json`、`source-contact-sheet.png`、`source-resources.json`、`resource-delta.json` 和 `public-materialization.json`。静态合成图只验证素材形状和透明性，不替代正式房间或 GPU 验收。

最终源／public manifest SHA-256 为 `c45d13ad6a966ba14c4dd5b3ae8f3ebb2d4de047b1587395681c74f9787e58f0`。1034 个发布输入已逐文件核验；相对上一版仅四张 PNG 和主清单变化，共五个新不可变对象、684996 字节。本轮未提交、推送、准备 CI 缓存或发布对象。
