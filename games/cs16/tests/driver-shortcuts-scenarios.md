# 原生编辑快捷键驱动诊断

在编写隔离诊断前列出失败分支。正式 `formal-first` 的真实聊天 textarea 已收到可信 Control+A 和 Meta+A，修饰位正确且均未被应用取消，但选区为零。保留该失败，不以源码检查替代复现。

- 同一浏览器的纯 textarea、无任何应用脚本时，原始 CDP Meta+A 是否同样不执行平台编辑命令。
- 在相同可信按键中附加 CDP 原生编辑命令 `selectAll`，是否产生全选；不得直接调用 DOM `select()` 冒充原生编辑。
- 对照原始事件的目标、修饰位、可信标记和 `defaultPrevented`；若只应用页失败，继续查公共控制器／焦点。
- 明确 host 平台和驱动范围，不据此声明其他操作系统的浏览器加速键支持。

脚本 `driver-shortcuts.mjs` 只连接本任务独立的空白标签页。结果保存在 `.local/cs16/verification/driver-shortcuts.json`，诊断完成关闭该浏览器并删除其临时 profile。
