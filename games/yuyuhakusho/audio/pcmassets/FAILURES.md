# PCM 原始资源目录的先行失败场景

本清单先于本目录的生产实现与正式资源导出。资源身份、codec 算术与播放器时序分别验证。

- 普通 `06..7e`、飞影黑龙的合法 `f0` remap、六个 music DAC 依赖遗漏或映射到错误 descriptor；将 `f0` 误当普通减六索引。
- 将 `00..05`、FM／PSG 命令或未恢复的 `f1..ff` 当样本。`7f` 的真实单字节静默 descriptor 必须独立保留为 InternalSilent／Diagnostic，不进入有限 WAV 可播放列表，也不能冒充 control。
- 普通八字节发布的第七字节与 music 六字节发布／track pan 混用；rate、flags、routing 或 descriptor 身份被篡改。
- 音频内容重复保存在两个资源文件或两个运行时 owner；score 原 JSON 的六个 payload 必须只借读，不改旧 schema 2、不再次写入新普通资源。
- encoded SHA、长度、codec 或 canonical base64 不符；缺少引用、重复 ID／code、外部 music 原样 JSON 摘要不符、未知或尾随 JSON 被接受。
- 非法输入部分返回可用 catalog，读取错误被吞掉，无界输入占用内存，或只读 handle 暴露内部可变 slice。
- DPCM 高／低 nibble 顺序、8-bit 回绕、最终累加器、末字节、完整长度有误；原生与 Wasm 解码摘要不同。
- 为 7f 的 128-byte 预取或普通尾部预取盲目复制相邻 ROM、用零填充伪造原读，或删掉原 bus read。目录只提供声明 payload，超长读取返回未知，由命令时基层保留实际读取边界及未消费语义。
- 将原 codec 母表对照扩大为全混音／扬声器／平台播放已通过；将 payload catalog 打包进正式服务端镜像或上传未授权素材。

预期先取得缺失 `Decode`／只读 lookup API 的真实 Go 编译 red，再实现。完整资源用固定 ROM、已有 native WAV descriptor 元数据、score 原 JSON 和原生母表的独立 fixture 对照；不从生产 exporter 或 Go 解码器生成预期值。
