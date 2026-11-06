# 世界中断入口与 PCM 传输等待

世界中断入口现在使用同一主时钟、扫描线状态和实际混音器推进。入口先取得后台总线、写入采样传输等待，再按真实暂停状态决定是否释放总线；随后完成临时数据保存、行计数等待和显示关闭。`session.BeginBattleWorldIRQ` 只在入口全部成功后提交，返回 `AwaitingWorld=true`，后续世界包装、DMA、逻辑更新与中断返回仍须继续处理。

## 唯一状态与顺序

`pcmcommands.Globals.BankBlocked` 保存实际传输等待，`Work.BankBlockedRead` 只保留播放器在上次读取时采样的值。它们分别表示当前值和已经发生的读取，不能合并。等待会影响预装与补充采样的特定分支，并不暂停所有缓冲输出。复位按原位置清零；当前标志和采样值都支持分叉、JSON 存档与恢复。

总线读改写先保留旧读值，再写请求，最后用旧值决定是否重复。暂停确认值为 FF 时保留实际总线。持有总线的播放器可以停留在早先的扫描线；没有实际持有总线时，入口继续拒绝落后的后台进度。原采样 IRQ 的校验没有放宽。

借用的主时钟、VDP、pin、mask、IRQ 阶段、世界包装标志和 pause 不重复存入音频存档。入口与音频通过同一个实际 PCM owner 操作；预算不足或外层世界事务后续失败时，不提交门控、总线、合成器或世界变化。运行时没有原 CPU、PC、原 RAM 或事件时间表。

## 对照范围

- PCM 等待取证使用两个固定核心，各有 8 组流程，覆盖 single、dual A／B 的预装、补充与反复短切换。312 帧、78,546 个完整输出事件及全部 PCM 字节、DAC DSP、总线取得／恢复与门控状态一致。Go 测试在每帧执行 JSON 恢复。
- 世界入口保留 15 个自然案例和 8 个声明了帧头设置的私有案例，共 23 个实际入口。逐项比较 4,013 个 CPU 工作边界、994 次 I/O、refresh、扫描线、IRQ 上下文及世界标志。观察器开启后，原有 2,400 帧视频／RAM／PCM 对照全部相同；私有整帧关闭／开启观察器结果相同。
- 私有设置在视频帧头施加，前面的采样和声音流程可能改写它。正式预期使用入口实际观察值：这批入口的 DisplayReady 均为 true，pause=FF 的入口已持有总线。不能把帧头设置的组合数量当成入口所有分支的原生覆盖。
- 独立的实际混音器事务测试使用明确给定的主时钟／VDP 入口，覆盖显示就绪与否、pause 0／FF、入口总线持有与否的全部 8 种组合，检查门控之后预算失败、完整状态恢复和外层世界失败回滚。它验证生产 owner 接入，不声称这些给定入口来自新建比赛的独立调度。

世界原生入口取证与离线导出使用真实后台反馈作为外部输入；PCM 本身的独立行为由另一组完整原生流验证。两类证明保持各自边界，不把观测输入当成从头计算的整局时钟。

| 受影响范围 | 顶层测试 | 含子项记录 | 结果 |
| --- | ---: | ---: | --- |
| 原生竞态回归 | 34 | 368 | 通过，129.67 秒 |
| 实际 Node／Wasm 回归 | 34 | 368 | 通过，102.01 秒 |

两端通过名称集合相同，未跳过测试。回归包含旧 PCM 连续流、控制码、物理端口、总线、完整混音器、采样 IRQ、完整声音调用和中断返回。vet 通过；本次未重跑未修改的其他游戏、全世界规则矩阵或浏览器性能矩阵。

## 测试先行与失败记录

[失败范围](FAILURES.md)、测试、原生预期及采集／导出源文件在相应实现前冻结，见 [PCM 冻结](pcm-test-first.json) 与 [世界入口冻结](world-test-first.json)。缺 API 的红灯保留。

PCM 初次运行发现 single 跳过分支将 INC IY 误计为 6 周期，以及等待分支将未跳转 JR 误计为 8 周期。生产成本分别改为 10 和 7，原生预期及冻结测试未改动，随后全部通过。

新的实际混音器测试适配器最初把可变参数 `RunUntil` 直接用于固定参数接口。仅补充显式委托方法与 import，没有改动断言或预期。原测试、准确差异、编译失败日志和当前摘要保存在[适配修正记录](world-runtime-test-adapter.json)。原有全部测试与原生预期保持原样。

## 复跑

在仓库根目录执行，`--out` 使用尚不存在的仓库内目录。验证器会把 Go 缓存、模块、遥测和临时文件限制在当前工作区。

```sh
python3 -B scripts/yuyuhakusho/verify_battle_world_entry.py --mode native-race --out .local/md-yuyuhakusho/world-entry-recheck-race
python3 -B scripts/yuyuhakusho/verify_battle_world_entry.py --mode wasm --out .local/md-yuyuhakusho/world-entry-recheck-wasm
python3 -B scripts/yuyuhakusho/verify_battle_world_entry.py --mode vet --out .local/md-yuyuhakusho/world-entry-recheck-vet
python3 -B scripts/yuyuhakusho/verify_battle_world_entry.py --audit
```

前三个命令应返回退出码 0。最后一个命令核对当前源码、测试、证据与日志摘要；前序验证清单保留为历史记录，不覆盖其原摘要。

重新采集使用现有固定 ROM 与核心，路径和摘要记录在 capture 文件中。离线导出可直接读取正式保留的世界原生 fixture：

```sh
python3 -B scripts/yuyuhakusho/capture_pcm_transfer_gate.py --out .local/md-yuyuhakusho/gate-recapture
python3 -B scripts/yuyuhakusho/capture_battle_world_entry.py --out .local/md-yuyuhakusho/world-entry-recapture
python3 -B scripts/yuyuhakusho/extract_battle_world_entry.py --capture games/yuyuhakusho/verification/world-irq-gate/world-native-entry.json.gz --out .local/md-yuyuhakusho/world-entry-reexport
```

## 剩余集成

下一段 `0654` 会消费待传输图形列表，不能替换为固定帧耗时或按零耗时处理。本次只静态确认其循环、寄存器屏蔽和 VDP 写入顺序，见[后续图形传输指令](../world-audio-barriers/world-dma-flush/vdp-and-irq-setup.txt)；没有执行或计价该阶段。

待办仍包括世界包装／DMA／逻辑更新与尾部、降屏蔽内部 IRQ、其他世界工作、跨帧联合时钟与存档、对向按键物理桥接、平台 PCM 流和资源发布，最后进行完整浏览器对局验收。试玩服务继续使用原构建，完整比赛音频仍关闭；此前未复现的 Go/Wasm heap 异常未宣称修复。
