# 版本更新日志验收

实现前确定的失败场景：历史版本遗漏、日志正文为空或被摘要替代、重复版本、无效日期、没有日志仍可发布、GitHub Release 与网站日志不一致、未登录无法阅读、版本链接中断当前对局、直接打开或刷新详情失效、历史导航失效、未知版本误显示其他日志、搜索无结果无法恢复、320px 溢出、长链接撑破布局、Markdown 原始 HTML 被执行。

发布门禁使用 `node --test scripts/release-source.test.cjs`。浏览器验收使用 `python3 -B scripts/check-release-history.py --url <站点地址> --out <仓库内结果目录>`，保留逐项 JSON 结果和桌面、320px 截图。历史来源覆盖以 GitHub Release 快照和 Git 标签逐个比对。

本次实现前的发布门禁检查已出现预期失败；完成实现后，31 项发布门禁、281 项公共前端检查、真实 HTTP 的本地／CDN 路由与静态资源回归均通过。历史比对确认 144 个正式 Release 正文完整保留，全部 145 个旧标签均有记录，加入本次版本后共 146 条日志。

浏览器结果见 [result.json](result.json)，包含 11 组检查、桌面／320px 实际尺寸、计算样式及文字对比度。截图：[桌面详情](desktop.png)、[手机详情](mobile.png)、[手机列表](list-mobile.png)。
