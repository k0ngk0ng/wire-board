# 乐器查找与 FM 传送时基的先行失败场景

本项将已绑定的具体乐器引用用于完整 FM 参数传送和 level 更新。阶段接口必须让 score 的逐次声音意图在实际写入点消费；不得在 resource 回调里先发完全部写入，再忽略后续意图。

实现前覆盖：

- 忽略 music／primary／secondary 不同 bank 选择路径、零次查找和较长查找的 DBF 成本；按音色 ID 合并不同工作量。
- 25 次 load 写入的 caller、operator 顺序、channel／bank 路由或 canonical YM 服务重复、漏计；最后一次 DBF 与 pan 入口边界错误。
- level 更新使用所选 patch algorithm 代替当前 loaded algorithm，丢失 carrier、原 TL 高位 carry、负 base 或 carry 抑制分支。
- claimed 路径仍访问资源／总线；非写入 operator 的成本被跳过，零写入时无法正确 Finish。
- 每个阶段重置时钟／预算；重复 Next／提前 Finish、错误意图、结束后重入、错误被吞掉或返回部分结果。
- 原生记录器改变实际状态、两份固定 core 的访问／时刻不同；原始硬件忽略位被误称为逐 raw byte 相等。
- 实际 mixed Runtime 的声音状态效果、分阶段与完整传送等价性、保存恢复、外部错误及 detached 分支失败处理不正确。

生产只保留临时语义阶段、唯一外部 bus／chip owner 和有界工作量，不保存源 bank、instrument index、PC、RAM、原栈或未来时刻表。原生私有 held-bus／mailbox0 profile 用于 caller＋service 对照；它不等于完整世界／IRQ 和独立 PCM 反馈已通过。pan 尾部仍明确交给其实际 owner。
