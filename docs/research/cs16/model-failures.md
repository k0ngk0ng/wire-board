# MDL10 转换失败场景

本清单与 `scripts/cs16/test_models.py` 先于转换实现编写。测试只验证原始模型数据转换，不代表原版规则或原版运行行为已经验证。

- 头部不足 244 字节，magic 不为 IDST，版本不为 10，声明大小超出原文件。
- 表数量、偏移、纹理尺寸或三角命令超出文件；负长度、非有限浮点和异常大的表不得分配无限内存。
- 骨骼父索引越界、前向引用或成环；顶点／法线骨骼索引和命令顶点索引越界。
- 动画 RLE 的 valid 大于 total、total 为零、值数据截断、帧数超限；跨 RLE 段边界产生错误的重复值。
- 外部序列组或纹理伴随文件缺失、格式错误或序列范围越界；不能默默丢弃动作。
- indexed 纹理透明索引错误、UV 没有按实际纹理尺寸归一化、原始多皮肤和 bodygroup 信息丢失。
- bone-local 顶点没有变换到 glTF bind pose，导致蒙皮后角色爆炸；inverse bind matrix 与骨架不互逆。
- strip／fan 解码缺三角面、退化面或材质映射越界；原子模型只有部分网格被转换。
- 动画骨骼旋转没有使用原版 XYZ 弧度含义，序列命名重复或 blend 丢失；事件、循环标志、命中盒和 attachment 不能恢复。
- 输出路径逃逸、源路径含符号链接；媒体写入 Git 路径；多次相同输入生成非确定字节。
- 8 个人物和全部 24 支枪的第一／第三人称／地面模型缺失；刀、三种投掷物、C4 与人质不能加载。
- gzip 的时间戳导致相同 GLB 得到不同字节；压缩／解压摘要或字节数不匹配。
- 转换缓存未绑定转换器源码、原模型与外部序列／纹理组；输出损坏后继续复用，或同源完整输出仍重复解析全部动画。
- 服务端命中盒丢失原分组，站姿／蹲姿骨骼矩阵数量与原骨架不一致，量化后使用了错误原点或轴序。
- 阿兹特克原实体引用的植物与骨骸模型遗漏；模型只在 `valve/models` 存在时没有按 GoldSrc 搜索路径解析，或缺失后使用替代几何掩盖错误。
- 人质原命中盒和脚底原点未导出；持枪姿态选择了 blend 0 的极端瞄准角度，或把原版 idle 姿态误当所有枪械的站／蹲姿态。

服务端规则仅导出原版 `idle1` 与 `crouch_idle` 的 blend 0 首帧全局骨骼矩阵，以及原版 bone-local 命中盒；平移和盒坐标乘 256，旋转矩阵乘 1048576，使用最接近整数且中点取偶数的量化方式。这两姿态不代表原版完整步态、瞄准混合或逐动画命中盒已经等价。

补充规则保留上述两姿态，并导出所有可用武器 extension 的 `ref_aim_*`、`crouch_aim_*` 首帧中央 blend；9 blend 序列选择第 4 组。每个姿态的序列名、blend、frame 在 `poseSources` 中记录。人质使用原 `hostage.mdl` 的 `idle1`，保持脚底原点。仍未宣称完整步态混合、实时俯仰插值或逐帧动作等价。

## 实际 GPU 缺陷记录

浏览器产物 `.local/cs16/browser/reports/diagnostic-third/de_aztec-first-frame.png` 显示 Glock 18 的白色手臂占据下半视区，材质与姿态均需核验。已有 Three PNG 尺寸 stub 仅证明载荷可解析，不能证明贴图被浏览器解码或实际 GPU 正确。

修改转换器前，通过 `scripts/cs16/test_model_poses.py` 建立独立数值对照：从原 MDL 的骨骼局部顶点和 RLE 帧，以直接欧拉旋转矩阵构造源姿态；与实际 Three GLTFLoader、SkeletonUtils.clone、AnimationMixer 的同帧蒙皮顶点逐点比较，覆盖原始方向和产品视模轴变换。阈值 0.001 个原版单位只用于浮点图形转换验证，不用于业务规则边界。该对照仍不替代真实浏览器中的材质、透视和 GPU 画面验收。

## 三角面正反与左右手

源姿态逐点一致不等于三角面朝向一致。旧导出保留原 strip／fan 绕序，并统一设置 `doubleSided: true`，掩盖 GoldSrc 与 glTF 的正面约定差异；原 Glock 大多数面叉积与原法线相反。

Valve 官方 Half-Life SDK 的 `utils/mdlviewer/studio_render.cpp` 在提交原 strip／fan 前调用 `glCullFace(GL_FRONT)`；glTF 则以逆时针为外表面。应在格式转换时统一反转每个三角形的索引顺序，并默认单面。不得按每张面与平滑法线的点积逐面猜测。MASKED／ADDITIVE 仍保留原纹理标志和 alpha mode；兼容 renderer 的这些分支调整 alpha／blend 状态，没有改变模型 cull 约定。

固定来源与代码片段摘要保存在 `.local/cs16/research/render-reference/evidence.json`。CS 兼容客户端的 `cl_righthand` 路径镜像视模原 Y 轴；反射不能编码为 quaternion，且刀／盾牌存在原版分支。左右手显示由客户端单独实现，不在通用模型导出时镜像所有角色或武器。

- [Valve 官方 MDL viewer，固定提交 b1b5cf5](https://github.com/ValveSoftware/halflife/blob/b1b5cf5892918535619b2937bb927e46cb097ba1/utils/mdlviewer/studio_render.cpp#L653)：`GL_FRONT` 与原 strip／fan 提交。
- [CS 兼容客户端，固定提交 ba5549f](https://github.com/Velaron/cs16-client/blob/ba5549fe32ce0fd94630a582401fc5b2c76f3f23/cl_dll/StudioModelRenderer.cpp#L713)：右手镜像 Y 轴；905 行起为刀／盾牌分支。
- [Xash 兼容 renderer，固定提交 cdcb29a](https://github.com/FWGS/xash3d-fwgs/blob/cdcb29aa86a5ab16154622e862dd05c62b7561fc/ref/gl/gl_studio.c#L2057)：视模镜像与 cull 处理；2090 行起为 MASKED／ADDITIVE alpha 状态。

这些源码证据确定转换所需的格式约定，不等同于对所选原版引擎二进制完成运行观察。

针对性验证先测试确定的 RLE、损坏输入和一把真实 AK47，再执行全部必需模型转换 E2E。产物位于 `.local/cs16/verification/models.json`，包含原文件与 GLB 的 SHA-256、骨骼／网格／动画数。复跑命令为 `python3 -B scripts/cs16/test_models.py` 和 `python3 -B scripts/cs16/models.py`。
