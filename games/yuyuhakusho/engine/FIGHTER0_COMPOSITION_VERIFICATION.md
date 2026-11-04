# 来源 0「幻海」完整动作、输入与人物组合：实现前范围

本阶段只新增原 CPU 取证、规范 fixture 与先行测试，不编写生产规则。边界为完整 `15a18→15b82` 动作、完整 `15540` 输入返回和 `12682→126ba` 人物规则阶段。真实菜单输入使用 `source0-next-inputs.json`；另追加实际触发散射弹与释放光效的 `fighter0-projectile-inputs.json`。来源 0 的自然人物为真人，不把私有电脑、录制／播放调用写成自然 AI。

实现前失败场景：仅更改 tag 而保留来源 9／10 的参数、动画或形状表；五个新增槽与共用 `11/15` 分派错误；地面／空中投技参数或空中继续选择顺序错误；命中停顿遗漏命令／输入准备或重复时钟；真实动画时钟与阶段位顺序错误；蓄力能量 word 非零、回绕及小数、Gravity 最高 byte 或弹速 long 的部分写入错误；`17` 主子槽忙或 `+300` 忙时错误回滚正常副作用；`18` 忙槽阻止轮转游标推进；四次 FX 分配部分成功及只有第 1／4 个 Phase=8 的改写遗漏；父／三个子槽、其他人物能量、FX、投技、形状、HUD、录制、声音与 RNG 的共享状态分裂；A5 正 24-bit 别名代替 canonical 符号扩展身份；输入中的可见 child `+65` 漏解码；AI 后录制、播放结束／停止或暂停／慢动作门槛错误；自然声音消费者没有实际 PC／地址证据便排除整队列；私有栈覆盖统计；动画库或录制／HUD 晚期错误泄露前置修改；连续流逐步注入期望中间态；将本模块误称子对象调度、完整世界或平台验收。

测试复用旧 Chu 完整原字节 snapshot／适配器，只在独立 wrapper 中补齐全部可见人物的 child 攻击值。四个私有源指针使用 `FFFF8400/FFFF8800/FFFF8C00/FFFF9000`，私有栈位于 `FFFF00` 下；每个完整调用另审计 64 KiB RAM 实际改写范围。连续流只初始化一次，以真实 ROM 动画时长和完整动作尾段自推进；后续只接收声明的外部按键／控制器、暂停与慢动作输入，不推进子对象或回灌人物、动画、FX、能量或 RNG。

三个 API 沿用现有签名：`StepFighterAction(ChuActionState, uint8, ChuActionParameters, CommandLibrary, AnimationLibrary)`、`StepFighterInput(ChuInputState)` 与 `StepFighterActor(ChuActorState, uint8, ChuActorView, CommandLibrary, AnimationLibrary)`。当前动作和人物 API 拒绝来源 0，完整输入已使用全角色 AI，预期可独立通过；以实际运行结果保存红灯，不伪造未发生的缺实现。静态入口复用直接 Ghidra 68000 证据，不称作 REA 反编译。

## 已保存的完整原 CPU 证据

两组自然流分别复用已有未观察原内核基线。本采集器从冷启动重新运行观察内核，逐视频核对完整 RAM／视频摘要：普通输入 3,600 条，特殊输入 3,602 条。特殊流只接受视频标签不晚于 3,600 的 API 入口，最后两步用于排空跨视频边界的完整返回，不把视频标签当作逻辑 tick。人物／动作／输入是同一实际调用链的嵌套边界，不将三列相加当作三倍独立覆盖。

| 范围 | 完整动作 | 完整输入 | 人物规则 |
| --- | ---: | ---: | ---: |
| 普通真实菜单自然流 | 2,066 | 2,066 | 2,066 |
| 散射／蓄力真实菜单自然流 | 2,067 | 2,067 | 2,067 |
| 声明私有完整调用 | 162 | 28 | 40 |
| 单初态流 | 9 条、577 步 | 3 条、48 步 | 4 条、96 步 |

普通自然流包含动作 `0e` 50 次、`11` 31 次。特殊自然流包含 `17` 128 次和 `18` 53 次；在这些父动作完整调用内实际观测到建立散射弹 3 次、蓄力附件 2 次、主弹和释放光效各 1 次。这里的数量是父体建立次数，不能与其他子任务的生命周期调用次数混用。两组来源 0 自然输入均为开局禁用／真人，没有自然 AI。

私有动作覆盖 21 个相关槽及四个 canonical 身份，另有有界成本／请求临界、地面／空中真实投技、主子槽及 `+300` 占用、能量小数、蓄力模式和 FX 部分分配。`16` 的四 FX 场景实际包含 0／1／2／4 次成功；四次成功时原 `+5a` 依次为 `[8,24,24,8]`，两次成功时第二个保留其旧值 1，不能把所有槽统一写 8。完整原字节与 `EffectExtras.Phase` 均保留。

九条完整动作流均从单初态推进到站立状态 0；空闲流保留 32 步。`0e/11/15/16` 分别为 24／30／24／74 步；`17` 为 61 步并实际建立 `132de/1313e/134c2`；`18` 为 74 步并建立 `12c96`；`19` 无接触 64 步、有接触 194 步，后者包含原完成后进入 `16`、落地并恢复。没有调度子对象，也没有用预期中间态刷新动画或父体。

输入／人物流另覆盖真人、显式 AI、真人录制、AI 后录制、播放、播放停止及禁用。私有完整输入的真实 AI 共 8 次，其 RNG 调用为 4 或 5 次；对应完整人物再多一次动作尾段 RNG。普通动作每次恰好调用一次 RNG；12 个暂停／慢动作人物调用为零次。

## 先行测试的实际红灯

六项测试完整文件先于来源 0 生产组合修改，SHA-256 为 `fb7ee9bd8ba9fdfe607b7bee54854f4a1638615d1835787ef319f6117d78e62b`。实际运行红灯保存为 `testdata/fighter0-composition-red.jsonl`：完整动作、人物及相应连续流均实际报 `unsupported complete fighter action/actor source`；完整输入的普通／特殊／私有调用已经通过，错误原子性检查通过。当时动作／人物仍在入口拒绝来源 0，其真正晚期动画／HUD 错误路径仍须在生产组合后的复跑确认。

首次整包构建被并行的 `fighter0_motion_test.go` 缺实现阻塞，另保留为 `fighter0-composition-sibling-build-red.jsonl`，没有把它算作本模块红灯。随后使用 `fighter0-composition-red-files.json` 中的显式源文件列表，只排除其他独立任务尚未实现的来源 0 测试，保留全部当时生产文件，取得上述实际运行错误。测试、原生产源码摘要及红灯摘要保存在 `fighter0-composition-test-first.json`。

本子任务没有编写生产规则。此交接阶段只确认 CPU 取证与测试先行，不声称独立动作／人物规则已经通过；后续生产组合的 native／Wasm 结果需由集成任务另行保存。

## 取证限制与重现

自然动作共 81 行、人物共 94 行有实际 `37c56/37cac` 声音清槽标记；只有标记地址属于 `F80A..F80D` 才排除该行四槽声音队列相等，其他规则与未受影响队列仍完整比较。Go 不接收清理后的期望队列。私有连续流无中断声音排除。

两次私有刺激配置校验失败已在采集器修正后重新完整采集：空中请求 3 的来源 0 成本为 16，散射流 `VelocityX` 高 word 应从游标 0 开始。Go 测试断言没有为此修改。最终特殊流另重采一次以绑定同一最终采集源码，全部 CPU gzip 逐字节不变，仅 manifest 的 `capture_sha256` 更新；证据在 `fighter0-composition-special-reproducibility.json`。

本范围止于人物规则精灵提交之前，不覆盖初始化、精灵／子对象／FX 生命周期、碰撞管理器或完整世界；原 CPU 仅用于开发取证。来源、采集器、复用适配器、12 组 fixture／manifest、先行测试和红灯摘要见 `fighter0-composition-validation.json`。只清理本子任务 51 个重复／中间文件，共 711,280,902 bytes；规范夹具、两组基线、输入、ROM／工具、共享缓存及 8799 服务素材保留，见 `fighter0-composition-cleanup.json`。

从 worktree 根目录复跑原 CPU 取证：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter0_composition.py \
  --out .local/md-yuyuhakusho/fighter0-composition-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter0_composition.py \
  --out .local/md-yuyuhakusho/fighter0-composition-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter0-projectile-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter0-projectile-reference-frames.json.gz \
  --baseline-manifest games/yuyuhakusho/engine/testdata/fighter0-projectile-reference-manifest.json \
  --natural-label special --natural-only
```

生产组合完成后运行此模块的聚焦规则验证，不使用 ROM 或模拟器：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestFighter0Composition' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -run '^TestFighter0Composition' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```
