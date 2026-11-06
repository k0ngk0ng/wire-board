# 音频资源接线验收

新增 `audio/resourcebridge` 已把同一批音序、parser metadata 与 PCM 资源连接到真实命令／合成层配置。编码采样只有一个不可变 owner；普通请求沿用现有优先级／声道仲裁，音乐只翻译已接受的采样与声像意图。

测试在实现前冻结，缺 API 的真实 red 已保留。原生竞态检查与实际 Node/Wasm 各通过 6 个顶层测试、33 条测试记录：

- 129 个原版描述的 ID、采样率、flags、路由、长度和全部编码字节一致；新配置及描述枚举不暴露可变内部数据。
- 真实生产资源替换测试专用 payload 回调后，23 条原版命令流程的 1,274 帧、519,256 个物理事件、937,609 个立体声采样帧及完整 YM／PSG／FIR 状态保持一致。各流程中途进行组合状态保存恢复。
- 在明确的普通请求 idle profile 中，1,350 个原版仲裁窗口一致：拒绝 512、single 113、slot A 346、slot B 379。未支持的 duck／choice profile 明确拒绝。
- 从原 42 条音序轨迹字面提取的 9,230 个 DAC 和 2 个 dac-pan 意图全部正确翻译，覆盖六个音乐采样描述，不重新仲裁或把声像更新变成重启采样。
- Blocked／Priority／PanRedirect 的独立含义、非法跨协议输入、损坏／不同来源的资源集和原子失败通过。`go vet` 及全仓库构建／测试链接通过；没有运行全仓库行为测试。

现有三份运行数据通过 gzip 无损还原与内容校验：压缩后共 810,853 bytes，原始 JSON 共 16,775,272 bytes。详情见 [input-compression.json](input-compression.json)。本项没有展开或上传预录 WAV，也没有上传任何 OSS 对象。

意图提取器从未修改的原轨迹生成夹具，第二次导出逐字节相同，重复副本已删除。旧原版夹具、期望和既有生产文件均未修改。命令、环境及完整来源／产物绑定见 [validation.json](validation.json)，可用 `scripts/yuyuhakusho/verify_resource_bridge.py --audit` 离线审计；复跑使用 `--mode native|wasm|vet --out <新的仓库内目录>`。

本项不安排世界／IRQ、不给尚未恢复的音序成本赋零值，也未切换平台试玩构建。完整对局声音调度、Root／score／PCM 的带时序协调与平台播放仍待完成。
