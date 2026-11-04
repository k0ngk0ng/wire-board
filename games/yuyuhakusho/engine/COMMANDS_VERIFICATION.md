# 通用命令识别：实现前验证范围

目标为原 `0x1e8d0`、`0x1e972` 两种识别函数与酎准备函数 `0x17d80` 的八槽调用顺序。先确认原指令和数据格式，再采集原版输入／输出，先写测试并观察缺实现失败，最后恢复独立 Go 规则。

失败场景：把持续按住、按下或 XOR 变化混用；朝向转换错误；槽索引／步长错误；byte 计时溢出、倒计时归零边界错误；序列完成标志、终止符或继续状态被猜测；错误输入本应重置却保留或相反；持有时长和输入窗口混淆；动作请求优先级或八槽顺序改变；把中间返回当成完整调用；丢弃未知写入或把寄存器残留误判成业务字段；观察器改变整帧基线；把隔离矩阵或命令成功数冒充完整对局；生产代码读 ROM 或每帧注入期望命令。

自然观察比较完整人物 256 bytes，并与固定输入的 3,600 帧 RAM／画面基线一致。原 CPU 隔离用例明确声明私有输入，覆盖自然记录未出现的成功／超时／重置边界。只读序列库由固定 ROM 提取，生产实现接收语义数据，缺失数据明确失败。

## 已验证结果

| 范围 | 数量 | 核对内容 |
| --- | --- | --- |
| 自然方向识别 | 20,670 | 完整函数、八槽状态和人物 256 bytes |
| 自然按钮识别 | 12,402 | 同上，使用原始输入高 nibble，包含 Start |
| 自然酎命令准备 | 4,134 | 按 `6,7,0,1,2,3,4,5` 顺序完成八槽调用 |
| 原 CPU 隔离用例 | 8,625 | 4,896 个方向、3,681 个按钮、48 个多槽优先级／超时用例 |
| 单初态连续流 | 12 条、435 步 | 仅外部原始按键变化，不逐步重置命令中间状态 |

自然记录中的准备函数与内部识别函数重叠，不能把 37,206 条记录当作独立动作次数。隔离矩阵包含 72 个显式私有序列用例，覆盖窗口 0／255、计时 byte 回绕和索引 254／255；私有数据没有替换酎的正式八条序列。

两套缓冲分别为人物 `+a8..af` 的八个 timeout bytes 和 `+b0..b7` 的八个索引 bytes。方向识别只比较原始按住输入低 nibble，朝向非零时交换左／右。按钮识别比较高 nibble。每次调用最多推进一个输入项；抵达 `0xff` 后，下一次调用才完成并提交 `slot+1`。超时清空当前槽，只在当前请求等于该槽请求时清除请求。

已保留实现前缺少类型／函数导致的实际失败输出：[commands-red-phase.txt](testdata/commands-red-phase.txt)。该输出来自早期测试版本，行号不对应随后格式化后的文件。实现后的原生竞态和 Node/Wasm 对照均通过，完整人物比较没有未建模写入，非法槽位／序列和缺失后部程序不会留下部分状态更新。来源摘要与本次运行结果见 [commands-validation.json](testdata/commands-validation.json)、[原生结果](testdata/commands-native.jsonl) 和 [Wasm 结果](testdata/commands-wasm.jsonl)。

## 复跑

从当前 worktree 根目录运行；Go 缓存和临时目录均留在工作区内。原版采样需要研究目录中的固定 ROM、钩子内核和 `hook_filter.dylib`。

```sh
python3 -B scripts/yuyuhakusho/capture_commands.py --out .local/md-yuyuhakusho/commands-capture
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" GOTELEMETRY=off go test -race -count=1 -json ./games/yuyuhakusho/engine -run '^TestCommands'
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" GOTELEMETRY=off GOOS=js GOARCH=wasm go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine -run '^TestCommands'
```

本阶段仅恢复两个通用识别函数和酎的八槽准备。其他角色的准备顺序、命令到招式的分派、能量／攻击限制和完整对局仍未完成，不能据此宣称全部角色招式可用。
