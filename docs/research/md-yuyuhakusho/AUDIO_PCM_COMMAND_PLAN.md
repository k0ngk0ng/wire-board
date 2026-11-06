# PCM 命令层接口与证据边界

新层将从 empty/idle 播放器接收已授权的 typed 提交与外部总线窗口，独立产生启动、播放、替换和自然结束的语义事件。既有 `pcmplayer` 的有限已初始化 profile 保持冻结，作为无外部暂停的回归对照。

## 输入分层

| 输入 | 归属 |
| --- | --- |
| 普通音效 intent | 调用者使用既有 `engine.SelectPCMCommand` 决定接受／目标；新层执行发布、确认和播放，不复制仲裁 |
| 音乐 DAC intent | score 输出 descriptor 逻辑 ID 与 pan；先按当前 PCM feedback 判断 blocked/priority，再发布 6 字节协议 |
| `BusHold{Begin,End}` | 68k／外部环境明确提供的持总线窗口；pause 使用长期 hold，resume 使用 release |
| DMA 区间 | 当前 frame 的 typed 环境输入；每次实际 bank 读调用既有 `phaseclock.ResolveBankAccess` |
| Reset | 实际硬复位输入；与自然结束、score track-end、FM／PSG 控制分开 |

普通描述保持一个 `RoutingOrPan` byte owner：single 为 pan，dual 为目标 routing。音乐描述不携普通第 7 字节，pan 来自轨道。生产资源由 encoded SHA-256 标识，头部的源起点／长度成为 typed metadata，不保存 ROM 地址。

## 建议 API 形状

接口需在原生证据后冻结：`New(Config)` 从 idle 开始；`Advance(until, Inputs)` 消耗有界、有序的命令与总线事件；`Snapshot/Restore` 保存唯一 typed 状态；`Status()` 只构造临时反馈给现有仲裁／score。输出包括 DAC、pan、enable、bank/metadata 读取、请求确认与忙临界区。

内部保留 pending descriptor、已锁存的独立命令字段、活动源／解码／预取与可中断的语义阶段。共享不可变资源可用逻辑 ID 复用。一个暂停若落在一个语义操作中间，应先按原有界原子操作完成该操作，再暂停；rate 循环等可用闭式边界计算。没有任意 PC、指令执行或通用 RAM／寄存器镜像。

## 已确认的控制边界

- 音乐 DAC track-end 清理音序行并退出本次 driver tick，当前 PCM 样本继续自然结束；它不发布 PCM stop、DAC enable 或 pan。
- 音乐 feedback 为 `Blocked=(start_request&80)!=0`、`Priority=mode_flags&C0`、`PanRedirect=(start_request!=0)`；feedback 不在 score 建立第二份状态。
- 普通请求的 single 入口通过 `start_request=80`，音乐通过 `start_request=1`。原程序据该标记选择普通 pan 或音乐 pan。
- `rate_request=0` 为无请求，`1..7F` 进入 single，`80..FF` 进入 dual 初始化。已初始化播放器中 rate0 的 256 次延迟测试不等于普通入口可以用 request0 启动。
- `02/03` 清理 FM／PSG 的不同声部组；原实测 `04` 也不立即停止本批 PCM。硬复位 `00` 是不同生命周期操作。
- 固定 core 申请 Z80 bus 时先 `z80_run(target)` 至有界原子边界；释放时直接写入 `ceil(release/15)*15`。外部窗口与播放器过冲分别保留。

最终验收将分别报告：给定 typed 外部命令／窗口下的独立命令生命周期、PCM/DSP 输出与未知的世界／IRQ 调度。起播相同不替代中途抢占和恢复证明。
