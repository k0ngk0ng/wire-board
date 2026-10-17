# Court 4 运动与世界状态接入

本次将 Court 4 接入原开场、世界动作、准备阶段和视频运行入口。Court 5～7 的完整运行门禁仍关闭。Court 4 使用已恢复的完整 `$EEFF` 运动，不复用纽约地形。

实现前列出的失败场景包括：方向标志重复保存、换边时丢失共享随机数、辅助对象种类被静态目录覆盖、骑乘者未分离、后续动作返回值覆盖物理写入、接触或显示读取旧种类，以及无效种类通过恢复校验。初始失败日志位于 `.local/basketball/rea/court-four/binding-before.log`；恢复校验的独立失败记录为 `kind-validation-before.log`。

原场地设备逻辑在辅助对象 4／5 的纵深位于 `[56, 72)` 时平移 2 像素。方向来自原 `$06D8`，仍由 `Actors[1].Player.NewYorkPassage` 保存；该字段在 Court 4 表示场地方向，不解释为纽约通道。越过横向边界后，原逻辑消耗共享随机数、从对应对象的原表选择种类、设置未进入的动作 0，并处理附着人物的分离和受击。辅助对象的种类会变化，不能每帧重新从场地编号推导。

`AuxiliaryState.Kind` 保存 Court 4 的当前种类；开场从原对象目录初始化，运动绑定保留选中及其他对象的写入。旧静态场地保留零值并沿用既有目录。`worldObjectKinds` 在进入世界循环、显示和转场时合并当前种类；接触和输入使用本次世界循环派生的目录，发射源也读取当前辅助对象种类。恢复 Court 4 时，缺少种类或种类超出原辅助对象范围均返回错误，不猜测或修补存档。

`scripts/basketball/verify_court_four_motion.py` 使用未修改的原 `$EEFF` 指令生成 640 组隔离输入，覆盖对象 4／5、两方向、横向边界、纵深 55／56／63／71／72、附着人物和两个固定随机状态。其中 168 组发生换边。脚本模拟原 MMC3 PRG 换页，仅截获音效请求；ROM 与压缩夹具 SHA-256 保存在 `engine/testdata/court-four-motion-source.json`。这是配置输入的原函数执行，不能称为 640 场自然比赛。

`TestCourtFourOriginalConveyorMotionBinding` 先核对完整原运动状态，再检查绑定后的所有对象语义字段、种类、随机数和链接。候选 XY 与原高度临时字段属于单次运动计算，不持久化到世界。`TestCourtFourDynamicKindsSurviveWorldConsumersAndRestore` 检查冷开场、动态对象显示、期末转场、JSON 重放、非法种类及未开放场地拒绝。

```sh
source scripts/env.sh
python3 -B scripts/basketball/verify_court_four_motion.py
go test -race ./games/basketball/engine -run '^(TestCourtFour(OriginalConveyorMotionBinding|DynamicKindsSurviveWorldConsumersAndRestore)|TestOriginalCourtFloorConfiguredMachines|TestCompleteOriginalCourtMotionNatural|TestWorldActionsEntireCapturedCorpus|TestWorldPreparationErrorsAreAtomic|TestOriginalOpeningStages)$' -count=1 -v
```

Native、race 和 Wasm 的定向验收日志位于 `.local/basketball/rea/court-four/binding-{after,race,wasm}.log`。既有回归范围包括 68,094 次原场地解析、13,229 次完整自然运动、37 次原开场调用和 1,998 个完整原动作阶段。Court 4 的调色板／动画接入与四种控制配置的正常完整比赛由各自验收记录覆盖；上述短测试不替代完整比赛。
