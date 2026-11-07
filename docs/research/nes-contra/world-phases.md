# 原版帧阶段与连续场景接入

2026-10-25：已将角色、真实武器与第一关水平地形滚屏分别接成可执行的独立阶段。它们仍需要与敌人、图形队列、调色、战役状态机组成完整世界；阶段差分通过不代表独立对局或八关已完成。

## 角色与子弹阶段

`CombatFrame.AdvancePlayersAndProjectiles` 对应日版 `$D296..$D2D8`，保存两名角色、真实 16 槽弹丸池、碰撞窗口、共享状态及 10 个逻辑精灵。输入只来自当前帧按键和原状态，输出没有从原版 after 记录回填。

阶段按原版顺序执行：清本帧滚动／敌人活动标志／武器强度；冻结活动玩家集合；先推进第一位活动玩家，再推进第二位；每个角色动作与精灵选择后才更新出生无敌、无敌道具、后坐力和外部水平推力；最后处理同伴镜头与借命，然后推进全部子弹，并按帧奇偶选择 8 个弹丸显示槽。

`AttackEnabled` 对应 `$8E`，表示至少存在一名正常、已结束出生保护的角色，供敌人开火／生成使用。它不表示该角色一定能受伤：无敌道具仍可存在。原始夹具中早期标签 `vulnerablePlayer` 保留为来源记录，代码采用更准确的名称。

原版水平镜头对非滚动发起者只减 1 像素，即使整帧滚动量更大也如此；竖向镜头则使用帧滚动量并保留小数与高字节进位。角色步进和共享修改在副本上执行，第二个角色发生规则错误时不会留下第一个角色已推进的状态。

- 第一关自然操作：1,220 次完整阶段，包含正常、死亡及重新入场。
- 基地、基地 Boss、瀑布：3,719 次完整阶段，分别为 1,815、765、1,139 次；起点及 Boss 设置均保留明确的受控记录。
- 对比覆盖角色全部已记录字段、同伴／共享状态、完整弹丸池、碰撞及最终逻辑精灵。模式录制的完整 RAM 也与对应原版角色录制一致，观察钩子没有改变运行。

本次模式覆盖不包含从第一关自然走到第八关。复查原版材料分别见 [自然阶段](evidence/world-phase-original.json.gz)、[模式阶段](evidence/world-modes-original.json.gz)。

## 第一关水平地形阶段

`JungleScroll.Advance` 对应 `$DB81` 的横向分支。两个超图块缓冲可以部分替换；它不会把新屏幕一次性覆盖到整个碰撞窗口。每滚动 8 像素提交一列 28 个图块，碰撞只在相应的偶数图块列和行更新；每 16 像素的 `+3` 位置提交一列 7 个属性字节。

图形输出是 `BackgroundWrite{Address, Step, Values}`，交给统一队列在显示边界应用；碰撞更新属于当前规则阶段。图块列对应原版 33 字节包，属性列对应 24 字节包。这样可以与敌人的图形请求共用真实编码预算，不能用「更新条数」代替原版 `$21` 的字节占用。

已有 600 次自然滚屏阶段逐项通过，包含 29 列图块、15 列属性、局部碰撞更新和半屏前瞻解码。另有屏幕翻页和错误回滚的针对性边界验证；该自然轨迹还没有跨完整一屏。翻页会返回 `ResetEnemyReadOffset`，世界在敌人阶段前应清 `EnemySystem.ScreenReadOffset`，对应原版 `$82`。

上述 `JungleScroll` 是早期阶段。当前通用 `TerrainScrollState` 已覆盖八关地图／替换图形及基地 Boss 地图切换，见 [地形滚屏恢复](terrain-scroll-recovery.md)。阶段差分通过仍不能代替连续战役验收。

## 世界仍需保持的调用顺序

日版正常关卡入口 `$CCCC` 为：暂停处理；未暂停时处理关卡特有的帧前逻辑；角色／子弹阶段；全灭判断；地形滚屏；当前敌人 AI 与碰撞；固定出生；随机跑兵；图案与调色更新。全灭进入日版关卡阶段 `$0B`，等待值为 `$60`；不能套用美版 `$0A`。

NMI `$F889` 先提交上一阶段准备的 OAM／图形数据，执行音频更新，随后读取按键和推进世界，最后组装下一次显示的精灵。世界里的声音请求也在下一次音频更新生效。后续 `World` 必须区分规则状态与待显示／待播放结果，保持命中、资源变化与呈现的顺序；不能把当前帧结束的 CPU 缓冲冒充本帧已经显示的画面。

原始 REA bundle 保留在 `world-*-rea.json.gz`，入口、来源与汇编索引见 [world-native-index.json](evidence/world-native-index.json)。需注意 Ghidra 对 `$C83B` 的自动分析会将 `$C848..$C857` 的分派表解释为指令；部分尾跳转的函数边界也会扩大。恢复依据必须结合真实分派地址和运行轨迹，不能直接翻译这部分自动伪代码。

## 复查与剩余工作

`ActiveWorld` 已将首关普通战斗接成连续独立阶段。从一个原版初态推进 600 帧，只输入控制器状态和明确标注的原版帧前熵；不再每帧重建角色、敌人或地图，不读取原版 after 状态作为执行输入。完整角色／弹丸、敌人池／生成器、地图／碰撞／共享状态、街机分数、图案动画、OAM、图形包和声音请求顺序均与原版一致。生产随机序列仍使用已批准的确定性适配器；这项对照使用原版熵以隔离规则差异。

`WorldPresentation` 负责显示边界，保存待提交／已提交状态。日版帧尾 `$CB1E` 添加的零命令单独由 `Terminated` 表示，不伪装成绘图包；该字节计入下一显示边界的 `$47` 调色板门槛。`$C83B` 的过渡参数指向唯一的 `CombatFrame.ScreenCleared`，不是地图流式加载的剩余列数。显示边界另验证了繁忙队列延迟颜色、保存恢复、无效状态原子失败和快照独立性。

本次记录为 [active-world-verification.json](evidence/active-world-verification.json)，完整原版输入输出为 [world-complete-original.json.gz](evidence/world-complete-original.json.gz)。使用正常 Go 包路径检查，包含平台构建约束，不使用源文件白名单：

```sh
source scripts/env.sh
go test -race ./games/contra/engine -run 'TestOriginalContinuousActiveWorld|TestWorldPresentation' -count=1 -v
```

`AdvanceWithPause` 另通过 180 帧连续原版对照，其中 100 帧暂停，包含两次暂停／恢复。两次独立录制的输入、完整 RAM 和逐帧记录完全一致。暂停位只使用 `Campaign.Paused`：进入暂停时跳过战斗；已经暂停的帧先按 `$D2B0` 投影交替弹丸槽，再经 `$FA52 → $A4EF` 更新角色显示，随后更新 CHR／调色板；恢复帧先执行这一显示前缀，再执行正常战斗。它没有移动暂停中的弹丸，也没有重建玩家状态。

REA 暂停决策证据为 `ev_4639bbecc3a85db5e7a06bd3faa5f1cbb8dc89a7c70e79c9a97470b81ac5b847`（[campaign-fixed-rea.json.gz](evidence/campaign-fixed-rea.json.gz)），弹丸显示证据为 `ev_5fdab793361e08464246565ef3ed20f8790283207b8261e00e8eea37d711b67a`（[world-display-rea.json.gz](evidence/world-display-rea.json.gz)）。新轨迹见 [world-paused-original.json.gz](evidence/world-paused-original.json.gz)，通过 `scripts/contra/world_capture_pause.py <新标签>` 可重录并校验一致性。回放复查及带源码摘要的结果由 `python3 -B scripts/contra/world_verify.py` 生成。

`ActiveWorld` 的两个基础入口限制首关普通战斗，暂停入口只复用战役暂停决策。完整战役调度使用下述 `World`；不能仅凭基础入口通过便认定完整首关或八关通关。声音请求的顺序已有对照，真实音频状态及其 carry 依赖仍待接线。

## 战役 World 集成

后续新增的 `World.Advance` 使用唯一 `Campaign` 和 `CampaignSceneState`，已接入 `BeginTick → AfterCombat → AfterEnemies` 及首关出口脚本。全灭判断发生在真实玩家／子弹阶段之后、敌人阶段之前；96 帧全灭等待继续运行环境，不再推进玩家。Boss 出口使用原版脚本生成移动／跳跃输入，再执行真实物理，不直接指定角色终点。每个成功 tick 必须结束全部 `Campaign.Pending` 工作，重复 tick 无副作用，倒退 tick 原子拒绝。

当前完整世界验收覆盖四条连续轨迹：

| 场景 | 帧数 | 初态与限制 |
| --- | ---: | --- |
| 普通战斗 | 600 | 原版双人开局状态 |
| 暂停／恢复 | 180 | 100 个暂停帧、两次暂停恢复 |
| 全灭等待 | 373 | 起点仅把两人剩余命数设为 0；按向右实际死亡，277 帧战斗后等待 96 帧 |
| Boss 与出口 | 6,472 | 起点仅把两人剩余命数设为 30；实际射击击败 Boss，5,987 帧普通阶段、162 帧清场等待、323 帧出口脚本 |

每条轨迹只从第一条 before 初始化一次，随后只提供按键和明确的原版帧前熵；不把后态传回执行器。每帧另进行完整 JSON 保存恢复、重复／倒退 tick 检查。两条新轨迹均独立重录一致。`scripts/contra/world_verify.py` 使用正常 Go 包路径及 `-race`，保留执行日志、全部引擎代码／数据及原版轨迹摘要，并拒绝把验证期间发生源文件变化的结果作为新的稳定验证记录。

原版证据：[world-loss-original.json.gz](evidence/world-loss-original.json.gz)、[world-boss-original.json.gz](evidence/world-boss-original.json.gz)。长轨迹额外发现并校正了 S 弹的 `$06` 状态传递、Boss 批量销毁炮台属性，以及独立的 `$FB` 出口渐隐计数误映射到 `$74` 调色板计时的问题。文字图形队列现在按 `$23` 选择独立格式解析，与矩形图形共享唯一 `Used` 预算；不能将两种格式混用或改成同长度的绘图包。

上述四条轨迹证明首关战役片段及保存恢复，不是标准三命或八关通关。新局启动和平台帧入口的验证范围见下；完整跨关／续关循环、联机、电脑和实际浏览器对局仍未验收。

## 标准新局与平台帧入口

`World` 现已接入初始化、Intro、准备战斗、入口和 Interlude 分派。新增 [world-start-original.json.gz](evidence/world-start-original.json.gz) 从未修改的原版开机流程记录 1,077 个完整世界调用：1 次初始化、669 次 Intro、1 次准备战斗、48 次地图铺设和 358 次普通战斗。每帧检查源状态、声音顺序、文字／矩形队列及真正的 nametable／palette PPU 数据，未回填后态。

原版大背景复制可能跨 NMI。录制时仅看 `$F8DB` 会把嵌套中断返回误当成世界调用完成，曾把准备战斗的中间状态当成后态。录制器改为同时匹配 C001 入口栈深加 2；两次正确录制一致，原始 RAM 与旧运行完全一致。完整依据见 [边界修正](evidence/start-capture-boundary-fix.json)及 [显示边界 REA bundle](evidence/world-display-boundary-rea.json.gz)。

`WorldPresentation` 现在保存背景、应用后的 OAM／滚动、期望与应用后的调色板／CHR，以及显示忙碌计数和遮罩。提交顺序保持原版：调色板门槛检查、排队图形／颜色写入、忙碌递减、显示遮罩及图案提交。文本模板可以直接写 palette，不能把所有队列数据都当背景。场景直接背景立即写入同一显示状态；队列解析模式与精灵排列模式分别维护，不能把 `$23` 和 `$07EB` 都叫同一个模式。

平台入口是 `NewSession(seed, players, epoch)` 与 `Session.StepPads(frame, heldPads, resources)`，不读取 ROM、CPU 状态或研究快照。标准新局使用原版资源数量；游戏计数器从平台帧 0 开始，不重演开机菜单等待。每个新权威帧依次提交显示、推进音频、应用已批准的熵适配、采样并执行游戏输入。`Audio.Context.EndPlayersAlive` 来自唯一 `Campaign.AliveAtExit`；敌人声音 carry 从入口音频状态与阶段前缀纯投影，实际音频请求只排队一次。

熵元数据保留精确的十进制字符串 seed／entropy／frame，唯一原版工作值仍在 World。两个标准新局和一个双人逻辑按键场景合计推进 1,460 帧，覆盖保存恢复、输入边沿、正常枪持续按住不制造额外射击边沿、重复帧不重放及非法跳帧原子拒绝。相关检查已纳入 `scripts/contra/world_verify.py` 的正常包 race 验收。浏览器控制器、实际渲染与网络还需单独验收。

```sh
source scripts/env.sh
python3 -B scripts/contra/world_verify.py
```

当前证据入口为 [active-world-verification.json](evidence/active-world-verification.json)，[world-core-verification.json](evidence/world-core-verification.json) 保留早期独立阶段记录。重新采样使用 `world_capture.py`、`world_capture_modes.py`、`world_capture_scroll.py`，分别指定新的运行标签。脚本拒绝覆盖原运行目录，重复捕获须与保留记录一致。

## 续关与终局提交

`World` 已消费 `CampaignReset`，接入全灭菜单和继续／结束选择。[续关轨迹](evidence/world-continue-original.json.gz)覆盖 898 帧，从实际全灭到恢复标准生命并重返战斗；[结束轨迹](evidence/world-quit-original.json.gz)覆盖 386 帧，从实际全灭到选择结束。两者只在起点将剩余命数设为 0，之后只提供控制器输入，没有写入死亡或结果。完整 RAM、实际 PPU、菜单计数、显示及声音顺序逐帧一致，两次独立重录也一致。

原版 `CD0E` 把图形流首字节置 0，但不倒退分配游标。`DiscardGraphics` 因此单独保留被丢弃命令的字节预算：不绘制旧命令，下一次调色板提交仍受相同门槛限制。不能简单清空预算来让调色板提前出现。

平台应在 `Campaign.Outcome` 确定后调用 `Session.FlushTerminal()`，待 `Finalized` 才发布 Result。它只提交剩余画面和音频，最多消耗原版显示忙碌的 5 个边界；不会推进游戏、重新采样输入或消耗熵。终局音频边界数单独保存，不冒充新的玩法帧。重复调用和终局保存恢复不会重复提交事件。

## 后续跨关兼容状态

`TerrainWork` 只提供具名的查询、扫行、流式地图打包、透视偏移和图形定位记录方法，不提供通用内存读写。2,677 个原版前缀／24,310 次操作及 50 个后续 Guardian 消费场景已通过契约对照；这不是生产 World 的实际调用链验收。

原版 Guardian 清墙有一个有限越界效果：最多触及碰撞窗口后紧邻的 5 个图形头字节。`GraphicsQueue.ClearPrefix` 保存这些 AND 清位，保持 `Used`、Clone 和 Reset 语义；`Encode` 返回真实变化后的字节，`DecodedWrites` 在首命令 0 处停止。解析仅限自有图形数据，不能从相邻 CPU 状态补出未拥有的字节。该行为与图形预算、调色板提交共同验证，没有把越界强行夹回碰撞窗口。

真实第六关原版记录另揭示武器回调需要 `DFB8` 的实心支撑判断：当前格是平台但下一行是实心时，子弹必须在位移前碰撞。800 个完整玩家／弹丸阶段已验证修正。相关契约与本轮主验证一起通过正常包 race。

下一步接通完整跨关循环和生产 World 的碰撞状态传递，再进行浏览器输入、联机及电脑完整对局验证。当前没有注册或发布游戏。
