# 音频无损压缩验收

实现前登记失败场景：

| 场景 | 预期 |
| --- | --- |
| 8、16、24 位整数 WAV，单声道／立体声或不同采样率 | 保持采样率、声道、有效位深、帧数和解码 PCM SHA-256，只有实际更小才发布 FLAC |
| IEEE float32 WAV | 不量化成整数，不把有损候选标成无损；保留兼容 WAV 并记录格式限制 |
| 极短声音的 FLAC 更大 | 保留原 WAV，记录候选实际体积和未节省依据 |
| 源文件缺失、摘要不符、格式不支持或发布路径非法 | 非零退出，原发布清单和有效发布文件不变 |
| 再次运行、已有产物或研究缓存 | 校验源身份和产物摘要后复用；不恢复已压缩的原始 WAV，不重复改写相同产物 |
| 研究缓存缺失但已有固定来源 URL 和摘要 | 仅恢复到研究缓存，核对原字节身份；不能把原 WAV 重新放入发布目录 |
| 原 MP3、图形、字体及逻辑声音 key | 不重压、不改规则；逻辑 WAV 路径通过 `aliases` 指向实际 FLAC |
| 资源清单和音频来源 | 实际大小、SHA-256、MIME 对应发布对象；保存原来源、原字节身份、PCM 身份、变换与压缩比 |

命令：`node games/pokepath/build/compress-audio.mjs`，再执行 `node --test games/pokepath/tests/audio-lossless.test.mjs games/pokepath/tests/native-reference.test.mjs`。验收产物位于 `.local/pokepath/verification/audio-lossless/`。源 WAV 留在 `.local/pokepath/research/audio-originals/`；保留 WAV 例外必须有逐文件依据。
