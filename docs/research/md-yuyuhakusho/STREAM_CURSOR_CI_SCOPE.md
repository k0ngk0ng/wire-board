# 默认 CI 的性能与竞态测试边界

本说明先于测试文件组织调整。原 500 ms 性能入口仍在无 build tag 的共享文件中，默认 `go test -race ./games/yuyuhakusho/platformtest` 会重新运行它；只在专用 runner 选择器排除，无法保证普通 CI 的边界。

## 先行失败范围

- 默认 race 仍发现并运行原 native 性能入口，重复触发已保留的插桩延迟失败。
- 为通过 race 提高 500 ms 阈值、减少 300 帧／60 次确认、删除双摘要或隐藏默认 native 性能测试。
- 移动入口时改写共享驱动、功能 race 用例或任何生产代码。
- native discovery 不再发现原性能测试，或 race discovery 意外丢失功能／鉴权／加载／恢复／慢端测试。
- 仅验证专用选择器，不实际执行完整默认 platformtest race；或用新的 manifest 覆盖此前来源与失败记录。

调整仅将 `TestCursorHTTPWSReplaySizesAndReconnect` 的原 wrapper 移入带 `//go:build !race` 的 `native_performance_test.go`。函数名、调用 `runCursorTransportScenario(t,true)` 和共享硬阈值保持不变。普通 native discovery 继续发现并运行性能要求；race discovery 排除该入口，默认运行原功能 race 用例及其 300 帧／60 次确认、完整双摘要、2048 字节视图、快慢端和重连检查。

验证保留调整前完整默认 race 红灯、原文件与 SHA，并记录调整后 native 性能测试、完整默认 platformtest race 和公共 cursor 所选 race。native／race discovery 名称差异必须只包含原性能入口。

`scripts/test_go_shard.py` 的 discovery 当前未携带 `-race`。本次不修改公共 CI 脚本；native 性能测试本身通过 build tag 在 race 编译时排除，符合它的性能验收用途。旧验证清单和此前插桩性能失败继续保留，新记录使用独立 CI revision。
