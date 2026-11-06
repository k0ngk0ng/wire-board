# 单路 metadata 捕获期间的双路请求

完整声音中断的原版 profile `106-delayed-empty-six` 已冻结并在修改前触发真实失败：主机接受双路 slot A 请求时，独立 PCM 正在单路 metadata 捕获阶段。现有 guard 一律拒绝 descriptor 发布，范围过大。

- 双路请求未改写 single descriptor 的来源字段，却因单路阶段被错误拒绝。
- 通过删除全部 capture guard 放宽为可混合两个不透明 descriptor 的来源，或放宽同一 dual slot 的半捕获覆盖。
- 将 marker、rate、flags 等独立 live 字段错误绑定到旧 descriptor，丢失实际读取顺序。
- 更改已冻结的主机端口时刻、原版 input／oracle，或忽略真实 publication 错误来完成验收。

修正仅允许未替换 single descriptor 的普通双路发布；真正改写 single 来源和同一 dual slot 的未验证覆盖继续拒绝。先行证据与测试使用 `sound-irq-runtime` 的完整原版调用，不改动采样或期望输出。
