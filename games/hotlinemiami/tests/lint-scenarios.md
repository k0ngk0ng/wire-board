# 代码与 CI 检查失败场景

此清单和 `lint.test.mjs` 先于检查器实现编写。检查范围仅为 `games/hotlinemiami/` 与 `scripts/hotlinemiami/`，不代表全仓库通过。

1. TypeScript、JavaScript、Go、Python 或 CSS 注释进入代码；字符串、正则和模板中的注释形状不应误报。
2. TypeScript、JavaScript、Go 绑定使用下划线；Python 绑定使用不合规范的大写；组件、Go 包名或文件名与目录不符。
3. 文件通过相对路径、绝对路径或目录中的符号链接逃出检查范围。
4. `eval`、`Function` 构造器、Python `exec`、`any` 或 `@ts-ignore` 绕过检查。
5. Go 业务规则引入浮点类型或小数字面量；输入和预测 TypeScript 引入小数业务阈值、随机数或浮点容差。
6. 临时文件指向 `/tmp`、用户目录、其他游戏的 `.local` 目录，或创建系统默认临时目录。
7. 游戏实现导入另一款游戏，直接创建音频播放器，或将二进制和媒体改成 Base64 载荷藏进源码。
8. JSON 语法错误、危险原型键、非法路径或命名不合规范。
9. 只检查手工列出的少量源文件，新增文件静默漏检；检查器必须递归发现受控目录中的全部支持语言。
10. CI 将单个游戏前端变更扩大到所有游戏，或遗漏新游戏 lint、前端测试、Go 规则测试与资源恢复入口。
11. 检查成功没有可核验产物，或未先幂等创建检查输出与缓存目录。
12. 已恢复的 `assets/public/` 官方运行时被误作自编源码检查；该子树只能在全量登记、大小和 SHA-256 均通过后作为外部载荷处理，不能漏掉子树外自编资产元数据。
13. CI 测试把某一版本的 workflow 命令文本当成唯一恢复入口；直接按游戏 hydrate 和显式缓存恢复均须执行真实选择、恢复及完整性检查，不能依赖未提交的 CI 改动。

`node --test games/hotlinemiami/tests/lint.test.mjs` 验证正负例与 CI 选择。`node games/hotlinemiami/build/lint.mjs` 检查当前全部受控文件并保存 `.local/hotlinemiami/verification/lint/result.json`。
