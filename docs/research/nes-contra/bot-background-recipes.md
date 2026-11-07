# 背景型敌人的视觉配方

本任务整理最终画面中可辨认的核心、炮台和关底门图案，供公平像素分类器在启动时构造全局不可变知识。配方不读取当前 World，不包含运行时 HP、计时、实体槽位、碰撞窗口或当前隐藏关卡。原始研究基于日版 ROM SHA-256 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`；已有 REA 与原版调用记录作为来源。

## 实现前失败清单

| 边界 | 需要阻止的错误 |
| --- | --- |
| 来源 | 混用美版常量；图案 ID 未绑定图块集；更新索引误当基本地图 ID；只凭恢复源码而未核对保留的原版请求／字节 |
| 尺寸 | 把 Base 的两列图块更新当成四列超图块；忽略行数；将更新码高位当成图案索引 |
| 识别 | 相同闭合图案硬推断为核心或炮台；破坏后的墙面／门洞当成活目标；仅换 palette 就改变语义 |
| 位置 | 混用两种绘制原点；忽略 8 像素对齐；将来源中原实体坐标作为运行时输入 |
| 全局知识 | 按当前隐藏 Level 选择模板；把地图生成、HP 或状态机阶段混入视觉类别 |
| 可变性 | 调用方修改返回切片污染后续模板；相同像素／HUD 因外部规则变化得到不同知识 |
| 证据 | 把受控原版边界或单图块对照写成自然完整 Boss 战；模板照片／预期由待测配方自身生成而缺少独立来源 |

## 已确认的公开图案约束

Base 墙炮与核心使用日版 `$EB70 → $F9F7 → $C593` 的两列更新，绘制坐标为来源角色坐标减 4，再按图块定位对齐。普通炮台、Boss 炮台与门洞使用 `$EB1B` 的四列超图块，坐标减 12。配方只保留相应视觉图案和静态锚点，不读取角色坐标。

Base 的更新码 `0x84`、`0x85` 分别是核心与墙炮共用的闭合／开启图。只有这些像素时不能区分两种规则实体。`0x80`、`0x81`、`0x82`、`0x87`、`0x8A` 属于核心系列，`0x88`、`0x89` 为墙炮特有外观，`0x83` 是共同破坏外观。清单必须显式保留共享／破坏类别，不能用隐藏实体类型消歧。

第一关门洞的更新索引 `0x1E`—`0x25` 来自 Boss 被破坏后的通道展开，不能把这些模板当成未破坏关底门。门体原始外观须从原版静态地图图案与可见帧单独确认。

## 已交付配方

`engine/bot_vision_background_recipes.go` 提供 `BotVisionBackgroundRecipe` 与无参数的 `BotBackgroundVisionRecipes()`，每次返回独立切片。共 18 条配方、89 个图案：

| 可见类别 | 静态来源 | 图案 |
| --- | --- | --- |
| Base 核心 | 第 2、4 关常规图块；两列动画 | 0、1、2、6、7、10 |
| Base 墙炮 | 同上 | 8、9 |
| 不能区分的闭合／半开炮位 | 同上；视觉类别 254 | 4、5 |
| Base Boss 炮台／护板 | 两关 Boss 图块集 | 更新 0—2／3—4 |
| 旋转炮台 | 第 1、3 关，以及第 1 关替换图形区 | 更新 3—16 |
| 红炮台可见外观 | 第 1、3 关 | 更新 0x11—0x15、0x18 |
| 第一关左右 Boss 炮台 | 第一关替换图形与调色上下文 | 更新 0x26／0x27、0x29／0x2A |
| 第一关完整门体 | 第一关替换图形区，右下门板 | 基本图案 0x35 |

`Level`、`Variant`、`Alternate` 只决定启动时读取哪组固定素材，所有配方进入同一个全局知识库；不能据当前隐藏关号切换知识。`Label.Stage`、`Scene`、`Choice` 保持未知，`Phase` 为空。`Kind` 仅是视觉类别的兼容编号，不是运行时槽位的直接读取。

每个 `Codes` 元素是同一配方的一个替代外观。`Table` 区分基本超图块、更新超图块与两列动画；`CropX/Y/Width/Height` 限定可见图案，`AnchorX/Y` 是图案内可见特征的锚点。Base 两列动画继承调色组 3，不能使用动画头里的 0；17 条连续原版观测均确认组 3 和实际对齐后的 `(8,8)` 锚点。32×32 超图块使用其四象限属性。左右 Boss 炮台以各自可见炮管带中心为锚点；门体使用下右面板左缘的门缝，不读取原角色坐标。

调色循环同样属于可见外观。以 Base 1 为例，组 3 在颜色索引 36、40、41 之间变化，视觉构造器须纳入这些全局合法颜色变体，不能只编译初始色。完全破坏的墙面／炮台、尚未露出的红炮背景和已打开门洞均不标记为存活目标。

## 来源和对照

`scripts/contra/bot_background_reference.py` 只读用户提供的原版压缩包，核对 ROM SHA，并把 `levels-catalog.json.gz` 的图块／属性原始区间和已录制 `$8731` 动画表再次与 ROM 字节比较。导出的 `testdata/bot-background-original-art.json.gz` 含 1,128 个独立图案和 17 个实际调色／锚点观测；没有从待测配方生成预期。

本轮复用已有完整 REA 记录 `enemies-base-rea.json.gz`、`enemies-shooters-rea.json.gz`、`enemies-shooters-bossdoor-rea.json.gz` 及原版函数调用轨迹，没有重新执行 ROM 来冒充新的完整通关。来源路径与 SHA 全部保留在验证记录中。

视觉核对图由原始静态图案与验证过 SHA 的 RGB／CHR 页直接生成：

- [核心、墙炮与颜色循环](evidence/bot-background-base-animations-palette-cycles.png)。A04／A05 共享外观，A03 是破坏外观。
- [第一关替换图形](evidence/bot-background-jungle-boss-supertiles.png)。B 表示基本 ID，U 表示更新 ID。
- [Base 常规图块](evidence/bot-background-base-room-supertiles.png)。
- [Base Boss 图块](evidence/bot-background-base-boss-supertiles.png)。

## 复验

```sh
source scripts/env.sh
mkdir -p .local/contra-background-vision/tmp
export TMPDIR="$PWD/.local/contra-background-vision/tmp"
export GOTMPDIR="$TMPDIR"
go test -race ./games/contra/engine -run '^TestBotBackgroundRecipes' -count=1 -json
python3 -B scripts/contra/bot_background_reference.py
.local/board-art-env/bin/python -B scripts/contra/bot_background_preview.py
```

三项正常引擎包 race 检查通过：89 个配方图案逐字节匹配独立原版来源；共享图案、破坏外观、调色和锚点限制正确；调用方无法修改全局配方。red／green、来源摘要和图片校验和见 [验证记录](evidence/bot-background-recipes-verification.json)。

分类器的 RGBA 拼装、调色循环模板、匹配精度和完整电脑战役由视觉策略任务验收。本组只覆盖上表的背景对象，不把未整理的其他背景 Boss 外观算作完成。已清理本任务临时图片、日志与空缓存目录，保留最终研究图、脚本和验证产物。
