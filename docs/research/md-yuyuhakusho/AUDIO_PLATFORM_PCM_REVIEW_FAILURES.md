# PCM 后续竞态与异常清理回归

原 16 项冻结测试不修改。先保存已经过早编写的复核修复及结果，精确恢复修复前实现，再编写本组独立测试、取得真实 red 后重施最终修复；完整历史保留，不把过早修复伪装成先行测试。

- 同一 element 上，初次 play Promise 在 pause／resume 后迟到成功，不得暂停当前 resume。旧 epoch 的 media 仍只清理自身。
- stop 的 pause、srcObject 清空、DOM remove 或 track.stop 任一抛错，其他清理仍必须逐项尝试，writer abort／unlock 必须执行。
- 清理确有异常时 released 不得假报成功；未 await released 的宿主 stop 不能产生 unhandled rejection，之后 await released 仍可观察错误。
- prepare 失败后的清理自身也抛错时，不能跳过 track.stop／writer unlock，不影响旧有效输出。

这些是受控错误／Promise 顺序证明，不能替代真实原生播放和设备输出验收。最终当前源码需保持原有99项、真实PCM矩阵及MSE／有限／旧音效回归。
