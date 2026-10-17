# 物理入口地形显示事件

## 实现前失败场景

- SurfaceCodes 由真实非依附物理入口的旧 X／Depth 更新；用最终位置或下一帧位置会在跨平台边缘时提前／滞后更新阴影。
- 依附人物跟随父对象、持球跟随和篮筐静止帧不能假写；跳跃重入调度如果尚未执行物理也不能写。
- 同对象同帧多次真实物理更新必须按发生顺序保留，不能按对象去重。
- 动作初始化、持球定位等可能在物理前修改位置，应从真正积分入口读取，而非调度入口猜测。
- 错误必须原子回滚：失败步骤不返回前半段的显示事件；下一次调用不能残留前次收集器数据。
- 录制不修改 RAM，完整原运行 RAM／PNG 必须一致；对照完整动作阶段与全部物理入口，而不只选有变化的阴影片段。
- 物理事件仅提供对象及 origin；显示端按原地形表计算缓存，不把绘制代码或回调放入球命令。

## 原版证据与事件边界

复用 REA 证据 `ev_b618a1d77454e328308befc87f536eeead9325fd771af4f40edcab7c95578f5e`（`.local/basketball/rea/auxiliary-rest/terrain/batch.json`）：bank4 `$8249` 调用 `$89FB`，后者用旧 `$75/$82/$9C` 查地形并写 `$05EA`。未提交的下一位置位于 `$058E/$059B/$05A8/$05B5/$05C2`，不是这次缓存的来源。

`capture_motion_surface.py` 同时监听真实 `$EF5F` 入口、`$05EA..$05F6` 写入和完整动作阶段边界。6000 帧参考产生 1998 个完整阶段、14456 次写；主动骑乘参考产生 57 个完整阶段、389 次写。长参考中另有 150 次物理入口没有地形写，均保留于 `extra-surface-writes.csv`；首例 phase49/frame3655 actor2 位于纽约穿道状态。此分支执行积分但绕过地形查表，事件也必须省略。捕获尾部事件保留在来源文件，未伪装成完成阶段。两组全部 RAM 与参考逐字节相同，截图亦完全一致。

`MotionSurfaceWrite` 只含 Object 和 Origin。有序事件位于 `WorldActionStep`／`WorldLoopStep`，错误时原子丢弃。`FlatCourt` 未导出的事务收集器仅由本次动作调用建立，未写入世界状态或全局，也不改变构造器的零／nil 行为。事件在物理函数实际入口采集；穿道、依附和未执行物理的重入不靠动作结果猜测。持球定位前的真实物理和同对象重复写保留顺序。

## 验收

先编写 `motion_surface_test.go` 与完整原样本，再添加实现。14456＋389 条有序事件逐项匹配原执行的对象、旧位置和次数，同时比较完整动作／连续世界状态；另验证错误时无事件泄漏、后续调用无残留、返回 slice 不跨调用共享。原样本中还保留每次 `$05EA` 的实际值供显示端独立核对地形表。

复跑采样须使用已有 Pillow 的工作区 Python：

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_motion_surface.py
source scripts/env.sh
go test -race ./games/basketball/engine -run TestMotionSurface -count=1 -v
GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run TestMotionSurface -count=1 -v
```
