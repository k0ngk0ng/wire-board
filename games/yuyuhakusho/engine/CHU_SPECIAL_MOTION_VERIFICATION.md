# 酎的特殊动作持续阶段：实现前验证范围

目标为 `SelectChuAttack` 实际设置的动作 `0f/15/41/42/44/45/46`，从各分派入口执行到共享尾段 `0x15afc`。先保留静态分支及原版完整输入／输出，再写测试并观察缺实现失败，最后编写独立 Go 规则。子对象建立属于本阶段；后续子对象／弹体更新如未恢复，必须单独标记。

可能失败的场景：十六进制动作槽与十进制编号混淆；使用错误的分派地址；忽略动画帧／计数触发边界；漏掉速度、重力、朝向、地面或生命的位宽回绕；把每帧动作错误简化为通用空中积分；省略子对象存在检查、重用槽残留、行为 ID、资源指针或父子引用；误将静态资源地址当成执行地址；特效池满时改变原游标语义；用自创弹体规则代替原版未恢复分支；只比较父人物而丢失子对象和全局写入；原版采样改变自然基线；在连续轨迹中回灌期望中间状态；把 pre-tail 阶段或建立弹体当成完整招式／对局完成。

## 已验证结果

| 范围 | 完整调用／步骤 | 覆盖 |
| --- | ---: | --- |
| 自然输入 | 311 | 全部来源角色为 9；`0f` 77 次、`15` 33 次、`42` 164 次、`43` 37 次 |
| 原 CPU 隔离 | 13,104 | 九个目标动作、旧槽复用、池满、坐标／朝向／计时边界及全部 256 个合法蓄力值 |
| 单初态轨迹 | 22 条、202 步 | 动画时钟与阶段组合，直到动作转换或弹体建立 |

自然输入保留原启动过程，在第 1,800 帧以后加入明确的双击方向及按钮脉冲。[输入记录](testdata/chu-special-motion-inputs.json)随夹具保存。下载的未挂钩原内核和观察内核均从冷启动执行相同输入，全部 3,600 帧 RAM／画面一致。自然样本实际建立了 `+100` 弹体 1 次、`+200` 子对象 2 次、`+300` 子对象 1 次；自然分配的 9 个效果全部成功。其他五种动作只在原 CPU 隔离矩阵及轨迹中覆盖。

每次比较完整父人物 256 bytes、三个保留子槽共 768 bytes、特效池 2,048 bytes、池游标及全部语义字段，无人物／子槽／池字节屏蔽。13,104 个隔离调用包含 832 次分配尝试，其中 704 次成功。独立 Go 规则的原生竞态与 Node/Wasm 聚焦测试全部通过，保留实际缺少类型／函数的实现前失败。来源、夹具和日志摘要见[验证记录](testdata/chu-special-motion-validation.json)。

## 动作与子对象

| 动作 | 原入口 | 本阶段处理 |
| --- | --- | --- |
| `0f` | `1ca9c` | 地面／空中冲刺、尘土、速度减半和结束后的站立／空中恢复 |
| `14` | `1cc28` | 空中招式落地后的滑动与站立／蹲下恢复 |
| `15` | `1a2f2` | 固定地面 Y，按请求与能量检查继续或结束 |
| `41` | `1c33a` | 帧门槛、残影、位移、速度减半及接触后的动画跳步 |
| `42` | `1c3ba` | 建立 `+200` 子对象、蓄力增长／能量消耗、取消或转入 `43` |
| `43` | `1c50a` | 第 0 帧建立 `+300` 子对象，第 3 帧建立 `+100` 弹体，结束后站立 |
| `44` | `1c6a0` | 按朝向移动，第 4 帧独立回退层级，结束后站立 |
| `45` | `1c6e2` | 斜向空中特殊移动、残影、接触反跳或落地进入 `14` |
| `46` | `1c7bc` | 前 18 帧固定上升，之后重力积分，落地进入已有动作 `3` |

三种子对象分别使用行为标识 `1313e/132de/1361e`。初始化保留旧槽的所有未写字段，包括仅写高 word 的 ground／速度字段及坐标小数；不主动补齐看似缺少的 X／Y、朝向或掩码。`43` 的两个八项姿态表由源 `1c5f4/1c68c` 直接核对，并逐个合法蓄力值运行原 CPU。0～255 之外的蓄力不属于已解码表范围，正式 API 在需要查表时原子失败，不读取原版相邻代码字节。

`42` 的蓄力增量按 word 回绕后算术右移，能量只改高 word。`0f/41/44/45` 的横向位移按朝向 bit0；效果方向另按朝向 byte 是否为零。`45` 的接触反跳先于地面判断；`46` 后半段按已储存横速移动，不再次按朝向取反，落地也不额外钳 Y。这些差异均保留原语义。

静态来源为[主阶段](../../../docs/research/md-yuyuhakusho/static/chu-special-motion-core.txt)、[后续动作与初始化](../../../docs/research/md-yuyuhakusho/static/chu-special-motion-helpers.txt)和[能量／重入构造](../../../docs/research/md-yuyuhakusho/static/chu-special-motion-costs.txt)。本轮使用直接 Ghidra 有限范围分析；REA 当前不支持 raw 68000。

## 连续轨迹边界

轨迹只恢复一次初态，每步使用原 `19c96` 动画时钟，再执行本阶段。Go 侧由自身 `StepAnimationClock` 和 `StepChuSpecialMotion` 连续推进，不注入原版中间值。蓄力轨迹的第 7 步按预定输入松开动作键；没有从期望结果生成输入。

时钟使用明确声明的私有初始记录时长／终止标志，途中不提交动画记录、不运行共享尾段、不更新已建立子对象。这证明组合时钟与阶段状态的连续性，不证明原版全部动画的自然连续可达性。`43` 轨迹止于弹体建立，其后 `1313e/132de/1361e` 生命周期、弹体命中和完整招式／对局仍未完成。

## 复跑

从当前 worktree 根目录运行：

```sh
mkdir -p .local/md-yuyuhakusho/tmp
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_chu_special_motion.py \
  --out .local/md-yuyuhakusho/chu-special-motion-replay
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestChuSpecialMotion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestChuSpecialMotion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

原版输出的输入、夹具与 manifest 可和 `engine/testdata/` 同名文件核对。ROM 和模拟器只用于研究，独立游戏代码不执行原版程序。
