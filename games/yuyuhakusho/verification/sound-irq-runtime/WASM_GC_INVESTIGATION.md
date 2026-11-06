# Wasm 运行时复核记录

第二轮完整 Wasm clock／Root 回归在新补跑场景 `121-catchup-fade-step-request-a0` 发生一次 Go 运行时错误：`found bad pointer in Go heap`。当时已通过 8,109 条记录，错误栈位于 `note_voice.go` 的 music／feedback 赋值路径。该失败不是业务断言，原始日志与命令记录作为失败保留；项目 audio／engine 没有 unsafe、cgo 或 linkname 路径。

没有据此修改业务规则、输入、oracle、编译器配置或生产代码，也没有认定工具链是已证实原因。使用同一份冻结断言做了以下复核：

- 新声音 IRQ 全部 124 条检查，在 `GODEBUG=gccheckmark=1` 下通过。
- 同一补跑场景在 `GOGC=20`、`GODEBUG=gccheckmark=1,clobberfree=1` 下连续运行 20 次，40 条父／子记录全部通过。
- 完整 clock／Root 矩阵在 `GODEBUG=gccheckmark=1,clobberfree=1` 下通过 8,527 条记录，无跳过；额外环境单独记录。

此前默认环境的完整 Wasm 矩阵也曾通过，但不能据复跑成功宣称这次运行时错误的原因已经解决。当前结论是未重现；最终平台浏览器完整对局仍须覆盖实际发布构建的运行稳定性，不能用本记录替代整场验收。
