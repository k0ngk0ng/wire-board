# 浏览器 Go 支持脚本来源

浏览器预测使用本项目的 `games/pokepath/engine` Go 规则与 `cmd/browser/main_js.go` 同步接口。Wasm 构建不导入原网页游戏的 JavaScript 或其他游戏实现。

`wasm_exec.js` 来自实际选定 Go 工具链的 `lib/wasm/wasm_exec.js`。清单记录 Go 版本、原始支持脚本摘要、转换方式、发布文件大小与 SHA-256。构建器只去除代码注释并压缩空白，保持程序语义；原版权声明与完整 BSD 许可证保存在同目录的 `go-runtime-LICENSE.txt`，随浏览器运行器一并发布。

Go 的版权与许可原文可从 <https://go.dev/LICENSE> 查阅。构建时优先读取工具链根目录的 `LICENSE`，Homebrew 分发读取工具链相邻目录的实际 Go BSD 许可证；均为只读访问，发布副本和缓存只写入本仓库。

资源加载先校验清单声明的实际字节与 SHA-256；标准支持脚本执行时再次使用 SRI。清单与各资源通过稳定逻辑路径分别解析，资源地址不依赖清单最终重定向目录，因此相同内容可以复用不同历史发布目录中的原 URL。

名称规范化另使用项目已固定的 `golang.org/x/text v0.34.0`，仅将 `transform` 与 `unicode/norm` 两个实际 Go 依赖包编入运行器。其模块校验和、go.mod 校验和与固定 BSD-3-Clause 许可证摘要须同时匹配；替换模块、其他版本、其他第三方和仓库外源目录均拒绝。`manifest.dependencies` 声明模块、版本、双校验和、包列表、许可证及标准化源路径前缀，`sourceFiles` 为每个实际编译源文件记录字节和 SHA-256。该库的版权与许可附在同一个 `go-runtime-LICENSE.txt`，不引入额外游戏运行时。
