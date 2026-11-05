# 来源4「飞影」完整组合：实现前边界

本子任务只新增完整input／action／active actor原CPU组合、六项先行测试／规范夹具和只读Root接线复核，不写生产、不改bundle／PROGRESS。边界为15540完整输入返回、15a18→15b82完整动作及12682→126ba人物规则。合法普通／特殊输入由Root提供，selector生成唯一原core3602基线，不重复control；自然human与private AI明确区分。

实现前失败场景：只换tag仍用其他角色参数／动画／形状；0e／12／25～2c路由错误；2a吞龙写MovementSource=15946却仍用默认移动／跳跃参数；旧Action／Actor不传+90或旧encoder保留previous +90造成漏比较；29 globalA038轮转、child+FE history index、Aux48／Held／Pressed／ContactFlags extras未建准确owner、写入藏在未知区域或错误复用其他offset字段；吞龙复制Held／Pressed低byte破坏扩展byte，子contact bit4读改写丢失；history buffer／游标和FX共用池顺序错误；父三子、其他人物资源、能量小数、FX Phase／Duration、投技／形状／HUD／声音及共享RNG遗漏；safe stack／canonical A5错误；无实际音频PC／地址便排除队列；晚期缺库／history／缓冲错误泄露前置修改；真实动画流回灌期望中间态；把人物规则或private实验称完整世界／自然证明。

旧ChuActionSnapshot／ChuActionState／ChuActorFighter目前无MovementSource，旧encodeChuAction不写+90；ChuSpecialChild也不含+FE，旧action snapshot不含globalA038。必须使用本模块独立wrapper显式decode／encode这些raw字段，不能通过保留previous bytes伪装正确。WorldObject已有MovementSource owner，但没有与+FE同位置的字段，不能拿Aux0A／Aux0E／StreamPosition等不同offset借用。字段与history合同由Root／MAIN／child peer先统一，接口确定后再冻结六测试，不改旧适配器。

完整流每步按当前MovementSource选择真实immutable参数，不固定使用stream.Initial参数；15946增强参数只作为已验证只读数据，不运行ROM解释器。四canonical私有A5为FFFF8400／FFFF8800／FFFF8C00／FFFF9000，栈在FFFF00下，refresh恢复为true；每完整调用审计全部64KiB RAM。流只初始化一次，仅交付声明外部输入，不喂期望人物／动画／history／FX／资源／RNG，不调度子生命周期。静态使用直接Ghidra68000，不称REA反编译。

## 最终验证

前述内容记录实现前的失败场景和接口边界。共享实现接入后，六项冻结测试在 native 竞态检查和 Node/Wasm 上全部通过。测试文件 SHA-256 仍为 `a29fcbeaa0fa2049d32bec75ba875c155c78b1675cd0c57b9114d8b6cb68f2a3`；验证时使用的生产文件及 input／action／actor 适配器均未变化。

| 证据组 | 完整 Action 返回 | 完整 Input 返回 | 完整 Actor 返回 |
| --- | ---: | ---: | ---: |
| 普通真人输入 | 2067 | 2067 | 2067 |
| 特殊真人输入 | 2067 | 2067 | 2067 |
| 私有边界 | 181 | 31 | 43 |
| 独立连续流 | 15 流／1281 步 | 3 流／48 步 | 4 流／96 步 |

普通和特殊输入各复用 selector 提供的唯一原 core 基线；两次带观察器回放的 3602 帧 RAM／画面摘要均与基线一致。只接受视频标签 3600 及以前的入口，另留 2 帧收完调用。自然输入中的 source4 AI 调用数为 0，AI 证据来自私有实验。特殊自然流包含 `25=23`、`26=43`、`27=130`、`28=58`、`29=55`、`2a=1`、`2c=106` 次动作；该流未观察到吞龙、`2b` 或移动源切换，不能据此声称自然吞龙闭环。

私有实验覆盖四个 canonical A5、24 个相关动作槽、全部 64 KiB RAM 写入审计、7 种输入路由和暂停／慢速门槛。未知区域写入为 0。`FF7000..FF77FF` 在实验前填入脏数据，完整人物调用未改写该历史区；人物动作只更新 A038 游标及 child `+FE`。连续流只初始化一次，后续仅交付声明的外部输入；增强／默认 AE 动作各自经过原 ROM 的 262 次完整调用后退出，没有回灌中间状态。

## 集成差异及修正

第一次 native／Wasm 集成运行均为 3 项通过、3 项失败。`phase-2a-116` 的规则状态已与原 CPU 完全一致，但旧 `encodeChuAction` 保留了切换移动源之前的参数元数据：WalkSpeed、Gravity 和 VerticalVelocity 共 3 处。Root 仅修正旧测试适配器，在非零移动源改变时由独立原版参数夹具刷新这些只读字段；生产规则和六项冻结断言均未因此修改。原始红灯日志和诊断保存在 `testdata/fighter4-composition-integration-*`。最终 native 为 51.612 秒，Wasm 为 14.365 秒。

## 共享接线复核

- Action 在入口及 motion 完成后、tail 之前按当前 `MovementSource` 解析 immutable 参数，覆盖同次吞龙改源；World 库含默认移动源及 `0x15946`。
- `MovementSource`、A038 和三份 ChildExtras 在 Action、Actor、World 间逐项传递。child Aux48 为完整 long，Held／Pressed 只写低字节并保留扩展字节，ContactFlags 为读改写，DragonBank 唯一对应 `+FE.word`。
- FX Phase 对应 canonical `Clock.Duration`。龙头／龙尾使用既有 canonical 字段的窄视图；历史仅有六个有效平面，Status 只拥有原高字节。prefix clear 保留未覆盖字段和不应清除的低 word。
- `0e`／`12` 和 `25..2c` 经 source4 路由；其他已支持角色保留原分支。MovementSources 在接线中只读。

完整人物返回仍止于 sprite 之前，不调度 child、collision manager 或世界回调。本报告不替代自然吞龙、完整世界、平台联机、托管或最终发布验收。静态材料来自直接 Ghidra 68000 分析，不能称为 REA 反编译。

## 复现与产物

在 worktree 根目录使用本地缓存运行：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" go test -race -count=1 -json -run '^TestFighter4Composition' ./games/yuyuhakusho/engine
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" GOOS=js GOARCH=wasm go test -count=1 -json -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" -run '^TestFighter4Composition' ./games/yuyuhakusho/engine
```

`testdata/fighter4-composition-validation.json` 绑定 12 组 CPU 夹具、manifest、先行测试、红灯、参数数据、采样器、旧适配器、验证时生产文件和最终日志。`fighter4-composition-producer-reproducibility.json` 记录最终采样器重采时 9 组既有 CPU 数据未变化。`fighter4-composition-cleanup.json` 记录逐项校验后清理的 5 个采样临时目录及重复摘要，共 1,190,885,984 字节；共享缓存、ROM、工具、Root 日志及 8799 页资源未修改。
