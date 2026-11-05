# 音序恢复与目录拒绝边界审阅

只读审阅主 driver；以下独立回归先于父分支修复，不修改生产实现或原连续检查。范围为 `score.Restore`、`DecodeCatalog` 及公开值的所有权。

- 已拥有声部的保存通道未知或与声明的硬件槽不一致，恢复后按默认 FM1 或其他通道发声。
- 活动声部没有 owner；尤其保留有效 ScoreID 与负 VoiceID 时，`nextEvent` 负索引会 panic。
- 有限轨道的 cursor 指向不存在的 cycle；envelope cursor 超出当前 typed 前缀／循环范围；启用 pan 的 step 无法在当前定义下读取。
- 保存的 DAC descriptor 不存在，或 PSG noise control 不是 noise latch。
- Modulation 已启用但缺少定义；当前 delay／step counter 超过定义，或当前 signed delta 不是初始方向及其反向。
- Tempo period 与 remaining 不能构成 driver 可产生的倒数状态。
- 同一音乐实例的声部引用互相冲突的 ScoreID。效果声部可以属于不同命令，不能机械套用同一约束。
- Envelope 表仅满足条目数量，实际 ID 集合缺少 profile 必需项且多出未知项。
- 16 MiB `LimitReader` 在第二个 JSON 前截断输入，把真实尾随内容当作 EOF 接受。
- 修改外部 snapshot、恢复输入或输出 intent 能改变 live state；或拒绝输入后提交部分 state／intent。

不要求 selected patch algorithm 等于 loaded algorithm：原版确有两个阶段，初始与 claim 抑制期间不能以这种等式拒绝合法状态。已终结的 pitch cursor、inactive voice 保留参数、PSG3／noise 的合法别名也应与非法越界区分。测试使用实际 v2 catalog，不将未实现平台或 DSP 纳入本审阅。
