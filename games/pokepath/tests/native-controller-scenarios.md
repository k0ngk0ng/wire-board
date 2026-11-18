# 控制器接入失败场景

实现前固定检查：

- 正式模块未注册导航方案，或 HTTP 无法读取游戏方案。
- 预设键位冲突、替代键或标准手柄映射不能原样保存。
- 改键修改了默认预设或另一账号；旧修订覆盖新设置。
- 未知快捷动作、超界队伍编号错误触发规则操作。
- 游戏区文本输入、平台弹窗、观众及托管时触发快捷操作。
- 原默认键在改键后仍绕过公共映射；手柄和键盘走不同规则入口。
- 地图方向直接执行部署、确认无法使用、拖动后的防误触阻止导航确认。

HTTP 验收命令：`. scripts/env.sh` 后执行 `go test -race -count=1 ./games/pokepath/tests -run '^TestNativeController'`。结果保存至 `.local/pokepath/native-verification/controller-http/result.json`。键鼠与虚拟 Gamepad 的真实页面验证另存浏览器验收，不能把 HTTP 保存通过当作可玩性证明。
