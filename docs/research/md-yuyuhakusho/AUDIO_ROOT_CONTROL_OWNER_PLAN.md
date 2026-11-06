# 比赛 Root 声音控制的唯一状态与 API 方案

本轮恢复比赛闭包中的 `01/04/05` 和已有原自然暂停／恢复路径。139 个 fight 候选和 11 首 BGM 的闭包见 [AUDIO_GAME_DISPATCH_COVERAGE.md](AUDIO_GAME_DISPATCH_COVERAGE.md)。[独立来源审计](AUDIO_ROOT_CONTROL_SOURCE_AUDIT.md)已订正早期的 duck 假设：只有闭包外 `7F` 有请求位，已查 grammar 也无 FF／E2。本轮不扩原菜单／duck，不把 isolated score 的硬零字段当作完整 Root 行为。

## 字段归属

| 原字段／语义 | 唯一 owner | 本层使用方式 |
| --- | --- | --- |
| `F800` current priority | 新 Root control state | 原 consumer／priority effect 更新；不另存 score 中。 |
| `F809` pending selected | 新 Root control state | 原 dispatch 在实际读取／清 `FF` 的阶段处理。 |
| `F80A..D` queue | `World.Queue` | 构造临时 `engine.SoundPriorityState` 调纯 helper，提交后丢弃复合 view。 |
| `F807` audio pause | `World.Clock.AudioPause` | 通过 live field 接口读取／写入；Root state 不再保存 pause byte。 |
| `F560..4` delay／delayed code／backlog／busy | 现有 `SoundSchedulerState` | 仅由四阶段 scheduler 管理，不复制到 control。 |
| `F801/2` tempo 与 `F840..FB9F` 18 声部 | score | 所有参数和活动状态变化经 score owner API，不在 control 保存 voice 数组。 |
| `F804` fade steps、`F806` fade remaining | 新 Root control state | 仅保存真实独立计数；后续 interval 3 为语义常量。 |
| `F828` control mode、`F829/A` FM／PSG attenuation delta、`F82B` duck phase | 已审 fight profile 的派生零值 | 不在 production State 保存恒零字段；域外／非 idle 显式 Unsupported。 |
| `F82C` slot-choice bit | 新 Root control state | `SelectPCMCommand` 返回 intent 后提交一次，helper view 不持久保存。 |
| PCM mode／start marker／slot flags／busy／mailbox | `pcmcommands.Device` | 即时 `Status` 或 bus 下的实际读取，control 不建 Z80 镜像。 |
| YM／PSG 当前参数、busy、FIR | chips | 真实端口写和 status 读，control 不建寄存器镜像。 |
| 暂停后等待恢复的 24 个有效 FM TL | 仅 pause 期间的 Root pending-restore payload | 从唯一 chip 临时 view 在已验证捕获点取得，resume 成功后清空；不复制 18 voices 或常驻整个 YM。 |

`SoundPriorityState` 和 `PCMArbitrationView` 现为纯 helper 的临时复合输入，不等于可以在 World、control、PCM 各保存一份 queue／duck／choice。新 root state 不含 `Queue`、`Pause`、`Scheduler`、`PCM` 或 `Voices` 副本。

`F803` 有 E2 写入口，但已审 214 轨／12,775 command（fight 子域 115 轨／8,859 command）均无 E2／FF，因此在清零初态与闭包前提下派生为零，不保存另一份 signal。`F805` 在现 fade 逻辑中没有承担每步 period：01 初始 remaining 为 3、05 为 32，后续直接置 3。不为未经证明的“fade period byte”新增独立状态。FM3／offset 则沿 fight source 无 FE／ED／EE 的既有有界结论，不扩为全 ROM 不可达。

## 拟 State 与事务边界

新 `audio/rootcontrol.State` 只持 `Priority`、`Selected`、`FadeSteps`、`FadeRemaining`、`ChoiceBits`，以及必要时的 `PauseRestore`。`Selected` 初态为 `FF`。不持久化已证明恒零的 control／duck／delta／signal／FM3 字段。字段含义与 pending-restore 必要性由 native oracle 再冻结，尚未写 production。

Root coordinator 先复制一个完整 detached audio transaction，各 owner 在其中各有一份。control 调用 score 变更 API、PCM bus 和 chip 后，只有全部阶段和输出校验成功才一起提交。control 不能自行回滚、重建或替换其他 owner 的隐藏副本。

Root 对外观察由这些真实 owner 合成。score 的旧 `View` 中 control／fade／pause 的硬零只属于历史 isolated scope；新完整 Root view 应从实际 owner 取值，不把 score 的硬零覆盖真实控制状态。

## 必须保留的原语义

`01` 置 steps=40、remaining=3；`05` 置 steps=50、remaining=32。两者都清 DAC music 行全部 typed flags，保留该行 owner 和其他参数，**不发布 PCM stop**。

fade 在 header 的 control→tempo→fade 顺序中推进。remaining 非零时只减 remaining；归零的同一 tick 不立即执行下一步。remaining 原为零时才减 steps；steps 减为零时直接进入 `04` full clear。否则重新置 remaining=3，再逐个修改 active music FM1..6 与 PSG1..3。

FM fade 的 BaseLevel 是 byte 加一后按 signed 判断，负值只清 Active；不是 clamp 到 127。PSG 加一后若达到 16 只清 Active。未达到阈值时使用既有 level／base-attenuation 写，不推进 envelope、gate、pan、modulation、NoteWait 或 tempo。

更宽原程序的 duck apply 与 fade 阈值效果不同，不能借其私有向量简化 fade。不过当前闭包不触发 duck，Root 已确认本轮不增加该 production profile；非 idle 控制保持 Unsupported。

`04` 的 Root bulk clear 包含 priority、selected／queue、pause、fade／control 字段和 score 的 tempo／18 voices，但不触及 Scheduler，也不立即停止已有 PCM。其后有明确的 selected=`FF`、FM keyoff／level mute 与 PSG mute 顺序。多个 bulk store 横跨字段组，必须按原语义阶段通知各 owner，不能只在 tick 末统一清理。

## 与 score owner 的接口

拟由 score owner 增加 `ApplyMusicControlObserved(action, hooks) -> Output`，仅做一次枚举语义动作，不推进 `Ticks`、tempo 或正常 voice tick。最小动作包括：

- `StopMusicDACFlags`：清 DAC 四个 flags，不重置 owner／参数，不触碰 PCM。
- `FadeMusicStep`：按上述原阈值逐槽变更，并提交有序 level／PSG base 写。
- `ResetScore`：按 bulk 清理的原位置清 tempo／voices，与 Root 字段和 chip 清理阶段协调。

score 可给 Root 同事务临时 Snapshot／只读观察，但 control 不持久保存 18 voices。PSG 的立即基准写要与通常 `volume()` 分开，避免淡出／duck 意外推进 VolumeCursor。所有实际 level 参数／最后写入与 claim 抑制仍由 score／chip owner 处理。

## 暂停的 pending level 载荷

不能以当前 Tone＋BaseLevel 重算暂停前的全部实际 TL：primary 覆盖、claimed music、carry-skip、inactive voice 和最后已写值可能不同。方案是在 pause 捕获点，从当前 chip 的临时 TL view 取得 24 个有效硬件 level，仅作为待恢复载荷保存；静音写入后它与当前 chip 的 mute level 有独立语义，resume 成功后释放该载荷。

原 M68K 请求 cache 与 chip 可见 TL 在该捕获点是否逐项相同，必须由原 oracle 检查。忽略位只按已证明的 chip-effective 投影处理，不保存整个原 RAM cache，也不无证据地丢掉合法可见值。pause 不重置 score 的 voice 参数，PSG 恢复行为也不能自行添加。

## 完整 tick 的必要拆分

原正常顺序为 gate、control header、tempo、fade、queue consume、dispatch、voice scan、return。旧 `AdvanceObserved` 同时做 tempo／dispatch／voices，不能让 Root 在外面先推进一次 tempo，再调用它重复推进。

需由 score owner 提供有序阶段 transaction 或等价的独立阶段入口，保持同一 working score 和唯一 tick 计数。Root controls 在正确阶段调用局部 mutation API，score dispatch／voice scan 仍由 score 执行。若跨参考 frame 或外部反馈等待，需要显式持有一个有界语义阶段，不使用 CPU／PC／RAM VM 或已观察耗时表。

本方案只解释谁拥有状态、何时应调用，不表示全函数成本或异步 bus／IRQ 组合已完成。现有 leaf 成本、真实 bus retry 和 full-chip 证明继续保持各自冻结身份。

## 下一步原向量

先复用自然 pause 的 3,602 步控制与 54 个 IO 窗口。新的 native 矩阵包括 01／05 从实际音乐开始到 fade 完成、04 在普通／mixed／claimed／PCM 活动状态下清理、fade boundary（remaining1→0、steps1→0、FM7F／FF、PSG0F／FF）、以及暂停／恢复捕获点的 24 TL／cache 对照和 checkpoint 延续。7F 私有 duck 不计入当前 fight 必需矩阵。

所有向量先列失败场景、再取得缺接口 red，然后实现。full-chip owner 已明确释放 23 组原采窗口，后续采集仍使用 nice 单进程，并避开 Root 浏览器性能窗口。Root 的 F50E／F516／VDP 硬件与 world→IRQ 状态不属于本 control owner。
