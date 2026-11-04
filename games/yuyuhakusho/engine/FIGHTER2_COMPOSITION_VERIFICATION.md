# 来源 2「桑原」完整动作、输入与人物组合：实现前边界

本子任务只新增原 CPU 取证、规范夹具与先行六测试，不修改生产规则。范围为完整 `15a18→15b82` 动作、完整 `15540` 输入返回和 `12682→126ba` 人物规则阶段。原 HUD 已确认来源 2 为「桑原」；普通自然输入为 `source2-next-inputs.json`，特殊输入及基线由并行取证共享。自然 AI 未观察时明确留空，私有 AI 不能替代自然证据。

实现前失败场景：只改 tag 而沿用其他人物参数／动画／形状；`1e/1f/20` 与共用 `0f/11` 分派错误；request1 正常低 nibble 0～10 和 11～15 相邻指令诊断混在同一正常矩阵，或用 Go fallback 冒充原 no-op；空中请求1增加原版不存在的方向／Changed／子槽门槛；Gravity 高 word 游标破坏低 word，busy 前的纵速和游标正常副作用被回滚；`1e` 错用完整横速作偏移、覆盖子速度／Power；`1f` 用父 VX 代替常量位移或改写父 VX、接触后 ticks／FrameIndex 顺序错误；`20` 发射帧／ticks、三槽顺序、属性 `2000` 被幻海 `4000` 代替；新 `12dd4` 的 HitStopTicks 视图错作为新 canonical 字段或创建阶段擅自清除旧 `+69`；父三子、FX Phase／Duration、其他人物资源、能量小数、投技／形状、HUD、声音与共享 RNG 改写遗漏；canonical A5 与正 24-bit 别名混淆；栈覆盖统计；没有实际 PC／地址证据便排除音频队列；晚期库／缓冲／HUD 错误泄露前置修改；连续流回灌期望中间态；把父体建立或人物规则返回说成完整子调度／世界／平台验收。

复用旧 Chu 完整原字节 snapshot 与 Go 适配器，在本模块独立视图 wrapper 中补齐可见 child+65。四个私有源使用 `FFFF8400/FFFF8800/FFFF8C00/FFFF9000`，栈在 `FFFF00` 下，完整调用另审计全部 64 KiB RAM 实际改写。正常三层组合不引入尚未验证的 request1 方向诊断；独立诊断属于 selector 取证。原动画目录由原 CPU 选择记录独立建立，不依赖待扩展的 Go bundle。

六测试代码必须在生产支持来源 2 前冻结，再取得 Action／Actor 实际 unsupported 运行红灯。完整 Input 已共用真实全角色 AI，按实际结果判断。连续流只初始化一次，后续仅交付声明的外部按键／控制器和暂停／慢动作，不喂期望动画、人物、子槽、FX、能量或 RNG，不运行子对象生命周期。静态入口来自已有直接 Ghidra 68000 证据，不称作 REA 反编译。

## 已取得的完整原 CPU 输出

普通流复用 3,600 条未观察原 core 基线；特殊流使用 `fighter2-projectile-inputs.json` 的统一 3,602 条基线。重新运行观察内核后，两组完整 RAM／视频逐视频一致。特殊流只接受不晚于 3,600 的入口，最后两步排空跨视频边界的完整返回；视频标签不是逻辑 tick。

| 范围 | 完整动作 | 完整输入 | 人物规则 |
| --- | ---: | ---: | ---: |
| 普通真实菜单自然流 | 2,067 | 2,067 | 2,067 |
| 特殊真实输入自然流 | 2,067 | 2,067 | 2,067 |
| 有界私有完整调用 | 163 | 31 | 43 |
| 单初态连续流 | 10 条、434 步 | 3 条、48 步 | 4 条、96 步 |

三层自然数量来自同一调用链的嵌套边界，不相加当作三倍玩法覆盖。特殊自然流包含 `1e` 108 次、`1f` 49 次和 `20` 53 次；在父动作完整调用内建立 `12dd4` 2 次、`12c96` 3 次。这是建立次数，不是子生命周期回调数。来源 2 自然输入均为真人／开局禁用，没有自然 AI。

私有动作覆盖 19 个相关槽及四个 canonical 身份。Ground request1 的 11 个正常方向都经过完整动作尾段，未引入 11～15 诊断；独立 Air request1 在有方向、无 Changed、主子槽忙时仍直接成功，未加原版没有的门槛。地面普通投技确实提交记录，普通 Air 没有投技前置；采集器最初从其他来源套用的 Air throw 断言已按原指令和原 CPU 校验纠正，没有为此修改冻结的 Go 测试。

`1e` 的完整建立调用保留旧子 VX／VY／Power 与 `+69`，实际旧停顿值 70／81／92 等非零 byte 保持。`1f` 的常量位移和帧 4 接触顺序在完整动作及完整人物调用比较；持续阶段不写父 VX，完成站立分支按原规则重置。`20` 三个发射时点分别写纵速 `0/−30000/+30000`，游标高 word `0→100→200→0`，低 word `A5C3` 始终保留；相应 busy 调用仍保留纵速与游标的先前副作用。子属性保留原 `OR 2000`，不使用幻海的 `OR 4000`。

每次私有完整调用审计全部 64 KiB RAM 改写，未知写入为零。父三子、其他人物资源、FX Phase／Duration、投技／形状、HUD 提交、声音、能量小数与共享 RNG 均比较实际字节。创建视图不负责子 `HitStopTicks` 生命周期；这里通过完整原子槽字节核对其保留，不把它复制为另一份 canonical 字段。

十条动作流均正常到站立 0：空闲 32 步，`0f/11` 为 24／32 步，`1e` 空槽／忙槽均 53 步，`1f` 的 a4／a5 为 32／52 步，另保留一次 a5 初始接触上下文 52 步，`20` 空槽／全忙均 52 步。空槽 `1e` 建立 `12dd4`，空槽 `20` 建立三槽散射弹；忙槽流不伪造创建成功。独立父流不模拟之后的碰撞，`1f` 帧 4 的实际接触处理另由对应完整单次调用覆盖。

## 先行门禁

六项 Go 测试完整文件在生产支持来源 2 之前冻结，SHA-256 为 `2e36a99be288fe5ab6333b1555a7c6a9a9e58658c8a33faf7b2d5c24208ec84b`。普通自然夹具到位后先运行已有完整 Action／Actor 两项，实际报 `unsupported complete fighter action/actor source`，保存在 `fighter2-composition-red.jsonl`。其余四项当时尚未执行，不把缺私有夹具说成自身红灯；显式文件列表只排除其他独立来源 2 任务的未完成测试，保留全部生产文件。初始生产源码、测试及红灯摘要分别保存。

最终普通自然流重采后，三组 CPU gzip 与红灯时逐字节相同，仅 manifest 的采集器 SHA 更新；见 `fighter2-composition-natural-reproducibility.json`。自然动作共 76 行、人物共 90 行有实际音频消费者清槽标记；只有 `37c56/37cac` 且地址属于 `F80A..F80D` 才排除该行四槽队列相等，其他规则及未受影响队列仍核对，Go 不接收清理后的期望队列。私有流无此排除或队列回灌。

本范围止于人物规则精灵提交之前，不覆盖子对象／FX 调度、碰撞管理器、完整世界或平台网络。原始范围、来源、日志、12 组规范 fixture／manifest 和源码绑定见 `fighter2-composition-validation.json`；完整世界和新建对局属于 Root 的外层门禁。

## 最终组合与只读接线复核

Root 在真实 unsupported 红灯之后完成生产接线。本子任务随后实际执行未修改的全部六项测试：native `-race` 46.906 秒、Node／Wasm 11.637 秒，首轮全部通过，包括真正晚期动画／录制／HUD 错误回滚。日志保存在规范目录的 `fighter2-composition-native.jsonl` 与 `fighter2-composition-wasm.jsonl`，未编写生产实现，也未修改冻结断言。

按 Root 要求只读复核最终接线，未发现这些位置的缺失或重复所有者：

- Action／Actor／World／session 显式允许来源 2，tail 显式调用 `SelectFighter2Attack`，不更改 Aux48；`1e/1f/20` 及来源 2 的 `0f/11` 只进入该来源分支，原 `0/3/9/10` 路径保留。
- `specialMotionView→acceptSpecialMotion→acceptActorAction→MergeChuFighter` 完整传递 Body、Controls、三子、能量分数和 AlternateVelocity。Gravity 高 word 游标与低 word 使用原 Body.Gravity 的同一 long；Aux48 与 AlternateVelocity 仍为同一 long。
- 三子 `MergeChuChild` 只合并父初始化视图字段，旧动画、图形缓存、攻击标志及停顿在视图之外保持。`1e` 本身不写的旧速度／Power 由 view 原值带回，`20` 明确拥有的速度仍正常合并。ChildAttackFlags 与可见 `+65` 是只读视图，不回写覆盖。
- `12dd4` 经共用 `stepWorldProjectile` 调用专用规则，临时 view 从 canonical `WorldObject.HitStopTicks` 读取并在成功后写回。部分清槽保留 `+69`；大前缀清除时 view 同步为零，没有第二份永久存储。错误由外层世界事务回滚，未把该 helper 当作独立事务承诺。

本次只清理本子任务 64 个重复／中间文件，共 946,071,382 bytes。完整私有采集附带的自然库比规范 natural-only 库更大，清理前核对自然调用区字节摘要相同；规范夹具、原基线／输入、ROM／工具、共享缓存和 8799 服务素材均保留。

## 复跑

从 worktree 根目录运行。首个命令复现私有／连续夹具，第二个命令复现规范普通自然夹具，第三个命令复现特殊自然夹具；完整采集附带自然库的 gzip 摘要不能与较小的 natural-only 库混用。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter2_composition.py \
  --out .local/md-yuyuhakusho/fighter2-composition-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter2_composition.py \
  --out .local/md-yuyuhakusho/fighter2-composition-natural-replay --natural-only
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter2_composition.py \
  --out .local/md-yuyuhakusho/fighter2-composition-special-replay \
  --inputs games/yuyuhakusho/engine/testdata/fighter2-projectile-inputs.json \
  --baseline games/yuyuhakusho/engine/testdata/fighter2-attacks-special-reference-frames.json \
  --baseline-manifest games/yuyuhakusho/engine/testdata/fighter2-attacks-special-reference-manifest.json \
  --natural-label special --natural-only
```

独立 Go／Wasm 验证不执行模拟器：

```sh
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -json -run '^TestFighter2Composition' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -json -run '^TestFighter2Composition' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```
