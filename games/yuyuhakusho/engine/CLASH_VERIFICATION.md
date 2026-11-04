# 攻击相消：实现前验证范围

本阶段恢复 `0x1f090..0x1f21c` 的完整形状相消循环与 `0x1f970..0x1f9c6` 的相消特效初始化。先保留原 CPU 输入／输出，再写测试并观察缺实现失败，最后编写独立 Go 规则。碰撞管理器、攻击受伤和特效生命周期另行恢复。

可能失败的场景：把有序双循环改成对称的一次处理；忽略同队、地面高 word 或双向目标掩码；将 M68K 动态位号直接当成数组下标；把 word 坐标差／取负、byte 宽高相加改为无回绕运算；把触边当成不重叠；错误比较攻击值的有符号性；漏掉双方写入或改变交换顺序；混淆地面高 word 与完整定点值的两种比较；错误重置非零动画计数；强度值 3 被自创为另一档；特效分配失败时丢失游标推进；用数学平均替代 word 相加后的算术右移；改写特效旧槽中原函数未写的坐标小数或其他字段；只比较选取字段而遗漏原人物、形状和特效池的写入；观察器改变自然帧；把组件调用数当成完整对局。

自然记录覆盖固定输入的 3,600 帧，与无观察器基线逐帧比较完整 RAM／画面。隔离记录明确声明替换的原版私有输入，覆盖双向掩码、同队／跨线、相等／强弱攻击、地面／空中、四人物顺序、多形状重复目标、池满、位号回绕及坐标／尺寸回绕。测试应比较完整参与人物、256 bytes 形状缓冲、2,048 bytes 特效池、池游标和待处理声音。

原版碰撞对象先于人物更新，消费上一轮已提交的形状。本组件不得把该时序改成同帧人物更新后碰撞；完整调度尚未在此阶段实现。

## 已验证结果

独立 Go 实现与 2,068 次完整自然调用、159 个原 CPU 隔离用例一致，原生竞态与 Node/Wasm 均通过。全部参与人物 256 bytes、形状缓冲 256 bytes、特效池 2,048 bytes、计数、游标和待处理声音完整比较；调用者的 owner map 保持不变，无效计数、缺失 owner 和分配器错误保持原子失败。

自然片段的 143 次调用只有一条相消记录，其余缓冲为空，没有实际相消。因此，相消双方改写和特效初始化由隔离用例中的 140 次接触验证，不能称为自然对战相消已验收。隔离矩阵覆盖四人物／16 条记录、双向目标位、强度 0～3、相等／不等攻击值、地面与空中两种比较、池满及旧槽残留、坐标／尺寸回绕。全部 3,600 帧自然 RAM／画面仍与原版基线一致。

保留的红阶段记录实际因缺少 `ClashState`、`ClashActor` 和 `ResolveClashes` 编译失败，先于实现。来源、夹具和测试日志摘要见[验证记录](testdata/clash-validation.json)。本阶段不覆盖原版特效后续动画、完整人物时序或完整对局。

## 复跑

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_clash.py \
  --out .local/md-yuyuhakusho/clash-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestClash' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestClash' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

原版复跑输出的同名夹具及 manifest 可与 `engine/testdata/` 中的记录比较。Go 测试直接使用已保留夹具；模拟器和 ROM 只用于开发对照。
