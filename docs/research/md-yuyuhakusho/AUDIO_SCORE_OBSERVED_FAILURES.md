# 音序操作观察与阶段反馈失败场景

本清单先于新 API 和实现。`92f27271` 的数据、42 条原版流、既有测试和结果保持原样；本轮单独保存来源与先红后绿记录。

- 观察接口不存在，或通过修改原 `Advance` 的输入／输出／State 来替代兼容接口。
- 把 DAC Blocked／Priority 与 PanRedirect 合成 tick 入口快照；chip 服务改变外部状态后仍读取旧反馈。
- `ReadDAC` 在原条件不满足时提前读取，例如已经 blocked 仍假称读取了 priority，或者没有 channel-six pan 也读取 PanRedirect。
- priority 在 tick 末统一清理，错过 key-off 与 restore 的中间位置；将音乐 reset 的 save／clear／restore 简化成硬编码最后写 0。
- callback 在 effect 之后触发，或失去与 flat intent offset 的相对顺序。
- PSG pitch 的两次 store 被拆成两个完整 leaf，重复计入共同返回路径；组内又重复通知并施加硬件效果。
- 原有限 parser、note lookahead、包络源控制等未恢复的结构被当成零次／零成本；以 before／after 猜测已消除路径。
- 把原 PC、原 opcode、原字节、跳转目标、调用栈或实测周期表放入新运行时；把 Op 历史存入 State。
- trace 无界，sequence／span 不合法，错误后仍继续调用外部 hook。
- operation 的 Pan／writes 等可变值与 Output 或 driver alias，hook 修改记录会改变真实输出。
- hook 或反馈校验失败后部分提交 score；误声称可回滚已经发生的外部 bus／chip／PCM 副作用。
- 在 authority owner 上试跑后再回滚；调用者必须先创建 detached 全 owner transaction，仅成功后统一提交。
- 新 API 只在少数样本通过，未对全部原 42 流比较状态、flat 输出、恢复和组 span；或改动旧原版 fixture 使结果通过。

首版只建立正常 score 路径的真实语义阶段，不宣称完成 Root pause／duck／fade、原 parser 字节结构或 master-cycle。缺失结构必须输出显式 unavailable 节点；成本消费者须拒绝，不能静默计为零。后续有限 flow sidecar 另绑原 catalog digest 和逻辑事件位置。
