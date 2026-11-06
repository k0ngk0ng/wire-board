# 音频资源与命令接线

`resourcebridge.Library` 连接同一批 score、parser metadata 与 PCM 资源。score 保留音序和采样描述，`pcmassets.Catalog` 单独持有全部 129 个编码 payload；Device 配置通过只读回调借用这些字节，不另存采样副本。

```go
library, err := resourcebridge.Decode(scoreJSON, pcmJSON, parserJSON)
pcmConfig, err := library.PCMConfig(origin, restorePan)
driver, err := score.New(library.Catalog())
hooks.Parser = library.Parser()
```

输入是按可信构建清单验证、解压后的原样 JSON。decoder 进一步验证音乐导入、六个共同描述和 parser 对同一 score 的绑定；任一失败返回 nil，不发布部分目录。`Identity` 由三份原始资源的有序内容摘要派生，用于 checkpoint 绑定，不能替代下载来源认证。原始 JSON 不留在 Library 中，描述配置按需派生为独立值切片。

`PrepareOrdinary(code, status, choiceBits)` 使用现有 `engine.SelectPCMCommand` 处理当前临时投影。调用方必须在原实际仲裁阶段传入 status，并在整体事务成功后提交返回的 choice。拒绝请求不生成 publication；接受后保留当时的 StartDual，后续不能重新读取模式替换。Root 请求范围与 priority／queue 的管理仍归 Root；本包只准备其已授权的普通请求，排除内部 `7F` 和未支持的 duck／choice profile。

`PrepareMusic(intent)` 只翻译 score 已接受的 `dac` 与 `dac-pan`。前者使用音乐专属六字段描述和独立轨道 pan，后者生成 `SetRestorePan`，不重启样本。该方法不再次仲裁 blocked／priority。`Feedback(status)` 生成当前阶段的只读 score 投影，分别解释 bit7 Blocked、mode 的两位 Priority 和整个 marker 非零的 PanRedirect，不缓存反馈。

准备出的 Input 不带新时钟决策。调用方须在真实 bus hold 与原实际发布阶段调用 `mixed.Runtime.PublishAccepted`，并与 Root、score、queue 等全部工作分支一起提交。本包没有计算完整 driver 周期、安排世界／IRQ 或调用平台播放器。

原版对照与复跑记录位于 [audio-resource-bridge](../../verification/audio-resource-bridge/)。验证入口为 `scripts/yuyuhakusho/verify_resource_bridge.py --mode native|wasm|vet --out <新的仓库内目录>`；离线冻结审计使用 `--audit`。音乐意图夹具由 `extract_pcm_bridge_intents.py --out <新的仓库内文件>` 从原有 42 条轨迹字面提取，不运行原 CPU 或改写原期望。

正式发布继续按通用规范对音频载荷做无损压缩。当前三份资源均为 gzip 文件；本包不要求把编码采样展开为大体积 WAV，也不直接把研究／验收夹具纳入游戏发布资产。
