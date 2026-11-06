# 比赛声音控制的规则恢复与验证结果

本轮恢复了 `01/04/05`、渐弱、暂停恢复，以及 Root 与 score 的规则阶段协调。原版对照中，1,113 个组合规则帧的 8,480 条有效 YM／PSG 请求全部一致，native race、实际 Wasm 执行和 vet 通过。完整声音时序、普通 PCM dispatcher 接线和 World→IRQ 组合仍有未完成项。

## 适用范围与状态归属

[来源审计](AUDIO_ROOT_CONTROL_SOURCE_AUDIT.md)确认了 139 个比赛候选请求和 11 首比赛 BGM。候选分类只限定请求来源，不表示 139 个请求的全部外部服务已经组合验收。ordinary `7F` 是已查描述中唯一带 duck flag5 的请求，但不在比赛候选中；已查 214 轨的 12,775 条 grammar 命令没有 E2／FF。该结果支持当前比赛闭包中的 idle profile，不扩展为全 ROM 的不可达结论。

68000 静态依据来自已保留的 direct Ghidra 列表与固定 ROM 原字节核对，运行依据来自原版与观察器 core 的对照。这不表示 REA 提供了原生 68000 反编译能力。

`rootcontrol.State` 仅保存 `Priority`、`Selected`、`ChoiceBits`、`Fade` 和可选的 `PauseRestore`。queue 与 AudioPause 继续由 World 持有；Scheduler、score、PCM 和 chips 各保留唯一状态。control／delta／duck／signal／FM3 在已审 profile 中派生为零，非零输入显式拒绝。具体接口见 [Root 规则阶段协调接口](AUDIO_ROOT_CONTROL_ORCHESTRATION_API.md)。

`AdvanceRules` 的外部 owner 和 hook 必须属于同一 detached transaction。成功后由调用方一并提交；任何错误后均须丢弃全部 detached owner。score 自身失败不提交，但本包不能撤销调用方已经执行的真实外部副作用。Root 在第一个 callback 之前取得 score lease，阻止 callback 重入另一次 Root／score 推进。

Root 与 score 的 operation 消费者均须显式提供；缺少任意一个时，在首个 callback 前拒绝。Root 包装 score operation 不能替代真正的 score IO 消费者。

## 已恢复行为

`01` 按 remaining=3→steps=40 的顺序启动；`05` 按 remaining=32→steps=50 启动。两者均清 DAC music 行的全部四个 flags，保留其他行参数与 owner，不停止 PCM。渐弱 remaining 非零时只减一；从 1 减至 0 的同一调用不做 level step。remaining 原为零时才减 steps；steps 减至 0 后直接请求 `04`，不再执行本次 level step。后续间隔常量为 3，没有为未经证明的 F805 period 增加状态。

`04` 的顺序为 YM27=0→232 long 来源范围的规则 bulk clear→selected=FF→六次 keyoff→24 次 FM TL 静音→四次 PSG 静音。bulk 清 Root、queue、AudioPause 和 score tempo／18 voices，保留 Scheduler 与 PCM。音乐 reset 的来源范围为 136 long，保留 primary／secondary，并在原 priority save／clear／restore 节点协调 Root。bulk 在本 revision 中是一个规则效果，不暴露或定价逐 long 的中断进度。

正常阶段为 Gate→ControlIdle→Tempo→Fade→Queue→Dispatch→Voices→Return。pause start／hold／resume 均跳过 tempo、fade、queue 和 voice scan，score Ticks 各只加一次。start 从当前 chips 捕获 24 个有效 FM level，再执行 24 次静音和四次 PSG 静音；全部成功后才提交 pending payload。resume 仅恢复 24 个 FM level，并在成功后释放 payload，不自行补 PSG 恢复。原 cache 的低 7 位在捕获点与可见 chip level 相等；静音后 cache 保留旧值而 chip 全为 127，说明 pending payload 有独立信息。原请求被硬件忽略的 TL bit7 保留在研究证据中，生产 payload 只保存有效 level。

## 原版对照与重复执行

| 对照范围 | 原采帧数 | 组合规则帧数 | 有效 YM／PSG 请求 | 解释 |
| --- | ---: | ---: | ---: | --- |
| strict v3 的五个压力场景 | 785 | 785 | 7,199 | 包含比赛闭包外 `73/A7`，通过显式 callable stress 接口进入 |
| 合法 `82+A6` 的清空与暂停 | 180 | 180 | 1,246 | 默认 139 候选，无 external dispatch 或 stress 扩展 |
| 合法 `1F` 活动 PCM 下的 `04` | 150 | 148 | 35 | 从已核对的 typed pre-04 checkpoint 开始；未组合前两帧普通 PCM 启动 |
| 合计 | 1,115 | 1,113 | 8,480 | 统计的是规则与请求对照，不是最终混音采样相等证明 |

当前 v3 的原版与带观察器 core 在全部 1,115 帧具有相同的完整 RAM 摘要、画面摘要与尺寸、928-byte driver 投影、PCM 状态、PCM 摘要、采样数和非零样本数。这证明本轮观察器未改变已比较的原程序输出；不表示 Go 组合已经产生相同 PCM。`1F` 的原 PCM mode C1 跨越 `04` 保留，规则组合则未调用普通 PCM 启动服务，不能用该 checkpoint 替代 ordinary acceptance／生命周期测试。

独立 fade helper 保留 785 帧的 746 个决策和 50 个恢复点。局部 fade 时基保留 91 个原生配置、154 个精确写点和两 core 对照；只覆盖 start 与 header 到 Done／LevelScan／FullClear 边界，动态 level body 与 `04` 成本尚未恢复。以上局部结论不能推导完整 tick 成本。

原始观察、无观察器对照和历史修订已按原字节保存在 [native 证据目录](../../../games/yuyuhakusho/verification/root-control/native/preserved-sources.json)，共 49 个文件。ROM、可执行 core 和临时构建不随证据归档。夹具、生产文件、验证记录和 SHA 由 [总验证清单](../../../games/yuyuhakusho/verification/root-control/validation.json)及对应 final-freeze 绑定。

无需加载原生 core，即可核对归档 SHA、原版／观察器相等、夹具计数和最终执行日志：

```sh
python3 -B scripts/yuyuhakusho/root-control-scope-check.py
```

规则测试的完整命令与工作区内缓存环境保存在 `verification/root-control-orchestration/final-*-command.json`。native 使用 `go test -race -json -count=1 ./games/yuyuhakusho/audio/rootcontrol`；Wasm 使用 `GOOS=js GOARCH=wasm` 并通过 Go 自带的 `go_js_wasm_exec` 实际执行同一包；vet 独立执行。复跑时需使用项目约定的工作区内 telemetry、module、build 和临时目录。

## 保留的真实失败与修订

- 初始 quiet world 未启用 pause；原程序正确清掉直接写入的暂停位。保留失败数据后，改为声明启用的 controller profile 与真实 Start 边沿，没有修改原程序或期望值。
- v2 的有限 PC filter 漏了九处直接 PSG store。严格组合发现音乐开始时缺第 12 条 FF；v3 改为观察所有实际 C00011 byte store，保留旧 producer、旧数据、覆盖检查和真实 red。该修订不构成全 ROM／完整端口 ledger 的证明。
- harness 曾将 frame 末 World producer 写入的 pause 1／80 当作音频 tick-return 值；现分别投影 tick 后状态和后续 World 请求。
- quiet checkpoint 实际已经执行过 `04`，其 TL 初态为 127；测试曾误用芯片上电时的 TL0。现按已验证静音写构造 typed 初态。
- priority 用例曾错误要求保留函数入口值；独立来源确认 StopPrimary 先清 priority。修订为在真实 Save 节点注入数值，验证 live save／restore。
- Root callback 重入可在取得 score lease 之前运行。先保留两条真实 red，再将 lease 获取移到任何 callback 之前，保留回归。
- 缺少 score operation 消费者时，Root 内部 wrapper 曾掩盖缺失，使 score IO 无人消费。先保留真实 red，再补入口校验；纯规则用例也须显式声明 noop 消费者。

既有 leaf、golden 和以上历史证据均未为制造通过而修改。测试来源、修订说明和失败日志均在各组件清单中保留。

已清理本 scope 中被 strict v3 替代的初次 smoke、两份已完成的 filter 构建，以及空工作目录，共 7 个文件和 12 个空目录；删除清单及原 SHA 见 [cleanup.json](../../../games/yuyuhakusho/verification/root-control/cleanup.json)。其他 owner 仍使用的原始数据、共享 Go 缓存和浏览器目录未清理。

## 尚未完成

完整 Root master-cycle 定价、动态 voice／resource body、bulk 逐 long 的 IRQ 可见性、异步 bus／Z80 steal、完整 World→IRQ／VDP 组合，以及平台最终混音／扬声器输出仍需独立验证。`RequireCost=true` 会在任何效果之前返回 `ErrRuleCostUnavailable`。当前结果不将这些缺口填写为零成本，也不使用 ROM／CPU／RAM VM 或原 PC 时间表替代独立规则。
