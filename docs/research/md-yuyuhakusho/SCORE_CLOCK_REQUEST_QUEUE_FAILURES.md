# 请求消费与普通 PCM 分派的先行失败场景

- D→A 的 live read／clear 顺序改变；从入口快照读取全部 queue，覆盖中途新写入的请求。
- priority 比较未屏蔽高位、相同优先级的胜出顺序错误，或特殊 FF 选择后未保留更低 slot 的其它特殊请求。
- 普通结束错误提交负 candidate priority，或特殊分支仍执行最终 priority store。
- canonical queue 与计时器各实现一次选择规则，或 Root 将新选中的结果重复应用；source effects 必须在唯一 owner 上执行一次。
- callback 遗漏、重复、过期或异常后继续执行；实际字段读取／写入没有发生在服务的计时点。
- 从 queue 选中普通 PCM 后，dispatcher 分类、F0 重映射、真实 arbitration、Root choice、publication 与 return 不共享同一时钟／预算。
- publication 使用入口反馈而非实际读取的反馈，丢失采样后的 StartDual 决策，或 dispatch 的 selected reset 错位。
- 把 queue／普通分派通过扩大为 music／primary header、非 idle fade／pause、world IRQ 或平台播放已完成；未实现分派继续拒绝。

本计划、原版证据与 E2E 先于生产改动。队列仍只有实际 World／Root 指针这一份 owner，临时 sampled code／candidate 只存在于当前消费调用。
