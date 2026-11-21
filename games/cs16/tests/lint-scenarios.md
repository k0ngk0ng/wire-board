# CS 1.6 lint 开发前失败场景

本清单与可执行正负例先于 lint 实现建立。检查范围为 `games/cs16/` 和 `scripts/cs16/` 中本项目编写的 Go、TypeScript、JavaScript、Python、CSS 和 JSON；不代表全仓库通过，不扫描 `.local/cs16/research/` 的第三方代码。

- 变量、函数、类、字段、文件、目录或 Go 包名违反语言规则，语法错误，或源码逃逸登记目录。
- Go、TypeScript、JavaScript、Python 或 CSS 含代码注释，Python 含文档字符串；字符串中的 URL、井号或注释形状误报。
- TypeScript 使用 `any`、动态执行、平台外播放器或跨游戏实现依赖。
- Go 业务使用浮点类型或浮点字面值；输入、预测、运动、战斗等 TypeScript 使用小数、误差容差或非确定性随机数。
- Python 使用不带明确目录的临时文件或目录；任意语言使用其他游戏临时目录或工作区外的默认输出位置。
- JSON 无法解析或含原型污染键；CSS 类名或自定义属性不符合 kebab-case。
- 源码目录中出现符号链接；资产清单缺项、大小或摘要不符；注册过的资源被替换或删除。
- 第三方 WASM 引导脚本误作本项目源码 lint；通过跳过公开资源目录掩盖未登记文件。
- 原版 BSP、WAD、MDL、图像、声音或 WASM 载荷进入 Git；把客户端媒体改成 data URL 绕过边界。
- CI 把 CS16 前端或 lint 工具改动误判为后端完整对局，或漏掉 CS16 lint；运行其他游戏的专属测试。

共享空间音频扩展前新增失败场景：`web/src/audio-spatial.ts` 与 `web/tests/audio-spatial*.test.mjs` 未登记或只检查部分文件；共享平台音频错误套用游戏内禁止创建播放器的规则；直接使用 `AudioContext.destination` 绕过原生媒体音量通道；通过别名或下标访问绕过目标检查；测试缓存落到工作区外。既有 `audio.ts` 和 `game-api/module.ts` 只检查本次新增行，不能因原有注释失败，也不能漏掉新增注释、命名错误或直接输出。

共享 Go 扩展前新增失败场景：两个新增 Unchanged 测试文件被漏掉、包名未按 `server`／`gameapi` 目录检查、注释和命名或临时目录违规被接受；`module.go`／`realtime_stream.go` 的既有注释或浮点代码被误报，或者新增行中的浮点、注释、跨游戏实现依赖被放过。本游戏规则仍检查全部整数业务约束，不能通过公共旧代码的例外放宽游戏检查。

会话缓存收尾扩展前新增失败场景：`session_cache.go` 与测试未做全文件检查，`server.go` 既有注释被误报或新增行被漏掉；新 Go 引用 `unsafe`，或通过包别名及函数别名使用 `sql.Register`／`sqlite.RegisterConnectionHook` 的全局注册器。每连接实例的 `RegisterPreUpdateHook` 等接口应继续允许，不能将合法的 SQLite 连接钩子误判为全局注册。

复跑：`node --test games/cs16/tests/lint*.test.mjs` 和 `node games/cs16/build/lint.mjs`。产物为 `.local/cs16/verification/lint/` 的正负例、外部资源边界、CI 范围和实际源码报告。
