# 来源 3「幽助」完整动作、输入与人物组合：实现前边界

本子任务只新增原 CPU 取证、规范夹具和先行测试，不修改生产规则。取证覆盖完整 `15a18→15b82` 动作、完整 `15540` 输入返回和 `12682→126ba` 人物规则阶段。原 HUD 已确认来源 3 为「幽助」；普通自然输入为 `source3-next-inputs.json`，实际特殊输入为 `fighter3-special-inputs.json`。自然人物为真人；来源 3 AI 只在明确私有上下文中验证。

实现前失败场景：只修改 tag 而沿用其他人物参数／动画／形状；`13/21/22/23/24` 与共用 `0f/11/15/17` 分派错误；ground throw 记录 SourceAction=3 被父动作 13 误覆盖、空中错误增加 throw；空中 selector 的 `GroundHigh-128` word 回绕、高度比较和主槽门槛错误；空中特殊 Aux48 速度不是完整 long 写入或没有合并；`21` 把横速 bit24 复制成独立锁、错误跳帧／清锁时点；`23` 完成标志先消费、成本 4 不足时的正常部分提交被回滚；`24` 强制 Pressed.A 没合并回完整 Controls 或破坏扩展 byte；`22` 主弹自身 Power 被 parent.Power>>5 覆盖、光效忙导致已经建立主弹被回滚；`1863a` 的负奇数分开算术移位、ExtraJump byte 回绕或 Facing==1 语义被简化；FX Phase／Duration、能量小数、其他人物资源、三个子槽、投技／形状、HUD、声音及共享 RNG 改写遗漏；AI 可见 child+65 遗漏；暂停／慢动作门槛、AI 后录制／播放停止顺序错误；晚期缺动画、FX、录制或 HUD 错误泄露前置写入；无证据屏蔽自然声音队列；栈与统计别名；连续流回灌期望状态；将人物规则边界冒称完整世界、子调度或平台 E2E。

复用旧 Chu 完整原字节 snapshot 和 Go 适配器，只在本模块视图 wrapper 中补齐四个人物的 child+65 攻击值。私有 A5 使用 `FFFF8400/FFFF8800/FFFF8C00/FFFF9000`，私有栈在 `FFFF00` 下；每次完整调用审计全部 64 KiB RAM 的实际改写。真实动画流只初始化一次，后续仅交付事先声明的按键、控制器、暂停／慢动作输入，不刷新期望动画、人物、子槽、FX、能量或 RNG，也不运行子对象生命周期。

六项三层测试需先于生产组合，随后保存实际 `unsupported complete fighter action/actor source` 红灯；当前完整 Input 已共用全角色 AI，按实际结果验收，不伪造缺实现。静态路径来自已有直接 Ghidra 68000 证据；REA 不支持此 raw 目标，不称作 REA 反编译。

## 已保存的原 CPU 证据

普通流复用原 core 3,600 帧基线，特殊流复用 3,602 帧基线；两者均重新从冷启动运行当前观察内核，逐视频核对完整 RAM／视频摘要。特殊流仅接受不晚于 3,600 的入口，最后两步排空跨视频边界的完整返回；视频标签不替代逻辑 tick。参数、动画和形状指针来自真实来源 3 初始化，而非只修改其他角色的 tag。

| 范围 | 完整动作 | 完整输入 | 人物规则 |
| --- | ---: | ---: | ---: |
| 普通真实菜单自然流 | 2,067 | 2,067 | 2,067 |
| 真实特殊输入自然流 | 2,067 | 2,067 | 2,067 |
| 声明私有完整调用 | 193 | 32 | 44 |
| 单初态连续流 | 12 条、657 步 | 3 条、48 步 | 4 条、96 步 |

三层自然数据是同一真实调用链的嵌套边界，不能相加当作三倍独立玩法覆盖。普通自然流包含 `11` 33 次；特殊流包含 `21` 91 次、`17` 142 次和 `22` 19 次。父体实际建立 `132de` 2 次、`1313e` 与 `134c2` 各 3 次；这是父体建立次数，不是子生命周期回调次数。两组来源 3 自然输入均为真人／开局禁用，没有自然 AI。

私有完整动作覆盖 23 个相关槽和四个 canonical 身份，并额外覆盖这些完整输出：

- 地面投技实际提交一条记录，记录 SourceAction=3，父体进入 `13/83`；空中普通攻击没有投技前置。
- 空中特殊执行完整 Aux48=`40000/60000` 写入、成本临界及 `GroundHigh-128` 的 word 回绕。原来源 3 参数块保持真实绑定；仅负奇数恢复诊断将同一参数块复制至 `FF7000`，明确覆盖两个速度参数，原 CPU 与 Go 均读取这份声明输入，不伪称原 ROM 参数为奇数。
- `21` 的三组跳帧、已有 bit24、当轮 Pressed 设置 bit24、ticks 非零时清锁的不同情况均由完整返回比较。
- `22` 保留主弹旧 Power word，例如 `3d86/4891/539c/5ea7`；光效忙时保留已建立主弹和父 Power=63。完成恢复覆盖 ExtraJump `255→0`，负奇数 `-3→-3`、`-7→-6`、最小负奇数移位结果 `a0000000`，以及 Facing 只有等于 1 的另一符号。
- `23` 完成时能量 3 保持动作 `23`、先清 Completed；能量 4 转入 `24` 且能量为 0，小数保留。`24` 覆盖强制 A、已有 C 的优先级和主／光效槽忙；完整人物调用从控制器读取扩展按下 byte `A5`，内层动作的 Pressed 从 `A500` 变为 `A540`，最终人物保持该扩展 byte。
- 私有输入覆盖真人、AI、真人录制、AI 后录制、播放、停止及禁用；8 个实际 AI 输入各消耗 4 次 RNG，完整人物再增加一次动作随机尾段。12 个暂停／慢动作人物调用为零次 RNG。

全部私有完整调用通过 64 KiB RAM 改写范围审计，未知写入为零；父体、三个子槽、其他人物资源、FX Phase／Duration、投技、形状、HUD 提交、声音、能量小数和共享 RNG 均保留实际原字节。

## 真实动画单初态流

连续流只初始化一次，完整动作本身推进时钟并选择实际 ROM 动画／形状记录。空闲保留 32 步；`0f/11/13/15` 分别为 24／32／38／32 步；蓄力 `17` 为 61 步；`21` 无输入为 46 步、合法 C 脉冲为 82 步；`22` 为 18 步后按原规则恢复 Air2；`23` 无接触 64 步、有接触 165 步；自动 `24` 为 63 步。除空中恢复 Air2 外，均到站立 0；`17/22/23` 接触流／`24` 实际建立主弹和光效，适用路径另建立蓄力附件。

首个连击刺激使用持续 B 脉冲，真实完整命令准备在连击结束后又进入 `15`，未在声明上限内恢复。采集刺激改为合法 C 脉冲，保留连击锁输入并避免重复 B 命令；没有修改冻结的 Go 测试，也没有回灌 Request、动画或其他期望中间态。此处不调度子对象，不能把父体建立描述为完整子弹／世界运行。

## 测试先行与最终组合验证

六项测试完整文件在生产支持来源 3 之前写完，SHA-256 始终为 `cdf866c122e055536551fff602069d406e1170e874df0dded4263210771ed9fe`。普通自然夹具到位后，先单独运行已有完整 Action／Actor 两项，实际得到 `unsupported complete fighter action/actor source`，保存为 `fighter3-composition-red.jsonl`。这次早期门禁不声称已执行尚缺私有夹具的其他四项；当时通过显式文件列表只排除其他独立来源 3 任务的未完成测试，保留全部生产文件，列表与原生产 SHA 已保存。

Root 随后完成生产组合，本子任务未修改生产规则。12 组规范夹具全部到位后，Root 实际运行冻结的六项测试：native `-race` 48.292 秒、Node／Wasm 13.064 秒均通过，包含真正晚期动画／录制／HUD 错误原子性。原日志留在原路径，并复制到规范的 `fighter3-composition-native.jsonl` 与 `fighter3-composition-wasm.jsonl`，报告绑定实际来源，不冒称本子任务重新执行。

最终采集器又分别重采普通和特殊自然流。六组 CPU gzip 与红灯时逐字节相同，仅 manifest 的 producer SHA 更新；记录见 `fighter3-composition-natural-reproducibility.json`。自然动作共 98 行、人物共 111 行有实际音频清槽标记，仅 `37c56/37cac` 且地址位于 `F80A..F80D` 才排除该行四槽队列相等，其他规则与未受影响队列仍完整核对；私有流没有此排除或期望队列回灌。

来源、完整夹具、采集器／适配器、测试／生产摘要、实际红绿日志和清理记录见 `fighter3-composition-validation.json`。本模块止于人物规则精灵提交之前；完整世界、新建对局及浏览器验收属于外层门禁，参见独立的世界验证说明，不能由本表数量替代。

本次只清理本子任务 77 个重复／中间文件，共 1,181,727,887 bytes。完整私有采集附带的自然库含额外私有帧，清理前单独核对其自然调用区字节摘要与规范自然夹具一致；规范夹具、两组基线／输入、ROM／工具、共享缓存、Root 原 native／Wasm 日志和 8799 服务素材均保留。

## 复跑

从 worktree 根目录运行原 CPU 采集。首个命令复现私有／连续夹具；第二个命令复现规范的普通自然夹具及其有界自然帧库，第三个命令复现特殊自然夹具。完整私有采集附带的自然帧库更大，不能把它的 gzip 摘要与规范 natural-only 图集混用。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter3_composition.py \
  --out .local/md-yuyuhakusho/fighter3-composition-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter3_composition.py \
  --out .local/md-yuyuhakusho/fighter3-composition-natural-replay --natural-only
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter3_composition.py \
  --out .local/md-yuyuhakusho/fighter3-composition-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter3-special-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter3-attacks-special-reference-frames.json \
  --baseline-manifest games/yuyuhakusho/engine/testdata/fighter3-attacks-special-reference-manifest.json \
  --natural-label special --natural-only
```

运行独立 Go／Wasm 验证只读取规范数据，不执行模拟器：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestFighter3Composition' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -run '^TestFighter3Composition' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```
