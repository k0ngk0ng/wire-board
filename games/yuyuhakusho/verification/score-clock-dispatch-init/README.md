# 音乐与 primary 分派初始化验收

本项将正式 catalog 的音乐与 primary 技能音轨初始化接入同一个 request／running 执行器。音乐停止旧声部、恢复必要参数并装入新音轨后结束当前拍；primary 装入后继续本拍声部扫描。所有阶段沿用同一时钟与预算，普通 PCM 接口的支持范围与调用契约保持兼容。

## 原版完整调用

63 个 profile 覆盖全部 23 个正式音轨：各自在空 score 和已有活动音乐上分派，共 46 例；12 例在切换音乐前停止各个 primary；完整多 primary、tied 和 claimed 共 3 例；已有 pending selected 而 queue 为空共 2 例。输入通过现有独立规则构造，再由两份固定原版 core 从实际 gate 执行到 return，不以独立规则生成期望输出。每例均确认到达所选 handler。

两份 core 分别以 observer 关闭／开启执行，共 252 次；完整 serialized state、RAM、PSG context、步骤和跨 core 结果一致。保留 91,624 个源步骤、21,603 次 Main IO、191 次 PSG 写入及 152 次 Root header 字段访问。固定 auxiliary 和主时钟边界沿用已验证的 running profile，不包含额外 IRQ、活动 fade／pause 或 world 请求生产。

Go E2E 比较实际 mixed 的完整端口值、顺序、时刻和最终 clock，比较实际 priority clear／save／restore 与 selected reset 时刻，并核对 native header／voice 投影。完整 Go 输出与 state 还使用同一原版 feedback 序列对照旧规则接口。catalog header 全 23 项顺序和返回值隔离、Root effect lease 与失败不提交 score 另外验证。

## 先行失败与生产修正

失败计划、63 例原版 oracle、E2E、输入构造和 capture 源码在生产改动前冻结，真实缺少 API 的 red 保存在本目录。实现后未修改正式测试、输入或 oracle。

首次 60 条记录通过、7 条失败，其中 6 个 profile 失败及其父测试失败。原版完整调用表明：all-primary stop 的 PSG3／noise 恢复 secondary slot 17，即使不活动；普通 track-end 才恢复对应音乐 slot。原规则将两者合并，导致额外噪声写入、错误的最后 flags，以及部分时刻错误。仅修正 canonical stop 目标与计时层对应目标后，第二次 67 条记录全部通过。

`diagnostic-*` 和 `first-trace-diagnostic.jsonl.gz` 只用于失败后的定位：临时 overlay 绕过最终 clock 比较以显示第一个 IO 分歧，不属于通过证据。正式 E2E 的全部断言始终保持冻结；原版 `0x38870` 至 `0x3888e` 的目标选择与完整 native fixture 支持生产修正。旧 driver 源码及四个原 coordinator 文件均保留原样归档。

最终原生 race 与实际 Node/Wasm 各通过 122 个顶层／8,273 条 clock／Root 记录，其中 scoreclock 为 93 个顶层／8,171 条，Root 为 29 个顶层／102 条。两端另各通过 34 条 score 契约及 32 条原轨迹记录；所选 15 条较短原轨迹通过普通／observed 两个接口验证，每个接口覆盖 1,951 拍。整轮用时分别为 499.769 秒和 305.291 秒，无跳过，三包 vet 通过。未重复完整长音轨或其他玩法矩阵。

当前来源、完整回归与压缩日志由 `validation.json` 绑定。已变更来源的旧清单继续作为历史证据，不回写其摘要。API、bulk clear 和联合提交边界见 [DISPATCH_INIT.md](../../audio/scoreclock/DISPATCH_INIT.md)。[完整复刻待办](../../../../docs/research/md-yuyuhakusho/TODO.md)仍保留控制、fade／pause、world、联合 checkpoint 和平台播放；试玩构建尚未开启完整音频。

## 复跑

使用仓库内新的输出目录。脚本运行完整 clock／Root 回归、相关 score 契约，以及 15 条较短原音轨的普通／observed 接口验证，不重复完整长音轨或其他游戏矩阵。所有 Go 缓存与临时目录限制在 worktree 内；Wasm 使用实际 Node 执行器。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_dispatch_init_clock.py --mode native --out .local/md-yuyuhakusho/dispatch-init-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_dispatch_init_clock.py --mode wasm --out .local/md-yuyuhakusho/dispatch-init-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_dispatch_init_clock.py --mode vet --out .local/md-yuyuhakusho/dispatch-init-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_dispatch_init_clock.py --audit
```
