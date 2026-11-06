# Root 规则阶段协调与验收

`audio/rootcontrol.AdvanceRules` 已组合 Root 的规则阶段、唯一 score 事务、World 队列和暂停字段。默认请求域与来源审计的 139 个候选逐项一致。完整 master-cycle 成本、逐 long 中断可见性、world→IRQ 调度和普通 PCM 接受／生命周期仍不属于本次组合。

原版新记录共有 1,115 帧，规则组合实际验证 **1,113 帧、8,480 笔有序有效 YM／PSG 请求**。差额是合法 `1F` 场景最初两帧：这两帧执行普通 PCM 接受，尚未接入本协调器，不能计作独立组合通过。

| 独立验收域 | 规则帧数 | YM／PSG 请求 | 前置条件与限制 |
| --- | ---: | ---: | --- |
| strict-v3 五条控制压力流 | 785 | 7,199 | `73`、`A7` 由临时测试 hook 显式接受；`73` 外部分派为空服务，仅验证条件成立后的 Root 控制，不证明 PCM 接受或生命周期 |
| 合法 `82+A6` 清理、真实暂停／恢复 | 180 | 1,246 | 默认 139 候选域；`AcceptDispatch`、`DispatchExternal` 均为空，无测试域扩展 |
| 合法 `1F` 活动 PCM 的 04 前状态延续 | 148 | 35 | 从第 2 帧的合法 typed checkpoint 开始；排除最初两帧 PCM 接受，不向独立实现注入原 RAM／CPU 状态 |

测试逐 tick 比较全部 Root 字段、World queue／pause、score tempo，以及 18 个声部的活动／休止／占用／连音标志和关键数值参数、等待与调制计数。全部有效 YM／PSG 请求按原顺序逐笔比较，未过滤已证明的直接 PSG 写入。完整 score 参数和音序游标仍由既有 42 条 score 流的独立证明覆盖；本报告不扩大本轮数值投影的范围。

## 状态与阶段

Root 持久状态仍只有 `Priority`、`Selected`、`ChoiceBits`、一个 `Fade` 和暂停期间必要的 `PauseRestore`。`World.Queue`、`World.Clock.AudioPause`、Scheduler、18 voices、PCM 和芯片各保留唯一 owner。协调器只接受 detached owner 引用，不增加 RootState、全局时钟、寄存器镜像或历史状态表。

正常顺序是 `Gate→ControlIdle→Tempo→Fade→Queue→Dispatch→Voices→Return`。协调器在第一个 callback 前取得 score 事务租约，Gate 仍早于任何推进阶段。queue 只在实际 queue 阶段构造临时 `engine.SoundPriorityState`，调用既有 consumer 后立即拆回唯一 owner。

`01/05` 先写 fade remaining，再写 steps，然后清 DAC music 四个 flags。它们保留当前 PCM，也继续当前 tick 的 voice scan。remaining 的 `1→0` 不提前执行淡出，steps 的 `1→0` 请求 04，并在消费 queue 前清理旧请求。

04 的规则效果依次为 YM `27=0`、一次明确的 rules bulk、selected=`FF`、六次 keyoff、24 次 TL mute、四次 PSG mute。`RuleBulkClear.HeaderLongs=16`、`VoiceLongs=216` 是 64-byte header 与全部 18 个声部的来源边界，不提供逐 long progress。music reset 则由 score 明确发出 save、header clear、restore、selected reset 节点，保留 primary／secondary 参数。

pause start、hold、resume 都直接结束本 tick，score 只计一次 `Ticks`，不推进 tempo、fade、queue 或 voices。start 在实际 chip view 捕获 24 个 TL，完成 FM／PSG 静音后才建立 pending payload；resume 完成 24 次 FM 恢复后释放 payload，不额外恢复 PSG。

## 原记录与投影边界

v2 的 PC 白名单遗漏了保存列表中的 9 个直接 PSG store。首轮严格比对因此真实失败；原实现与旧 score 原证据均包含 music-start 尾部 PSG `FF`。本轮保留 v2 fixture、原 producer、首轮失败日志和初始测试 SHA，没有删除正确写入以制造通过。

v3 使用独立被动 C filter，记录全部实际 M68K byte store 到 `C00011`。原 `hook_filter.c` 未修改。先行旧 filter 检查真实失败，新 filter 检查覆盖 13 个保存列表中的直接 store、空 PC 白名单、24-bit alias、源 PC／值传递以及 read／word／其他地址的拒绝。capture 的 PSG event 只有实际 byte-store 分支，PC marker 不重复发布数据。

规则回放的芯片起点与原 quiet checkpoint 相同：先按已证明的 04 写入 24 个 TL=`7F`。硬件 power-on 的 TL=0 不能替代这个起点。无时基回放使用实际 YM／PSG owner 消费写入，不把大量 cycle=0 输入堆积到要求 frame flush 的 Interactive DSP 队列。

原真实 Start 请求在 pause 场景第 30、70 帧的 Root 返回后由 World 写入。投影分别保存 `after_tick_pause` 和 `world_pause_request`：先比对 Root 返回值，再施加来源 `start_edges` 明确说明的外部 World 写入。不会把未来的 frame-end World 值反填为 Root 的本 tick 结果。

`1F` 的 checkpoint 先验证原 tempo 与全部 864-byte voice 区均为零，才映射为 `score.State{Ticks:2}`。Root 只映射已公开的语义字段，其中 `ChoiceBits=1`；04 随后将它清零。原 PCM mode `C1` 在 04 前后保持活动属于 native 记录，本协调器没有据此宣称已实现 `1F` 接受或 PCM 生命周期。

## 失败处理与成本

Root 和 score 的 operation consumer 都必须显式提供。所有 hook 在实际阶段执行，DAC feedback 现场读取。错误后 score 不提交；caller 必须丢弃所有 detached Root／World／芯片／PCM owner。`RuleOutput.Score` 的 intents 与兼容 priority bool 已在真实节点处理，只用于诊断，不能在 tick 末重放。

先行用例保留两条 Root 重入 red：Gate callback 曾能递归完成第二个 tick，score Tick-enter callback 的嵌套 Root 也曾先执行 Gate callback。修复后，嵌套入口在任何 Root callback 前被唯一 score 租约拒绝。另有缺少 score consumer 的真实 red，修复后两个 consumer 的缺失都在首个 callback 前拒绝。

`RequireCost=true` 始终在效果前返回 `ErrRuleCostUnavailable`。未恢复的 bulk 粒度、资源成本与异步 IRQ 不能按零成本、Go 函数树或原 PC 时间表补齐。非零 control／duck／delta／signal／FM3 投影、未服务的外部 dispatch 和 `7F` 私有 duck 均不支持。

## 复跑与产物

在本 worktree 根目录执行：

```sh
python3 -B scripts/root-control-orchestration-verify.py
python3 -B scripts/root-control-orchestration-verify.py --run-tests
```

第一条核对冻结来源、历史 test-first SHA、原 fixture 绑定与确定性重建；第二条再顺序执行本包 native-race、实际 Wasm 测试和 vet。Go telemetry、cache、module cache、临时输出都限制在 worktree 的 `.local` 中，不安装工具，不启动原 core。

接口见 [API](AUDIO_ROOT_CONTROL_ORCHESTRATION_API.md)，先行失败与订正见 [FAILURES](AUDIO_ROOT_CONTROL_ORCHESTRATION_FAILURES.md)。验收入口为 `games/yuyuhakusho/verification/root-control-orchestration/validation.json`，冻结来源为同目录 `frozen-sources.json`。原 red、v3 与 legal-v3 来源及所有最终命令／结果均单独保留。

仅清理本次被动 filter／确定性重建的临时目录；验证记录、必要来源副本和 fixture 保留。未修改冻结 chips、PCM、engine、leaf、平台或总索引，未提交或暂存。
