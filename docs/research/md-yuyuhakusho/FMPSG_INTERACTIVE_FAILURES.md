# 帧内交互 DSP 的先行失败边界

时基调用者需要在同一视频帧反复读取 YM busy，只有读到释放后才继续写入。现有 `Queue`／`FrameBoundary` 只在帧尾返回读值，不能直接满足反馈循环。本项先建立原生逐操作状态与反馈循环证据，再实现交互 API；旧生产基线与 oracle 不修改。

## 接口与状态约束

新增交互实例直接拥有一份 YM、PSG、busy、raw cursor、当前 FIR buffer、未读 integrator 和当前 FM 采样进度。当前 FIR buffer 取代已提交状态的 18-tap tail，不同时持久保存尾表或原初态。半帧快照使用单独的 typed `InteractiveState`；只有未开始新帧时才能导出旧 `State`。带有未消费 `Pending` 的旧状态不能静默切换交互模式。

`WritePort`／`WritePSG` 即时提交；`ReadStatus` 按原 port0 的规则先推进 FM，再即时返回，其他端口返回零且不虚构推进。忙标志只由芯片 owner 维护，调用者不得缓存 status／deadline。跨帧由调用者显式 `FrameBoundary`，不在轮询中暗中 flush；保留现有 1,008 master cycles raw overrun 合同。

为避免出现两套不一致的芯片执行器，旧 `Stream.FrameBoundary` 只作最小转接：把 `Pending` 交给相同帧事务，再 flush；公开签名与旧快照语义保持不变。新交互状态不保留已返回读值或输入历史。不能提供一个随意推进所有芯片的 `AdvanceTo`，因为额外 YM update 分块可能改变参考 timer-B 的可观察边界。

## 必须先覆盖的失败场景

| 场景 | 可能失败 |
| --- | --- |
| 原生 busy 自适应循环 | 返回旧 status、先写后 poll、错用未来 deadline，或停止循环条件不同 |
| Busy42网格，边界前／等于／后一周期 | 7／42 混用、round-up 错一格、帧内重基错误 |
| 同 cycle address／data、跨 bank data port | 丢失顺序、误用 data port 选择 bank |
| 连续 port0／其他 status 端口 | 额外生成芯片采样、timer 状态提前或对无效端口返回 busy |
| PSG 与 FM busy 轮询混合 | 两路不同推进节奏丢失，或同 cycle PSG delta 被合并 |
| 帧末 raw overrun 和显式 flush | FM 已推进状态被回退、FM 超前输出误入 PCM、PSG ahead tail 被丢弃 |
| 每一步 snapshot／restore | 缺少当前 FIR 或 FM 进度、返回 slice 别名、复制旧初态产生第二 owner |
| 交互与旧 Queue API | 双重执行输入、重复返回读值、最终 PCM 或旧 State 不同 |
| 超过 4,096 操作、非法端口／时间、overflow | 部分提交芯片、FIR 或资源计数；隐藏自动 flush |
| 半帧导出旧 State、带 Pending 切换 | 缺失尚未 flush 的 FIR 数据或跳过未消费输入 |
| 伪造 InteractiveState | 无界 FIR、错误 FM 进度、非法游标或时钟被接收 |
| Native／Wasm／race／旧全 oracle | 接口扩展破坏已冻结的 raw YM、完整混合、PSG 或 DAC 证明 |

原生中间 FIR 表示需要明确区分：原 core 暂存 FM raw buffer，Go 直接将已生成的帧内 FM delta 放入当前 FIR。研究 oracle 只把原 raw FM buffer 注入一个独立原生 blip 副本，生成等价的当前 FIR 投影；原音频、YM、PSG、原 blip 和非音频 guest 状态不得因此改变。不得把该投影写成原字段的直接读值。
