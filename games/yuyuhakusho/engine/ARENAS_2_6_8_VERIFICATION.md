# 场地 2／6／8 回调验证

`StepArena2`、`StepArena6`、`StepArena8` 分别恢复 `A060.word` 十六进制偏移 `2`、`6`、`8` 的主回调 `20ae0`、`20e28`、`20fb4`，包含公共相机 `21556` 和反馈复制 `21690`。三项顶层测试在原生 Go 竞态检查和 Node/Wasm 中通过，测试源码保持缺实现红灯时的 SHA 不变。完整摘要见 [arenas-2-6-8-validation.json](testdata/arenas-2-6-8-validation.json)，实现前清单见 [ARENAS_2_6_8_FAILURES.md](ARENAS_2_6_8_FAILURES.md)。

## 来源与覆盖

自然采集固定引用根任务提供的三份合法菜单输入及其唯一 `original3602control`。每场 3,602 个视频步的完整 RAM／视频摘要与未挂钩原内核对照逐帧一致。checkpoint 为各场第 2,400 步，并核对 `A060` 和 `FF8180` 的真实回调身份；私有采集每次恢复都显式使用 `restore_refresh_cycles=True`。

| 原 ID | 自然完整返回 | 脏状态返回 | 单初态连续步骤 | 越已知视图参考样本 |
| --- | ---: | ---: | ---: | ---: |
| `2` | 1,205 | 364 | 3 × 320 | 24 |
| `6` | 1,205 | 364 | 3 × 320 | 18 |
| `8` | 1,204 | 364 | 3 × 320 | 18 |

共保留 7,586 次完整返回。7,526 次属于已恢复视图范围，独立状态与原 CPU 逐项一致；其余 60 次实际读取已知 HUD／滚动视图以外的地址，只保留为 `reference_only`，独立函数必须原子返回错误。样本覆盖阶段 0／2／`fffe`、暂停 bit 7、任意非零慢动作、反馈冻结漂移、dirty 低 word、相机样本、signed byte 波形、word／long 回绕及负反馈选行。

三场各保留 3 条 320 步序列，分别从初始化状态、dirty 回绕状态和负反馈状态开始。每步只补明确记录的外部相机样本与门控／反馈输入，持续使用独立规则上一步结果，不回灌本场原版状态。

菜单阶段还在 `FF8380` 以相同 PC 执行场地预览。三场分别有 371、170、71 次这样的入口；各次实际 A5、header、返回 PC 和帧号保留在自然 manifest 的 `other_entry_contexts`。本批完整返回范围是 ready-match 的规范槽 `FF8180`，没有把菜单预览入口计入该槽的规则验收。

## 完整变化与归属

每次入口到实际 RTS 后都比较整个 64 KiB RAM。原始净变化与有序变化段同时保存，逐段重放必须重建完整出口 RAM。场地主回调、其实际 8-byte 调用栈、IRQ 及 CPU 自动压入的 6-byte 异常栈分别归属，未知写入为 0。

自然流实际捕获 IRQ 134／6／78 次，以及音频队列清除 68／12／24 对。IRQ 原始 PC 的 `FFFFF000` 别名保留并正确识别；每次 IRQ 从实际入口到实际 RTE 配对。音频消费者 `37c56`／`37cac` 的清除前后与 IRQ 顺序一起保留。测试检查完整事件生命周期和净变化，不扩大允许地址集或删除未知变化。

缺表、短表、错误相机样本以及在更新末段才出现的反馈 B 源越界均验证完整回滚。暂停或慢动作的早退不检查未用资源。场地 6 本身不读取波形表，也验证在省略波形表时结果保持一致。

## 独立实现与表数据

对象 scratch 只存于已有 `WorldObject` 字段。`+08.long` 使用 `ScreenX/Aux0A`，`+0c.long` 使用 `ScreenY/Aux0E`；其余 word／long 使用速度、`Aux18`、`GraphicsSet`、`TilesBase` 和属性的真实重叠位置。word 写保留实际低 word，场地 8 的曲线指针也保留在真实 `+0c.long` 位置。

三场分别使用原曲线的 `[16:104]`、`[32:104]`、`[0:104]` 切片。共用 [arena-scene-linear-rows.json.gz](../data/arena-scene-linear-rows.json.gz) 含 241 × 128 个有符号 word，压缩大小 10,233 字节；来源为 ROM `24800..33900` 的 61,696 字节。全部 241 个 `[40:96]` 切片与既有默认场地尾表逐项一致。signed byte 波形继续使用已证的 `34000 + (phase & 7fe)` 取字节语义。运行时只消费 typed 规则数据，不读取 ROM，不依赖原内核或 68000 CPU／VM。

`F6DF` 已确认是 VDP 寄存器 7 的背景色索引，由临时状态 `BackdropColor` 承载。三场阶段 0 都写 0；暂停早退保留当前值。公共反馈按两个实际 sign-extended 源指针读取 `HUDPrefix/RowsA/RowsB`，不把场地 6／8 的半速 B 源改成默认场地公式。

## 测试结果与复跑

- 原生竞态检查：49.582 秒，3 项顶层测试通过，日志 [arenas-2-6-8-native.jsonl](testdata/arenas-2-6-8-native.jsonl)。
- Node 24.15.0／Wasm：15.876 秒，同一组测试通过，日志 [arenas-2-6-8-wasm.jsonl](testdata/arenas-2-6-8-wasm.jsonl)。
- `go vet`：通过，记录 [arenas-2-6-8-vet.json](testdata/arenas-2-6-8-vet.json)。
- 实现前真实红灯：只缺 `StepArena2/6/8`，记录 [arenas-2-6-8-red.json](testdata/arenas-2-6-8-red.json) 和 [原始日志](testdata/arenas-2-6-8-red.log)。

在 worktree 根目录重采任一场地，`--out` 应使用新的本地目录；ROM 和两个固定内核须按研究工具说明准备。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_arenas_2_6_8.py \
  --arena 2 \
  --baseline docs/research/md-yuyuhakusho/arena-2-inputs.json \
    games/yuyuhakusho/engine/testdata/arena-2-reference-frames.json \
    games/yuyuhakusho/engine/testdata/arena-2-reference-manifest.json \
  --out .local/md-yuyuhakusho/arena-2-rules-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_arenas_2_6_8.py \
  --export-linear --out .local/md-yuyuhakusho/arena-linear-replay
```

将上面的 `2` 和三条基线路径中的 ID 同时换为 `6` 或 `8`，即可复跑其他两场。独立 Go 测试无需 ROM 或原内核。本批实际检查范围为全部非测试 engine 源码加本批冻结测试，避免其他开发中的测试影响缺实现红灯归属：

```sh
task_root=$PWD
cd games/yuyuhakusho/engine
arena_sources=($(rg --files -g '*.go' -g '!*_test.go'))
GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" TMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestArena268' \
  "${arena_sources[@]}" arenas_2_6_8_test.go
env -i PATH="/Users/jason/.nvm/versions/node/v24.15.0/bin:/opt/homebrew/bin:/usr/bin:/bin" \
  GOCACHE="$task_root/.local/go-cache" GOMODCACHE="$task_root/.local/go-mod" \
  GOTMPDIR="$task_root/.local/md-yuyuhakusho/tmp" TMPDIR="$task_root/.local/md-yuyuhakusho/tmp" \
  GOTELEMETRY=off GOOS=js GOARCH=wasm \
  /opt/homebrew/bin/go test -count=1 -json -run '^TestArena268' \
  -exec /opt/homebrew/Cellar/go/1.26.2/libexec/lib/wasm/go_js_wasm_exec \
  "${arena_sources[@]}" arenas_2_6_8_test.go
```

该验收覆盖主回调持续规则，不代替对应资源包、背景像素、平台选场、准备就绪或完整对局的整合验收。world／bundle／UI 的接入由根任务完成；本批未修改这些共享入口，也未提交。
