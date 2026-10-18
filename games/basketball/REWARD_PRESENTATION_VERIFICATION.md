# 奖励阶段复原记录

## 实现前的验证范围

先在 `verify_reward_phase.py` 和 `verify_reward_presentation.py` 列出失败场景，再生成原版调用夹具、编写 Go 对照测试并保存失败记录，最后实现阶段处理器。覆盖重复发奖、满库存、计数器回绕、退出时机、消息和声音顺序、随机流、角色恢复、退出后的角色更新以及 JSON 恢复。

来源为 `POSTRESULT_ANIMATION_SOURCE.md` 对 bank4 `BD8E..BE6D` 的分析，ROM SHA-256 为 `0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b`。不把声明初始状态的 CPU 调用称为自然通关。

## 阶段逻辑

`StepOriginalRewardPhase` 恢复入口发奖、背景 26 和 CHR 页请求、计数器、消息、随机声音及退出，停在 `BE63` 的角色循环之前。

- 仅首次进入尝试插入奖励；库存已满仍展示奖励，不覆盖已有物品。
- 第 40 次继续调用按 28、30、29 的顺序请求原消息。
- 第 112 次继续调用直接退出，不再消耗阶段内部随机数；其他继续调用先取随机数，再检查 A／Start。
- 退出恢复原角色缓存，然后切换到 Scene 2、phase 0。

`StepOriginalRewardPresentation` 在上述处理后调用三个原角色处理器，包括已经切换到 Scene 2 的退出调用。外层 Scene 1 的一次随机调用和绘制列表清理由调用方负责。

## 可重复执行证据

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_reward_phase.py --check
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/verify_reward_presentation.py --check
go test ./games/basketball/engine -run '^TestOriginalReward(PhaseSource|PresentationChains)$' -count=1
```

阶段夹具包含 695 次原版调用，覆盖已有战役奖励组合及计数器、输入、随机种子边界；保存完整调用前后 RAM。阶段对照已通过。压缩夹具 SHA-256：`f3fecac5866d80ea070b793255c9c11b5bd2284b8357f259e16600ca2057f39b`。

完整处理夹具包含 15 条连续链、1,146 次调用；每条从原版确认阶段的四角色尾部开始，然后连续执行奖励阶段及每次的三个角色更新，保留原 C00C 投影。覆盖空库存、已装备、满库存，以及自然计时退出和第 0／1／40／112 次调用的按键。前后状态连续传递，不用后续快照覆盖执行状态。压缩夹具 SHA-256：`619cc026764e8a6d0eab1ad12ffdcaeaec48bb2abe053d52fc13f99f206ae6f2`。两份夹具重复生成校验一致。

实现前记录为 `.local/reward-phase-red.log` 和 `.local/reward-presentation-red.log`。接入完整角色处理之前，连续链准确暴露 action 62、cursor 3 尚未复原，保存在 `.local/reward-presentation-actors-red.log`。

## 尚未完成

完整连续链已与扩展角色处理器一起通过，记录为 `.local/reward-presentation-green.log`。夹具明确省略音频驱动、背景／消息上传及淡入淡出的 NMI 等待；不证明实际视频时序。平台生命周期、奖励画面与文字、音频和下一场随机流交接尚未接入本组件，不能将本记录视为完整战役或发布验收。
