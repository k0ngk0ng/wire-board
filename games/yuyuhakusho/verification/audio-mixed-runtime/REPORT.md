# 音频组合运行层验收

`audio/mixed` 已将独立 PCM Device、真实物理 YM 端口和 FM／PSG 合成器接为可恢复的生产接口。每项操作使用临时工作分支；失败不提交任一 owner 的部分状态，也不返回部分音频。

原生竞态检查和实际 Node/Wasm 各通过 5 个顶层测试、37 条测试记录：

- 23 条原版 PCM 流程，共 1,274 帧、519,256 个物理事件、937,609 个立体声采样帧。完整 YM／PSG／FIR 状态与原版一致；每帧前经过 JSON 保存恢复。
- 四条原版总线场景的 71 次 Main 访问和 7 次实际 mailbox 忙碌重试一致。组合层直接参与真实 `ServeYM`，未使用反馈回放或假总线。
- 无效组合快照、可变 FIR 共享、芯片容量耗尽时的整体回退，以及分支的资源借用、pending ledger 与 bus hold 独立性通过。
- `go vet` 通过；新增运行层后的全仓库构建及测试链接通过。构建检查没有运行全仓库行为测试。

测试先于新增实现，并保留缺 API 的真实 red。第一轮所有原版采样流程已通过；Fork 独立性测试的准备代码误将原版双声部描述作为 Single 发布，被原有校验正确拒绝。修正为原版已有的第一条完整输入；旧测试、失败日志、修正说明和实际差异均保留，原版夹具与期望未修改。

可重复命令、环境、源码／证据 SHA 见 [validation.json](validation.json)。`scripts/yuyuhakusho/verify_mixed_runtime.py --audit` 可离线核验本批冻结来源；行为复跑需指定 `--mode native`、`wasm` 或 `vet`，并通过 `--out` 使用新的仓库内输出目录。

本批没有把 Runtime 接入平台试玩版本。完整世界／IRQ 排程、音序周期成本、Root／score／普通 PCM 协调和平台 `openPCMStream` 接线仍未完成；本批不声明完整对局音频通过。
