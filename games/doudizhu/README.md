# 斗地主

独立实现三人经典与四人上海模式。四人王炸固定为四个王，压过八张炸弹。规则边界和验收范围见 `docs/poker-implementation.md`。

开发先行测试：`go test ./games/doudizhu/server ./games/doudizhu/tests`。自然模块对局产物写入 `.local/poker/doudizhu/`，不把模块轨迹冒称浏览器整局。
