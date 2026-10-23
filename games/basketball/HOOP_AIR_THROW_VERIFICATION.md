# 持篮筐空中动作回归

实现前需覆盖的失败场景：线上机场球场第 8964 帧 action 0x3A 拒绝推进；首次进入未将正确篮筐切换为动作 5／同步朝向；已经进入后重复发出篮筐命令；第六个篮筐或所有权查找边界错误；地面转换到 0x36 的时机错误；被其他角色携带时遗漏原版 8C33 分支；动作、动画与球场物理的写入顺序不一致；恢复存档后同一错误复发。

线上存档只包含引擎状态，不含账号或认证信息。原版 bank6:9949 的指令与隔离执行对照作为动作依据；不通过跳过动作或直接修改得分来继续比赛。异常恢复请求与音频重置另由浏览器回归验证。

## 原版语义与覆盖

`0x3A` 是空中持篮筐瞄准：bank6 `9949` 将篮筐设为持有动作 `5`，不是投出。随后的 `0x3C` 才是空中投掷：`9417` 将篮筐设为动作 `6`。二者保留原版的第六项所有权查找回退、初始化与动画顺序。`0x3A` 在普通落地前选择 `0x36`；`0x3C` 在物理落地后选择站立动作。被携带时都执行原版 `8C33` 配对运动，但只有 `0x3A` 随后额外调用一次角色物理。

原版 `0x3C` 加上键会写入 `056D`。当前引擎中消费该字节高两位的篮板回弹配置，通过动作阶段的提交命令更新既有 `RuntimePlayingState.Context.BackboardStrength`，不建立另一份持久状态。普通投掷不触发该写入；JSON 存档恢复保留同一配置。

`verify_hoop_air_throw.py` 执行未修改的原版指令，产出 2,048 个受控单角色调用和 3 个完整 13 对象动作阶段。测试逐字段比较角色、球、篮筐、辅助物、声音和调度顺序，包含首次／已进入、左右朝向、所有篮筐与无匹配回退、空中／地面、被携带和上键分支，并按运行时实际路径分别检查纽约快路径和机场原始物理绑定。受控隔离输入不等于这些组合都曾在自然原版比赛中出现；原版 ROM、基础 RAM、线上引擎存档及夹具摘要记录在 `engine/testdata/hoop-air-throw-source.json`。

现有夹具文件名保留 `hoop-air-throw`，同时覆盖瞄准与投掷。自动测试只读取提交的语义状态夹具，不加载原版 ROM。

## 完整继续与存档恢复

从真实故障存档开始，`verify_hoop_incident_continuation.py` 通过正式原生 runtime-stdio 接口继续比赛；只提供物理按键，不修改物理状态、得分或 AI。默认路径仅在中场按 START；`--controls` 路径持续输入方向、A、B 与组合键。每批推进均恢复前一份存档，在独立会话重演同一输入，并比较完整引擎状态。

两条路径都须经过中场、下半场并到达原版确认结算。`.local/basketball-incident-20261023/continuation-idle.json` 与 `continuation-controls.json` 保存完整状态摘要、恢复次数和结果。此验证证明本次故障状态可继续并可重复恢复，不代替对所有设备或所有可能玩法的穷尽验证。

```sh
source scripts/env.sh
go test ./games/basketball/engine -run '^TestHoopAirThrow' -count=1
python3 -B scripts/basketball/verify_hoop_incident_continuation.py
python3 -B scripts/basketball/verify_hoop_incident_continuation.py --controls
```

重新生成原版夹具需要本地保留的已校验 ROM、Court4 参考 RAM、py65 和 Pillow；执行 `scripts/basketball/verify_hoop_air_throw.py`。发布验证使用已经提交并有摘要的夹具。

## 同类地面动作边界

运行时的 `0x30`／`0x32`／`0x39`／`0x3B` 原先复用仅限平地的组件，会在合法空中状态直接拒绝推进，纽约通道运动也遗漏了原版 EEFF 的入口。`verify_hoop_ground_boundaries.py` 先捕获 128 个原版单角色调用和 8 个完整动作阶段，再由 `TestHoopGroundOriginalBoundaries` 对比。恢复后的世界处理器保留地面／空中转换、通道行为、篮筐命令写入时机与动画顺序。

这次取证还发现纽约附着角色若误走平地快路径会产生 10 像素位置偏差。测试保留该回归；附着动作使用原版支持物物理绑定。篮筐跟随站立、跌落及骑乘角色的偏移由既有原版 ECF5/ED65 表提供，避免角色合法改变动作后又被旧的狭窄偏移 switch 拒绝。
