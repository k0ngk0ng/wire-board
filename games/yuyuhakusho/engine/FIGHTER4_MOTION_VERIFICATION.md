# 飞影 MAIN 动作验证

`StepFighter4Motion` 已通过完整原版 CPU 字节对照、原生竞态和 Node-WebAssembly 检查。原 `8400` 四测试夹具有 397 次自然调用；另有真实 `9000` 双真人场景的 `2a` 102 次、`2b` 57 次自然返回，补齐全部十个动作的自然来源。双真人 Go 验收使用独立的三层组合 E2E，原四项测试及原夹具保持不变。具体证据见[双真人吞龙记录](FIGHTER4_MOTION_DUO_EVIDENCE.md)。本轮通过不表示完整游戏或对局已经验收。

## 实现前验证范围

范围为 `25/26/27/28/29/2a/2b/2c` 及共用 `0e/12`，截至原版公共尾处理入口 `15afc`。入口依次是 `1b1e8/1b21e/1b246/1b2bc/1b2d4/1b3f0/1b48a/1b51a`，共用入口为 `1ca46/1cb68`；`1b1ce` 属于 `24`，不列入本任务。只恢复动作及其当次子对象／FX 创建，不执行子对象、FX 生命周期、公共尾处理或世界调度。

拟用 `Fighter4MotionState` 嵌入 `ChuSpecialMotionState`，另带 `MovementSource`、`DragonBankCursor`、`ChildExtras[3]` 和既有 `EffectExtras[16]ReactionEffectExtra`。`MovementSource` 只是现有 WorldObject `+90.long` 的临时视图；不新增强化模式或重复移动参数。`ChildExtras` 只暴露既有 child Aux48 的完整 long、Held／Pressed 低 byte、ContactFlags，以及唯一新增 `+FE.word` DragonBank 的视图。`DragonBankCursor` 对应全局 `A038.word`；MAIN 不拥有历史缓冲。正式类型名与 child peer、Root 统一，不复制 canonical 字段。API 为 `StepFighter4Motion(state, JumpParameters) (state, error)`，参数必须来自当步当前 MovementSource。`19092/191ba` 使用 selector 任务唯一提供的 `FinishFighter4Rush`／`BeginFighter4DragonFollowup`。

在原版取证、测试和实现之前，列出以下失败场景：

- 来源身份、真实参数／动画／形状指针或初始化错配；普通输入没有自然调用时假报覆盖，或把自然活动片段写成完整生命周期。
- `0e` 的地面高 word 相等、ticks 0 尘土门槛、Facing.bit0 和完成值恰为 1 错误；`12` 的独立八项表被误套为 `11` 六项表，移动帧 1／2／4／5、后退方向或完成恢复错误。
- `25` 活动时未直接加存储 VX，误按朝向再次反转；完成后 X 高 word 的正负 64 偏移、Facing.bit0、清完成与 `19092` 顺序错误，或误清 Request。
- `26/27` 的七项／十一项帧表边界、快慢段与空操作帧错误；`1b282` 先把 ticks 写入 D0 低 byte 并只掩码该 byte，导致位移小数改变；调用 `20202` 后 allocator 对 D0 的真实破坏被忽略。空池、满池、ticks 的四种余数均须原 CPU 直接确认，不能根据常量指令猜位移。存储 VX 不应被位移覆盖。
- `28` 完成时先 ExtraJump 加一、随后 `1860c` 清零的顺序被简化成保留加一；完成值不是 1 时误恢复；默认／强化 Gravity 取错，或空中恢复误依赖 Y 与地面相等。
- `29` 完成优先级、`191ba` 清 Request 或 frame 0 每 16 ticks 三 FX 的门槛错误；三次独立分配被合并、Phase 0／4／8 对应 `+5a`、朝向与 X 高 word 的正负 16 偏移错误；FX 未写字段被清零，满池被当作错误。
- `29` 在 frame 2／ticks 0 只用第一保留子槽；忙碌时误写游标／字段，创建时遗漏完整 Ground、Aux48 方向 long、Held 低 byte、TargetBit byte 回绕、原属性掩码或坐标高 word 偏移；误写未被原版初始化的 VX／VY／Pose／Pressed。`A038` 必须先 word 加 `200` 再与 `7ff`，仅成功创建时更新，并复制到 child FE；不得新增另一方向 long 或历史游标。
- `2a` 每次都先复制 child Held／Pressed 低 byte，误覆盖扩展 byte；有符号 word 距离、NEG(-32768) 回绕、水平 24／垂直 32 门槛、近距离优先于 child.Size／取消按钮错误；近距离以外的存活／C 取消分支错误。吞龙先用当前原版 `203f8` 创建跟随 FX，再设 Energy 高 word 为 223、置 child ContactFlags.bit4、改姿势／动作及 MovementSource `15946`；满池仍须吞龙，且保留 Energy 小数、其他 contact bits 和未被写入的 EnabledStages。诊断错误必须回滚先前复制及全部变更。
- `2b` 只在 frame 1／ticks 0、第一子槽空闲时创建 `13fc8`；原版未写的坐标、Ground、Facing、方向／历史字段及速度必须保留。完成时恢复站立，不把创建或强化参数重置到默认表。
- `2c` 完成优先于方向／重力／落地，ExtraJump byte 加一回绕；当前参数 VY 分别算术右移一位、两位再相加，Facing 恰为 1 才保留水平参数正号；结果为 `02/AF`、EnabledStages 0。活动中 Right 优先 Left、加速后有符号 clamp、32 位回绕、先更新 VY 再加 Y、下落才判着地的顺序错误；`A8` 姿势的有符号 word 高度门槛与 B／A 取消、其他姿势 contact.bit2／3 跳帧、VY `a0000`、落地 `03/A9` 的优先级错误。
- 改 MovementSource 后仍使用默认移动参数；把不可变参数表复制进可变模式字段；历史缓冲在 MAIN 被误写；现有 FX Owner／OffsetY／Phase、子对象 FE／Aux48 与世界 canonical owner 串用或重复保存。
- 父 256 字节、三子 768 字节、FX 2,048 字节、游标、声音请求／队列或 RNG 漏比较；私有 SP 或 refresh cycles 未恢复；全 64 KiB RAM 审计出现未解释写入；只用逐条期望状态灌入代替单初态的真实动画时钟流程。
- API 对未恢复槽位、活动有限表越界及未对齐 FX 游标应报告诊断错误，并保持完整输入状态；错误不能泄露 child 输入复制、位移、游标、FX 或 MovementSource。原版正常忙槽／满池不应报错。

验证顺序为：失败清单 → 独立原版 CPU 普通／私有／真实动画时钟证据 → 冻结完整测试并保留真实缺实现 red → 只新增 MAIN 生产文件 → 同范围原生竞态与 Node-WebAssembly。普通原版基线由 selector 任务独立生成，观察器流程逐视频步核对全部 RAM 与画面。私有调用使用 canonical `A5=FFFF8400`、`SP=FFFFFF00`，每次恢复原版检查点及 refresh cycles，审计完整 RAM 和 SP。连续流程只刷新当前 pose／index 选择的不可变 Duration／Flags，不回灌期望中间状态。静态入口证据来自已有直接 Ghidra 和新 `fighter4-main-helpers.txt`；REA 不支持 raw 68000，不声称 REA 反编译或模拟执行。

## 8400 四测试夹具结果

| 动作 | 自然调用 | 私有边界调用 | 实际时钟流中的调用 |
| --- | ---: | ---: | ---: |
| `0e` | 6 | 72 | 132 |
| `12` | 39 | 384 | 156 |
| `25` | 23 | 180 | 46 |
| `26` | 43 | 361 | 86 |
| `27` | 66 | 553 | 138 |
| `28` | 58 | 64 | 120 |
| `29` | 55 | 296 | 220 |
| `2a` | 1 | 432 | 700 |
| `2b` | 0 | 240 | 456 |
| `2c` | 106 | 1,236 | 1,170 |
| 合计 | 397 | 3,818 | 3,224 |

普通输入 `source4-next-inputs.json` 的 SHA 为 `e00525d883a103b4631332ac793b2569ff0ee384134e8fd58407bbc4123d0352`，提供 45 次 `0e/12` 调用。最终特殊技输入 `source4-special-discovery-inputs.json` 的 SHA 为 `5ba6b13d2bb0b285c4307981a5e903d75a7936de3d2632a9f1bf449189650424`，只在八个 MAIN 动作范围内新增 352 次调用。两个观察器冷启流程各核对 3,602 个视频步，完整 RAM／画面均与 selector 任务先行生成的唯一未观察原版参考一致；追加没有另采原版 control。

自然 `25` 在 1,875 至 1,897 返回 `03/A1`，`26` 在 2,000 至 2,042 返回站立。`27` 在 2,052 至 2,181 之间分五个阶段调用窗口，合计 66 次并返回站立；视频帧内保留动作状态不等于该帧执行 MAIN。两个 `28` 流程分别有 17／41 次调用，均恢复 `02/07`。`29` 在 3,435 至 3,489 共 55 次调用后进入 `2a/A7`；`2a` 仅在 3,490 执行一次，返回 `03/AA`，参数源仍为 `12578`，没有吞龙。`2c` 第一段 2,191 至 2,230 共 40 次，末帧仍为 `2c/A8`；第二段 2,389 至 2,454 共 66 次，已落地为 `03/A9`。这份 `8400` 夹具没有吞龙强化及 `2b`；对应缺口由独立 `9000` 场景补齐，没有混入旧数据。

这次只更新自然 oracle。原有 45 次普通样本及只读动画库的 JSON 投影逐字节保持不变，3,818 次私有边界、40 条连续流程和两条移动参数记录均保持原文件字节。追加来源、原版 control、阶段窗口及旧投影 SHA 见 `fighter4-motion-natural-append.json` 和 oracle manifest。

40 条连续流程均只提供一次初始状态，之后执行原版 `19c96` 时钟与当前动作。只从自身动画集、姿势和帧索引选择 121 条实际只读记录，刷新 Duration／Flags。流程覆盖空池／满池、前后冲与空中恢复、短长冲刺、两个 `28` 姿势、黑龙创建后取消、既有近距离子对象吞龙、空子槽吞龙、强化子对象创建、空中着地／取消／接触，以及默认／强化参数的 `2c` 动画完成。终点为 16 条站立、12 条空中、12 条 `03` 动作；10 条保留 `15946` 参数源。既有子对象是显式私有初态，流程未执行子对象或历史段回调。

每次完整比对父对象 256 字节、三个子对象 768 字节、FX 池 2,048 字节、FX 游标、DragonBankCursor 和逻辑字段；没有屏蔽字节。私有调用的完整 64 KiB RAM 审计只允许父／子、FX、声音请求／两个游标和 `FEC0..FF04` 栈范围变化，未发现未解释写入；`FF7000..77FF` 历史缓冲保持原样。调用返回时 SP 一致，所有检查点恢复包含 refresh cycles。MAIN 的 RNG 消耗、声音请求变更及未解释队列写入均为 0。

## 关键语义

`26/27` 的原版寄存器记录确认：ticks 的低两位不为 0 时，位移为 `40000/20000` 常量与余数 1／2／3 的按位 OR；为 0 时，`20202→1fa96` 将 D0 改为 `149ac`，随后直接用于位移，满池时亦同。实现保留该行为，不把它修整为理想化常速，且不覆盖存储 VX。

`29` 仅在成功创建第一保留子对象后，把全局 `A038.word` 加 `200` 再与 `7ff`，复制到 child DragonBank；只写原版初始化的字段。`2a` 每次先复制 child 输入低 byte，近距离吞龙分支先分配 `203f8` 跟随 FX，再恢复 Energy 高 word、置 contact.bit4 并切换 `MovementSource=15946`；满池仍完成吞龙。`2b` 沿用这个参数源，并保留原版未初始化的子对象坐标／速度／方向等字段。

默认 `12578` 和强化 `15946` 的原版 16 字节记录保存在 `fighter4-motion-parameters.json`。真实时钟测试按自身当前 MovementSource 查不可变记录，不逐步灌入期望参数。`2c` 完成时使用对应 VY 的算术半值加四分之一值、对应 Gravity 和水平速度；`28` 则遵循原版先增加 ExtraJump、随后由 `1860c` 清零的顺序。

## 冻结与复跑

失败清单、原版证据和四项完整测试先于 MAIN 类型及实现。实际缺符号 red 明示 `Fighter4ChildExtra`、`Fighter4MotionState`、`StepFighter4Motion` 不存在；最初失败清单全文与 SHA 保存在 `fighter4-motion-test-first.json`。测试 SHA 固定为 `d4ce7a8d890cce355f6c8dfb95ff415de02e2b5f0a8d39afa682a2a080c3447d`，生产 SHA 为 `a236eac8bded286b9b35241c206d0de35e763d5097651862f14476f7f787f154`。首次可执行检查四项全过：原生竞态检查 13.852 秒，Node-WebAssembly 3.586 秒，原始绿色日志另存为 `fighter4-motion-ordinary-native.jsonl`／`fighter4-motion-ordinary-wasm.jsonl`。追加自然样本后，同四项冻结测试再次全部通过：原生竞态检查 14.452 秒，Node-WebAssembly 3.706 秒。Go、规则及测试断言均未修改。

在工作树根目录复跑，沿用工作树内的缓存和临时目录：

```sh
export GOCACHE="$PWD/.local/go-cache"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
export TMPDIR="$PWD/.local/md-yuyuhakusho/tmp"
go test -race -json -count=1 -run '^TestFighter4Motion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm go test -json -count=1 -run '^TestFighter4Motion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter4_motion.py \
  --out .local/md-yuyuhakusho/fighter4-motion-recapture
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter4_motion.py \
  --out .local/md-yuyuhakusho/fighter4-motion-natural-recapture \
  --append-input docs/research/md-yuyuhakusho/source4-special-discovery-inputs.json
```

原版采集使用现有本地 ROM、Genesis Plus GX 对照核心和 hook；身份、输入／参考及原版参数记录的 SHA 绑定见夹具 manifest。仅自然追加模式读取 manifest 声明的输入／control，并只输出 oracle，不重采私有边界或时钟流。原版取证时的采集器源码另存 `fighter4-motion-capture-before-append.py`，与未变的边界／时钟 manifest 绑定；当前采集器额外提供上述追加模式。最终来源、夹具、red 和两平台日志集中在 `fighter4-motion-validation.json`。
