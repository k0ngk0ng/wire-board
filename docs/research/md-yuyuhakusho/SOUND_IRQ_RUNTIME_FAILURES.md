# 声音中断与联合音频状态的先行失败场景

- 把世界的固定帧边界当作 IRQ 接受点；本阶段只接收实际外部 main clock，不推定世界 CPU／VDP 接受时刻。
- delay 为 1 时当次错误提交 delayed code，队列满时错误清 delayed code，或者 delayed enqueue 读取入口队列快照而非实际 owner。
- busy 重入仍运行 driver，backlog byte 回绕错误，catchup 在 driver 后递减入口旧值而丢失中途新增 backlog。
- 中断、补跑、实际 driver wrapper、Root 与全部声部没有使用同一时钟、refresh 和整体预算；漏掉 wrapper 的固定跳转或重复 return。
- 另实现一套 enqueue／scheduler 选择规则；实际字段 mutation 与观察时刻不一致，effect lease 遗漏、重复、过期、伪造或异常仍继续。
- Scheduler、Root、score、PCM、FM／PSG 保留多份 owner；把 World 的 queue／pause 字段也放进音频存档，导致恢复后串用。
- 联合 checkpoint 缺少资源身份、未消费输出或独立芯片时间差；用入口快照补齐未验证状态，或恢复时重播诊断 intents。
- driver、外部 IO、资源、预算或恢复校验失败后只回滚部分 owner；成功提交后又重复应用字段或端口。
- 只验证空 driver；遗漏普通请求、音乐／primary、fade／pause 与真实 mixed JSON 恢复后的继续执行。
- 将给定 clock 的完整声音调用与联合状态通过误称为世界 IRQ、请求生产时刻或平台整场播放完成。

本计划先于新测试、原版调用采样和生产实现。扩展现有 canonical queue／scheduler 与共享 driver 执行器，保持 World 的 queue／pause 作为外部唯一 owner。世界帧首、manager 后、arena 前和帧尾的既有反例继续作为后续世界调度约束，不安装原版时间表或 PC／RAM 程序。
