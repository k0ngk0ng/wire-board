# FM／PSG 独立时间域验收

本项验证原版允许的跨芯片生产者顺序：较晚的 PSG 回调可以先发生，PCM 的较早 YM 回调随后在总线同步中提交。每个芯片域仍须单调；不排序、夹紧或丢弃事件，不提前执行 PCM 以丢失主机争用延迟。

8 组原版场景覆盖延迟 YM、反向交错、相同时刻、普通 reset、PSG 超前时的 after-DAC 194 边界、跨帧 carry、共享地址锁存和重复 reset。两份固定 core 分别开启／关闭被动状态投影，共 32 次执行，完整 native state、RAM、PSG 与 PCM 一致；跨 core 结果也一致。保留 2,889 次调用、15 个帧边界、11,032 个立体声采样帧，每份 core 完成 2,897 次无副作用投影。

E2E 在每次调用后比较完整 YM、PSG、FIR、busy 和 cursor 状态，逐采样比较 PCM，并通过 JSON restore 继续。`YMLag` 只保留被 PSG 遮住、不能从现有状态推导的 YM frontier 距离；原单调入口的拒绝契约保持不变，同域倒退、无效 reset 与伪造 checkpoint 继续拒绝且不修改 owner。

失败计划、原版数据、producer 和测试在实现前冻结，真实缺少 API 的 red 保留。第一轮新增 10 条测试记录全部通过，未修改测试或 oracle。调用者使用 actual mixed owner；原有 103 组控制整拍及暂停存档后的继续执行，另由[控制验收](../score-clock-driver-control/)验证。

API 与状态边界见 [INDEPENDENT_LANES.md](../../audio/chips/INDEPENDENT_LANES.md)。当前源码与最终完整芯片、clock／Root 原生竞态和实际 Node/Wasm 回归由[联合清单](../score-clock-driver-control/validation.json)绑定；旧 source-bound 清单继续保留历史摘要。
