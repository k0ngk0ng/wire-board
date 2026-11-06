# 乐器选择身份与资源绑定验收

新增 `BindInstrumentWork` 将具体乐器的 sound／work 引用保存在唯一 `VoiceState.Tone` 中。音色内容继续共享，不新增源 instrument index、第二份音色字段、RAM 或事件历史。native View 派生原音色 ID；bound Snapshot 保留具体选择，拒绝含糊的旧引用或旧资源集合。

新 gzip 资源为 9,001 bytes，含 106 个定义、68 个初始选择与 892 个控制选择。元数据逐条按正式 score 的逻辑事件、原 ROM 音色内容及原选择来源核对；同音色／不同查找工作的 8D 条目 8／13、95 条目 8／14 保持不同引用。正式 F5 FM 选择还含旧研究列表未列出的较大条目，导出器重新按原数据内容校验，没有以旧列表推断。

`DecodeWithInstrumentWork` 把第四份资源纳入统一音频身份与 PCM checkpoint 身份，同时继续绑定同一 score、parser 与唯一 PCM payload owner。三参数旧入口、原 score／PCM／parser 资源和原存档含义不静默更改。

测试均先于各自新增实现冻结，真实缺 API 的 red 保留。原生竞态与实际 Node/Wasm 各通过 6 项顶层测试、61 条记录，无跳过：

- 旧 42 条完整音序轨迹，加上 8D／95 和抢占、恢复、未初始化 fallback 等 6 条 bound 轨迹，共 153,340 帧、1,780,636 个有序意图。原 native View、有序声音请求及保存恢复结果一致。
- 两组同音色别名保持独立工作引用；不可变目录不被修改，快照保存具体选择，旧／新资源引用混用明确拒绝。
- 缺少或重复 selector、缺定义、身份／fallback／目录不符、混入原索引字段等均拒绝，不返回半成品。读取资源与返回定义没有可变共享。
- 统一资源入口创建实际 driver，parser 与同一目录绑定，129 个编码 payload 保持完整；跨三／四资源集合的 PCM checkpoint 恢复拒绝。

额外的 Root／声音时序消费者竞态回归和静态检查通过。持久 `State`／`VoiceState` 的字段声明未增加，见 [persistent-layout.json](persistent-layout.json)。导出及来源证据再次生成逐字节一致，重复静态输出目录已清理，未运行新的原生 core。

命令、环境、来源与产物绑定见 [validation.json](validation.json)。离线审计为 `scripts/yuyuhakusho/verify_instrument_work.py --audit`，复跑为 `--mode native|wasm|vet --out <新的仓库内目录>`。原 goldens 和旧测试未修改，必要旧实现已归档。

本项完成了具体选择身份与工作量绑定。完整乐器传送、现场参数、PCM 发布、独立世界／IRQ、整场音频调度及平台播放仍未完成；`OpResourceUnavailable` 没有因此被改成零成本。
