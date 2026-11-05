# 混合声部 claim／restore 验收失败场景

本清单先于新的混合采集器与状态归一化 helper。已有原版、观察版内核、静音原点、23 定义原读和候选数据均作为只读输入。

- 只使用隔离音效证明混合覆盖；没有音乐和效果同时活动的原程序记录。
- 原版与观察版的 RAM、视频、928-byte driver、PCM 内容或采样帧数存在任何差异，却仍采信观察事件。
- 使用恢复存档而漏掉 `refresh_cycles`，或两内核没有分别从同一已验证冷启动建立静音原点。
- primary claim 未先设置 music 的 claim 位便清空效果行；把 hardware channel 与 score ordinal／平台席位混同。
- 效果初始化后等待下一 tick；或音乐初始化时错误地在当前 tick 推进。
- claim 期间停止隐藏 music 的 note、gate、modulation、envelope 或 pan 演进。
- 隐藏期间选择新 FM patch 就提前刷新已加载 algorithm cache，或结束后恢复 effect 开始时的旧 music 快照。
- primary 结束不清优先级、漏掉 forced-rest、在 inactive music 上无条件恢复 FM patch，或 PSG noise latch 恢复顺序错误。
- 高优先级替换低优先级效果时先错误地执行旧 track-end；低优先级请求被丢弃却仍重新初始化声部。
- 新音乐初始化清理已有 primary 效果时漏掉 keyoff／restore，或错误处理同一 tick 的 clear 和初始化顺序。
- `ab` 的 `7d` 直接覆盖首队列槽被当作普通 enqueue，或把后续 PCM 命令误当 score 定义。
- 只根据当前视频帧猜测 DAC 被接受；不记录 DAC guard 处的外部 pending／priority，只以最终 descriptor 请求倒推反馈。
- 将 signed pending 字节的任意非零值都视为阻塞，而原程序只测试最高位。
- 将 effect 的 instrument-bank 指针字节解释为禁用 pan automation 的演进状态；或把 RAM、PC、ROM 地址、loop stack 搬进 portable 生产候选。
- 将 pan 的 period 与 elapsed 字段颠倒，修改已独立证明的 Go 参数 helper 来迎合错误 oracle，或在修正投影时顺便扩大原固定排除类别。
- 漏掉 `39186/3889a/3890c` 的直接 PSG noise restore 写，仅依据通用 PSG 请求入口声称完整有序输出，或为迎合漏点 oracle 删除 Go 的正确恢复请求。
- 把按 driver 顺序捕获的 YM／PSG 请求当作已证明 master-cycle cadence；忽略 YM 保留位的原请求证明边界。
- 省略未到达的 group2 启动／层叠路径，仍宣称所有音乐和音效域完整覆盖。

本轮目标：`82 + a0..ab` 的 12 条独立混合流，以及高替低、低请求拒绝、新音乐清理 primary 的边界流。每个流同时保留原始 provenance 和无源地址的 typed 初始／逐 tick 状态、有序 chip intent 与 DAC feedback。真实原程序探针不是生产实现；Go driver 后续仍须从单初态独立推进对照。
