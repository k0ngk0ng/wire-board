# FM／PSG 独立时间域的先行失败场景

整拍控制验收的 `81-fade-step-request-a0` 已证明：最后一次主机 YM 写入为 104,903，PSG 写入为 118,105，随后在 123,453 申请总线时，PCM 才提交先前 105,495 的 YM address。主机 IO 在失败前与原版逐项相同；把这条 YM 事件拿去比较 PSG 推进后的统一 cursor 会错误拒绝合法调用。不得通过排序、夹紧时间、丢弃输入或修改整拍 oracle 解决。

- 仅允许原有单调输入，错误拒绝原版跨 FM／PSG 的生产者顺序；或者反向放宽为同一 YM／PSG 域内任意回时。
- PSG 推进时提前生成 FM 或清除 FM busy／DAC reset 因果；YM 的迟交事件反过来改变已生成的 PSG 状态。
- 仅修改时间校验但丢失不可推导的 YM frontier，使下一次写入、恢复存档或 frame boundary 无法判断同域顺序。
- 同时持久化可互推的 aggregate cursor 与两份同义时间，或保存事件历史、输入 tape、复制的芯片状态来回放。
- 新的独立域入口悄悄改变原单调 API 的拒绝契约，或者深拷贝遗漏可变指针使 snapshot／fork 串用状态。
- FM／PSG／FIR／PCM 与原版不同，frame flush 重算旧采样、忽略有界 lookahead、重复输出或丢失跨帧尾部。
- reset 误用 PSG 的时间判断 YM 回时，把 after-DAC 的特定 194 边界推广成任意时间退回，或者因 PSG 事件丢失最后 YM 事件的 DAC 因果。
- mixed 的原子失败改变权威 owner；暂停、渐弱后的技能输入或 JSON 恢复后再次出现同一问题。

修复前新增最小原版混音对照，保留实际 callback 顺序，比较完整 YM／PSG／FIR 状态与最终 PCM。既有整拍 oracle、端口时刻与原有单调入口的反例保持不变。只保存不能从现有各芯片 owner 推导的信息；不增加事件历史或第二份持续寄存器缓存。
