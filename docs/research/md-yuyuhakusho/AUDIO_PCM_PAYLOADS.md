# 原始 PCM／DPCM 资源与唯一归属

新增 `audio/pcmassets` 统一普通效果、扩展 `f0`、内部静默 `7f` 和音乐 DAC 的原始 DPCM 资源。129 个 payload 均以完整 encoded SHA-256 为 ID，共 437,940 个编码字节；从初始累加器 128 连续解码的 875,880 个 byte 及每条尾部，均与固定原生 codec 母表投影一致。Native race 和 Wasm 的完整证明摘要相同，`go vet` 通过。

目录、资源和接口位于 [pcmassets](../../../games/yuyuhakusho/audio/pcmassets/README.md)、[audio-pcm-payloads.json.gz](../../../games/yuyuhakusho/data/audio-pcm-payloads.json.gz) 和 [资源 manifest](../../../games/yuyuhakusho/data/audio-pcm-payloads-manifest.json)。最终检查、来源、真实失败和文件 SHA 由 [verification/pcm-payload/validation.json](../../../games/yuyuhakusho/verification/pcm-payload/validation.json) 记录。

## 范围与内容

| 域 | 数量 | 资源归属 |
| --- | ---: | --- |
| 普通 `06..7e` | 121 | 已有原生有限 WAV 采样的 descriptor 元数据，encoded bytes 从固定 ROM 逐项核对。 |
| 扩展 `f0` | 1 | 飞影黑龙发出的合法命令；经扩展分派映射到独立 descriptor，不替换为直接 `7a`。 |
| 内部静默 `7f` | 1 | 真实长度 1 的 DPCM descriptor，独立 Diagnostic lookup，不加入有限 WAV 可播放列表。 |
| 音乐 DAC | 6 | 保留已冻结 score schema 2 的 descriptor ID 和 payload；新普通资源只记录原样 score JSON 摘要及 descriptor 引用。 |

这 129 个 payload 互不相同。新普通文件保存 123 个，共 417,592 个编码字节；6 个音乐 payload 共 20,348 字节，仍只存在原 score 文件。普通资源 JSON 为 600,207 字节，gzip 为 415,208 字节，gzip SHA-256 为 `dbfa09e9893dcd9c661ef085f8d5215a202c502ae4277c6b9f9992a3384827de`。

score 资源原样解压 JSON 为 3,159,957 字节，SHA-256 为 `735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830`。其 gzip 摘要是另一份身份，不能拿 gzip SHA 校验传入 `Decode` 的 JSON。旧 score 文件与源码均未改动。

## Descriptor 与内存

普通 descriptor 的 ID 来自 `sample_id/native_rate/flags/protocol/routing_or_pan` 的规范 JSON 摘要。music ID 继续使用 score 已有的 `sample_id/native_rate/flags/routing` 摘要。代码、原地址和 descriptor 数组索引不作为 payload 身份。

普通八字节 producer 的第七字节仍保留其 routing 语义；音乐只发布六字节，声像来自 track。库通过不同的 `Protocol` 表达两者，不把音乐描述项后面的原 byte 误当 ordinary mode。

`Catalog` 私有 map 持有 encoded arrays；只读 `Payload` handle 不暴露可变 slice。`Bytes` 显式创建调用者副本，命令设备应优先按 sample ID 使用 `PayloadLength`／`PayloadByte` 函数适配，避免把整个目录复制进设备状态。descriptor lookup 返回值副本。

score 当前只校验 payload 后丢弃其 bytes，因此两个 decoder 读取同一个临时原 JSON 不会产生两个持久 byte owner。将来若迁移 score schema，须另行测试资源身份与兼容性；本轮没有重写旧的 schema 2 冻结证据。

## 原始来源与证明边界

固定 ROM SHA 为 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`。普通 121 项逐一对照已冻结的 `data/audio-assets.json` 中 descriptor、header、长度、encoded SHA、rate、flags 与 routing。音乐六项同时核对 score encoded bytes、旧 descriptor ID 和已有 source provenance。

`f0` 的扩展分派使用原表 index 122，源 descriptor 为 `0b80008014814000`，6670 个编码字节的 SHA 为 `c6409ebd28c594205970069fbc3233241cf15dc08ce8b847a5fa118b3e1a5382`。原地址仅保留在研究 provenance，不进入生产目录。`f1..ff` 未经可达性和源范围验证，不由该目录盲目开放。

完整解码预期由既有原生捕获的 256-entry 高／低 nibble 母表生成，不调用新 exporter 或 Go 解码器。原母表合并 SHA 为 `be79e4eb4329b2f21dfda3d284a5b03afbd2dd8b1694c4366190072de0ef0fe8`，原 `audio/pcm` 已逐项验证所有 65,536 个 accumulator／encoded 组合。本轮另将全部 129 个 payload 从同一初态连续推进，比较 encoded SHA、decoded SHA、最后编码 byte、最后两个输出和最终累加器。

该检查不是重新运行 129 条原生播放流。121 普通声音沿用已有原生 WAV 采样证明，`f0` 本轮验证原资源映射与完整 codec 值，命令生命周期由独立 `pcmcommands` 验收；全部混合 PCM、实际输出时刻和平台播放仍分层处理。

`7f` 的源 descriptor 长度为 1，并不意味着原播放器只做一次总线读取。可能超出声明长度的启动预取仅用于原流水阶段；本目录的越界读取明确返回未知，不复制相邻 ROM、构造零 padding 或宣称未证明的 byte 值。

## 先行失败与复核

首次 Go 运行保留了 `Descriptor/Catalog/Decode` 缺失的真实编译 red；所有主要 E2E 场景先于生产实现。后续复核发现 music flags 的合法零值会掩盖 missing／null 字段，新增四个失败子场景后才加入严格字段存在性检查。原五项测试不改断言，新旧源码与日志分别保存。

两次工具准备错误单列保存：旧 finite helper 拒绝伪 code128，因此 `f0` 改用显式扩展源解析，没有扩大旧 helper 的合法范围；缩短的 Wasm PATH 遗漏现有 Node 目录，修正 executor 环境后重跑。两者都没有更改原音频数据或放宽对照。

普通目录拒绝未知字段、重复 code／ID、重复 JSON 字段、尾随 JSON、非 canonical base64、长度／SHA／codec 不符和域混用。music 部分验证整份原样 JSON 身份、顶层结构和共享 PCM 字段；其余 score 规则交给 score decoder，资源认证仍由调用方可信构建清单完成。

## 复跑与发布边界

无需 ROM 或原核心即可运行 portable Go 验收，输出目录必须是工作区内尚未存在的新路径：

```sh
python3 -B scripts/yuyuhakusho/pcm-payload-verify.py \
  --out .local/md-yuyuhakusho/pcm-payload-review
```

该脚本串行执行 native race、Wasm 和 vet，使用工作区内 Go cache、telemetry、临时文件和 clang cache，并拒绝测试期间发生源码变化。重新导出资源使用固定本地 ROM：

```sh
python3 -B scripts/yuyuhakusho/pcm-payload-export.py \
  --out .local/md-yuyuhakusho/pcm-payload-review-data \
  --proof .local/md-yuyuhakusho/pcm-payload-review-proof
```

第二次资源、manifest、provenance 和独立 `expected.json` 导出逐 byte 相同；相应 SHA 保留在 `determinism.json`。先行 fixture 的 provenance 记录“生产实现尚不存在”，再次导出时该诊断字段变为 true，故不把这两份 provenance JSON 冒称逐 byte 相同。

库不包含 `go:embed`、下载器、播放器或平台接线。资源逻辑路径供后续 OSS／CDN 不可变清单绑定，当前未上传、发布或部署，也未把这些音频载荷加入正式服务器构建。本轮不替代最终服务器包／镜像剥离验收。
