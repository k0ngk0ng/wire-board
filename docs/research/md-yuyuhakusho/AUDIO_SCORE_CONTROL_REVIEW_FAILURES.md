# Control 与事务 lease 独立审阅场景

以下回归先于父分支对事务重入的修复；本分工只新增独立测试和证据，不修改生产、现有测试或 goldens。

- 同一 transaction 的阶段在 callback 内再次执行自身，导致 Tempo、control、dispatch 或 voice 推进重复。
- `Finish`／`FinishSuspended` 的 leave callback 在外层标记 closed 前再次完成事务，导致双增 Ticks 或双 commit。
- 重入虽然返回错误，却没有毒化外层；调用者忽略内层错误后外层仍成功。
- 重入拒绝或 callback 内 Abort 立即放掉 Driver lease，同一 callback 随即 `d.Advance` 劫持 authority；旧阶段返回后出现不受支持的中途 commit。
- 错误、乱序或 Abort 在外层运行栈退出后没有释放 lease；失败仍返回 working snapshot 或部分输出。
- callback 中的只读 Snapshot 被错误当成重入；修改返回的 working snapshot 改变 transaction state。
- 临时 running／lease 保护进入持久 State，或偷偷复制 Root owner。保护应只属于 Driver／TickTransaction。

零 tick 动作、Fade 的 signed byte／PSG 阈值与 3a126 抑制、Reset 保 Ticks 同时按原指令和已有 94 个原窗口只读核对。它们目前没有发现需要修改的实现差异，旧窗口与原参数数据保持原样。
