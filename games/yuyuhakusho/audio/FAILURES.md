# 有限音频目录：实现前失败范围

本批只恢复已经验收的 134 个有限 WAV 的运行元数据和完整性校验：`06..7e` 共 121 个 PCM 命令、`a0..ab` 共 12 个 FM／PSG 序列，以及有限结算音乐 `8f`。原目录 `games/yuyuhakusho/data/audio-assets.json` 的 SHA-256 固定为 `617737f8ef07043401fa74f7f48ba551a64dd91cedd3a94dbfcf4883e0ff1541`。原 WAV 保持在 `.local/md-yuyuhakusho/audio-assets-final/`，不复制进仓库，不运行原内核或重新采集。

先保存本清单和独立验收脚本，运行目标目录缺失的真实失败；再保存 Go API 测试，运行缺少 `Catalog`／`DecodeCatalog`／`Clip` 的真实失败；最后实现导出器、decoder 和 WAV 校验。不得修改旧音频目录、采集证据或 WAV。

## 目录身份与结构

- schema、ROM、原目录摘要、格式或 `source_origin` 不符；把声明的 ROM／原目录摘要当成任意元数据真实性证明。
- 缺少字段、字段为 null、类型错误、未知字段、重复 JSON key、尾随 JSON、截断对象或过深／超量数据。
- 缺少 134 项之一、重复／冲突代码、混入控制 `00..05`、静音诊断 `7f`、有界 `82/87` 或其他 BGM；代码与 kind 不符。
- 大写、非法或长度错误的 SHA；零长度或超出 RIFF 范围的帧数；onset 不在原 PCM 内；`loop` 缺失或为 true。
- 运行目录携带 `.local`／绝对路径、URL、ROM 地址、音频正文或另存可与帧数冲突的 duration。
- 角色缺失、重复或越界，AN 代码缺失、重复、错误排序或绑定到错误的 source character；将 78 个动画引用称为完整玩法可达或必要预加载集合。
- decoder 部分失败后泄漏部分可用目录；调用方修改 `Lookup` 值或 `AnimationCodes` 返回切片后改变目录。

## WAV 与媒体绑定

- 未知／诊断／控制代码被校验为有限媒体；正确代码使用另一个有限声音的 WAV。
- RIFF／WAVE 标识、RIFF 总长、fmt／data chunk、chunk 长度或 padding 错误；fmt／data 缺失或重复；伪造／截断头部与尾随正文。
- PCM 格式、双声道、44,100 Hz、16 位、block align、byte rate、data 对齐或 stereo frame count 不符。
- 仅 WAV SHA 相等或仅 PCM SHA 相等就通过；音频数据损坏、头部改变、前导静音被裁切、尾部被裁切、归一化或 onset 不符。
- `8f` 旧元数据没有 onset，错误当作静音；必须从已验 WAV 实读，预期第 971 个立体声采样帧开始非零，不改旧文件。
- 只验证元数据自洽，没有对全部 134 个现存 WAV 验证完整文件 SHA、PCM SHA、格式、帧数和 onset。

## 确定性与边界

- 相同输入的两轮导出 JSON／manifest 不同；导出覆盖旧目录、WAV 或证据，或产生工作树外缓存。
- native 与 Node/Wasm 对同一目录、媒体或损坏输入得到不同结论。
- 有限媒体目录通过被误称声音调度、混音、`82/87` 连续 BGM、浏览器播放或 CDN 发布通过。

目录 decoder 负责结构和已知 ID／绑定校验；元数据整体必须由调用方信任的不可变 manifest 摘要认证。`ValidateWAV` 将实际媒体绑定到目录中的完整 WAV／PCM 摘要和原始采样位置。攻击者同时改写合法格式的元数据及其声明摘要，不属于单凭 ROM 字段可解决的信任问题。
