# 声像与 DAC 重定向的先行失败场景

本项接上乐器 load 的 pan 尾部和普通 voice pan。实现前覆盖：

- 把 busy mailbox 与当前 DAC request marker 混用，或只测 marker bit7；低位非零也必须重定向。
- 在 bus acquisition 之前缓存 marker，读错实际阶段，或对 pan 路径额外执行 mailbox 等待。
- 重定向写入错误的 PCM owner／字段，覆盖另一份状态；写恢复声像时丢失 AMS／FMS 或把声像更新当成重启采样。
- 普通 channel 与 channel six、claimed、voice entry 与 instrument tail 的 caller／分支／return 成本混用。
- Source marker 读取和 ZRAM 写缺少额外访问成本；写后忘记释放同一 bus、或把后续 TST／branch 放错顺序。
- 在反馈回调里提前完成写入；分阶段 Apply 应在对应 intent 到达时执行，错误意图、重复 Apply、提前 Finish 或失败后重入必须拒绝。
- 真实 mixed Runtime 不要求持 bus 就读 marker／写恢复值，时间越界、预算、外部失败或 detached 分支错误造成半提交。

原 fixed profile 使用合法 FM voice row。源中的特殊 header-pointer 路径不由普通声部调用，本项不伪造其可达性。所有 metadata／pan 字节继续由唯一 score／PCM owner 管理，helper 只有临时 clock／结果，不新增 RAM、原索引、源程序或事件历史。Go 测试和原 oracle 均在新增实现前冻结。
