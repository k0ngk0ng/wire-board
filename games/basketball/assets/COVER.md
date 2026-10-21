# 封面人物素材与处理

封面采用热血系列官方运动主题人物插画，并重新绘制街头球场和三层篮筐。人物为国夫与阿力；游戏名称统一为「热血篮球」，显示在平台卡片标题中。

人物原件来自官方《超热血高校躲避球部》角色介绍页：

- 页面：<https://www.kuniokun.jp/dodge/charactor/>
- 图片：<https://www.kuniokun.jp/dodge/charactor/chara01.jpg>
- 原始尺寸：237 × 264。没有找到该独立人物图的高清版本，不将放大处理描述为高清重绘。
- 权利方标注：`(C) Million co.,Ltd. / ARC SYSTEM WORKS`。未找到开放再分发许可，不将其标为原创人物或可自由商用素材。

这不是 1993 年篮球游戏的原画，也不是包装盒扫描图。处理保留原人物轮廓、夹球姿态和衣服遮挡关系，移除外侧背景，重新绘制持球的颜色与篮球接缝。球场、街景、篮筐及整体横向构图由 `web/Cover.tsx` 绘制；没有把闭合拳头拼成扣篮手势。

原件与中间图保留在工作区忽略目录。仓库提交来源清单、处理脚本和内容校验值；发布图片通过 OSS 不可变内容地址提供，并由构建步骤下载、校验后打入本地回退。`cover-manifest.json` 保存原件与成品的 SHA-256、尺寸、字节数和出处。

复跑处理：

```sh
.local/board-art-env/bin/python -B scripts/basketball/prepare_cover_art.py
```

该环境只需要 Pillow。处理脚本验证原件身份，输出须与已选封面校验值一致；不会自动上传或改变发布清单。原始姿态是背靠背持球、前伸拳，未宣称生成过 AI 原画。

## 验证

- `games/basketball/tests/cover-assets.test.mjs` 验证冷下载、本地回退、缓存损坏、错误 MIME／校验值／尺寸以及路径边界。
- `scripts/basketball/asset-build.test.mjs` 验证应用只包含列入清单的篮球媒体，包括独立封面。
- `scripts/basketball/accept_cover.py` 使用实际 React 组件和浏览器检查 CDN、本地图片、真实 404 后的回退及 320px 布局，保存截图与像素差异。报告对亚像素边缘采用明确的微小容差，不将容差比较写成逐位相等。
