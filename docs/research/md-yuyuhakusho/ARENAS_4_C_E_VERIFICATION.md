# 场地 4／c／e 规则恢复验证

本范围完成 `A060.word` 原偶数偏移 `4/c/e` 的主 callback `20cb2/212f8/21412`，以及场地 4 的 `20dca` 色盘子规则。独立实现位于 [arenas_4_c_e.go](../../../games/yuyuhakusho/engine/arenas_4_c_e.go)，没有运行 ROM、MD 内核、CPU 虚拟机或 RAM 指令解释器。平台接入、场地素材与像素验收由主任务另行完成。

[实现前失败清单](ARENAS_4_C_E_FAILURES.md)先于 CPU 采集及实现。原 CPU 参考完成后冻结 [测试](../../../games/yuyuhakusho/engine/arenas_4_c_e_test.go)，保存只缺本组四个符号的[真实 red](../../../games/yuyuhakusho/engine/testdata/arenas-4-c-e-own-symbol-red.txt)，随后加入实现。最终测试 SHA 与 red 记录一致；未调整冻结断言或参考来使实现通过。

## 结果与边界

| 场地 | 合法菜单自然返回 | 脏初态返回 | 单初态连续返回 |
| --- | ---: | ---: | ---: |
| `4` | 2,418（主入口与色盘各 1,209） | 1,899（主入口 1,359、色盘 540） | 360（两条各 180 步） |
| `c` | 1,204 | 1,551 | 180 |
| `e` | 1,201 | 1,551 | 180 |

共 10,544 次完整返回全部一致。每场观察回放的 3,602 帧完整 RAM、视频及尺寸均与统一提供的唯一未挂钩原版控制相同；本组没有另采原版控制。输入及原控制路径为 `arena-{4,c,e}-inputs.json` 与 `arena-{id}-reference-frames.json`，其摘要逐份绑定在各自然／边界 manifest。

原生竞态测试为 **108.241 秒**，Node/Wasm 为 **25.035 秒**，均通过三个顶层测试、六组自然／边界子项、四条连续流及错误回滚／门控断言；`go vet` 通过。隔离验证编译全部既有 tracked engine 文件、Root 提供的公共场地视图／反馈及本组新文件，未将并行开发中尚缺符号的其他新场地测试混入红灯。实际源码参数、测试 SHA、命令环境和结果见[完整报告](../../../games/yuyuhakusho/engine/testdata/arenas-4-c-e-validation.json)，[原生结果](../../../games/yuyuhakusho/engine/testdata/arenas-4-c-e-native.jsonl)与[Wasm 结果](../../../games/yuyuhakusho/engine/testdata/arenas-4-c-e-wasm.jsonl)可核验。

## 字段归属和公式

场地只使用临时 `ArenaSceneState`；所有 object scratch 均直接读取、修改 `WorldObject` 的既有 canonical 字段，不新增速度、相位、模式或计时器 owner。相机复用 `StepDefaultArenaCamera`；反馈统一调用 `ApplyArenaSceneFeedback`，使用各场实际 `SourceA/SourceB`，不把它们强制截成 16 位。`PanA/PanB` 只作为临时视图，持久反馈仍归 `Stream`。

| 原地址 | 场地 | Canonical 字段及实际用途 |
| --- | --- | --- |
| `+06` | 三场 | `Object.Header.Stage`，只区分零与非零 |
| `+08` | `4/c` | `Object.ScreenX`，表填充值／横向基值 word |
| `+20/+22` | `4` | `Object.TilesBase` 的高／低 word，四态色盘相位与倒计时 |
| `+10/+14` | `c` | `Object.Body.VelocityX/VerticalVelocity` 两个 long，每次分别加 `10000`／减 `30000` |
| `+1c/+1e` | `c` | `Object.GraphicsSet` 两半 word，前者为波形相位，后者初始化清零后保留 |
| `+08` long | `e` | `Object.ScreenX/Aux0A`，加 `8000` 的定点累积器 |
| `+0c` | `e` | `Object.ScreenY`，当前表首 word |
| `+10` long | `e` | `Object.Body.VelocityX`，实际 ROM 表行地址 |
| `+14/+18` word | `e` | `Object.Body.VerticalVelocity/Aux18` 的高 word；低 word 原样保留 |

`F572` 是 `Clock.PanOffset` 的 word。它非零时冻结各场相应累积器／色盘，但仍执行相机、行填充和反馈。`F589.bit0` 来自 `Clock.LogicFrame` 的低 byte，经 `DefaultArenaView.FrameCounter` 只读传入 c 场，决定 B 行两种漂移的顺序。

`F6DF` 已追到 VDP 寄存器 7 的背色索引：`4aa` 建立影子命令高 byte，`438/5d8/60a` 将 `F6DE.word` 送往 VDP 控制端口。c／e 初始化清零该 byte，4 场保留；临时字段为 `BackdropColor`，不将其误作 flags 或另一滚动模式。依据见[消费者静态](static/arenas-4-c-e-consumers.txt)。

场地 4 保留三个初始化带，随后将同一 112-word 表片段反向写入 `RowsB[8:120]`、正向写入 `RowsB[168:248]`，并以表的第 8 个 word 填充 `RowsA[112:176]`。色盘子规则本身只受 `F501/F572` 门控；先按 word 减计时器，再判断符号，并将 `(+20+4)&000c` 指向的三色写至 `Primary[9:12]`，不改当前色盘或整盘。

c 场保留 signed byte 波形、逐组相位递增及双 word 交替漂移。c／e 的 B 垂直量为 `asr.w(dy,1)+10`，B 源必须先将 `dy` 加倍为 word，再算术右移、偶数对齐及减 `15e0`；测试覆盖该中间值的回绕，不能直接代数约简。e 场从公共完整行的 `[40:104]` 读取 64-word 片段，保留默认场地未使用的最后 8 个 word。

## Typed 表和静态证据

[arena-scene-4-tables.json.gz](../../../games/yuyuhakusho/data/arena-scene-4-tables.json.gz) 仅含 64×112 个场地 4 行 word、四组三色及来源 metadata，共 2,043 bytes，SHA-256 为 `7c5a79efa8b6dc52298f530e4433f595641e13e29516be4bf1406dfa6d36509f`。表行来源为 `2a000+(X&63)*256+30`，色表 `20e00` 的四指针为 `20e10/20e16/20e1c/20e22`。

e 场复用独立导出的 [241×128-word 公共行表](../../../games/yuyuhakusho/data/arena-scene-linear-rows.json.gz)；c 场复用已有 `ArenaRuntimeLibrary.WaveX`。没有重复导出 e 尾表或再建立一份波形 owner。静态主入口依据为[既有指令清单](static/arena-assets-scenes.txt)，补充消费者通过已安装 Ghidra 的私有 project 生成；所有 temp、cache、user.home 和日志均在 worktree。REA 不支持 raw 68000，本记录没有将直接 Ghidra 分析称为 REA 反编译。

## 完整 RAM 与外部穿插

采集桥记录每一笔原 CPU RAM 写，保留完整 32 位 PC。每次回放均由调用前 65,536 bytes 按写入顺序重建调用后全部 RAM，并核对摘要。独立[指令审计](../../../games/yuyuhakusho/engine/testdata/arenas-4-c-e-instruction-audit.json)进一步按具体 PC 检查目标区间和写入宽度，不以整个对象、整个栈或大 RAM 区间屏蔽未知变化：

- 1,984,798 笔组件写，逐个写入指令与目标区间对应。
- 14,630 笔 BSR 栈写，仅允许七处实际调用 PC 及其精确返回 long。
- 66 次中断、396 bytes 异常栈，逐次识别紧邻 IRQ 入口前的 4-byte 返回 PC 与 2-byte SR。
- 6,697 笔 IRQ 内写均保留实际地址、值和 raw PC，完整闭合 33 次 `FFFFF09A` 与 33 次 `FFFFF0A8` RTE。

自然调用实际出现 `219ac..219e8` 声音推进与 `37c56` 外部清除；后者 c／e 分别为 52／4 笔。观察器也覆盖 `37cac` 和任意实际 RTE opcode，但本批 callback 窗口没有 `37cac` 写或 `FFF13A` 返回，不声称其自然执行。没有把 IRQ、输入或声音结果归为场地规则，也没有 mask 未解释变化；未知写入总数为 0。

## 复跑

从 worktree 根目录执行，固定研究 ROM、原版／挂钩内核及共享控制须已按研究工具要求准备。所有输出目录应另选 worktree 内的新路径。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_arenas_4_c_e.py \
  --arena 4 \
  --inputs docs/research/md-yuyuhakusho/arena-4-inputs.json \
  --control games/yuyuhakusho/engine/testdata/arena-4-reference-frames.json \
  --out .local/md-yuyuhakusho/arenas-4-c-e-replay/4
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/audit_arenas_4_c_e.py \
  --out .local/md-yuyuhakusho/arenas-4-c-e-replay/instruction-audit.json
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
  GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
  go test -race -json -count=1 -run '^TestArenas4CE' ./games/yuyuhakusho/engine
PATH="/Users/jason/.nvm/versions/node/v24.15.0/bin:/opt/homebrew/bin:/usr/bin:/bin" \
  GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
  GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
  go test -json -count=1 -run '^TestArenas4CE' \
  -exec /opt/homebrew/Cellar/go/1.26.2/libexec/lib/wasm/go_js_wasm_exec ./games/yuyuhakusho/engine
```

将首条命令的场地与对应控制文件改为 `c/e` 可重采其参考。重采由同一原 CPU 执行，不生成新的未挂钩原版控制。三场真实菜单可达、函数规则一致，不代替主任务的全部世界调度、素材、呈现、存档与平台 E2E 验收。

已回读核验全部九份冻结夹具和 native／Wasm／vet 产物摘要，随后清理本组 65,328,394 bytes 临时重复夹具、观察桥二进制、私有 Ghidra project／cache／日志和验证中间目录。共享 Go cache、原版控制、其他 worker 文件及正式研究证据均保留，详见[清理记录](../../../games/yuyuhakusho/engine/testdata/arenas-4-c-e-cleanup.json)。
