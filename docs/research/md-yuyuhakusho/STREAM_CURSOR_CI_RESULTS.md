# 默认 CI 测试选择的验收结果

本 revision 只把原性能 wrapper 从 `stream_cursor_test.go` 原样抽到 `native_performance_test.go`，后者使用 `//go:build !race`。共享驱动、功能 race 用例和 500 ms／300 帧／60 次确认／双摘要／2048 字节要求均未改动，6 个生产文件的 SHA 与调整前相同。

调整前已实际执行完整 `go test -race ./games/yuyuhakusho/platformtest`：整包 64.155 s，只有原性能用例失败，ACK 4.818841375 s；红灯、原共享文件和 SHA 保存在 `verification/platform-stream-cursor-ci/`。

| 检查 | 结果 |
| --- | --- |
| Native discovery | 7 个测试，保留原性能入口 |
| Race discovery | 6 个测试，唯一差异是原性能入口 |
| 原 native 性能用例 | 通过，仍执行原 500 ms 硬阈值 |
| 完整默认 platformtest race | 6 项全部通过，48.531 s，无 `-run` 过滤 |
| 公共 cursor／旧流／恢复 race | 7 项全部通过，15.262 s |

`TestCursorHTTPWSFunctionalRace` 在 native 与 race discovery 中均存在，完整默认 race 仍覆盖本作 HTTP 鉴权、加载、独立席位、托管／恢复、慢端、重连、逐帧规则与呈现双摘要。原性能测试只是按用途排除插桩构建；没有从默认 native 测试中删除。

重复执行当前组织验证：

```sh
python3 scripts/yuyuhakusho/verify_stream_cursor_ci.py discovery
python3 scripts/yuyuhakusho/verify_stream_cursor_ci.py native-performance
python3 scripts/yuyuhakusho/verify_stream_cursor_ci.py default-race
python3 scripts/yuyuhakusho/verify_stream_cursor_ci.py host-race
```

新的机器可读清单为 `games/yuyuhakusho/verification/platform-stream-cursor-ci/final-validation.json`。它验证“当前共享文件等于原文件删除原 wrapper”和“新 wrapper 等于原样提取”两个字节级关系，并保存各项命令、实际测试名、源文件 SHA 和网络结果。

此前 cursor 最终清单、263 来源清单和所有原性能 race 红灯没有覆盖或改写。旧 `verify_stream_cursor.py` 继续绑定上一 revision 的冻结来源；当前测试文件组织使用本次独立验证入口。公共 `scripts/test_go_shard.py` 未修改，其未携带 `-race` 的 discovery 行为保留在先行范围说明中。本次未重跑游戏规则矩阵或正式浏览器验收。
