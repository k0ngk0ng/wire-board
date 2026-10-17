# 新对局配置与原开球验证

实现前失败场景：使用 world-loop 夹具终值作为运行时初态；从配置漏掉队伍／人物／控制器／装备产生的原能力；装备在每一步提前钳位而改变最终等级；开球球高直接赋最终值，未执行原 0→7 动作、重力和动画；开球计数错把视频帧当逻辑调用；D1B5 漏掉六个篮筐位置和挂载状态；隐藏姿态覆盖错误；原 main-loop RNG 与场内开球重复消费；默认键盘映射混入游戏逻辑。未经恢复的场地／菜单路径须明确报错或作为外层选择配置，不可假装已经运行。

原菜单 ACBB/AD14 的静态队伍／人物参数与装备程序是运行时 Go 数据的来源。输入配置产生状态，测试夹具只作为验证 oracle。自然对照从原槽 2 使用既有菜单输入向前到开球后的 1550 帧，先无探针再同输入采样，禁止 RAM 注入。

## 运行时数据路径

`NewOriginalMatch(OriginalMatchConfiguration)` 从队伍、人物、控制器席位、模式、装备和显式 RNG 种子创建冷状态。C9B6→FA8C 清零 005B..00FF、C9B6→F8BD 清零 0200..07FF，是其余零值的原始依据；运行时只导入 Go 常量，不读取 ROM、存档或测试期望。

`DefaultOriginalMatchConfiguration` 提供纽约、国夫队对纽约队、每队前两个人物、第一席真人其余电脑及空装备。调用方可更改配置并分配、持久化种子。当前公开冷开局只接受纽约；其他场地在物理恢复前明确返回错误。8 队共 32 个基础人物参数已经解码，不等于其他场地也已实现。

之后按原阶段调用：

1. 阶段 0 未进入：`InitializeOriginalCourt`，隐藏原对象、初始化六个篮筐、挂载标志及镜头，发出背景请求。
2. 阶段 0 已进入：`StartOriginalCourt`，播放该场地原音乐编号，进入阶段 8。
3. 阶段 8：`StepOriginalRefereeOpening`。首次安排人物、裁判和球；第 8 次后续调用发哨声，第 28 次后续调用将球转原动作 7，并赋原上抛速度。第 32 次后续调用请求阶段 1。球的轨迹和旋转复用 `StepKnockedBackBall`，没有硬编码最后位置。
4. 阶段 1 未进入：`InitializeOpeningPeriod`，复用已恢复的原开节程序；返回 `BeginWorldInputs`。
5. `WorldFromOriginalOpening` 把状态映射到真实 `WorldLoopState`／`WorldLoopContext`，交给 `StepWorldLoop`。

镜头提交、draw-list 重置、CAA4 的主循环 RNG 调用和视频计数仍由外层场景驱动处理。场内阶段函数自身不重复消费它们。原阶段 3 的 `EnterOriginalTeamTransition` 也保留为独立入口，设置团队策略 4 后请求前置世界阶段；本次自然开局没有走此分支。

裁判开球运行 33 次（首次＋32 次后续），物理自然算得交接时球高 **73 像素、上升速度 11 像素／更新**。球高度、速度和姿态由原动作生成，不取测试夹具的终值。

## 人物参数与装备

固定 C1D8 的表给出 8 队×4 人物的恢复、攻击、跑动、起跳、投篮等级。`character_grade_data.go` 是可编辑的语义 Go 数据，`extract_character_grades.py` 记录 ROM 哈希与每个人物的源地址。`CharacterProfiles()` 将等级映射到现有动作配置；攻击和恢复等等级由 `Grades` 暴露给上下文。

装备恢复 AD8F 的 3 槽程序，包含减速、组合减级、半攻击值和全属性设置，并且只在三个槽全部执行后按原有符号字节规则钳到 0..7。原单人模式仅给前两席应用装备，双人对战模式跳过属性增益。原菜单正在预览的未提交装备使用另一组暂存输入，本接口明确拒绝以 Scene 2 预览状态当成已确定配置。

## 与世界循环的适配

`opening_world.go` 只新增适配器，没有修改 `WorldLoop`。`OpeningWorldOptions` 显式接收外层逻辑计数和两队真人战术。控制器配置仍由平台映射到逻辑按键；开局数据不含键盘键名。

`PursuitAlias74` 显式设为 `{ActionByte:0,DepthByte:0}`，来自原冷清零覆盖的 055C 和 00E6；不是从 fixture 抄回兼容值。nil 继续表示未知。人物装备尾部七个字节也按配置解码成原通用抛物线索引所需上下文。

原菜单留下的头像动画游标／时钟与亚像素位置不作为新平台必备初态。例如自然参考的人物 0 Depth 小数为 0.5；原开局会保留它，阶段调用测试也逐字保留。平台冷配置从原清零值开始，不伪造这段菜单动画历史。未进入人物动作的动画状态会由原动作入口重新初始化。

## 验证与边界

自然参考从原槽 2 按既有正常菜单输入运行 1,550 帧，再同输入加探针。完整 RAM 与五张截图均一致。**37 个原调用**逐字段及声音匹配：属性初始化 1、场地初始化 1、进入开球 1、裁判开球 33、开节 1。另有全部 32 项 ROM 参数核对、装备边界和纯配置开球测试。

REA 证据位于 `.local/basketball/rea/opening/`：

- 属性：`ev_40057efcf991665ca15ce10475fca6517cb3df7c53cb4b75307e8d25b9460d53`。
- 场地／开球：`ev_1695bcef64f07da4c4d8f62847d1e933817d3c10244e70814f3d21ca35e1b3fe`。
- 六篮筐初始化：`ev_b6b4be9a4daf42401198bc0e48e3aa6cbbf4f683bf796272aedb3475a492f135`。
- 球动作 0：`ev_33c8f03e19d26fe7f1067158348ef80d62a4186c1de9022378400d54db0c1a4a`。

Ghidra 对 B4E8 的间接跳表过度展开；只采用核对过的入口及原动作 0／7，不把其巨大伪代码当作所有特殊球均已恢复。

定向原生竞态与 Wasm 的 37 原调用、人物表、装备、配置开球与拒绝过早交接测试通过。冷配置接真实世界的固定目标 **120 次更新也已全部通过**，全过程不读取任何 fixture 初态或期望状态。首次实跑在 update 57 暴露了未进入接球动作的旧游标被碰撞 profile 过早解析；主线程按原搜索入口门槛修复，并根据原玩家共用形状表恢复已知形状的合法复用。测试没有缩短、跳过错误或回填状态。此处仅证明配置创建和该 120 步世界推进；整个比赛、浏览器与其他场地仍按主任务继续验收。

复现：

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/capture_opening.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/extract_character_grades.py
source scripts/env.sh
go test -v ./games/basketball/engine -run 'TestOriginalOpening|TestNewMatchConfiguration|TestOriginalCharacterGrade|TestOriginalEquipmentGrade|TestConfiguredOpening|TestOpeningWorld'
```

原始 RAM 与调用索引位于 `engine/testdata/opening.{ram.gz,json}`；源清单包含原槽 2 的 SHA-256。属性清单为 `character-grades.json`。日志位于 `.local/basketball/engine/opening-{before,after,race,wasm}.log` 和 `opening-world-{before,after}.log`。
