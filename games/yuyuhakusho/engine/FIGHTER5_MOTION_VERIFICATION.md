# 来源 5 MAIN 运动恢复

已恢复仙水的 `11/2d/2e/2f/30/31/32`，入口 `1cb08/1b67a/1b6c4/1b704/1b7fa/1b8f4/1b9bc`，停止于共享尾段前的 `15afc`。`11/2d` 复用已经恢复的来源 10 实现，source5 的真实人物、动作时钟、参数来源与完整未写字节另经独立对照。四个选择器及组合结束／最终发射构造由 `fighter5_attacks.go` 负责，三个子回调由独立 projectile 模块负责。

失败场景在 [FIGHTER5_MOTION_FAILURES.md](FIGHTER5_MOTION_FAILURES.md) 中先行列出。静态证据复用 `fighter-action-slots-focused.txt`、`fighter-action-constructors-focused.txt` 与 `fighter-action-coverage.json`；这些 direct Ghidra 指令只说明静态行为，不代表自然执行。REA 不支持该 raw 68000 输入。

接口为 `StepFighter5Motion(Fighter5MotionState, JumpParameters)`。状态嵌入既有 `ChuSpecialMotionState`，仅增加 `[3]Fighter5ChildExtra`，其中 `Gravity` 映射各子对象既有 `Body.Gravity` full long，`Owner` 映射真实 `+fe` word。Root 保留 `WorldObject.DragonBank` 的物理唯一存储和兼容名称，按实际 Tag 选择窄语义视图；source5 的 Action／Actor 使用 `Fighter5ChildExtras.Owner`，其飞影窄视图 `DragonBank` 为零。不存在同址的两份持久化字段。

父 `AlternateVelocity` 是 `+48` full long 的唯一存储；`31` 临时读取、增加其 signed 高 word 并保留低 word。父 Gravity 的最高 byte 是发射 variant；子对象初始化只替换子 Gravity 的最高 byte，保留低 24 bits。`31` 建立第二子对象时只写速度高 word；`2f/30/32` 建立第一子对象时复制完整速度和坐标后调整坐标高 word。完整脏槽字节对照负责验证这些窄写入。

原 CPU 采集由 `scripts/yuyuhakusho/capture_fighter5_motion.py` 执行，只重放 Root 已冻结的无观察原版控制。每次原 MAIN 调用都保留完整 64 KiB RAM 的变更摘要，允许域仅为当前人物 family 与实际栈。独立音频消费记录实际 PC／地址，中断记录实际入口／退出及每次变更，域外变化必须能沿该链完整解释。人物、三个子槽、完整 FX 池和游标另以全部字节及全部建模字段精确比较。

连续流只建立一次初态，每步执行原 `19c96` 时钟，并由 Go 的独立 `StepAnimationClock` 对照；之后只根据独立选择的 immutable 原 ROM 记录刷新 Duration／Flags。参数每步按当前 MovementSource 选择。不回灌 expected 时钟、子对象或中间人物状态。

已保存的原 CPU 证据如下。采集两次的三个压缩夹具均逐字节一致，第二次明确绑定第 2,400 视频步的实际 header `0001266801000002`、source tag 5、`+90=125b4`、`AN=ca6da`、`shape=cb38a`；不以构造入口地址替代 movement 参数来源。

| 范围 | 调用／步骤 | 结果 |
| --- | ---: | --- |
| 普通与特殊合法输入的自然 MAIN | 519 | `11` 33、`2d` 93、`2e` 130、`2f` 152、`30` 28、`31` 37、`32` 46；两组各 3,602 视频步的 RAM／画面摘要与各自独立无观察原控制一致 |
| 完整脏对象私有矩阵 | 5,474 | `11` 1,152、`2d` 630、`2e` 512、`2f/30/32` 各 708、`31` 1,056 |
| 真实 ROM 时长的单初态连续流 | 48 条、2,749 步 | 42 条站立、4 条空中恢复、2 条动作 `03`；含 12 条 `31→32` 完整继续链 |
| 子槽窄初始化 | 1,821 | 第一子槽 `2f/30/32` 各建立 543 次；第二子槽 `31` 建立 192 次；另有 144 次最终发射构造 |

全部私有调用和连续流的完整 64 KiB 审计均没有域外变化、FX 申请、随机调用或独立音频消费。所有人物、子槽和池字节都参与比较；原池游标 `ff80` 及全部池字节原样保留。特殊自然记录另观察到 8 次 IRQ，位于第 1,838 视频步的 `2e`、2,085／2,132 步的 `2f` 和 3,393 步的 `31` 内；4 个调用存在域外变化，1 个调用存在独立音频消费，全部由实际入口／退出的逐字节变更链解释，没有无法解释的变化。

自然 `31` 记录中 variant 1 只有 5 次，variant 2 有 32 次；唯一自然 `31→32` 转移位于 3,419 视频步，对应 `19444`。没有 variant 0 的自然 MAIN `31`，也没有 `19408/19426` 的自然完成证据。三个 variant 的完整继续链由明确私有的 12 条连续流覆盖，不混为自然可达。

测试 SHA-256 为 `e2755b908063572fdf59dd43c609698f8a65642c4405274e4d37c67343c255cd`。先有失败场景、两次原 CPU 夹具和冻结测试，再运行真实缺少 `StepFighter5Motion` 的编译红灯，最后写运动行为。批准的类型曾先于测试提供给组合层；冻结时 `fighter5_motion.go` 只有类型，无 Step 行为。该顺序与逐文件 SHA 见 `testdata/fighter5-motion-freeze.json`，真实输出见 `fighter5-motion-red.jsonl`。实现首版通过所有冻结断言，没有修改测试来获得通过。

五项冻结测试已在完整 engine 包编译下通过 native `-race`（18.992 秒）和 Node/Wasm（5.681 秒）检查，没有文件排除或 stub。精确命令、全部五项结果、最终源码／采集器／夹具／日志 SHA 见[验证报告](testdata/fighter5-motion-validation.json)，原始日志为 `fighter5-motion-native.jsonl` 与 `fighter5-motion-wasm.jsonl`。早期在子对象 API 尚未定义时使用过明确文件选择做诊断；最终完整包验证已覆盖该临时限制，重复诊断产物已清理。

自然追加报告 `fighter5-motion-natural-append.json` 保留旧 oracle manifest／摘要与新控制的来源链，并核对运动实现、测试、原采集器、私有矩阵、连续流和参数数据都未改动。本模块不涵盖共享尾段、子对象／FX 生命周期或完整世界。子对象 worker 的创建者—时钟—子回调连续流是另一份组合证据，不替代世界级验收。

从 worktree 根目录运行以下命令。原 CPU 采集依赖已经固定的研究 ROM、两内核和 Root 提供的无观察原控制；不会创建新的原控制。采集结果写入新输出目录，Go 验证读取规范夹具。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter5_motion.py \
  --input docs/research/md-yuyuhakusho/source5-next-inputs.json \
  --reference games/yuyuhakusho/engine/testdata/fighter5-attacks-reference-frames.json \
  --manifest games/yuyuhakusho/engine/testdata/fighter5-attacks-reference-manifest.json \
  --out .local/md-yuyuhakusho/fighter5-motion-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter5_motion_natural.py \
  --input docs/research/md-yuyuhakusho/source5-special-discovery-inputs.json \
  --reference games/yuyuhakusho/engine/testdata/fighter5-attacks-special-reference-frames.json \
  --manifest games/yuyuhakusho/engine/testdata/fighter5-attacks-special-reference-manifest.json \
  --out .local/md-yuyuhakusho/fighter5-motion-special-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighter5Motion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighter5Motion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

第一条命令复现普通自然记录、私有矩阵与单初态连续流。第二条命令只追加或替换同名特殊输入场景，保留全部冻结实现、测试及私有证据；同一场景不会重复计数。原控制、输入或摘要不匹配时，采集器停止。清理范围与逐文件摘要见 `testdata/fighter5-motion-cleanup.json`；只删除本模块的重复采集和诊断产物，保留规范夹具、真实红灯、最终两平台日志和来源链。
