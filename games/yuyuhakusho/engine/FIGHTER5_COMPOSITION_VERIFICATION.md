# 来源 5「仙水」完整组合验收

六项先行测试已在 native 竞态检查与 Node/Wasm 全部通过，分别用时 51.361 秒和 12.534 秒。测试文件 SHA-256 始终为 `34016fa0083cdf3e67ab5676de4b5c6e6bd19312029121ac9eb961fe0319926d`。普通真人、特殊真人、私有边界、实际动画时钟连续流和完整 API 原子错误均按原冻结断言验收。

本任务验证 `0x15540` 完整输入返回、`0x15a18→0x15b82` 完整动作和 `0x12682→0x126ba` 完整人物规则。只新增独立采样器、六项先行测试、原 CPU 夹具和验收产物；共享生产实现与旧测试适配器由 Root 集成。普通与特殊真人输入复用 Root 提供的唯一原 core 3602 帧控制组。

## 实现前失败场景

- 只改 source tag，仍沿用其他角色的参数、动画或形状；把全角色 AI 输入策略当作完整 source 5 action／actor 已恢复。
- source 5 的 `11`、`2d..32` 动作槽被拒绝或错误分派到其他角色；完整调用顺序、实际动画时钟、hit-stop 输入准备、公共动作或 tail selector 顺序有误。
- ground／air／special selector 的成功、能量不足、子对象占用、按键变化、朝向或其他门槛错误；动作构造器的速度、能量小数、Power、接触位或动画状态写入丢失。
- canonical Body 的完整 VX／VY／Gravity、Aux48 long、Energy long、Power、StunTicks、MovementSource 或三份 ChildExtras 在 action／actor 间丢失；用不同 raw offset 的现有字段代替新字段。
- 参数元数据未按当前 `+90` 读取；encoder 保留旧 raw bytes，掩盖 owned field 未写回；若原 CPU 改写移动源，同次 tail 仍使用旧参数。
- 子对象与 FX 的分配顺序、完整初始化范围、池满行为、child size 同次可用性、其他人物资源或共享 RNG 漏比较；输入 Envelope 与 AI 的控制状态分裂。
- disabled、human、AI、human-record、AI-record、playback、playback-stopped 路由错误；非 AI 输入消耗 RNG；记录、回放、终止和流边界写入有误。
- actor 的暂停／慢速门槛、注册、bounds、HUD、draw 和输入／动作事件遗漏；AI／记录／动作已执行后遇到晚期错误，泄露状态或事件。
- 非 canonical A5、栈位置错误或恢复时漏掉 refresh cycles，造成伪差异；原 CPU 调用未到真实返回便截断。
- 只审计已知字段，漏掉 64 KiB RAM 其他写入；把未知写入整体遮蔽，或错误排除 `FF7000..FF77FF` 历史区。
- 未观察到音频消费者 PC 与队列地址就忽略声音差异；hook 回放与无观察器控制组不同仍输出通过。
- 连续流回灌期望动画、人物、资源、子对象、history、FX 或 RNG 中间态；将 private 实验误称自然输入或完整世界证明。
- 缺少命令／动画数据、输入记录空间不足、HUD／注册缓冲不足等早期或晚期失败，返回部分修改后的状态、改写调用者切片或发出事件。

## 先行合同

六项 wrapper 固定覆盖普通、特殊和私有完整原 CPU 调用；单初态 action、input、actor 连续流；完整 API 的原子错误。数据采样不使用 Go 实现或期望状态解释器。每条流只初始化一次，后续仅交付声明的外部输入，动画时钟和全部规则状态由原 CPU 自行推进。

当前复用既有 Action／Actor 的 `MovementSource`、完整运动／资源字段和 `ChildExtras`。MAIN 静态核查指出 source 5 创建 child 时还需要 `+4c` 完整 Gravity 和 `+fe` Owner；前者只修改最高 byte。Root 已确定新 view 为 `Fighter5ChildExtras [3]Fighter5ChildExtra`，包含 `Gravity int32` 和 `Owner uint16`。source 5 的旧 `ChildExtras.DragonBank` 置零，只有新 Owner view 持有真实 `+fe`；World 原有同位置 word 保留旧名以兼容飞影存档与冻结测试，按角色上下文映射。

`testdata/fighter5-composition-schema-red.jsonl` 保存缺少新 view 的真实 compile red，`fighter5-composition-test-first.json` 绑定冻结测试、文档和当时共享文件。Root 随后只增加 view 通道，再等实际 `unsupported5` runtime red 后接入完整 source 5 实现。MAIN 未发现 source 5 改写 `+90`，完整流也验证了默认移动参数保持不变。

## 自然原 CPU 与 runtime red

普通输入复用 `source5-next-inputs.json` 和 `fighter5-attacks-reference-frames.json` 的唯一无观察器原 core 控制组。带观察器回放的 3602 帧 RAM／画面摘要完全一致；只接受视频标签 3600 及以前的入口，另留 2 帧收完调用。完整 input、action、actor 各取得 2067 次返回，A5 全为 `FFFF8400`。checkpoint 的真实 source tag 为 5，AnimationSet 为 `0xca6da`，ShapeSet 为 `0xcb38a`，MovementSource 为 `0x125b4`。

schema-only 接入后，冻结六项测试已真实运行。完整 action 首个自然调用返回 `unsupported complete fighter action source`，完整 actor 返回 `unsupported complete fighter actor source`；自然 input 2067 次比较已通过，AI 调用数为 0（human 1794、disabled 273）。第一次运行时特殊与私有／连续流夹具尚未采集，其缺失失败按原样保留，没有制作占位数据。

本次 runtime red 的命令使用 engine 目录的显式 Go 文件列表，仅排除同期尚未定义隔离 API 的新 `fighter5_projectile_test.go`；全部生产文件、既有 helper 和六项冻结 wrapper 都参与编译。`testdata/fighter5-composition-runtime-red.jsonl` 保留原始结果，`fighter5-composition-runtime-red-files.json` 绑定实际命令、排除列表、全部源文件和自然夹具 SHA。包级构建的同期阻断另保存在 `fighter5-composition-schema-ready.jsonl`，不称为 source 5 的 runtime red。

## 私有边界与单初态连续流

私有实验已取得完整 action 165 次、input 31 次、actor 43 次，覆盖四个 canonical A5、22 个动作槽、7 种输入路由和暂停／慢速门槛。每次完整调用均比较全部 64 KiB RAM，未知写入为 0；`FF7000..FF77FF` 预填脏数据并保持未改。私有 AI 与自然真人证据分开记录。

独立连续流包含 action 16 流／923 步、input 3 流／48 步、actor 4 流／96 步。`2d/a8` 经过 93 次真实时钟调用退出，`2e/a4` 与 `2e/a6` 各 65 次；`2f` 三姿态分别为 53／55／47 次并构造 `1414e`，`30/ac` 为 29 次并回到空中动作。`31/a3` 的三种 variant 分别经过 85／87／79 次，原 CPU 自行构造 `146ba` 和 `143e4` 后恢复站立；没有调度 child 生命周期，也没有回灌人物、动画、资源或 RNG。

追加私有数据的带观察器重放再次与同一控制组完全一致，三份先行自然 gzip 逐字节未变，见 `fighter5-composition-producer-reproducibility.json`。全部 9 份当前规范夹具的 raw／gzip SHA 已核验。第二次六项 runtime red 保存于 `fighter5-composition-private-red.jsonl`：完整 action／actor 及其连续流仍真实报 unsupported；48 步 input 流已比较通过，原子错误测试通过；特殊真人夹具缺失单列。

## 特殊真人输入与完整 runtime red

特殊输入复用 Root 提供的 `source5-special-discovery-inputs.json` 与 `fighter5-attacks-special-reference-frames.json` 唯一无观察器控制组。3602 帧 RAM／画面摘要完全一致，完整三层各取得 2067 次返回；source 5 AI 调用数为 0。自然 action 中，`2d=93`、`2e=130`、`2f=155`、`30=29`、`31=37`、`32=46`。

| 证据组 | 完整 Action 返回 | 完整 Input 返回 | 完整 Actor 返回 |
| --- | ---: | ---: | ---: |
| 普通真人输入 | 2067 | 2067 | 2067 |
| 特殊真人输入 | 2067 | 2067 | 2067 |
| 私有边界 | 165 | 31 | 43 |
| 独立连续流 | 16 流／923 步 | 3 流／48 步 | 4 流／96 步 |

12 份规范夹具全部到位后，原六项测试已在正式 engine 包运行，见 `fighter5-composition-full-runtime-red.jsonl` 和 `fighter5-composition-full-runtime-red-files.json`。2 项通过（全部原版 Input 与原子错误），4 项均在完整 action／actor 单调用或连续流报出实际 unsupported source 5；没有缺夹具或编译问题，运行期间全部源文件 SHA 未变化。此前一次包级编译遇到两名 owner 的新私有 helper 同名，修正后未改测试；原始编译日志另保留在 `fighter5-composition-red.jsonl`，不将其混作 runtime red。

全部规范 raw／gzip SHA 已核验，见 `fighter5-composition-capture-integrity.json`。随后清理本采样器的三处临时原始／重复输出目录，共 712,226,620 字节，逐文件清单保存在 `fighter5-composition-cleanup.json`。所有规范 CPU 夹具、manifest、采样摘要、原始 red 和 SHA 绑定保留；未修改共享缓存、ROM、工具或 Root 控制组。

## 最终接线与验证

Root 接入 shared Action／Actor 与 tail 后，六项冻结测试在首次完整接线验证中均通过：`fighter5-composition-native.jsonl` 保存 native race 原始结果，`fighter5-composition-wasm.jsonl` 保存 Node/Wasm 原始结果；未为测试结果调整冻结断言或夹具。验证期间所有生产 Go 文件和本组实际使用的 Action／Actor／Input 旧 helper 未变化；Root 同期修改了未被六项测试调用的 `chu_world_test.go`，其前后 SHA 单独记录。

`MovementSource`、完整 Aux48／Energy、三份 Gravity／Owner extras 与其他既有字段在 action／actor 间往返。source 5 的 `+fe` 只有 Owner view 持值，飞影 DragonBank view 为零；独立 encoder 显式写入真实 offset，不依靠保留旧 raw bytes 掩盖漏写。World 的同位置 word 与本组 action／actor view 映射由 Root 独立先行验证，本报告不代替该 World 验收。

完整人物规则止于 sprite 之前，不调度 child、collision manager 或世界回调。本报告不替代完整世界、平台联机、托管、渲染或发布验收。CPU 证据来自固定原版 core；静态指令材料来自既有 direct Ghidra 68000 分析，不称为 REA 反编译。

## 复现与产物

在 worktree 根目录运行：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" go test -race -count=1 -json -run '^TestFighter5Composition' ./games/yuyuhakusho/engine
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" GOOS=js GOARCH=wasm go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestFighter5Composition' ./games/yuyuhakusho/engine
```

`fighter5-composition-final-commands.json` 记录三次原 CPU 采样及最终验证命令。`fighter5-composition-validation.json` 绑定 12 组 CPU 夹具、manifest、固定 ROM／core、唯一普通／特殊控制组、先行测试、全部 red、最终日志、采样器与验证时源文件 SHA。`fighter5-composition-final-files-before.json` 保留验证启动时的全部文件绑定。未写 shared production、旧 helper、bundle、UI 或 PROGRESS，未提交。
