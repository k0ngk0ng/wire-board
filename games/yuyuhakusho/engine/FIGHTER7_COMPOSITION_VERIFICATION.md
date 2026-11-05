# 来源 7「树」完整组合验收

完整 Input、Action、Actor 三层的六项组合测试已通过 native race（104.800 秒）和 Node/Wasm（33.494 秒）。最终测试 SHA-256 为 `c449c984ccbdadaf4bce67b6abd0660ffc00c502b693596c4285d46dca2627c6`。12 份规范夹具保留 19008 次完整原 CPU 返回；普通真人、特殊真人和私有实验分别留证。范围截止完整人物 API，不包含子回调、碰撞管理器或世界验收。

## 实现前失败场景

- 将已有全角色 AI 输入误认为来源 7 的完整动作和人物已经恢复；只改 source tag，仍使用其他人物的参数、动画或形状。
- `0f/11/13/14/45` 公共执行或 `39..3d` 专属 MAIN 被拒绝、分派错误、调用顺序错误；实际动画时钟、hit-stop 输入准备或 tail selector 顺序不符。
- selector 的地面／空中、C→B→A 优先级、按键变化、子槽占用、能量不足和弱生命门槛错误；完整 Energy long 小数、Power、接触位或构造状态丢失。
- `39` 将已经带方向的 VX 再次镜像；`3a` 未按 Aux48 最高 byte 选相机位置，或清掉 X 小数。
- `3b` 两个子槽没有独立尝试、分配顺序错误，或未对 Attributes 执行 OR `6000`；`3c` 未逐次减完整 Energy long 的 `4000`，或向未命中的子对象写入 Held／Pressed。
- `3d` 漏写四个父体中的异队 HitStopTicks；错误增加 Size／Health 过滤；把 `+69.byte` 与 `+84.word` 混同，或将 self 重复放进 peer map。
- 参数元数据没有按当前 `+90` 读取；encoder 保留旧 raw bytes，掩盖状态字段没有写回；Action 与 Actor 的临时视图丢失 canonical 完整运动、资源或子对象输入。
- disabled、human、AI、human-record、AI-record、playback、playback-stopped 路由错误；非 AI 输入消耗 RNG；共享 Envelope 与 AI controls 分裂，记录或回放漏写。
- 暂停／慢速门槛、注册、bounds、HUD、draw 或其他事件错误；AI／记录／动作已经执行后的晚期错误泄露状态或事件；失败修改调用者切片或 map。
- 非 canonical A5、错误栈返回或恢复遗漏 refresh cycles 造成伪差异；完整调用未到真实返回便截断。
- 只比较当前 family，漏掉其他三个父体或其子槽写入；只审计已知字段，漏掉完整 64 KiB RAM 的未知写入；把 IRQ、音频消费者或堆栈变化整体遮蔽。
- 未观察到音频消费者 PC、地址和实际 delta 便忽略队列差异；带 hook 的回放与唯一无观察器原 core 控制组不一致仍输出通过。
- 连续流中途回灌期望动画时钟、AN、父体、子体、FX、资源、history 或 RNG；将私有初态宣称为自然真人输入。
- 缺少命令／动画、输入记录空间不足、HUD／注册缓冲不足等早期或晚期失败，没有返回原始完整状态和空事件。

## 先行合同

冻结六项 wrapper：完整原 CPU Action、Input、Actor；单初态 Action 连续流；单初态 Input／Actor 连续流；完整 API 原子错误。测试与采样不调用 Go 实现来生成期望值。所有连续流只初始化一次，后续只交付已声明的外部控制器输入与暂停／慢速值，规则、动画和对象状态由原 CPU 或被测实现自行推进。

Root 确认的最小通道为 `TailFighterEnergy.HitStopTicks uint8` 和 `ChuActorWorld.OtherHitStopTicks map[uint16]uint8`。Action 复用 `OtherFighters` 同一 peer view，始终包含另外三个固定父体，覆盖 empty／dead；self 只由已有 `Action.HitStopTicks` 表示。MAIN 的两个子输入复用已有 `ChildExtras[3]Fighter4ChildExtra.Held/Pressed`。唯一物理 `+FE.word` 保持现有 canonical owner，不新增来源 7 的重复解释。

先保存缺字段 compile red；Root 只补通道后，使用普通真人三层原 CPU 证据取得真实 `unsupported7` runtime red，再开启规则接入。special 和 private 数据后补，不改冻结断言。唯一无观察器 3602 帧控制组由 Root 生成，本采样器只复用，不另采第二份。

## 当前证据边界

完整 Actor 截止 `0x126ba`，位于 sprite emission 前；不调度子对象、碰撞管理器或世界循环。子回调 `136e6/1391e/15094` 由独立 owner 恢复，组合层只核验 MAIN 的创建和输入传播。原 CPU 回调与 Go/Wasm 最终实现分离，最终独立包不携带 ROM、core 或 CPU VM。

## 冻结与普通真人 red

六项测试源码冻结 SHA-256 为 `7e1477727855f72ab03efd31bc6350d5399def191c0487d936b3daeab8083f29`。`testdata/fighter7-composition-schema-red.jsonl` 保存 `TailFighterEnergy.HitStopTicks` 和 `ChuActorWorld.OtherHitStopTicks` 尚不存在时的真实 compile red；`fighter7-composition-test-first.json` 绑定当时源码、失败清单和共享 Go 文件。Root 其后只增加通道，继续拒绝完整来源 7。

普通输入复用 Root 的 `source7-next-inputs.json`（SHA-256 `fed38aa84737d5d17bdf7d02f78f109ec0836f1cb244de6e2def47a8b9fc904f`）和 `fighter7-attacks-reference-frames.json`（SHA-256 `ffebd6741b4a5a44a76832fc658045e144fd3607fec5fefd563a01be1809a65d`）。带观察器的 3602 帧 RAM／画面摘要与唯一原 core 控制组完全相同；只接受视频标签 3600 及以前的入口，额外 2 帧用于收完返回。完整 Action、Input、Actor 各取得 2067 次真实返回，A5 均为 `FFFF8400`。完整 Input 的真人路由 1794 次、disabled 273 次，AI 调用和 RNG 消耗均为 0。

三层每次完整返回均比较全部 64 KiB RAM，保留全部地址的 before／after delta、栈区、IRQ 的逐段变化及音频 clear 的 PC／地址／原值／实际清零值，未知写入为 0。Action 另保存 `8400..93ff` 全四 family 的前后原始 bytes，避免仅比较当前 family 掩盖跨人物副作用。自然音频消费者涉及 Action 41 次、Input 3 次、Actor 51 次；冻结测试只允许实际观察到的地址和值解释队列差异。

schema-only 阶段的实际运行保存在 `fighter7-composition-runtime-red.jsonl`：完整 Action 的首个普通调用返回 `unsupported complete fighter action source`，完整 Actor 返回 `unsupported complete fighter actor source`；完整 Input 的 2067 次普通调用比较通过。special 和私有／连续流当时尚缺，缺失失败原样保留。

同期包级构建被尚未实现隔离 API 的 `fighter7_motion_test.go` 阻断，原始日志另存 `fighter7-composition-runtime-package-build-red.jsonl`，不将编译阻断称为 runtime red。实际 runtime red 使用 engine 目录显式 Go 文件列表，包含全部生产文件、旧 helper 和六项冻结 wrapper，只排除同期的新 motion／projectile 隔离测试；命令、排除列表和文件 SHA 见 `fighter7-composition-runtime-red-files.json`。

## 私有边界与单初态连续流

最终私有原 CPU 取得完整 Action 253 次、Input 35 次、Actor 47 次返回，覆盖 24 个动作槽、四个固定 A5、七种输入路由及暂停／慢速门槛。Action 中包含 53 个完整 tail selector 场景，核对 GS／AS 成功、能量不足、按键优先级、主子槽占用、弱生命与 Down 门槛，以及 `0f/11/13` 构造。四个父体分别作为调用者，原 CPU 已核对 `3d` 对异队 empty／dead peer 的 `+69=ff` 写入和同队保留。

独立连续流包含 Action 17 流／590 步、Input 3 流／48 步、Actor 4 流／96 步。全部流只初始化一次，后续不注入期望时钟、AN、父体、子体、资源或 RNG。Action 覆盖 `39→03`、`3a→03`、`3b→3c→03`、`3d→00` 和共享动作退出；完整 Actor 流保留实际输入、记录和回放的连续状态。

三层私有返回和全部连续流均完成 64 KiB 审计，未知写入为 0，脏填的 `7000..77ff` 未变。最终 private 采样再次重放同一普通控制组，三份普通 gzip 与先行 ordinary 产物逐字节一致。`fighter7-composition-producer-reproducibility.json` 记录 9 份当前规范夹具的 raw／gzip SHA、7270 次完整返回、IRQ 和音频消费者计数。

一次新增采样守卫将 `Held.Right + A` 的期望误写为 `13/87`；原 CPU 实际进入 `03/83`。已保留 `fighter7-composition-capture-constructor-initial-state-failure.jsonl`，并将该声明场景改为投技，另加正确的 `Down+Right + A→13/87`。这只修正新增采样场景的初态期望，六项冻结 Go 断言和自然控制组均未改。

在完整来源 7 规则尚未开放时，经 Root 确认，采样将首个 ai-record Actor 初态加强为 `3d/aa`、frame 2、ticks `ff`，保留七种 route 和全部冻结断言。新的真实原 CPU 完整返回执行 AI 1 次、record 1 次、RNG 5 次，并将 peer `8800/8c00/9000` 的 `+69` 分别从 `d5/f2/0f` 写为 `ff`。冻结的晚期 HUD 错误 wrapper 会选择这条调用，在 AI、记录与跨父体 map 写入后触发缺少 HUD 数据的失败。证据见 `fighter7-composition-late-hud-cpu-evidence.json`；原 private v1 的 6 份 gzip、6 份 manifest、当时场景源码及 SHA 保存在 `fighter7-composition-private-v1/` 和 `fighter7-composition-private-v1-lineage.json`。规范 private v2 仍为 Action 253 次、Input 35 次、Actor 47 次返回。

## 特殊真人输入

特殊流程复用 Root 的唯一无观察器原 core 5502 帧控制组，入口截止 5500，额外 2 帧收完调用。输入 SHA-256 为 `5354fe1e66a6875483398763ce7644c363682e864537f827786fd42b0fab05fd`，控制组输出 SHA-256 为 `98a17dffa4906b63a6ad2e309c7cfb402ca38e0a46fdfce7a5b0bd81456d55dd`。带 hook 的冷重放与全部 RAM／画面摘要完全相同。旧 ordinary producer 已按原 SHA `611f37123518ca994806f298ec9d4ec5e1b8e5b14ebe558dbd7bb583371fe890` 保留为 `capture_fighter7_composition_ordinary_v1.py`；private 和特殊 producer 的旧源码也已归档，见 `fighter7-composition-producer-lineage.json`。

特殊 Action、Input 各取得 3886 次返回，Actor 3966 次，包含 80 次暂停／慢速导致的不活跃调用。特殊 Input 为 human 3430 次、disabled 456 次，AI 调用及 RNG 消耗为 0。自然 MAIN 覆盖 `39=61`、`3a=228`、`3b=39`、`3c=118`、`3d=47`；另有共享 `45=62`。私有连续流和边界保留完整公共支持与 `13/87` 构造证据。

## 音频验收适配器修正

首轮正式 native race 和 Wasm 中，四项 wrapper 已通过，Action／Actor 在同一特殊调用失败：视频标签 3142，动作 `0d`，API 入口队列为 `[a2,0,0,0]`。原 CPU 的 IRQ 消费者 `37c56` 将 `f80a` 的 `a2` 清零，随后动画声音 `a6` 进入该空槽，CPU 返回 `[a6,0,0,0]`。同步独立 Go 调用保留旧 `a2`，将 `a6` 放入下一个空槽，结果为 `[a2,a6,0,0]`。原验收适配器只允许被消费槽最终变为零，无法表达清槽后重新入队；这是适配器边界错误，没有业务规则变更。

经 Root 明确确认，保留 v1 测试完整源码、SHA、首轮真实失败和全部 12 份旧 gzip／manifest，另冻结仅修正音频投影的 v2。新增原 CPU 有序事件记录实际 `21960` 请求、`2196c/2197e/21990/219a2` 写槽、`219a8` 满队列丢弃，以及 `37c56/37cac` 外部清槽；实际写入后的 RAM 值在下一观察点逐项核验。全部物理事件重放必须严格等于 CPU 返回的四个槽；仅去掉外部 clear 后，按已观察请求形成的同步队列必须严格等于 Go 返回的四个槽。没有全队列遮蔽，也没有注入期望时钟、AN、人物、子对象、资源或 RNG。

v2 测试 SHA 为首段所列值。`fighter7-composition-v2-missing-audio-events-red.jsonl` 保留旧夹具缺少精确事件时的真实 guard red。两条流程仍复用同一原 core 控制组重新冷采；逐份剥除仅新增的 `audio_before/audio_after/audio_events` 后，全部 v1 library、调用、状态、原始 bytes、clock、AN、人物、子对象和 RAM 审计字段完全相同。逐 API 事件记录共包含 468 条 request、310 条 write、158 条明确 drop 和 1420 条 clear；嵌套 API 各自留存其观察到的事件，这些数量不表示互不重叠的硬件事件。

完整证明与可重复执行审计分别在 `fighter7-composition-audio-projection-verification.json` 和 `fighter7-composition-audio-audit-source.py.txt`。v1 及 v2 冻结资料见 `fighter7-composition-v1-lineage.json`、`fighter7-composition-v2-test-first.json`；旧夹具保存在 `fighter7-composition-v1/`。晚期 HUD 原子错误函数与原 v1 源码逐字节相同，并在最终两种运行时通过实际 `3d` 跨 peer 写入后的回滚验证。

## 最终验收与复现

`fighter7-composition-native.jsonl` 和 `fighter7-composition-wasm.jsonl` 各有六项通过，没有失败。Wasm 使用最小环境和短定向正则，满足 8 KiB 启动参数／环境区域限制。最终运行期间，全部 engine Go 文件保持相同 SHA；音频适配器修正期间 Root 对 Action／Actor 的两处变化仅为来源覆盖注释更新，另有明确记录。

所有产物 SHA 和范围见 `fighter7-composition-validation.json`、`fighter7-composition-capture-integrity.json`。完整采样、审计及 native／Wasm 命令见 `fighter7-composition-final-commands.json`。在 worktree 根目录执行以下命令，可仅用已保留夹具重复核验四槽物理事件和全部 v1 非音频字段：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 games/yuyuhakusho/engine/testdata/fighter7-composition-audio-audit-source.py.txt
```

本任务只新增组合测试、采样器、文档和证据，没有修改 production、旧 helper、world、bundle、UI 或 PROGRESS，也没有创建提交。独立 Go/Wasm 验证不加载 ROM、原 core 或 CPU VM。采样中间目录的清理范围和保留产物见 `fighter7-composition-cleanup.json`。
