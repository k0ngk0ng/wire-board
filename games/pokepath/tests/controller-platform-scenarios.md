# 正式房间控制器验收

先定义场景：HTTP 保存改键后刷新读取，原默认键不能绕过方案；新键打开真实训练家面板；返回关闭游戏弹窗；标准虚拟 Gamepad 经过公共导航采样，方向只移动地图光标、快捷键只执行一次、放开后可以继续操作；全部规则动作仍由正式服务端接受。失败保留截图与错误。

改键后设置面板的快捷键提示必须显示同一账号当前生效的键位，不能继续显示原版默认值；语言切换也不能让提示与平台方案矛盾。

执行 `node games/pokepath/tests/controller-platform-browser.mjs`，复用当前 `native-platform-browser.mjs --serve` 创建的活跃认证浏览器。结果保存在 `.local/pokepath/verification/controller-platform/`。这里使用明确标注的虚拟 Gamepad API，不代表实体型号兼容性实测；测试结束恢复账号映射并刷新，移除虚拟设备。
