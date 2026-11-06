# 包络绑定、音高计算与分阶段音量

`score.BindEnvelopeWork` 为 immutable Catalog 加入与原 catalog JSON 精确绑定的 580 个逻辑 work 值。键为 family／envelope ID／现有 `EnvelopeCursor`，允许的 prefix-end 别名映射到 cycle 0；绑定资源本身只允许 canonical 键。重复、缺项、未知字段、非法位置或超限被拒绝。

`EnvelopePathWork` 仅含 0／1 的 restart 工作量：正式闭包只有 pitch 2 的 cycle 20 和 pitch 4 的 cycle 8 需要额外 restart。资源不包含原 byte、地址、原始游标、目标或导航列表。现有 voice cursor、声音 tick、scale 和 State 继续由原 score driver 唯一持有，绑定返回独立 catalog 视图。

`resourcebridge.DecodeWithEnvelopeWork` 连接正式五资源，保留三／四资源构造器与其身份语义。全部五份准确 raw resource 进入共同 identity，PCM checkpoint 不能在四／五资源配置之间混用。该 identity 用于内容绑定；下载仍须由外层可信发布清单认证。

`PricePitchCalculation` 消费当前 semantic pitch tick、work、scale 和 modulation enable，覆盖普通 calculator、none-envelope 与 terminal 前缀。乘法使用 16 位 sign-extended scale 的 popcount：`38 + 2*N`，没有 DIVU 的内部 refresh skip。terminal 返回 `Next=key-off`，停在真正 key-off leaf 之前；正常返回包含 calculator return。它不推进 cursor，不计算或持有第二份 pitch 状态，原 `param.PitchWord` 仍为唯一声音语义。

`BeginEnvelopeVolume` 覆盖包络读取、hold／keyoff、8 位 level 加法／clamp 和 note-off／claimed／tie-gate 抑制。Begin 不提前写端口；对应实际 intent 到达时验证 `ExpectedWrite` 并调用一次 Write，Finish 消费唯一 return。全部阶段共享一份临时 clock／refresh／预算；失序或失败后不能继续。PSG 始终使用既有唯一 synthesis owner。

`ServeEnvelopeVolume` 复用相同阶段，供已确定完整调用边界的调用方使用。失败时外层丢弃全部 detached owners；本项不自动 flush、处理 world IRQ、提供联合 checkpoint 或启用整场音频。尚未计费的 pitch caller／key-off、其它 voice／Root 阶段继续保留 unpriced 错误。

正式 work 文件使用 gzip，1,948 bytes，解压 SHA 与冻结来源一致。原生／Wasm 验收、五资源拒绝混用、全部 1,030 个原版 profile 与复跑入口见 [score-clock-envelope](../../verification/score-clock-envelope/)。
