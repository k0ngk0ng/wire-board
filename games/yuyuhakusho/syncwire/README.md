# MD 输入窗口协议

`syncwire` 只保存最近 120 个明确输入帧。帧号是独立 `uint64`，不使用会回绕的规则帧计数；MD 的全部 12 个按键使用 16-bit 字段，自动化标记占低 4 bit。它不修改世界、控制器历史、AI、渲染或声音，也不替平台决定托管。

```go
window, err := syncwire.Encode(history)
frames, err := syncwire.Decode(window)
frames, err := syncwire.DecodeJSON(raw)
history, err = syncwire.Push(history, next)
```

`Push` 复制调用方数据并只保留最近 120 帧，返回值容量也不超过 120。窗口内必须连续，重复、倒序、缺帧、非法按键／自动化位及计数溢出都原子拒绝。错误不返回部分窗口或部分解码数据。

## 版本 1 格式

JSON 字段为 `version`、`firstFrame`、`frameCount`、`data`。`firstFrame` 是规范十进制字符串，因此大于 `2^53` 的帧号可经 JavaScript 转交 Wasm 而不丢精度。JavaScript 不需要把整个输入拼进一个 32-bit 数字。

每个 RLE run 固定 10 bytes：

| 字节 | 含义 |
| --- | --- |
| 0 | 1～120 个连续帧 |
| 1 | 服务端派生的自动化 mask，低 4 bit |
| 2～9 | 控制通道 0～3 的四个 16-bit 大端 Held，各自只允许低 12 bit |

`data` 使用标准带 padding 的 canonical base64。相邻相同 Held／automation 的 run 必须合并，不接受同一状态的不同拆法、空白或非零 padding bit。版本 1 不猜测未来版本；未来格式需明确增加新版本解码器。

空窗口只有一种形式：

```json
{"version":1,"firstFrame":"0","frameCount":0,"data":""}
```

非空帧号从 1 开始，最后一帧可以等于 `MaxUint64`，但不能继续回绕。严格 JSON 入口另外拒绝缺失、重复、未知或 null 字段、错误类型、非规范数字字符串和尾随内容。

## 接入边界

`InputFrame.Automated` 是本帧的显式控制状态，来源由 host 鉴权；本包不会把客户端数据变成托管授权。世界的 `Controllers` 是实际规则采样历史，不能清零后从 digest 排除。调用者在本帧共享 tick 边界应用 automation，再让原引擎计算 pressed／released。

输入窗口不是存档或恢复快照。规则 digest 和呈现 digest 由各自 owner 计算；首连、重连、失步时的 checkpoint 使用单独通道。常规网络消息只需输入窗口、帧／序号、确认及摘要，不携带像素、音频、完整世界或长期事件历史。

## 验收

[失败清单](FAILURES.md)与八项测试先于生产实现，缺 API 的真实 red 与冻结 SHA 保存在 `testdata/test-first.json`。低层验证覆盖四个通道的 4,096 种合法按键值、全部 MD 高位、完整 uint64 边界、RLE／JSON／历史原子性。

完整规则 E2E 从空引擎创建 2／3／4 人混战、4 人组队及真人闲置五种配置，覆盖全部 11 个角色与固定 host automation。直接输入与经过窗口的输入分别推进独立世界，逐帧比较完整规则状态、有序事件和 outcome，包括全部 `World.Controllers`；没有 mask。运行到 KO、判胜和清场，再比较 native／Wasm 的完整事实和轨迹 SHA。

本阶段不声称已实现动态托管变更的共享 tick helper、客户端本地预测、慢连接或真实键鼠延迟；这些由平台模块的完整 E2E 继续验证。没有加载 ROM、原 CPU 状态或逐帧参考对局。

从 worktree 根目录复跑：

```sh
TEST_TELEMETRY_DIR="$PWD/.local/go-telemetry" \
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -json -count=1 ./games/yuyuhakusho/syncwire
```

Wasm 的精确命令与最小环境保存在 `testdata/wasm-command.json`。完成两端运行后，用 `PYTHONDONTWRITEBYTECODE=1 python3 games/yuyuhakusho/syncwire/verify.py` 核对日志、冻结测试和实际完整事实；它不代替运行测试。结果写入 `testdata/validation.json`。
