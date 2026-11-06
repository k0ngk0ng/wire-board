# 请求消费与普通 PCM 分派验收

本项验证实际队列字段访问和普通 PCM 请求的完整 driver 调用。队列只有 World／Root 的实际 owner，普通 PCM 分派复用资源目录、仲裁与发布实现；所有阶段和后续声部扫描使用同一个临时时钟与预算。完整音乐／primary／控制指令、非 idle fade／pause、world IRQ 和平台播放仍待接通。

## 原版对照与先行测试

409 个原版 profile 分为 287 个 queue consumer 和 122 个完整 ordinary driver 请求。queue 覆盖既有 12,156 个边界输入的控制分支形状、空队列及各 slot 的普通／特殊请求；ordinary 覆盖正式目录中的所有请求，包含黑龙 `F0` 重映射。queue 从 consumer 入口开始，ordinary 从 driver gate 执行到实际 return。实际只读 priority table 位于研究 ROM 的 `0x3a50e`，共 255 字节。

两份固定 core 分别以 observer 关闭／开启执行，共 1,636 次；完整原生状态、RAM、字段访问和跨 core 结果一致。保留 43,239 个源步骤、4,438 次 owner 字段访问和 3,776 次端口／总线事件。固定 auxiliary 使用已知 empty factory，在 master 0 持有总线，普通描述和初始状态与实际 mixed 适配器一致；这些 profile 不声称覆盖 world IRQ 或活动声部与新请求的全部组合。

失败计划、Go E2E、原版 oracle 和采样脚本先于生产改动冻结，`test-first.json` 绑定其摘要，`missing-api-red.jsonl` 保存真实缺少 API 的失败。生产实现后未修改测试、采样脚本或 oracle。

首次 engine 验证通过两项新增测试，包含全部 12,156 个原边界输入。首次 clock 验证有 1 条通过、411 条失败；原版步骤显示 predecrement queue read 为 10 CPU，生产代码误用 8 CPU。修正生产常量后，287 个 queue profile 全部通过，完整记录为 290 条通过、122 条失败。此时 ordinary 的结束时刻和结果已相同，但选择字段直接访问 Root，绕过了实际适配器的访问记录。生产改为使用委托同一 Root 的 choice port，第三次新增 412 条记录全部通过。各轮日志按原样无损保存。

## 验证边界

`ConsumeSoundQueueObserved` 与旧纯值接口使用同一套规则，新增验证还在读取较低 slot 前写入新请求，证明消费使用 live owner。遗漏、重复、过期和失败 effect lease 均拒绝。整拍验证比较实际 Root／mixed 的端口、仲裁、choice、发布区间、顺序、时刻及 return clock；预算耗尽不提交 score，未计时的分派继续明确拒绝。

最终原生 race 与实际 Node/Wasm 各通过 118 个顶层测试、8,206 条 clock／Root 记录，无跳过；其中 scoreclock 为 89 个顶层／8,104 条，Root 为 29 个顶层／102 条。用时分别为 430.974 秒和 277.326 秒。独立 engine 队列检查两端各通过 4 个顶层／6 条记录，相关三包 vet 通过；未运行其他玩法矩阵。

当前 source 和最终验证以 `validation.json` 为准；此前绑定了已修改文件的清单保留为历史证据，不回写旧摘要。API、唯一 owner 与联合提交要求见 [REQUEST_QUEUE.md](../../audio/scoreclock/REQUEST_QUEUE.md)。[完整复刻待办](../../../../docs/research/md-yuyuhakusho/TODO.md)中的 driver／world／平台播放仍未完成，试玩构建尚未开启完整音频。

## 复跑

使用仓库内新的输出目录，脚本拒绝覆盖已有日志，并将所有 Go 缓存与临时目录限制在 worktree 内。原生与实际 Node/Wasm 的完整 clock／Root 回归分别运行；engine 只选声音队列检查，不运行其他玩法矩阵。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_request_queue_clock.py --mode native --out .local/md-yuyuhakusho/request-queue-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_request_queue_clock.py --mode wasm --out .local/md-yuyuhakusho/request-queue-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_request_queue_clock.py --mode engine-native --out .local/md-yuyuhakusho/request-queue-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_request_queue_clock.py --mode engine-wasm --out .local/md-yuyuhakusho/request-queue-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_request_queue_clock.py --mode vet --out .local/md-yuyuhakusho/request-queue-repeat
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/verify_request_queue_clock.py --audit
```
