# 对象绘图准备验收

`engine.PrepareObjectDrawOnOwner` 直接借用调用者的图形 owner，执行原版 `125A` 完整调用的独立语义与逐操作成本。`engine.PrepareObjectDrawWork` 委托同一执行体，提供值状态回滚接口。直接 owner 重构后，native 通过 89 项测试（9 个顶层测试）。两个固定核心的 320 个完整调用、117,120 个工作边界保持一致，值 API 和直接 owner API 分别通过同一冻结夹具。原生夹具 SHA-256 为 `40b83b1aad4d76c35ff577dea58d08a13acaf802631f72a9cebbdaa625ee9261`，夹具和原冻结测试均未修改。

重构前的 native、race、Wasm 各 85 项测试及 vet 记录完整保留在 `owner/value-api-baseline-validation.json`。用户随后选择优先交付稳定可玩声音，因此本次只补完必要 native 回归，没有重新捕获原生数据或启动新的 race／Wasm／vet 检查。当前源码与结果绑定见 `validation.json`。

## 状态归属与调用约定

`presentation.FrameState` 的数量、标志、片段与传输是当帧窗口；`renderstate.State` 保存图块槽与 HUD 引用。主时钟图形 owner 使用 `ObjectDrawState` 保存仍会独立变化的字段，不保存 RAM、SAT、64 个固定链头、PC 或执行记录。

| 字段 | 语义 | 原版对应 |
| --- | --- | --- |
| `ReservedCount` | 包括首项在内的保留精灵数量，支持 1～80 | `F57A + 1` |
| `Count` | 当前精灵计数，保留 16 位语义 | `F578` |
| `WriteIndex` | 下一次提交位置，以精灵项为单位 | `(F57C - E200) / 8` |
| `FrameStartIndex` | 本帧保留项之后的起点，后续提交不推进 | `(F57E - E200) / 8` |
| `Flags` | 活动／溢出标志，保留完整字节 | `F54F` |

`PrepareObjectDrawOnOwner` 要求传入已经独立分叉的事务 owner。`ObjectDrawWorkHook` 收到的指针与入参完全相同，在当前操作语义写入之前执行。hook 完成当前操作计价并成功返回后，函数立即在 owner 发布对应字段，IRQ 可以通过同一 owner 读取已完成的写入。`ReservedCount` 在 `object-retained-count-read` 的 hook 之前锁存；后续修改保留在 owner 中，但不改变本次循环次数。其他字段不通过入口快照整体覆盖。

当前流程没有 IRQ 写入这五个字段。将来若增加这类写入，须结合主时钟的操作前置机制重新验证读取时点。调用者须将音频、时钟和图形 owner 一同分叉，任何失败均丢弃本次分支。直接 owner API 遇到 hook 错误时立即停止后续工作，保留此前操作和 hook 已经完成的字段更新；外层联合事务负责回滚。值 API 遇到错误时返回入口状态。空 owner 和入口无效数量均在执行任何工作前拒绝。

## 原生观察与成本

原生捕获使用 pinned ROM（SHA-256 `c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06`）和现有两个固定核心。所有 1～80 的保留数量分别覆盖 4 个 refresh 相位，直接执行完整 `125A`，中断屏蔽级别为 7。每次调用均在进程内核对全部 64 个固定链头、全部保留项 link byte、完整输出与其他 RAM 字节未变；原表不写入产物。

两个核心各自运行固定 2,400 帧输入，全部 RAM／视频哈希与已有基准一致。带观察器的核心实际捕获 2,087 次自然调用：保留数量 1 为 1,200 次，保留数量 2 为 887 次。1～80 的支持上限来自已有 H40 显示容量与 `sprite-submit.txt` 的 `CMP #50` 证据；私有输入域不代表全部数量都在自然流程可达。

| 工作 | CPU cycles |
| --- | --- |
| 清绘图标志 | 16 |
| 链头准备 | 8、12、8 |
| 每个链头 | 12、4、10；最后一次循环判断为 14 |
| 保留项准备与数量读取 | 8、4、12 |
| 每个保留项 | 4、12、8、10；最后一次循环判断为 14 |
| 发布数量、写入位置、本帧起点 | 各 12 |
| 恢复活动标志、返回 | 各 16 |

总 CPU 成本为 `1808 + 34 * ReservedCount`，工作边界数量为 `204 + 4 * ReservedCount`。每个边界分别调用 `MainWorkOperation.Advance`，保留真实 refresh 次序。调用者 `3A5A` 的 absolute-long JSR 成本 20 由联合主时钟另外计价。

## 可重复执行

从 worktree 根目录执行。原生重捕获需要现有 pinned ROM／两个核心，新输出路径必须不存在。

```sh
TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" python3 -B scripts/yuyuhakusho/capture_object_prepare_work.py --out .local/md-yuyuhakusho/object-prepare-work-recapture
python3 -B scripts/yuyuhakusho/verify_object_prepare_work_owner.py --mode native --out .local/md-yuyuhakusho/object-prepare-work-native-repeat
python3 -B scripts/yuyuhakusho/verify_object_prepare_work_owner.py --audit
```

验证脚本固定将 Go telemetry、缓存和临时目录限定在 worktree 内。`test-first.json` 绑定先于实现的失败清单、测试、原生夹具和实际缺失 API 红测；`owner/test-first.json` 独立绑定直接 owner API 的新增失败清单、4 组测试和实际缺失 API 红测。原有 5 组测试与新增 4 组测试共同覆盖 89 项测试。`validation.json` 绑定最终源码、命令、日志和验收产物 SHA。

## 尚未由本项证明的范围

本项只证明给定入口状态与时钟的完整绘图准备语义、成本、写入次序和错误回滚。实际世界入口、调用者工作、跨帧 VDP／IRQ 接受、背景音频与后续对象绘制须由联合主时钟验证。没有运行其他游戏或全量世界矩阵。

本项清理范围为独占的 `.local/md-yuyuhakusho/object-prepare-work-native-v1/` 重复捕获目录；可核验的原生夹具、自然调用记录、命令和日志保留在本目录。共享 Go 缓存由联合任务管理。
