# 雪地背景视觉配方

## 实现前的失败场景

- 把雪地 Boss 的 CHR 页 107 错用成普通雪地的 82，或把 Tank 调色板与 Carrier 图案任意交叉组合。
- 把消失后的全黑、擦除 U1B、出口 U16–U18、岩石 U19–U1A、车轮碎片当成可瞄准的完整敌人图案。
- 把原版敌人槽位坐标、HP、计时或隐藏关卡编号用作视觉定位、存活判断和策略输入。
- 把四个 Carrier 拼块当成 `Codes` 的复合图案；该字段实际表示彼此独立的图案备选。
- 锚点落到逻辑原点或图案外，忽略可见炮口／舱体位置；调用者修改一个配方后污染其他配方或全局知识。

测试先固定独立 ROM 图块与成组外观，检查像素、公开锚点、负例及返回值隔离，再添加生产配方。目标接口为 `BotSnowBackgroundVisionRecipes() []BotVisionBackgroundRecipe`；规则、策略和公共构造器由各自任务维护。

## 原版来源与识别边界

源 ROM 为日版 `Contra (J).nes`，SHA-256 `6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe`。`scripts/contra/bot_snow_background_reference.py` 直接从该 ROM 的图块、NES 双位平面 CHR、调色板与换页数据生成测试像素；不读取配方实现。已有雪地 REA、表格和原版执行轨迹用于确认合法图案及颜色来源。

Tank 使用 U0F、U12、U13 的完整 32 × 32 炮塔图案；Family 为 `snow-tank`。锚点分别为可见炮口附近的 `(3,12)`、`(6,22)`、`(10,27)`。车轮及静态车身不独立声明为目标。普通 CHR 为 `[1,4,14,15,80,81,82,83]`；调色板组 2 使用原版表中的 3F／5F／60／61，组 3 固定 41。颜色仅表示合法外观，不反推 HP。

Carrier 使用 U09、U0A、U0B、U0D 的完整左上拼块；Family 为 `snow-carrier`。锚点 `(28,16)` 位于拼块内侧的可见银色舱体，不是原版敌人原点。右上和下方喷口不额外产生目标。Boss 使用替换后的 CHR `[1,4,14,15,80,81,107,83]` 与完整调色板 `[61,62,64,66,0,1,34,6]`。过场淡化颜色暂不识别；全黑必须保持未知。

两类都只声明可见图案家族，不保证活着、可受伤或当前处于哪一阶段。毁灭序列中尚未擦除的同一完整图案无法仅凭像素区分，不能虚构存活状态。`Stage`、`Scene`、`Phase`、`Choice` 均保持未知。静态 Level 4 仅选择全局知识构造时的原始图块，不用于过滤实时观察。

正常背景色与原版全局闪光使用 15／32／22／17，保持每一行 CHR 和调色板完整绑定。不存在实时状态字段，也不通过隐藏关卡选择模板。

## 验证与复验

三项先行测试因缺少 `BotSnowBackgroundVisionRecipes` 出现预期编译失败后，才加入实现。针对性 race 检查通过：4 条配方、64 组成组像素外观及可见锚点、15 个负例；28 份原版替换图块和属性均与内置资源一致。返回值修改不污染全局或另一条配方。保留 [先红记录](evidence/bot-snow-recipes-red.jsonl)、[通过记录](evidence/bot-snow-recipes-green.jsonl) 和 [来源摘要](evidence/bot-snow-recipes-verification.json)。

已目检 [原版图案与锚点](evidence/bot-snow-original-art.png)。紫色十字只用于研究图标明锚点，不进入测试像素或游戏画面。测试 fixture 来自 ROM 直接双位平面解码，另与正式视觉目录的索引像素逐页比较；颜色表和静态换页数据均核对原版字节。

```sh
.local/board-art-env/bin/python -B scripts/contra/bot_snow_background_reference.py
source scripts/env.sh
go test -race ./games/contra/engine -run '^TestBotSnowBackgroundRecipes' -count=1 -json
```

测试只使用 Snow 配方构造独立观察器；公共构造器合并、真实连续画面及后续策略由集成任务验收。本项不是自然雪地通关或整款游戏完成证明。清理范围仅为本次 `.local/contra-bot-snow-recipes/` 下的初步图案预览；最终图、测试输入、red／green 和说明均保留，没有删除共享构建缓存或其他任务文件。
