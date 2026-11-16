# 版本更新日志

网站提供 `/releases` 版本列表和 `/releases/vX.Y.Z` 详情页，无需登录。列表支持按版本号、标题或正文搜索，每次显示 20 条；详情可以直接打开、刷新、分享，并通过浏览器前进／后退切换。大厅、对局顶部和更新提醒中的版本号在新标签页打开详情，保留当前页面。

日志保存在 `web/src/releases/history.json`，随应用源码和构建归档。详情页按需加载，保留 Markdown 的段落、列表、表格和链接，不渲染原始 HTML。页面阅读不依赖 GitHub API。

## 新版本记录

在构建前向数组顶部加入本次版本：

```json
{
  "version": "v1.59.0",
  "date": "2026-11-16",
  "title": "版本更新日志",
  "body": "- 新增按版本查看更新详情。\n- 补齐历史版本记录。"
}
```

`version` 使用未发布的 `vX.Y.Z`，`date` 为版本日期，`title` 简述本次更新，`body` 记录完整更新内容。数组按版本号倒序排列。历史正文来自对应 GitHub Release；早期只有对比链接的版本补充对应版本区间的 Git 提交记录，不能把提交日期当作已核实的发布日期。仅有标签的 v1.0.11 使用标签对应提交的日期和提交记录。

发布前运行 `node --test scripts/release-source.test.cjs`。发布流程从当前提交读取目标版本的日志，使用相同标题和正文创建或同步草稿 Release；日志缺失、空白、重复、日期无效或排序错误时停止发布。已经正式发布的 Release 仍禁止覆盖。

## 验收

浏览器验收先构建应用，再运行：

```sh
npm --prefix web ci
npm --prefix web run build
python3 -B scripts/check-release-history.py --out .local/release-history-browser
```

脚本启动真实 HTTP 服务，使用实际构建页面，验证未登录阅读、版本列表与搜索、历史详情、直接打开与刷新、浏览器前进／后退、未知版本、320px 布局及登录后的版本链接。结果目录保留 `report.json`、Go 服务日志和截图，临时数据库、浏览器 profile 和 socket 在结束时清理。检查线上页面时使用 `--url <站点地址>`；线上模式不创建账号。
