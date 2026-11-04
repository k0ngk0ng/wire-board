# 来源 10 的持续动作：实现前验证范围

恢复动作槽 `11/2d/47/48/49`，对应原入口 `1cb08/1b67a/1c85e/1c8e0/1c9aa`，终点为共享尾段前的 `15afc`。原版指令证据复用 `fighter-action-slots-focused.txt`；REA 不支持 raw 68000，本次不声称由 REA 反编译或执行该 ROM。先取得原 CPU 全人物、三个子槽、FX 池及游标的输出，再写缺实现测试，最后建立独立规则。

可能失败的场景：将动作槽十六进制与角色来源 ID 混淆；后退八项结构表简化成每帧移动，或把来源 10 的七帧姿态错误扩为八帧自然可达；`2d` 将完成 bit0 当作整 byte 等于 1，漏掉 ticks／帧 2／帧 14 与 A／B 新按下门槛；`47` 错用酎 `41` 的接触 bit2／bit3，漏掉第 0 帧残影／位移，或在非零帧也进行接触跳步；`47` 完成时忽略地面高 word 的空中恢复；`48` 错加重力或接触反跳，比较小数而非 signed 高 word，遗漏先站立构造、钳 Y、进入 `14/ae`、横速算术减半及完整重置；`49` 错改 `+48` 低 24 bits，未按 byte 回绕后每四次建立残影，或帧 28／29／30／31 速度及任意非零朝向判断错误；FX 满池改变申请游标或清旧槽未写字段；坐标／速度／帧／byte 回绕丢失；晚期申请失败泄露计数或前置写入；逐步回灌期望状态；把明确私有时钟的单初态轨迹描述成完整自然招式、尾段或对局。

规则接口拟为 `StepFighter10Motion(Fighter10MotionState, JumpParameters)`。状态仅持有 `ObjectID`、`Body`、`Controls`、`Aux48`、`ContactFlags` 与 `Pool`；`Aux48` 是 `+48` long 的唯一存储，最高 byte 供槽 `49` 使用。调用已恢复的尘土／残影／重置小函数时仅构造临时视图，不持久化另一份同址字段。五槽没有新的角色子对象建立；其完整未写字节仍纳入原 CPU 验证。

自然输入必须保留真实冷启动和菜单选择，不改人物 RAM。尚未选择来源 10 的已有输入可作为观察内核基线，但目标槽自然计数必须按实际记录报告，不能用私有调用冒充自然覆盖。连续验证只初始化一次，后续按预先声明的外部按键与私有动画时钟推进；不回灌原 CPU 中间态。

## 已验证结果

| 范围 | 调用／步骤 | 结果与边界 |
| --- | ---: | --- |
| 原 CPU 私有矩阵 | 8,586 | `11` 1,152 次，`2d` 630 次，`47` 3,856 次，`48` 1,636 次，`49` 1,312 次；全部来源 ID 为 10 |
| 单初态私有轨迹 | 27 条、285 步 | 独立时钟与动作组合至预先声明的动作转换或帧里程碑，不回灌中间态 |
| 来源 10 自然输入 | 35 次 | 全部为槽 `11`，视频标签 2,052..2,086，覆盖实际第 0..6 帧和完成恢复；其余四槽没有自然记录 |
| 独立规则验证 | 4 项顶层测试 | native `-race` 与 Node/Wasm 全部通过，准确耗时与当前来源绑定见验证报告 |

每次比较父人物 256 bytes、三个原保留子槽 768 bytes、完整 FX 池 2,048 bytes 和游标，同时比较全部已建模语义字段；没有屏蔽任何人物、子槽或池字节。私有调用还核对全部 64 KiB RAM 的改写范围，只有这些对象、池／游标与明确私有栈允许变化。观察入口 SP 与 `15afc` 的 SP 必须相同。原 CPU 私有矩阵含 1,920 次申请尝试，连续流另有 36 次；满池时仍按原规则推进申请游标，旧槽未写字段保持原值。

测试先于实现，真实缺少 `Fighter10MotionState`／`StepFighter10Motion` 的失败日志保存在 `testdata/fighter10-motion-red.jsonl`。8,586 次私有矩阵和 27 条轨迹均在写测试与实现之前取得；35 次自然调用是获得真实菜单输入后的追加交叉验证，没有新增生产规则。实现、测试、静态指令、原始表字、采集器、夹具和最终日志的 SHA-256 绑定见[验证报告](testdata/fighter10-motion-validation.json)。新增代码只提供这五个持续阶段；尚未把来源 10 接到完整人物、世界或平台运行时。

## 保留的原语义

`11` 的 `1cb20` 表有八项：第 0／6／7 帧不移动，第 1..5 帧按朝向 bit0 后退。来源 10 的实际后退姿态 `0c` 只有 7 条已验证动画记录，第 6 条带结束标志；第 7 项只作为结构范围的原 CPU 隔离覆盖，不称自然可达。需要查表且帧大于 7 时，独立 API 明确拒绝；完成值为 1 时先走恢复，不访问帧表。

`2d` 检查完成 byte 的 bit0；其他四槽的完成分支是整 byte 等于 1。`2d` 的新按键必须命中 A／B，且 ticks 非零、帧不小于 2、帧不等于 14；递增保留 byte 回绕。`47` 从帧 0 就允许残影和移动，接触 bit0 仅在帧 0 使动画跳步；非零帧且 ticks 为零时才算术减半横速。`11/47` 完成时按 ground／Y 的高 word 是否相等，选择站立或原 `1860c` 空中恢复。

`48` 没有重力积分和接触反跳。它先按帧和离地高度决定残影，再移动 X、加原纵速、以 signed 高 word 判断落地。落地钳完整 Y，先调用站立构造，然后改为 `14/ae`、横速算术减半、阶段掩码清零并再次执行原重置。`49` 在帧 4..27 增加 `Aux48` 的最高 byte，低 24 bits 不变；增加后低两位为零才申请残影。帧 28／29／30／31 速度依次为 `0x30000/0x20000/0x10000/0`，不覆盖人物储存的横速。此槽按朝向 byte 是否为零取向，其他横移按 bit0；残影属性仍沿用原任意非零朝向判断。

残影、横移、站立和动作重置复用已恢复的小函数；没有复制完整动作前置或尾段，也没有增加运行时 ROM／CPU 解释器。三个保留子槽在本五槽中完全未写，原 CPU 全字节对照确认这一点。

## 连续与自然证据的限制

27 条流各只初始化一次。原 `19c96` 时钟与独立 `StepAnimationClock` 逐步推进同一声明的私有 duration／flags；只有 `2d` 的一次预定新按键输入允许外部变化。轨迹覆盖 `11` 的 0..6 帧窗口、`2d` 到帧 14、`49` 计数与帧 28..31 衰减，以及独立完成恢复和 `48→14` 转换。轨迹中的时长是明确私有时钟上下文，不刷新真实动画记录；“七帧窗口”不等于按原动画时长完整执行一次自然后退。

自然流通过真实菜单把人物选择配置置为 `[11,0,0,10]`，实际来源 tag 为 `[10,0,0,9]`；相对原输入，只把第 1,350 帧左方向的结束从 1,380 改为 1,359，不修改人物 RAM。`fighter10-motion-natural-inputs.json` 保留这个独立输入，SHA-256 为 `d7859f794df2a2a9cf356eae01d239bf658632f58e69cee0cc5abb979ad3776e`。下载的未观察原内核与观察内核均冷启动执行全部 3,600 视频步，RAM／画面摘要完全相同。原私有矩阵仍保留其自身来源 9 基线与 `fighter10-motion-inputs.json`；在新来源 10 基线上重跑同一私有矩阵与连续流，两个压缩夹具逐字节完全相同，未替换旧私有 manifest。

本模块不运行共享尾段、不更新残影后续生命周期，也不证明完整选择器到动作结束或对局可玩。槽 `2d/47/48/49` 的自然动作、完整人物与世界组合仍由后续集成继续验证。非法帧／游标的 Go 错误是独立实现边界，不声称原 CPU 对这些坏输入返回同样错误。

## 复跑

从 worktree 根目录运行；采集需要已有固定研究 ROM 和两内核，Go 验证只用规范夹具及已验证语义库：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter10_motion.py \
  --inputs games/yuyuhakusho/engine/testdata/fighter10-motion-inputs.json \
  --out .local/md-yuyuhakusho/fighter10-motion-replay
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter10_motion.py \
  --inputs games/yuyuhakusho/engine/testdata/fighter10-motion-natural-inputs.json \
  --out .local/md-yuyuhakusho/fighter10-motion-natural10-replay
mkdir -p .local/md-yuyuhakusho/tmp
GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -race -count=1 -run '^TestFighter10Motion' ./games/yuyuhakusho/engine
GOOS=js GOARCH=wasm GOCACHE="$PWD/.local/go-cache" GOMODCACHE="$PWD/.local/go-mod" \
GOTMPDIR="$PWD/.local/md-yuyuhakusho/tmp" TMPDIR="$PWD/.local/md-yuyuhakusho/tmp" \
go test -count=1 -run '^TestFighter10Motion' \
  -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/yuyuhakusho/engine
```

第一条采集命令复现私有矩阵及连续夹具，第二条复现来源 10 的自然夹具；比较时按各自 manifest 的输入和摘要绑定。采集器可显式传入 `--inputs` 和 `--end`，分别执行两内核基线并保留实际输入与自然计数。

来源 10 新基线下的完整私有 manifest 与两份夹具逐字节一致的记录保存在 `testdata/fighter10-motion-natural-crosscheck.json`。最终只清理本任务 16 个重复采集／stdout 文件，共 1,810,859 bytes；保留全部规范夹具、真实缺实现红灯、最终日志、来源和交叉验证说明。共享 Go 缓存、固定 ROM／工具和其他任务文件均保留，清单见 `testdata/fighter10-motion-cleanup.json`。
