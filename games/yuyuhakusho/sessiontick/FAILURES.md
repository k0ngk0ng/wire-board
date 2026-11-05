# 共享 tick 失败范围

先于代码和测试：非法 MD bits、automation 超过有效席位、错误／重复原 slot 或 human channel、unready world、缺 graph metadata、后续 draw 错误造成输入 caller mutation；控制变化在 tick 之外发生；清 Controller 历史丢 Released／extension；托管变更重置 HP／pose／位置；server 与 replay 顺序不同。队列、暂停和 cache 继续使用既有 owner，helper 不保留双份历史或执行 CPU／RAM VM。
