# 完整运行拍的先行失败场景

- Gate、control-idle、tempo、fade-idle、queue 检查与 no-dispatch caller／return 重复或遗漏；selected 为零的重初始化不能当作 FF no-dispatch。
- 18 个 slot 的真实扫描顺序、DBF 末次成本、domain 切换或 DAC 后标记清理缺失；DAC stop 仍执行后续 scan 或最终 return。
- DAC held-note 的零值回绕、long branch 和 return 尚未接通，却按普通 FM 处理。
- 新音符、held-note、终止恢复各自创建预算／refresh；整拍运行需要一个临时 service，预算失败不得提交 score。
- true primary normal path 使用隔离 profile 的 positive/global 分支，未按 actual slot 的 negative／voice-local bank 选择。
- tempo reload 从事务前状态推导，重复递增 NoteWait；跨声部恢复读到未推进或重复推进的 sibling 状态。
- Root 的 working-state、feedback、priority／queue effect lease 与实际硬件顺序错位；把返回的诊断 Intents 再执行一遍。
- 输入包含待消费队列、非 FF selected、非 idle fade／control 或 pause 时仍默认为零成本。本阶段只完成明确正常运行 profile，未知 dispatch／fade／IRQ 成本继续拒绝。
- 将原版整拍 IO／计时通过扩大为 world IRQ、联合 checkpoint、连续平台音频或资源发布已完成。

失败计划、整拍原版 oracle 与 E2E 测试先于本项生产改动。运行时不保存原 CPU、ROM、程序或第二套动态 score／Root 状态。
