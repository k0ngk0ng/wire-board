# 平台构建与界面阶段记录

本索引说明早期记录的适用阶段，保留原始文件内容。

- `platform-all-package-compile*.log` 是音频 API 尚在先行测试阶段时的历史编译记录；文件名中的 `current` 不代表最新验收。补齐 API 后的构建检查见 [首次整合检查](audio-integration-preflight/compile-all.json)；新增音频组合层后的最终构建及测试链接结果见 [组合层构建记录](audio-mixed-runtime/final/compile-all-command.json)。两次后续检查均通过，均未运行全仓库行为测试。
- [platform-compile-input.json](platform-compile-input.json) 只标识编译时的最小 embed 输入。该目录不包含完整客户端，不能用作发布包或替代实际试玩服务的独立 dist。
- `platform-host-cpu-profile.log` 保留早期平台宿主性能诊断输出；实际游戏帧率与输入延迟以 [正式页面网络结果](platform-app-browser/public-ui-network09/result.json) 为准。
- [公共资源阶段报告](platform-web/PUBLIC_MODE_RESOURCE_REPORT.md) 和 [先行失败范围](platform-web/PUBLIC_MODE_UI_FAILURES.md) 记录基于 `2ac14ca9` 的中间阶段。后续公共布局、分组、对比度与全屏接手验收见 [最终公共界面报告](platform-web/PUBLIC_UI_OPTIN_REPORT.md)。同目录的 `app-build-compact-notice.log`、`typecheck-public-energy-v1.log` 对应中间构建与类型检查。
- `scripts/yuyuhakusho/verify_client_bridge_performance.py` 是此前 typed Fork／无像素重放性能修订的复跑工具；其当前字节与既有 [性能修订清单](../runtimebridge/testdata/performance-v1/revision-manifest.json) 中的来源绑定一致。本次只补收该已冻结工具，没有重跑、覆盖旧结果或将历史 source 证明扩大为当前全平台证明。
