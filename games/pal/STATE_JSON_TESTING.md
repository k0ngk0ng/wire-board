# 存档外层 JSON 解析与回调耗时

本阶段保持存档外层 JSON 和 `native` 的 Base64 字符串格式。仅对程序自身生成、以原生载荷字段开头的标准表示单独解码载荷，再严格解析剩余字段；其他合法表示继续使用原解析器。旧档、原生内存摘要和规则版本的校验保持。

## 实现前失败场景

1. 旧字段顺序、前后空白、字段名转义、字符串转义和斜线转义不能读取。与原严格 JSON 解析器逐项比较。
2. 将嵌套字段或对白里的同名文字当作顶层原生载荷。仅处理首个顶层字段的固定前缀，其他布局回退原解析器。
3. Base64 缺失、截断、错误填充、非法字符、未转义换行或回车被接受。JSON 与 Base64 均须合法，不能用 Base64 解码器忽略换行的行为绕过 JSON 语法。
4. 缺失、空、`null`、数值、数组或对象原生载荷绕过现有存档结构检查。解析与游戏结构校验分别保留。
5. 重复 `native` 字段（含转义字段名）改变原解析器的最后字段语义。发现后回退原解析器；不能在最后字段为 `null` 时重新注入第一个载荷。
6. 未知字段、非法剩余结构、尾随 JSON、尾随非空白、缺少分隔符或结束括号被接受。剩余结构继续禁止未知字段，并验证文档结束。
7. 超过整个外层 48 MiB 或原生载荷 32 MiB 的输入提前分配失控。原大小上限继续有效，快路径分配前检查解码容量。
8. 解析或校验失败修改调用方存档字节、共享原生缓冲区或另一房间状态。使用独立输出并验证原输入不变、并发解码与后续恢复。
9. 变更只让孤立解析更快，却改变规则时钟、输入队列、阅读、随机状态、音频或手动回退。复用真实认证 HTTP／SQLite 存档与旧核心续玩验收。
10. 为达标修改 20 毫秒预算、删减回调工作或更换原版核心。保留固定预算，在真实两个独立单人房间、约 200 毫秒 RTT 和 320px 触屏重新测量；孤立采样不能代表完整性能。
11. 新测量覆盖已冻结的旧证据。使用 `native-json-before`、`native-json-after` 独立输出，原始运行器、资源与旧产物保持。

## 验收范围

先比较新旧解析器的表示和错误边界，再验证真实原版规则、个人续档和房间响应。菜单、战斗、持续动画、网络恢复及发布仍按既定性能要求逐项验收，不能因客栈探索通过就宣称完整性能完成。

## 实测结果

优化前与优化后的真实客栈探索四项测量均达到原预算，规则回调 P95 从 19.71～19.82 毫秒降至 15.75～17.25 毫秒；同一正式核心、两个独立单人房间、每场景 12 次原生输入。固定延迟后的实际 RTT 为 202 毫秒，手机画布响应 P95 为 34.4 毫秒，消息 P95 最大 2220 字节。正常场景存在单次超过 20 毫秒的调用，验收指标为 P95。

隔离检查覆盖旧／新布局、转义、重复字段、空值、损坏 Base64、尾随结构、48 MiB 整体上限、32 MiB 原生载荷边界、输出独立性及并发解码。特别保留超限的首个载荷被后一个合法重复字段覆盖时的原解析语义。模糊测试发现去掉首字段后可能将非法尾逗号变成空对象，已先保留失败输入再修正，随后 882394 次模糊执行通过；竞态检查通过。

三项真实认证 HTTP／SQLite 回归通过：个人保存与读取失败回滚、服务重启和陌生账号限制；保留旧原生核心与缺失核心时原档不变；手动回退、原页面和更高操作代次。回归复用原测试流程，只将本轮输出切到独立文件名，未覆盖旧验收。旧字段布局仍能读取，新布局仍可由原严格 JSON 解码器读取；原生核心、资源身份及原生快照格式未变。

阶段汇总为 `native-json-stage.json`。旧 7718 个不可变产物与隐藏点进度逐字节保持，本批核心和资源未改、无上传。当前没有重新宣称全剧情或完整性能通过。

## 复现

在仓库内的独立验收副本恢复本批固定资料与所需核心、资源。以下命令输出到本批独立文件；复跑不要覆盖已冻结的证据目录。

```sh
. scripts/env.sh
go test -race ./games/pal/server -run '^TestStateJSON|^FuzzStateJSONMatchesStrictDecoder/' -count=1
go test ./games/pal/server -run '^$' -fuzz '^FuzzStateJSONMatchesStrictDecoder$' -fuzztime=8s -parallel=4
PAL_STATE_JSON_E2E=1 PAL_VERSION_CATALOG=1 PAL_CHECKPOINT_E2E=1 go test -v ./internal/server -run '^TestPALOriginalSoloPrivateSaveRestartHTTP$|^TestPALRetainedNativeVersionsRestartHTTP$|^TestPALOriginalManualCheckpointReloadAndOldPageHTTP$' -count=1 -timeout=5m
PAL_BOARD_BROWSER=1 PAL_BOARD_ENTRY=performance go test -v ./internal/server -run '^TestPALBoardBrowserFixture$' -count=1 -timeout=32m
```

最后一条命令保持真实测试服务运行。在另一个同一仓库终端执行 `python3 -B games/pal/tests/performance-browser.py --tag native-json-after`，完成后在测试服务输出目录创建 `stop` 文件。原基线使用归档的优化前适配源码，不能拿当前代码重跑并冒充优化前结果。完整阶段聚合由 `python3 -B games/pal/tests/state-json-evidence-e2e.py` 复核原样本、日志及旧产物，单独执行一个测试不等于重新完成全部阶段验收。
