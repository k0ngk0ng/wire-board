# 来源 8「户愚吕兄」完整组合：实现前边界

本子任务只新增原 CPU 完整 input／action／active actor 取证、规范夹具、六项先行测试和只读接线复核，不编写生产规则。规则边界分别为 `15540` 完整输入返回、`15a18→15b82` 完整动作及 `12682→126ba` 活跃人物阶段。合法菜单和特殊输入由 Root 提供，唯一未观察原 core 基线由 selector 任务生成；不重复采集 control。

实现前失败场景：仅改 tag 而沿用其他来源参数／动画／形状；`3e/3f/40` 及共用 `0f/11` 分派错误；地面投技与无投技前置的空中普通选择混淆；成本／方向／Changed／子槽门槛或部分写入错误；大端 `+48.w` 错当 Aux48 低 word，清零／递增／signed 比较破坏真正低 word；`40` 的 ad／ae 姿态切换、128／129 门槛、按 C 终止、速度加减边界与算术移位错误；`3f` 在 busy 前错误回滚正常分支、147ae 建立遗漏队伍 byte＋10、坐标／缓存／旧字段；`3e` 位移、减速、接触 bit2/3 和帧跳转顺序错误；父三子、FX Phase／Duration、其他人物资源、投技／形状、HUD、能量小数、声音及共享 RNG 改写遗漏；canonical A5 与正 24-bit 别名混用；栈覆盖统计；无实际 PC／地址便屏蔽自然队列；晚期缺库／录制／HUD 错误泄露前置写入；连续流回灌期望中间态；将私有 AI 写成自然 AI，或将人物规则边界写成完整世界／子生命周期。

`1991c` 的清零及 `1c298/1c29e` 的增比操作作用于 Aux48 **高 word**，低 word 保留；这是对启动消息中低 word 表述的明确纠正。不新增另一份计数。复用旧 Chu 完整原字节 snapshot／适配器，独立视图 wrapper 补齐可见 child+65。私有源使用 `FFFF8400/FFFF8800/FFFF8C00/FFFF9000`，栈在 `FFFF00` 下；完整调用审计 64 KiB RAM 改写。恢复存档使用 `restore_refresh_cycles=True`。

六测试在 shared driver 支持来源 8 之前冻结；取得自身真实 unsupported8 Action／Actor 运行错误后通知 Root。其他未完成测试可以按明确源文件列表排除，生产文件不得排除。真实动画单初态流只接收声明的外部按键／控制器、暂停／慢动作输入，不回灌人物、动画、子槽、FX、资源或 RNG，也不推进子对象。静态入口来自已有直接 Ghidra 68000 证据，不称为 REA 反编译。

## 已取得的普通与私有证据

普通真实菜单输入的原 core 3,602 条基线由 selector 任务唯一生成，输入 SHA-256 为 `e70b955c9be3e197fd5ab91189e0621c6e8d251987a5a1d57cdffbf3f14ffb76`。本采集器重新从冷启动运行观察内核，逐视频完整 RAM／视频一致；只接受不晚于 3,600 的入口，末两步排空完整返回。普通 input／action／actor 各 2,067 次，三层为同一调用链的嵌套边界，不相加当作三倍独立覆盖。输入均为真人／开局禁用，未观察自然来源 8 AI；普通流没有专属 `3e/3f/40`，不能据此声明特殊自然覆盖。

菜单发现受阻期间，按 Root 明确授权先使用已证来源 2 的旧普通 checkpoint 作为声明私有背景，只从经过原 CPU 验证的来源 8 初始化 fixture 复制实际动画／形状／参数指针，输出严格标记 private，不生成来源 8 natural。该早期数据与红灯保留独立历史摘要。合法菜单解决后，全部有界私有矩阵与连续流又从真正来源 8 checkpoint 重新采集，后者为规范最终 fixture；不把修改 tag 的旧背景称为自然证据。

最终私有完整 Action／Input／Actor 分别 156／31／43 次，覆盖 19 个相关槽与四个 canonical 身份。三层共享的父三子、FX Phase／Duration、其他人物资源、投技／形状、HUD、声音、能量小数与共享 RNG 均比较实际原字节；每次调用的 64 KiB RAM 改写审计未知写入为零。完整动作每次一轮 RNG，私有实际 AI 输入使用其原随机路径，不按自然 AI 报告。

`40` 计数边界实际为 `007fabcd→0080abcd` 仍 ad、`0080abcd→0081abcd` 转 ae，`ffffabcd→0000abcd`、`7fffabcd→8000abcd` 均按 signed word 比较继续 ad；低 word `ABCD` 不变。C 触发在计数递增之后进入 ae。`3f` 的 busy、Team byte 回绕、无 mask 的属性加法、只写 Ground 高 word及旧子速度／Power保留均由完整返回覆盖。

九条动作流共 767 步：空闲 32，`0f/11` 为 21／33，`3e` 两条各 80 步后按原规则恢复 Air2，`3f` 空／忙各 73 步，`40` 超时／C 结束分别 239／136 步后站立 0。空槽 `3f` 实际建立 `147ae`；忙槽不伪造成功。另有 3 条完整输入流 48 步、4 条完整人物流 96 步。每条流只初始化一次，依据实际 ROM 动画时长和终止标志自推进，不调度子对象。

六测试文件 SHA-256 始终为 `d7e2c3aab12de322f69f52d964d7b4374b44ee8d4e3ec610115b0b66a9a96098`。早期明确 private 流的 Action／Actor unsupported8 另存为 `fighter8-composition-private-red.jsonl`；真正菜单普通自然夹具到位后再次运行已有 OriginalActions／OriginalActors，两项实际报 unsupported8，保存于正式 `fighter8-composition-red.jsonl`。此时不声称已执行尚缺特殊自然 fixture 的全部六项；生产文件没有排除，其他独立来源 8 未完成测试的排除列表明确记录。

## 特殊自然覆盖与产物

Root 提供的合法 `fighter8-special-inputs.json` SHA-256 为 `b26515ad1e9f558fc54699fd0fe69c7c1cf7e3e081682f136bc8da1121121cce`，selector 生成唯一原 core 3,602 条基线。本采集器独立观察三层完整返回并逐视频比较全部 RAM／视频，全部一致。特殊 input／action／actor 各 2,067 次，包含 `3f` 73 次、`40` 完整 ac→ad→ae 239 次、`3e` 80 次及实际恢复 Air2。父体完整调用内建立 `147ae` 1 次；它与 child 任务的生命周期回调次数不能混用。特殊自然输入仍为真人／禁用，没有来源 8 自然 AI。

12 组规范 fixture／manifest 的动画／形状目录由原 CPU 实际选择记录独立采集，不依赖待扩展的 Go bundle。普通及特殊自然共 76 行动作、95 行人物有实际声音消费者清槽标记；只在 `37c56/37cac` 且地址属于 `F80A..F80D` 时排除该行四槽队列相等，其他规则与未受影响队列继续比较，不向 Go 喂期望队列。私有连续流没有此排除。

本次只清理本子任务 70 个重复／中间文件，共 719,273,491 bytes。早期 legacy private 原件按已保存历史 SHA 核对，最终规范 private 来自真正来源 8 checkpoint；完整私有采集附带的较大自然帧库，其自然调用区字节摘要与规范 natural-only 文件一致。规范夹具、两组基线／输入、明确 legacy 历史与红灯、ROM／工具、共享缓存及 8799 服务素材均保留。

原 CPU 复跑须区分三类产物：默认完整命令复现真实来源 8 checkpoint 的 private／sequences；natural-only 命令复现较小的规范普通自然目录；special 命令复现特殊自然目录。`--private-only` 仅重现早期明确 legacy 背景实验，不能作为自然或最终 private 的替代。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_composition.py \
  --out .local/md-yuyuhakusho/fighter8-composition-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_composition.py \
  --out .local/md-yuyuhakusho/fighter8-composition-natural-replay --natural-only
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter8_composition.py \
  --out .local/md-yuyuhakusho/fighter8-composition-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter8-special-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter8-attacks-special-reference-frames.json \
  --baseline-manifest games/yuyuhakusho/engine/testdata/fighter8-attacks-special-reference-manifest.json \
  --natural-label special --natural-only
```

## 最终规则与只读接线复核

Root 在真实自然 unsupported8 红灯之后完成 shared driver／world／session 接线。本子任务随后实际运行未修改的六项测试，首轮全部通过：native `-race` 45.989 秒、Node／Wasm 11.532 秒，包含早期／真正晚期错误回滚。规范日志为 `fighter8-composition-native.jsonl` 与 `fighter8-composition-wasm.jsonl`；本子任务未写生产实现、未改 bundle 或冻结断言。

最终只读复核未发现接线问题：source8 的 `3e/3f/40` 及 `0f/11` 受来源条件限定，旧 `0/2/3/9/10` 分支保留；tail 的 `Fighter8AttackState` 完整传回唯一 Aux48 long，motion 经 `acceptSpecialMotion`、actor 和 world 完整回写 Body／Controls／三子、能量分数与 AlternateVelocity，不另存计数。子初始化视图外的标志／缓存保持，只读的 ChildAttackFlags／可见 `+65` 不被回写。

`147ae` 使用独立窄 state，从 canonical Animation／Header／图形缓存／生命／抖动／屏幕坐标和共享形状／声音建立 view，不构造 `ChuProjectileState`，也不读取 parent、速度、Power 或 FX 池。返回后先按实际前缀清 canonical 对象，再合并已经按同一前缀清理的窄 view 和自有字段；保留逻辑 ObjectID，没有复活已清字段。错误由外层世界事务处理，未承诺 helper 是另一个独立事务。

最终来源、12 组夹具、历史私有上下文、真实红绿日志、采集器／适配器及生产源码 SHA 绑定见 `fighter8-composition-validation.json`。本范围仍止于人物规则精灵提交前，不以此替代完整世界、子调度、fresh 对局或平台浏览器验收。

独立规则复跑不执行模拟器：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestFighter8Composition' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -run '^TestFighter8Composition' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```
