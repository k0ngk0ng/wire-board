# 客户端 fork 与权威追赶性能 revision v1

实际平台 profile 确认同步反复进行 JSON fork，且无像素目标的权威追赶仍逐帧 raster。客户端新增 typed `Fork()` 和不生成 RGBA 的 `AdvanceControlled()`；共享规则、metadata、校验、唯一呈现 owner 和存档格式保留。正式 RPC 调用调整与真实页面帧率验收由平台页面的专项结果记录，微基准不代替该验收。

## 实测与 API

基准为 Apple M4、魔性之森双真人、已推进 320 帧，每项固定执行 50 次。原生与 Node／Go Wasm 分别运行，数据单位为每次调用的毫秒。

| 操作 | 原生原实现 | 原生新接口 | Wasm 原实现 | Wasm 新接口 |
| --- | ---: | ---: | ---: | ---: |
| JSON fork → typed fork | 3.141 | 0.0198 | 13.894 | 0.0946 |
| 带像素 Step → 无像素 Advance | 1.044 | 0.0864 | 3.416 | 0.2910 |
| Preview | 4.225 | 1.069 | 17.478 | 3.515 |

Wasm typed fork 的分配从约 433 KB／2,048 次降至 51 KB／101 次；无像素推进保留逐帧 metadata 和轻量 scene 校验，分配约 177 KB，而原带像素推进约 1.90 MB。旧双摘要独立测得 Wasm 2.165 ms；本 revision 未改变摘要算法。

| API | 约束 |
| --- | --- |
| `Session.Fork() (*Session, error)` | 直接复制可变 owner，复用私有只读资源；保留 common、选项、legacy tile／HUD 或计算引用校验。 |
| `Session.AdvanceControlled([4]engine.Buttons, uint8) (View, error)` | 仅 schema 2；原样调用共享 `sessiontick.Advance`，推进规则、cache、slots、HUD 和帧号，省去临时像素材质化与 raster。 |
| `Session.StepControlled(...)` | 保留原逐帧像素路径与错误原子性；需要目标帧画面时在该帧调用。 |
| `Session.Preview()` | 使用 typed fork；原权威状态不变，非终局预测 F+1，已清场终局保持 F。 |

权威追赶可以先 fork，在中间帧调用 `AdvanceControlled`；如果需要最后一帧像素，该帧调用 `StepControlled`。不能先把目标帧也无像素推进，再从最终对象状态猜 callback-time 画面。逐帧 metadata 仍全部执行，任何错误均不提交当前 Session。

## 可变字段审计

直接深复制以下容器，保留 nil 与非 nil 空切片的 JSON 区别：`Options.Players`、`World.Objects`、`World.Stream`、`World.Throws.Records`、`World.Registration.Owners`，以及互斥的 `computed.HUD.Slots` 或 `legacy.HUD.Slots`。

`WorldObject`、DragonHistory、ShapeBuffers、SpriteSlots、HUD Cells／Rules 均为值或固定数组，随结构值复制。`Assets` 是解码校验后封装在 Session 内的私有只读目录，可被 fork 共享。测试通过反射枚举所有引用容器；未来增加可变字段但没有扩展深复制清单时会失败。

## 测试与来源

失败清单 `runtimebridge/PERFORMANCE_FAILURES.md`、基准和新测试先于实现，缺少新接口的真实编译失败保留在 `testdata/performance-v1/missing-api-red.jsonl`。原 schema 2 冻结清单及原 49 条轨迹算法未修改。

一个 scene 负例在第一轮实际被原场地回调合法修复，因而不能代表错误状态。该 fixture 改为已结束状态，使场地回调不修复坏 source；前测试完整副本、前后 SHA 和真实失败日志保存在 `scene-fixture-lineage.json`，未放宽实现校验。

新测试覆盖七类 owner 的双向隔离、nil／空区别、实际 fork 后推进、结构／引用校验、连续双场地 600 帧状态相等、每 17 帧 callback-time 像素相等、电脑控制切换、坏输入／通道／掩码、帧溢出、scene 错误原子性及 fork 分配减半。旧 schema 1 字节和 schema 2 资源／checkpoint／预览边界继续复验。

所有本 revision 日志位于 `games/yuyuhakusho/runtimebridge/testdata/performance-v1/`。旧 `schema2-final-freeze.json` 保留原版本身份；新性能验收使用单独 revision manifest，不把旧通过结果当作新源码验收。

最终本 revision 原生与 Wasm 各 49 条完整轨迹（每端 203,428 参考帧）全部通过，world／slots／HUD／像素累计摘要与原冻结基线相同；两端各 10 项针对性检查和 `go vet` 通过。完整矩阵按原生、Wasm 顺序运行，固定四分片、最多两分片并行，曾按正式页面测量窗口 SIGSTOP／SIGCONT 原进程；暂停与恢复记录保留，未重启或覆盖通过结果。

最终清单为 `runtimebridge/testdata/performance-v1/revision-manifest.json`，来源与交付索引为同目录 `final-index.json`。正式页面正常双人／120 ms 慢端的帧率、输入与确认时差、改键、触屏和重连另见平台页面验收，不将本文件的微基准替代实际运行数据。
