# 音符开关与门限先行失败场景

- FM key-on 错用 tie 抑制，key-off 错用 note-off 抑制；各自 note／claim／tie guard 与短分支成本不同。
- PSG key-off 误走 FM bus／mailbox，或错误按 tie／note-off 抑制；PSG 只按 claimed 决定端口写入。
- tail-call canonical YM 已包含 return，外层又加一次；PSG 直接端口写入遗漏唯一 return。
- gate 为 0、1、2、255 的路径混用，expire 后遗漏父调用跳过语义，或把原栈操作保留为生产运行栈。
- gate 的 key 子调用重置预算／refresh，提前发出期望写入，重复／错误意图或失败后仍继续。
- 原版私有固定入口或局部访问通过，被当成完整 voice／world 时序；已有 unpriced 阶段不得清零。

测试与固定原版对照先于实现；只复用 canonical YM／PSG owner，源栈与 PC 留在研究证据，生产接口仅保留语义结果和临时 clock。
