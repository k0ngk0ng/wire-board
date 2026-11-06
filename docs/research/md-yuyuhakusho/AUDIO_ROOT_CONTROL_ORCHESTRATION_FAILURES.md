# Root 规则阶段协调的先行失败场景

本清单先于 orchestrator 实现与测试。范围仅为已审 139 个 fight 请求与 11 首 BGM 的规则阶段；完整 master-cycle、异步 IRQ 和逐 long 中断可见性仍未恢复。

- Gate 复用入口 pause 快照，漏掉三次真实 live-field 读取；pause start、hold、resume 误推进 tempo/fade/queue/voice。
- resume 重算 TL 或额外恢复 PSG；mute 尚未成功就建立 pending payload，restore 未成功就释放它；缺 pending 的 hold/resume 被默许。
- 暂停恢复载荷、World.Queue、AudioPause、Scheduler、PCM、18 voices 或全芯片状态被持久复制到第二 Root。
- 正常顺序偏离 Gate→ControlIdle→Tempo→Fade→Queue→Dispatch→Voices→Return；控制请求后重复 tempo，或 01/05 错误跳过本 tick voice scan。
- queue 在入口复制后再消费，忽略实际 queue 阶段的新请求；特殊请求后残余队列被覆盖；音序 replace-first 未在真实节点写唯一 World queue。
- 01/05 先写 steps 后写 remaining，停止 PCM 或遗漏 DAC 四 flags；remaining 1→0 提前淡出；steps 1→0 又执行一次 level step。
- 04 顺序遗漏 YM27、Root/score/world bulk、selected FF、六次 keyoff、24 TL 和四 PSG；清除 Scheduler 或 PCM；bulk 伪装成 232 个已支持中断可见的 store。
- music reset 丢失 priority 的真实 save/clear/restore 节点，额外按 bool 收一次 byte-clear 成本，清掉 primary/secondary 参数或把其后 keyoff 算入 reset leaf。
- score callback 失败后还继续调用外部 owner；失败后仍提交 score；caller 没有将已变更的 detached Root/world/chip/PCM 一起丢弃。
- 非 idle control/duck/delta/signal/FM3 注入被接受；外部 PCM/group2 dispatch 没有服务仍静默成功。
- 用零成本、Go 函数树、原 PC 表或 CPU/RAM VM 填补未定价；OpResourceUnavailable 被吞掉。
- 785 个既有 canonical native frames 被重新采集或只比较 Root 而忽略 score 变化/有序 YM、PSG 请求。

04 在本 revision 只有一个明确 rules bulk effect，保留 16 header long +216 voice long 的来源边界；不会在该 effect 中对外声称逐 long 的 score/Root 中间态。成本消费者必须拒绝尚未恢复的粒度。

## 第一轮独立证据订正

`first-implementation.jsonl` 保留真实首轮结果。priority 用例最初错误地要求 music reset 保存入口 priority=85；原 `startMusic` 先 StopPrimary 清 priority，再进入 reset 保存真实 live 值。修订后的用例只在 `OpPrioritySave` callback 现场注入85，证明 latch 在真实节点读取而不是保存 tick 入口副本；production 未为错误期望改变。原前测 SHA 与修订记录分别保留。

canonical-v2 的 music-start 请求只观察到 PSG 9f/bf/df，缺最后的直接 ff store。既有 score native `music82-a7` 首帧记录12个请求包含该ff，而本轮记录只有11个；当前 E2E 保留这个真实 red，不删 production 请求。该不足属于 observer 覆盖，不能宣称已严格通过全部原端口 ledger。

## 入口租约审阅的新增失败场景

Root 的第一个 callback 如果发生在 score 事务建立前，可以递归调用同一 Root/score 入口并偷偷完成第二个 tick；即使 score 自身拒绝重入，嵌套 Root 也可能已经读取/写入 live World pause 或执行外部 callback。先以 Root Gate callback 和 score Tick-enter callback 两条重入路径取得 red，再在任何 callback 前取得唯一 score 租约。该租约不新增 Root 持久锁或第二 state。

## 最终 owner 合同审阅的新增失败场景

Root 内部为协调 priority、queue 和 header 安装 score observer 后，如果 caller 未提供 `Score.Operation`，内部 wrapper 仍可能令 score 观察接口通过校验，却丢掉实际 score chip 效果。先验证缺少 Root 或 score operation consumer 均在首个 callback 前拒绝，再补入口校验。需要纯规则验证的 caller 仍须显式提供空 consumer，不能由缺省值静默选择。

## 最终结果与历史保留

strict-v3 改为实际 PSG byte-store 记录后，原五流 785 帧、7,199 笔请求严格通过。合法 `82+A6` 的 180 帧、1,246 笔请求使用默认 139 候选域通过；合法 `1F` 只从明确 typed pre04 checkpoint 验证 148 帧、35 笔请求，最初两帧普通 PCM 接受未组合。总计 1,113 个规则帧、8,480 笔请求，不能写成 1,115 帧全部独立实现。

v3 初轮还暴露了两个验证起点错误：Interactive DSP 在 cycle=0 累积输入超过容量，以及使用硬件 reset 的 TL=0 替代原 post04 的 TL=127。验证改用实际 YM／PSG owner，并按已证明的 04 mute 写建立起点。真实 pause 的 World 请求在 Root 返回后发生，投影现显式分开 `after_tick_pause` 与 `world_pause_request`。这些修订保留失败日志，没有改生产行为迎合期望。

最初冻结的测试／文档均有 SHA 副本。unit 与 failures 的初始文本通过反转已记录的后续订正恢复，逐字节 SHA 与实现前 `test-first-manifest.json` 相同；文件名明确为历史快照，不伪称新增执行记录。原缺接口、observer 覆盖、priority 期望、重入和缺少 consumer 的失败产物继续保留。
