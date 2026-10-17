# 人物攻击道具的接触反应验收

隔离验证前的失败场景：

- 把人物→道具套用普通人物受伤、抗击余量或攻击者回弹；原871C分支没有这些副作用，也不发接触音效。
- 先请求道具7，然后仅在未腾空且高度整数为0时覆盖成13，不能按浮点高度、旧动作或速度猜测。
- 道具Facing由双方整数X比较决定，相同整数但不同小数也应按原相等分支处理。
- 道具旧动作6应重置SpecialHolder所指人物动作并清标记；其他旧动作不能误清。
- 不应在接触时重置道具位置、速度、Owner、动画或附着；后续道具动作阶段负责实际运动。
- 攻击variant在本分支不被读取，不能因无关variant越界而阻挡此接触；未恢复的道具发起攻击仍须明确失败。
- 同一交互阶段先前的战斗、声音以及SpecialHolder修改，在后续非法候选出错时须完整回滚。

依据：原bank5 8653→871C→87EA→86FC。先记录自然接触，并用原8653机器码执行显式合成的函数输入作为隔离对照；不写运行中游戏RAM，不把隔离样本冒称自然场景。固定Runtime的seed、clock和目标不变。

道具13隔离验证失败场景（实现前）：

- 入场必须保留朝向、清动画时钟并设下一动作0；横向速度为±10，不能沿用人物跑速。
- 动画、物理、VX==0转idle的顺序不能颠倒；地形边界使用道具47..480范围。
- 入场停止平面及竖向速度并清重力，所有权、附着不能误清；后续帧不得重复初始化。
- 左右运动、零/小速度、边界碰撞、动画循环均需原函数逐帧对照；自然样本缺失必须明示。
- 后续碰撞搜索必须覆盖道具12/13的原始几何与flags；不能沿用idle flags，或把kind12的idle无碰撞形状用于滑动动作。

## 结果与证据（2026-10-16）

- REA 接触分析：`ev_f6e731592066697b42f37583f36d80c55443f8942fdce1468eea33fc8b60aa3e`，bank5 8653/871C/87EA/86FC 均解析成功；保存于 `.local/basketball/rea/prop-contact/analysis/`。
- REA 道具滑动：`ev_e8b2de50ff2fd15a18cf4ff5c4418295bcfef8389e3e6e0b7c430e073df1561f`，bank6 B3EA 与动画、停止速度、物理调用均解析成功；保存于 `.local/basketball/rea/prop-slide/analysis/`。
- `prop-contact-isolated.json`：288 条原机器码函数调用，覆盖四位攻击者、两件道具、整数/小数位置、地面/空中、旧动作6及普通状态。所有状态字段和无音效均匹配。
- `prop-slide-isolated.json`：844 次原机器码 B3EA 连续调用，覆盖入场和已入场、左右朝向、左右边界与场中、零/正/负速度和动画结束。通过完整 MMC3 PRG bank切换执行原物理函数，没有用替代物理生成期望结果。
- `contact-auxiliary-source.json`：从原bank5种类表、flags表和shape指针导出道具12/13两帧几何；kind11与kind12形状分别核对。几何恢复不等于道具主动攻击、携带和抛掷动作已完整恢复。
- 两轮各三场自然采样仍未触发人物→道具接触；当前自然fixture为0条，保留来源及RAM一致性记录。隔离oracle不冒称自然原版对局覆盖。
- 固定seed `0x12345678`的冷启动真实video runtime已越过先前video9638/logic3210阻塞。两次均在11100个video tick、3685个logic step完成首节并进入第二节，比分12:24，分别执行46次保存恢复，轨迹SHA-256均为 `a84d8432711f503313762bca1c355fae3611eab86ca63ab02d29a9ac9ff35496`。时钟、seed、目标帧数及输入策略未改。
- race定向验收：17项接触/道具/首节测试和10项interaction/world preparation测试全部通过，记录于 `.local/basketball/rea/prop-slide/race.json`、`race-integration.json`。这是冷启动集成验证，不是完整比赛、所有玩法或原版逐帧一致的声明。

可重复执行：

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_prop_contact_original.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_prop_slide_original.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_contact_profiles.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestOriginal(PlayerProp|Prop|.*Combat|.*Interaction|.*Contact|Auxiliary)|TestAuxiliary|TestRuntimeColdFirstPeriodAndRestore' -count=1 -json
go test -race ./games/basketball/engine -run '^TestInteractions|^TestWorldPreparation' -count=1 -json
```

夹具的`*-source.json`包含原ROM、输入RAM和输出夹具SHA-256。安装目录、原ROM和自然采样属于开发对照资料；游戏生产代码不读取或执行原ROM。

## 道具反向打人：实现前失败清单

- 道具打人不能套用普通人物攻击variant，Endurance必须保持；冲量固定±4/8，court与depth分支决定VY。
- 人物49动作请求、重力与音效21必须同相位更新；不能提前释放持球、推进动画或改PoseSource。
- 道具action10或>=16没有反冲请求；其他旧action将道具置7，整数X比较决定朝向。
- 旧动作6经87EA重置SpecialHolder及其人物动作；非法holder仍需原子失败。
- 道具打另一道具先修改接收道具再处理攻击道具，不发打人音效。
- kind10存在独立捕获分支；不以普通受击覆盖未恢复的捕获玩法。当前NewYork只开放kind11/12反向路径。
- 后续非法候选必须回滚先前人物/道具动作、冲量与声音；真实HTTP失败冷配置须固定重放，不重抽seed规避。

道具7入场与落地的失败场景（实现前）：保留朝向但清动画时钟；清Owner与Attachment，保留ObjectLink和NextAction；先停全部速度和重力，再设VX±4/VZ10/重力1；仅高度整数非零且depth>=72写VY=-1；object4入场清SpecialHolder，object5不清；动画→物理→落地切idle并停水平速度的顺序不可变。挂接位仅允许未进入7时由原初始化清除，已进入7仍挂接的未知物理不能默默忽略。

## 道具反向接触与动作7证据

- bank5 `876F/8821/87EA` REA证据：`ev_2cdb34db10d8ce617a74f17388eb37edfde2d07fbf2951e4a40489b09270b65c`，位于 `.local/basketball/rea/prop-attack/analysis/`。
- bank6 `B1BD/8726/EEFF` REA证据：`ev_073391e62f9e99d499b5e89f051a8d52d30eaf26056a30716dd6025ee18754ff`，位于同目录的 `rebound-analysis/`。
- 原bank4 `8857/88BA`明确区分人物进入通道与道具反弹：道具7/13翻朝向、请求7并清通道标记。`8249`对负高度保留初始候选地面0；落地仍执行`86AF`侧向校正，不能直接抬升到旧地形缓存64。复用已保存的 `.local/basketball/rea/auxiliary-rest/terrain/` 原分析，并用B1BD连续调用验证。
- `prop-attack-isolated.json.gz`保留828条原代码执行结果：822条受支持的kind11/12人物或道具接触完全匹配，6条双action6的人工输入明确拒绝并验证完整回滚。这6条原函数第二次释放使用已变成255的SpecialHolder，写入`0512+255=0611`，即人物0的Endurance字节；原结果完整保留，当前尚未证明该人工输入在自然原版中可达或不可达，不声明等价支持。
- `prop-rebound-isolated.json.gz`保留1688次原B1BD连续调用，覆盖左右边界、场中、进入/未进入、正/零高度、后台阶与落地；全部动作、动画、物理、Owner、Attachment、SpecialHolder字段逐次核对。
- 固定真实HTTP失败输入保存为 `prop-attack-http-before.json.gz`，原seed为2963483346、before位于video875；输入与来源SHA见同目录`prop-attack-http-source.json`。回归推进剩余24tick并对每一步执行JSON保存恢复一致性检查。
- 对原HTTP报告SHA `41232c67532a926f778e5f2700e92589cd94ab9d0167f357ef9a1d311b18c879`重新执行受限重放：原先额外760tick后失败，现在784tick全部推进至原平台idle deadline之前。没有改变seed、角色、时钟或托管截止时间。完整源报告归档及结果在 `.local/basketball/rea/prop-attack/http-replay/`。此项是实际平台输入的集成复现，不是原ROM自然对局证明。
- 正常包级定向race已通过，完整结果在 `.local/basketball/rea/prop-attack/race.json`，无文件排除。初期曾在并行court模块红阶段执行单独编译诊断，范围记录为`targeted-scope.json`；最终结果采用正常包级验收。
- 原固定冷启动首节及完整比赛回归仍通过，每种各两次。首节11100video/3685logic、比分12:24、恢复46次；完整比赛22383video/7435logic、比分12:45、恢复84次。完整比赛两次轨迹SHA均为 `32c8fa757593a3be5792020bc451e84b434671e7efea109e9d9c157429c8089c`，记录于 `.local/basketball/rea/prop-attack/runtime-regression.log`。这些摘要包含同期canonical状态和显示字段变化，不与旧版摘要直接比较。

复跑新增证据与测试：

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_prop_attack_original.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_prop_rebound_original.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestOriginalProp|TestPropAttack|TestOriginalPlayerProp|TestOriginal.*Combat|TestInteractions|TestAuxiliary|TestOriginalAuxiliary|TestWorldPreparation' -count=1 -json
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/replay_http_runtime.py .local/basketball/rea/prop-attack/http-replay/evidence-41232c67532a926f778e5f2700e92589cd94ab9d0167f357ef9a1d311b18c879.json --output .local/basketball/rea/prop-attack/http-replay-repeat
```

## Court0 kind10推进：实现前失败清单

- kind10与kind11/12有独立形状表，不能只改kind名称或复用NY形状。
- 0..15已恢复action域与30组shape指针边界来自原ROM；未恢复kind/action/frame必须仍明确失败。kind12在此域之后还有骑乘动作，不在本项范围内。
- action0保留旧动画cursor，因此normal geometry应按原shape数组边界导出，不能只凭初始cursor0支持idle。
- kind10 action3可触发专属捕获人物50/道具6；其他kind10普通反向碰撞才走固定抛飞，不能忽略原特例。
- Court0冷局固定0x12345678与完整48000video上限推进，不以换seed、跳过候选或截短时钟验收。原始失败before保留供重放。

Court0 profile结果：

- REA `ev_d30e0e7c55c9c7ee2120b4bac25f7aff5248a168442bec224232465dc366ffda`记录bank5 `8318`与`876F`原选择/分派，位于 `.local/basketball/rea/prop-kind10/analysis/`。
- `auxiliary-geometry-isolated.json`本轮包含252个原8318机器码选出的shape指针及其原形状字节；flags另标记为原ROM源表证据。覆盖kind10/11/12的action0..15域与对应完整shape数组，保留源SHA；不声明所有动作已实现，kind12额外骑乘动作不在本轮域内。
- kind10原idle形状是X[-8,8]、height[1,15]，不同于kind11的[1,39]和kind12的idle禁用形状。导出还覆盖idle保留旧cursor的合法源数组项。
- 原Court0混合固定冷局越过video116首差并完成两次22452video/7458logic，84次恢复，比分15:45，轨迹均为`0b25a8fd3b3b8a9ffa471afcef45fb484948cb956a7d98cd36053bee45f9e32a`；日志`cold-after.log`。同期全电脑运行的新mode7篮筐目标首差与修复见`AI_ENGAGEMENT_VERIFICATION.md`，完整验收状态以最终runtime检查为准。
- 最终同轮Court0验收已包括显示列历史保存恢复修复：混合完整两次保持22452video/7458logic、84次恢复、比分15:45，摘要更新为`fb6c7801f25f251e118a13e47bf69572189bdf23c44a068a50104af421dcfe04`；全电脑完整两次22407video/7443logic、84次恢复、比分52:29，摘要`26eca49b43278bbfa82b2534d573cfb2c6f7779d3ff5ad026e60924296265f98`。来源为 `.local/basketball/engine/runtime-court-zero-after.log`，已带SHA归档于 `.local/basketball/rea/prop-kind10/court-zero-final-runtime.log`。未再暴露新prop分支；kind10独立捕获动作等未验证域继续明确报错，不能由两条固定轨迹外推为所有玩法已恢复。
- 最终30项定向race全部通过，含profile、接触、engagement和world preparation原阶段/原子性检查，日志 `.local/basketball/rea/prop-kind10/race-final.json`。旧Court0负例已由Court模块维护者改为仍未接线的Court2，不绕过原子性断言。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_auxiliary_geometry_original.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_contact_profiles.py
source scripts/env.sh
go test -race ./games/basketball/engine -run '^TestOriginalAuxiliaryGeometryCompleteTables$' -count=1 -json
```

## Court2 kind13：实现前失败清单

- kind13 action map位于8F85，不是kind12地址直接加16；kind12在后方另有动作条目，不得把这部分当kind13起点。
- kind13形状使用group8，特殊动作3使用group29；按原数组边界覆盖全部cursor，不能套用kind11/12尺寸。
- kind13普通反向接触走876F的非kind10路径；人物受击、冲量、Endurance不变和音效必须经原函数对照；不能因未支持kind而无条件吞接触。
- Court2完整motion由独立模块接入，不能把NY地形私有适配换个Court编号使用。新增动作必须由实际固定冷局首差或原执行证据驱动，未恢复分支继续明确失败。

Court2基础证据与范围：

- REA `ev_28ff5df98684db5f3fe58534b0506c5cfc3e41fbad9c8a013ccdc808f97f542a`保存8318/876F/87EA分析于 `.local/basketball/rea/prop-kind13/analysis/`。
- 原8318几何oracle扩充至340条，新增kind13的88条；kind13原普通形状为X[-4,4]、height[1,15]，不同于其他辅助物。现恢复kind10/11/12/13的action0..15域，kind12额外骑乘动作和kind14仍不在此域内。
- 原876F接触oracle扩充至1212条，新增384条kind13发起的普通接触（四个人物、两道具索引、朝向、旧动作、court/depth分支），沿用已恢复的固定冲量与反弹流程；全部受支持记录匹配，仍有6条双释放人工输入明确拒绝。
- `TestAuxiliaryCourtTwoUsesOriginalMotionBinding`先红后绿，验证Court2缺原motion绑定必须报错，有绑定时06DB=7仍作为Court2 switch tick保留并正常平移，不进入NY地道。
- 20项接触/辅助物/交互定向race通过，日志 `.local/basketball/rea/prop-kind13/race.json`。此结果仅覆盖已恢复动作和接触，不替代Court2固定完整冷局验收。
- Court2绑定开放后，原固定seed/48000video门禁真实推进到混合video10280/logic3424、全电脑video13301/logic4418；均未出现新的aux动作阻塞。新首差是人物设备Action62且持球进入未恢复的AI入口，完整前态保存在 `.local/basketball/rea/prop-kind13/court-two-failure-0.json`、`court-two-failure-1.json`，源日志及SHA同目录保留。两者CarriedPlayer=-1、链接均64，不能按旧“carried-player”错误名推断有搭载者。AI入口与人物设备动作交由对应模块恢复；本报告不声明Court2完整比赛已通过。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_auxiliary_geometry_original.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_prop_attack_original.py
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/export_contact_profiles.py
source scripts/env.sh
go test -race ./games/basketball/engine -run 'TestOriginalAuxiliaryGeometryCompleteTables|TestOriginalProp|TestOriginalPlayerProp|TestPropAttack|TestAuxiliary|TestOriginalAuxiliary|TestInteractions' -count=1 -json
```
