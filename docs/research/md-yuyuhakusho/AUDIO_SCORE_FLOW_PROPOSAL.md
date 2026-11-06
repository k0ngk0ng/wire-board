# Parser 工作 sidecar 初步契约

绑定正式 schema 2 原 JSON SHA-256 `735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830`。每个逻辑键为 `{score_id, voice_id, section: prefix|cycle, event_index}`；值为有序有限 `Work[]`。它不改变音符或参数语义，不提供原 PC／offset、原字节、目标、栈或时刻。

初步 Work 包括有限 jump／call／return、loop 的 first／taken 语义分支、token 分类读取、参数读取角色、pitch／duration probe 的 take／reuse，以及 duration scaling 的有界加法次数。原已消除 flow 工作拟归到后一个保留 typed event 的 prelude；同一源 token 多次 co-emit 要明确零重复 fetch。

参数读取必须保序：F0 为 delay、rate、delta、steps；E4 正常模式在 mode、pattern、start step、limit 后，分别读取 period 和 elapsed seed，即同一值实际读取两次。disabled E4 只读取 mode。

验证确认有 8 个 `cycle[0]` 键首次／回绕不同。研究形状保留同 key 的 Work 变体，并按主 owner 确认的临时 `Entry = continue | from-prefix | cycle-wrap` 精确选择：prefix 及循环非首项为 continue；prefix 耗尽后的循环首项为 from-prefix；循环耗尽后的首项为 cycle-wrap。Entry 从既有 nextEvent 转换现场产生，不写入 State，也不保存圈数。未知 Entry、人工构造但没有入口历史的 cycle index 0 必须拒绝成本映射，不选择首变体兜底。

claimed／tie、gate 期限、envelope／pan／modulation 启用状态和有效性由 Operation 现场语义提供，sidecar 不重复持久保存。bus／chip／mailbox／IRQ、refresh deadline 和全局时钟归原 owner。Work 只声明固定语义结构、分支和次数上界，不声明实际 cycles；未来 clock 对未知 Work 必须明确拒绝。

以上粒度已与 phase-clock owner 对齐；本轮先研究并验证数据，不生产新 Go 资源或更改正式 catalog。
