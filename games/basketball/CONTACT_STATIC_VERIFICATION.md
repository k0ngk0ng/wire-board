# 原版静态碰撞资料

## 实现前失败场景

- 冷配置第 322 次更新进入 BallRolling，Kind 8／Action 5／Frame 0 的原碰撞资料存在于 ROM，但旧表只收集观察过的键，错误拒绝合法状态。
- 非攻击动作的查表链由 kind→action map→frame pointer array→8 字节 shape 决定，不应受是否在某条参考录像出现限制。玩家 kind 0..7 的源表相同；球表独立。
- 还原源数组范围，不把相邻数组或错误 cursor 当成当前动作的任意帧；保留未恢复／越界键明确报错。
- 不为未 Entered 的人物残留旧 cursor 增加映射；WorldPreparation 在原分支提前跳过。攻击 variant 的含义独立，本修改不扩张 variant 表来掩盖输入路由错误。
- 非攻击动作忽略 variant，高位进入标志不改变 geometry；攻击／受击 flags 必须从相应源表读取，不能根据形状猜。
- 静态表扩大仅说明几何资料存在，不代表相关动作或电脑策略都已恢复。长运行仍在下一个未知分支停止，不降低 6000 目标。

## 原版证据和范围

新 REA 完整函数证据 `ev_e5ee9618afe7af1df31ab2462a11aa2c6af8d954fd246d832b5a312ab587c8c0` 位于 `.local/basketball/rea/contact-static/analysis/function-0x8318.json`：`$8318` 屏蔽动作高位，`$60` 以上跳往独立攻击 variant 入口；正常动作依次读取 `$8CF2` kind 表、动作 group、`$8FA5` frame pointer 表及 cursor 对应 shape pointer。源码未增加 ROM 解释器；ROM 仅供离线参数导出。

正常 group 表范围 `$8FA5..$8FE0` 共 30 项，指向 `$8FE1..$912E` 的帧数组，按相邻唯一数组起点界定范围；`$912F` 是独立攻击 shape 基址指针，不并入正常帧。玩家 action map 为 `$8ED8..$8F37`，球 action map 为 `$8F38..$8F50`。这 30 组含 175 个源帧条目，覆盖 1077 个正常动作／帧键。其存在不代表未实现动作已经可玩。

BallRolling 路径为 kind8→`$8F38[5]=4`→`$8FA5[4]=$900F`，源数组 15 帧均指向 `$9151`，八字节为 `00 10 08 F8 00 10 08 F8`；攻击／接收 flags 同为 `$938B[5]=0x20`。没有根据冷轨迹拟造几何。测试先验证完整 15 帧、进入高位、忽略正常 variant 和边界，再修改实现。

生成器检查固定 ROM SHA-256，并将全部 93 个已观察正常 profile 与静态表逐字段比较；原 54 个攻击／辅助对象 profile 保留既有白名单。不会用正常表扩大攻击 variant 的可接受范围。参数来源、指针地址、组表、flags 与独立帧范围保存在 `testdata/contact-normal-source.json`。

## 验收与下一边界

测试通过 2392 个原接触搜索阶段、1197 个 contact reset 阶段、1997 个完整 preparation 阶段，以及新增源表边界检查。原 cold 配置第 322 次更新已经通过 preparation，随后在 action 阶段遇到 `object 6: ball action 0 not reconstructed`；完整前态保存在 `.local/basketball/rea/contact-static/cold-next.log`，交球动作恢复工作处理。6000 目标未改变、尚未通过。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/export_contact_profiles.py
source scripts/env.sh
gofmt -w games/basketball/engine/contact_profiles.go
go test -race ./games/basketball/engine -run 'TestBallRollingContact|TestNormalContactSource|TestPlayerKinds|TestOriginalContact|TestWorldPreparation' -count=1
```

完整包编译下的原生竞态和 Wasm 定向测试均通过，未排除任何并行测试文件；结果位于 `.local/basketball/rea/contact-static/{native-tests.json,wasm-tests.log}`。

## 攻击 variant 源表：实现前失败场景

- cold1598 的动作103／variant93首先核对选型与请求：原 bank6 A022[7]→A1F4 的 team0／character1 pair 为 [93,0]；原 E18B 对当前103拒绝 pending29，保留请求。不得以补形状掩盖不正确的请求转换。
- 原 bank5 82E4 按 variant 和动画 cursor 查形状索引，action 只选 flags；不能把两个源维度混成观察过的 (action,variant,frame) 白名单。
- 导出范围来自已经导出的原攻击动画及各自合法帧数，不扩展未知 variant／越界 cursor。未进入的旧 cursor 仍由 preparation 原分支跳过。
- 对所有已恢复动画帧执行原几何读取函数的隔离对照，独立保留原 pointer／八字节形状；不把函数测试称为自然整局。
- 共享玩家 kind 不改变 variant 表，攻击与受击 flags 来自原 action 表。新 source geometry 不代表所有攻击物理 handler 都已恢复。

## 攻击源表恢复结果

REA `ev_8f6ec94a9284a3c93bcce71dc0be376ee57c73f229f13970beaee02150e5a6c7` 的 bank5 `$82E4` 明确按 Variant → `$8D10` 帧索引表 → `$912F` 形状基址读取；动作编号只控制 `$9321` flags。导出器现从 bank6 `$A022` 的原角色选型及 `$AC8F` 的已导出动画获得 97 个 variant 和 226 个合法帧，不以某个样本 action／variant 组合为门槛。旧 52 个攻击观测 profile 逐字节与此源表相同；只有两个辅助对象种类仍保留原观察域。

`verify_attack_contact_function.py` 先隔离执行原 E18B 和 9FA7，确认 cold1598 请求29在动作103下被拒绝，角色0/1真正选 variant93、handler A40A；随后执行原 8318→82E4 的全部226次合法形状读取，保存指针和8字节结果。Go测试将其在8个玩家kind和10个攻击action flags下逐项比较，并检查动画帧覆盖、未知variant和帧末边界。它是隔离原函数证据，不冒充自然整局。运行时只使用导出的Go参数表。

TDD初次失败、导出摘要、原生竞态和Wasm结果位于 `.local/basketball/rea/contact-attack/`；2392接触阶段、1197 reset阶段、1997 preparation阶段、1998单初态连续更新及57主动更新仍通过。源表记录为 `testdata/contact-attack-source.json`，独立原函数执行记录为 `contact-attack-function.json.gz` 及来源JSON。

固定cold轨迹推进到1909／6000，下一首差是人物2 mode26 dribble-evasion分支；完整前态为 `contact-attack/cold-next.json`。攻击选型、请求路由及物理没有被替换，下一AI分支另按原证据恢复。

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_attack_contact_function.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/export_contact_profiles.py
source scripts/env.sh
gofmt -w games/basketball/engine/contact_profiles.go
go test -race ./games/basketball/engine -run 'TestColdAttack93|TestAttackContactSource|TestOriginalContact|TestWorldPreparation|TestWorldLoop' -count=1
```
