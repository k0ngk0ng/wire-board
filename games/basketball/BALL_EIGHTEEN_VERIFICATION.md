# 特殊球动作 18：消失与重现

实现前失败场景清单：

- 首入不沿用原投篮目标，球权释放或声 30 晚于场地物理。
- `$0571` 与 ContactLatch 被另存成第二份状态；场地／设备改变该字段后仍使用物理前的副本。
- 越线判断使用错误的篮筐侧、±16 偏移、整数 X 或速度高字节（0～255 的正速度不能触发右侧越线）。
- 物理修改动作后仍按旧动作读取转线参数；不得用动作 18 常量掩盖设备分支。
- 越线后的隐藏动画游标 7、计数 6→5、深度 45、保留小数坐标及随机高度 328／232／136／136 错误。
- 重新出现时应清空全部速度和重力，不能保留 `$BAAB` 临时设置的下落速度。
- 随机数只在越线瞬间消耗；倒计数使用有符号结果，负值时转 10，接地最终覆盖为 6。
- 动画终点／下一动作与 JSON 恢复错误；物理后重新提交准备命令覆盖设备状态。
- 只通过单步夹具，缺少原状态连续链及真实球队 7 首失败状态续跑。

下列验收执行完成后再记录结果；本文件不是通过声明。

## 原函数与实现

REA / Ghidra 对 bank 6 `$BCAA`、`$BAAB`、`$BDD2` 和 `$B998` 的分析保存在 `.local/basketball/rea/ball18/analysis/`；函数证据 ID 为 `ev_ab466be88002c3df1f3f7ee04a161d6315baf1c0f5e13b814ad7491f978f6f51`。原 ROM SHA-256 与夹具 SHA-256 见 `engine/testdata/ball18-source.json`。

`shot_ball18.go` 将动作分成准备和完成两段。首入用既有原投篮目标和发射函数，设置计数 6、共享 ContactLatch=128，并释放球权。准备命令及声 30 在场地物理之前提交。完成段在物理之后重新读取共享 latch、选中篮筐和随机数，而不是重新提交物理前的副本。

越线时，原 `$BAAB` 将球移至对应篮筐 X 与深度 45，随后 `$EC59` 立即清空竖直速度和重力。原随机数只在该次转折更新；四个分支高度为 328、232、136、136，保留球原有的小数坐标。转入游标 7 的重现动画并立即递减计数；计数出现负值时转动作 10，接地检查最后执行并可覆盖为动作 6。球碰撞和计分仍由原世界阶段处理。

物理将当前球动作替换后，`$BDD2` 会按新动作查表。动作 18 的适配器保留对此未恢复分支的明确错误；没有以旧动作常量掩盖设备变化。这与动作 14 的音频工作字节边界是同类尚需独立恢复的上下文问题。

## 已执行验证

- 436 次不修改原指令的受控调用，比较完整 World 准备边界与结果、动画、元数据、随机数及声音。覆盖首入／已进入、双朝向、latch 五种值、计数边界、两侧越线、亚像素速度、接地和动画末尾。
- 8 条只初始化一次的原函数连续链，共 496 步；左右方向及四个随机高度选择分别经过 18 → 10 → 6，不用后续原状态回填 Go。
- 真实球队 7 冷启动的首失败状态（video 9449）续跑 180 视频帧，逐帧 JSON 保存恢复与未恢复结果完全一致。
- 定向 native 通过；race 10.325 秒、Wasm 3.051 秒通过，包含旧动作 12／14／15 的回归。
- 同一固定 seed `0x12345678`、正常两节时钟、全原版电脑策略继续整局诊断，前进至 video 21080 / logic 7011；下一缺口是人物 3 的动作 `0x59`。Native 诊断源码摘要在运行前后相同；Wasm 诊断在同一帧遇到同一缺口，失败前状态 SHA-256 相同，但运行期间其他引擎文件被修改，故保留 `sourceStable:false`、不接纳为完整配置验收。未把此次运行标为完整比赛通过，也未开放球队选择。

保留红灯与修复记录：`.local/basketball/rea/ball18/before.log`、`after.log`、`after2.log`。首次实现遗漏了未越线但已经落地时的最终动作 6 写入，原函数夹具捕获后修正。完整诊断及下一首失败状态保存在 `.local/basketball/team7-after-ball18/`。

可重复执行：

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ball18.py
source scripts/env.sh
go test -race ./games/basketball/engine -run '^TestOriginal(Ball18|TeamSpecialBall)' -count=1
GOOS=js GOARCH=wasm go test ./games/basketball/engine -run '^TestOriginal(Ball18|TeamSpecialBall)' -count=1 -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec"
python3 scripts/basketball/diagnose_team_rosters.py --court 1 --teams 0 7 --characters 0 1 0 1 --output .local/basketball/team7-ball18-repeat
```

此处证明的是原规则函数、连续链与自然配置续跑；不是所有特殊球、完整球队矩阵、浏览器视觉或人工试听通过。没有增加生产注册、上传素材或发布。
