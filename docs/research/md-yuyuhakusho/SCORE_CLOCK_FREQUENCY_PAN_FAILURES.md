# 频率写入与自动声像先行失败场景

- FM frequency high／low 的 caller、voice bank routing、ignored high bits 或 return 重复／遗漏；不能提前把两次实际 intent 合成一次外部提交。
- PSG noise channel 的 tone remap、nibble／六位 continuation、shift 和两次 store 成本错误。
- 自动声像 note／held 两套入口跳表混用；mode 4 的二次 dispatch、reset、once stop、period／elapsed 与 step limit 的 signed-byte 判断遗漏。
- 当次 pan output 覆盖持久声像、增加另一份 counter，或 pan 子调用重新预算／提前执行硬件。
- Begin、对应实际 write 与 Finish 的次序／预算／clock／owner 验证不足，错误后还能继续；局部周期被当成完整 voice／world 周期。

原版固定入口、测试和失败计划先于实现；原 ROM／core、旧 fixture、State 和 canonical param.AdvancePan 语义保持原样。
