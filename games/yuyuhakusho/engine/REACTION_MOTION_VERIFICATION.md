# 受击与攻击完成动作：实现前验证范围

本阶段恢复动作槽 3～9、12／13 的完整共享尾段前阶段，入口位于 `0x19f90..0x1a2f1`，退出到 `0x15afc`。先列失败场景并保存原版完整输入／输出，再写测试并保留缺少实现的实际失败，最后加入独立 Go 规则。共享尾段中的输入分派、动画提交和随机更新另行恢复。

可能失败的场景：把动作槽与姿态编号混用；省略攻击完成后的站立／蹲下构造；错误处理地面高 word、完整定点坐标与符号比较；将 byte／word／long 回绕改为数学整数；错误应用朝向、横速衰减、重力或落地顺序；提前清除旧动画完成标志；漏掉硬直／KO 的后续构造或对象池副作用；把没有实际执行的静态分支当成自然覆盖；单次隔离调用用期望输出回灌，伪装成连续轨迹；只比较已知字段而遗漏人物或池中的未知写入；缺少数据时部分改写输入；观察器改变自然 RAM／画面基线；把本阶段退出当成完整人物／世界步或完整对局。

取证应保留完整人物 256 bytes、被触及对象池与全局字段。自然输入使用固定 3,600 帧基线；未自然出现的状态由明确声明私有输入的原 CPU 隔离调用验证。连续轨迹仅从一个初态推进，期间只改变声明的外部输入，不恢复或注入期望中间态。

## 已验证结果

- 原版自然完整阶段 1,486 次：动作 3 为 840 次、4 为 31 次、5 为 432 次、6 为 144 次、12 为 39 次。观察器未改变固定输入的全部 3,600 帧 RAM／画面基线。自然阶段共有 25 次特效分配，全部成功。
- 原 CPU 隔离完整阶段 1,229 次，覆盖九个目标动作；包含 1,257 次分配尝试，其中 630 次成功。池满、单槽可用、附着标记／三连状态效果、落地尘土、姿态 24 的地面控制与切线构造均保留副作用。
- 20 条单初态轨迹连续推进 368 步，直到动作转换。覆盖地面滑动、空中受击与弹起、撞墙、硬直恢复、蓄力结束、恢复跳跃和攻击完成；空池／满池分别执行。期间没有恢复存档、回写原版中间态或推进共享尾段／动画时钟。这些是明确隔离的阶段轨迹，不能称为完整人物或完整对局。
- 每次比较完整人物 256 bytes、特效池 2,048 bytes、池游标、轨道占用、待处理声音、`FFF568/FFF56A` 两个全局字段和场地边界，不屏蔽未知人物／池字节。完整状态一致；错误动作和分配器错误保持原子失败。
- 先保留真实缺类型／函数的编译失败，再加入 Go 实现。聚焦原生竞态与 Node/Wasm 测试通过；测试输出、夹具与来源摘要见[验证记录](testdata/reaction-motion-validation.json)。本轮聚焦执行使用显式源文件范围，避免其他并行动作的缺实现测试干扰；父级仍需运行整个游戏包的集成回归。

## 保留的原版语义

动作 4 先减横速再应用位移，最后一步允许横速过零；朝向恰好为 1 时才向另一侧应用。动作 5 落地把 Y 钳到地面，动作 6 的对应落地路径保留越过地面的 Y，并在构造后把负生命改为 `-1024`。动作 7 按更新后 X 的高 word 触发场地边界反弹；动作 9 的纵速由参数分别算术右移后相加。动作 12 的硬直倒计时按有符号 word 判断、只读基础按下 byte，并保留动画计数的无符号 200 边界。动作 13 对蓄力 long 加速后，按其有符号高 word 与 223 比较。

`0x203f8` 附着效果使用普通分配器，写入 owner、纵向偏置和朝向相关属性；三种硬直效果使用 `0x1faa6` 分配器（行为 `0x14ec6`），连续尝试三次并分别写入相位 0／21／42。未写入的复用槽字段保留。`EffectExtras` 只承载既有 `EffectSlot` 尚未涵盖的 `+48/+56/+5a` 字段，不改公共效果类型。

静态证据为 [主阶段](../../../docs/research/md-yuyuhakusho/static/reaction-motion-core.txt)、[构造与效果](../../../docs/research/md-yuyuhakusho/static/reaction-motion-helpers.txt)和[分配器](../../../docs/research/md-yuyuhakusho/static/reaction-motion-allocator.txt)。使用直接 Ghidra 有限范围反汇编；当前 REA 不支持 raw 68000，未将结果写成 REA 反编译。

## 复跑

从本 worktree 根目录运行：

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_reaction_motion.py \
  --out .local/md-yuyuhakusho/reaction-motion-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestReactionMotion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestReactionMotion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

原版输出的同名夹具及 manifest 可与 `engine/testdata/` 记录比较。ROM 和模拟器仅供研究，不进入正式运行时。动作 7／8／9／13 没有自然样本，只在隔离矩阵验证；后续特效寿命、共享尾段、动画／命令分派、完整 KO 流程和世界调度尚未由本阶段证明。
