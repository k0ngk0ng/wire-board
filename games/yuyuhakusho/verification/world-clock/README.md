# 世界中断、DMA 与音频的联合时钟

世界中断前置、图形传输包装和中途的垂直中断现在使用同一主时钟、refresh、CPU 预算与跨帧协调器。真实后台播放器按主机批次目标推进，跨帧时结算同一个混音器的音频输出。完整调用到达逻辑 World 入口后才提交；后续预算或外层世界失败时，图形、世界字段与音频一并撤销。

本次边界是逻辑世界函数入口，以及 busy 分支返回后的外层中断尾部入口。逻辑世界、角色与碰撞工作及外层尾部仍须继续计时，完整比赛音频尚未开启。

## 接入与状态

`session.PrepareBattleWorldIRQLogic` 完成整个前置与包装流程。`session.PrepareBattleWorldLogic` 可从已经完成前置的入口开始；二者分别对应 `scoreclock` 中的联合推进函数。前置与包装共享预算，不能在交接时重新获得一份限额。

主时钟与 refresh 使用连续坐标，VDP 和实际 PCM 各保留自己的规范状态。每帧重置的原参考 IRQ 相位用当前时刻减去 FrameOrigin 计算。前置流程也走同一个跨帧协调器，能在帧尾结算、处理新帧垂直信号后继续进入传输阶段。

`BattleGraphics` 只保存尚未消费的 typed 传输及必要的显示／颜色锁存。缓存目标显示值可先于端口写入变化，实际显示位仍由 VDP owner 保存。`BattleMainContext.DMAWindows` 是零或一份当前 VDP 总线状态；它不保存 DMA 启动历史。音频存档继续只保存实际音频 owner，主上下文、图形及世界由外层一起保留。

`AwaitingLogic` 表示可以进入逻辑世界，`BusyReturned` 表示将继续外层中断尾部。这两个状态都不表示整次 IRQ 已返回原调用者。返回的音频帧是本次新产出的数据，未进入持久化历史。

## 原版对照

- 19 次真实世界包装包含 2,268 个 CPU 工作步骤和 994 次 I/O，覆盖 89 项 DMA、6 次整段包装跨帧、6 次恢复 SR 内接受垂直 IRQ、4 次 busy 返回。每步比较主时刻、refresh、VDP、pin、mask、IRQ 阶段与世界标志；完整序列通过。
- 原有 11 次自然世界中断前置与这批包装在实际入口精确相接。合并后逐步比较从 RAM 中断入口到逻辑 World 入口的全部 CPU／I/O，并验证同一预算连续消费。
- 真实混音器逐例执行相同入口，验证实际音频帧、完整音频／图形／世界状态、JSON 恢复、前置结束后的预算失败、包装末尾预算失败，以及外层世界失败后的共同回滚。另用明确声明的晚期入口验证前置流程本身跨帧；该案例不冒充新原版绝对时刻对照。

主机原版对照使用记录的后台／总线反馈作为明确外部输入。这批记录中的反馈均为零，仍保留逐目标和逐请求的完整契约。真实混音器事务测试另外使用生产后台 owner，不能据此前一层就声称已从新建比赛独立计算全部世界时刻。

## 实际 PCM 的 DMA 等待

新显式 `RunDMAUntil` 和 `AdvanceDMA` 接入真实 VDP 更新。后台在等待时保留已完成的 ROM 操作和当前采样阶段；后续 VDP 更新重设等待者时刻，完成传输后清除等待。旧未调度 FramePlan 的 remaining 拒绝契约继续通过。

两个固定核心各执行 10 条完整输入流、546 帧。全部物理 YM 端口、采样／元数据／dummy 事件、完整 PCM 字节、DAC DSP、玩家状态和调度结果一致。Go 逐步恢复 JSON 状态，包含带真实等待及未消费等待事件的中间状态，完整匹配 263,798 个事件、2 次真实等待置位和 5 次已有等待时的 VDP 更新。

原参考先运行主机，再运行同批次的后台。后台查询当时最新的全局 DMA 状态，可能看到晚于自身游标的主机写入；不能按后台旧时间重放一段 DMA 启动历史。详见[参考顺序](DMA_REFERENCE_ORDER.md)。

## 验收结果

| 受影响范围 | 顶层测试 | 含子项记录 | 结果 |
| --- | ---: | ---: | --- |
| 原生竞态关联回归 | 53 | 488 | 通过，152.45 秒 |
| 实际 Node／Wasm 关联回归 | 53 | 488 | 通过，121.80 秒 |

两个后端的通过名称集合相同，没有跳过测试。回归包括旧 PCM 全流程、总线／物理端口、完整混音器、VDP、采样前置、完整声音 IRQ、采样返回及原世界事务。相关 vet、格式和链接检查通过；未修改的其他游戏、全世界规则矩阵和浏览器网络性能矩阵未重复运行。

## 测试与来源保留

[失败范围](FAILURES.md)、世界测试及原生输入在实现前由 [test-first.json](test-first.json) 冻结。显式 PCM 调度、前置合并和前置跨帧能力分别先冻结测试，见 [pcm-test-first.json](pcm-test-first.json)、[flow-test-first.json](flow-test-first.json) 和 [boundary-test-first.json](boundary-test-first.json)。新 Go 测试、原版预期均未在实现后改写。

包装与显式 PCM 流的首次实现直接通过原版对照。前置跨帧测试先真实失败于旧单帧限制，随后接入同一个主协调器，通过原断言；红灯保留。缺 API 编译记录也全部保留。

取证准备时，静音 checkpoint 的画面尺寸不保证为 320×224，现为这些明确私有 DMA 输入显式设置声明尺寸。原尝试及日志保留。最初六条流覆盖垂直消隐和帧尾，另追加四条普通扫描线 DMA，原六条原生文件可从新文件前六条逐字节重建，见 [扩展校验](pcm-active-extension.json)。世界主机原生 fixture 继续直接引用上一批冻结数据，没有重采或改写。

## 复跑

在仓库根目录执行，输出目录须位于仓库内且尚不存在。验证器保存命令、环境及输出，并将 Go 缓存、模块、遥测和临时文件留在当前工作区。

```sh
python3 -B scripts/yuyuhakusho/verify_world_clock.py --mode native-race --out .local/md-yuyuhakusho/world-clock-recheck-race
python3 -B scripts/yuyuhakusho/verify_world_clock.py --mode wasm --out .local/md-yuyuhakusho/world-clock-recheck-wasm
python3 -B scripts/yuyuhakusho/verify_world_clock.py --mode vet --out .local/md-yuyuhakusho/world-clock-recheck-vet
python3 -B scripts/yuyuhakusho/verify_world_clock.py --audit
```

各命令应返回退出码 0。`--audit` 验证当前源码、冻结输入及历史／最终记录的摘要。原生和离线导出使用现有固定 ROM／核心，不安装新工具：

```sh
python3 -B scripts/yuyuhakusho/capture_pcm_scheduled_dma.py --out .local/md-yuyuhakusho/scheduled-dma-recapture
python3 -B scripts/yuyuhakusho/export_pcm_scheduled_dma.py --capture .local/md-yuyuhakusho/scheduled-dma-recapture --out .local/md-yuyuhakusho/scheduled-dma-reexport
python3 -B scripts/yuyuhakusho/extract_world_clock.py --out .local/md-yuyuhakusho/world-clock-reexport
```

## 剩余边界

当前联合包装验证准备好的战斗场景模式 52／IRQ 模式 4，尚未完成场景切换初始化、逻辑世界和角色／碰撞等全部工作。遇到未计价的声音中断或其他场景会明确失败。当前显式 PCM 分段输入要求 DMA end 位于尚未结算的参考帧内，超出窗口的分支仍待接入和验收。

下一步把逻辑 World 的实际工作边界和图形传输生产者接入同一协调器，再完成外层中断尾部、联合主状态、物理对向按键桥接、平台 PCM 流／加载／无损资源发布和完整浏览器对局。试玩服务保持原构建及数据。此前未复现的 Go/Wasm heap 异常仍未宣称修复。
