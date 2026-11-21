# 实时投影未变化通知失败场景

先编写失败场景与真实 HTTP／WebSocket 验收，再实现可选 `StreamViewResult.Unchanged`。本次只修改公共实时投影传输，不改变已有游戏的默认推送行为。

| 场景 | 预期 |
| --- | --- |
| 同一已发送游标对应的游戏投影未变化 | 省略重复 game 包；输入 ACK 继续立即发送 |
| 模块在未变化结果中返回不同游标 | 保留最后成功发送的游标，不前移 |
| 跳过重复投影 | 使用内部跳过信号，不回退成完整 state 推送 |
| 初始化、重连、房间或席位变化 | 空游标取得完整投影；不能跳过初次状态 |
| 规则代次或事件游标出现缺口 | 模块返回完整恢复投影，公共传输正常发布 |
| 公共 loading、ready、setupVersion、clockOffset、deadline、actors、attention、eliminated、paused、restartable、finished、winners 或 log 变化 | 使用空游标取得完整游戏投影与对应元数据，不能被游戏 Unchanged 吞掉 |
| 只有输入版本号或 serverNow 改变 | 不使稳定公共元数据指纹失效 |
| 未声明 Unchanged 的模块 | 保留原有逐次投影行为 |
| 普通 room state、托管或权限通知 | 保留原有通知与鉴权 |
| 模块错误地对空游标返回 Unchanged | 从该席位普通 View 取得完整投影，保持空游标，不猜测或共享其他席位状态 |

新增测试使用正式鉴权、SQLite 与真实 WebSocket；仅宿主元数据组合覆盖在受控夹具中触发，不以此证明游戏规则。新增数据库、临时文件及证据均放 `.local/cs16/`，复跑记录见 `.local/cs16/verification/stream-unchanged/`。
