# Kind 10 接触与捕获分支

Court 4 的 16 个角色／控制配置首次运行有 12 个通过、4 个停在 `original prop kind 10 contact not reconstructed`。4 个失败前态分别来自视频帧 15,479、11,357、5,699 和 20,276，均为 Kind 10 道具的动作 13。完整前态保存在 `engine/testdata/kind10-runtime-failures.json.gz`；没有修改种子、计时或缩减可选配置。

原 bank 5 `$876F` 的 REA 档案为 `ev_85a7bb29ede12dea40b0e12334c9754763ff0df7584b2b182200d7f250638b62`，位于 `.local/basketball/rea/prop-attack/analysis/function-0x876f.json`。`$8786..$87BA` 明确区分捕获和普通碰撞：

- 目标须为人物，整数高度为 0；高度小数不会阻止捕获。
- 扫描对象 0～5 的附着标志。只有最高位开启且 `link & 0x7f` 等于目标时才阻止捕获，不能改用低四位父对象掩码。
- 道具当前动作须为 3。满足条件时设置人物动作 `0x50`、道具动作 6，并写原 `$056C = 4`；即使独立函数输入的发射对象为 5，也写固定值 4。
- 捕获不修改冲量、耐力、动画、准备请求或所有者，也不发出普通撞击音。条件不满足时才进入原普通反弹分支；目标是另一道具时直接走道具响应。

`CombatContext.PlayerAttachments` 明确提供玩家部分的扫描输入；另外两个附着值来自当前辅助对象。缺少输入时，不猜测可以捕获。`CombatStep` 和 `InteractionStep` 通过 `ChangeCaptureCounter`／`CaptureCounter` 传出原写入；公共准备阶段保存到 `WorldActionState.CaptureCounter`。后续响应失败时，整个阶段返回初态，不泄漏捕获写入。原 RAM 提取器也保留非零 `$056C`，零值保持旧夹具格式。

`verify_kind10_contact.py` 执行未修改的原 `$8653 → $876F` 指令，只截获 `$C000` 音效请求。1,162 组输入覆盖人物／道具目标、动作 0／3／6／10／13／16、整数与小数高度、六个可能的附着来源、掩码差异及原固定对象引用，其中 32 组进入捕获分支。这是配置输入的原函数隔离验证，不是自然比赛。源摘要和 ROM 映射摘要保存在 `engine/testdata/kind10-contact-source.json`。

先写原函数、未知上下文／回滚和真实前态测试，再实现分支；初始失败记录为 `.local/basketball/rea/court-four/kind10-before.log`。4 个真实前态分别继续 24 个正常视频帧，每帧同时验证 JSON 恢复后的重放一致性。定向原生、race 和 Wasm 记录为 `kind10-{after,race,wasm}.log`，另有 `capture-source-{before,after}.log` 验证原字节提取。

```sh
source scripts/env.sh
.local/board-art-env/bin/python -B scripts/basketball/verify_kind10_contact.py
go test -race ./games/basketball/engine -run '^(TestKind10.*|TestOriginalCompleteCombatPhase|TestOriginalCompleteInteractionPhase|TestInteractions.*|TestOriginalPropAttack|TestOriginalPlayerPropContacts|TestWorldPreparation.*)$' -count=1 -v
```

本次恢复的是原接触阶段及捕获写入。人物动作 `0x50` 的后续处理仍需独立恢复，不能将接触函数的捕获验证称为完整捕获玩法验收。4 个真实失败走普通反弹路径；完整比赛重跑由各配置的独立记录确认。


后续原人物 `$50..$56` 追踪证明 `$056C` 是箱内挣脱倒计数，初始 4，每次有效落地递减，减至负数才脱离；不是恒定的对象编号。Go 字段已改为 `CaptureCounter`。早期开发存档及原样本的 JSON 键 `captureObject` 保留兼容，不新增第二份状态。
