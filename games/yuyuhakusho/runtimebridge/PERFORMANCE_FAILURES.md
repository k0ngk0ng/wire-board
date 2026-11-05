# 客户端推进与 fork 性能 revision v1 的先行失败范围

本清单先于本 revision 测试与实现；原 schema 2 的冻结清单、49 条轨迹、JSON 格式和校验保持不变。

- 网络正常同步仍反复 Save／JSON Restore；追赶权威的中间帧重复完整 RGBA raster，忽略真实主线程成本。
- typed fork 共享可变 Options.Players、World.Objects、Stream、Throws.Records、Registration.Owners，或 HUD.Slots；数组之外新增引用字段而深复制遗漏。复制 nil／空切片改变旧 JSON 字节。
- fork 同时建立 legacy／computed owner，或复制 renderer cache 为新 owner；跳过既有格式／引用校验来提高速度；fork 后 Ready／Step／错误影响原实例。
- 无像素 Advance 的 world／cache／slots／HUD、网络 frame、输入边沿或电脑掩码与原 StepControlled 不同；绕过逐帧 metadata推进。
- invalid MD bits／unoccupied channel／automation mask／frame溢出或metadata错误提交半帧；错误操作修改共享容器。
- Advance 产生的错误或无像素路径污染随后真实最后一帧的 callback-time绘制；以最终对象状态猜画面。
- Preview 的权威不变或 completed F／非终局 F+1 语义改变。
- 旧 schema 1 的十份完整字节基线或 schema2高Present／opaque checkpoint改变；49×203,428参考帧的state/pixel摘要改变。
- 基准只测规则而避开JSON／Raster／Digest热点，或只报告Native速度却当作Wasm/UI性能；把microbenchmark等同真实双人网络验收。

验收新增 `testdata/performance-v1/`，不覆盖原冻结manifest或旧日志。真实平台页面profiling与RPC修复由web owner独立负责。
