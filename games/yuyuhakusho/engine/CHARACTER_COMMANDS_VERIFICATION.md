# 全角色命令准备：实现前验证范围

本阶段恢复 `0x15c20` 表中 11 个角色来源 ID 对应的完整命令准备函数。现有通用方向／按钮识别器及酎准备已验证；本阶段继续核对全部调用顺序、槽位与只读程序，并用原 CPU 完整父调用验证组合行为。角色来源 ID 不等于平台席位，尚未验证的人物名称不作猜测。

实现前失败场景：角色表相对偏移基准或索引错误；把全部角色都套用酎的八槽顺序；遗漏某角色的槽位、程序或额外写入；把仅有六／七个调用误补成八个；未调用的槽被清除或更新；后续完成请求覆盖顺序错误；超时清理错误覆盖其他槽的请求；用保留输入／按下沿代替 `+74` 原始 held；朝向转换错误；只比较已知字段而遗漏人物未知字节；只读数据提取边界、终止符和来源失配；观察器改变自然帧；夹具少于所有角色；每步回灌原版中间状态；缺失角色／程序或非法索引后部分修改状态；修改调用者的程序切片；将准备函数完成当作招式或完整对局完成。

步骤：核对有限静态指令及 ROM 身份，采集 3,600 帧基线不变的自然完整调用；对 11 个入口运行声明私有输入的原 CPU 组合矩阵及单初态连续流；先写独立 Go 测试并保留缺实现失败，再实现组合规则。每次比较完整人物 256 bytes。原版 ROM、模拟器与临时数据不进入生产运行时。

## 已确认语义

11 个入口都由 `MOVEA` 槽位／程序参数和通用识别器 `JSR` 组成，末尾为 `RTS`，没有额外人物写入。先执行槽 6、7，再按各自顺序处理其他槽；未调用槽必须保留。每个入口及字节摘要保存在[只读程序目录](../data/command-programs.json)，有限静态指令见[准备函数证据](../../../docs/research/md-yuyuhakusho/static/character-commands-preparations.txt)。

| 来源 ID | 实际调用槽顺序 |
| --- | --- |
| 0、9 | 6、7、0、1、2、3、4、5 |
| 1、3、4、6、8 | 6、7、0、1、2、3、5 |
| 2 | 6、7、0、2、3、4、5 |
| 5 | 6、7、0、2、3、5 |
| 7 | 6、7、0、1、2、5 |
| 10 | 6、7、0、1、2、5 |

槽顺序相同不代表程序相同。来源 6 的槽 3、5 共用同一个只读程序，计时和索引仍各自独立。完整目录共 16 个程序，生产代码通过 `CommandLibrary` 接收语义数据；不读取或执行 ROM 地址。

## 验证结果与边界

- 4,134 次自然完整父调用全部来自来源 ID 9；所有 3,600 帧 RAM／画面与原版基线一致，没有未完成或错误调用。该自然片段不证明其他 10 个角色已实际游玩。
- 14,080 个原 CPU 隔离完整调用，每个来源 ID 1,280 个。每个角色覆盖全部 256 种完成槽掩码的优先级组合、两种朝向和输入，再覆盖全部 256 个原始输入 byte 的混合有效索引／超时／请求状态。未调用槽以非零值填充，验证其完整保留。
- 33 条单初态连续流，每个角色 3 条朝向流，共 3,786 步。每条只初始化一次，之后仅改变原始按键；不恢复原存档、不把期望命令、计时或索引写回 Go。
- 每次完整比较人物 256 bytes，保留未知字节；只读库保持不变。非法来源 ID、缺失后续程序、后续槽非法索引均整体返回初态，前面的完成请求不泄漏。

原生竞态测试以完整 engine 包编译并聚焦运行上述测试，通过。Node/Wasm 聚焦运行同样三个测试，通过；该次使用四个命令源码／测试文件显式编译，避开并发编写的其他组件红阶段。完整 engine 包的最终集成回归由父阶段统一执行，不将这次显式源码测试冒充整包回归。

这恢复全部角色的命令准备组合；没有恢复命令到招式分派、招式限制、共享准备尾段、原版电脑或完整人物／对局更新。只读程序与函数入口的来源 ID 也不证明中文人物名映射。

## 复跑

从 worktree 根目录运行：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_character_commands.py \
  --out .local/md-yuyuhakusho/character-commands-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestCharacterCommands' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestCharacterCommands' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

原版复跑输出的同名夹具、manifest 及程序目录可与已保留记录比较。真实实现前失败、原生竞态和 Node/Wasm 日志，以及来源／夹具摘要保存在[验证记录](testdata/character-commands-validation.json)。
