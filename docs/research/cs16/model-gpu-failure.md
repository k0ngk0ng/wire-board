# 第一人称模型 GPU 失败记录

真实浏览器在 Aztec 的首帧出现白色、异常占据下半画面的 Glock 手臂。失败截图位于 `.local/cs16/browser/reports/diagnostic-third/de_aztec-first-frame.png`，同轮 `de_aztec-active-controls.png` 记录进入控制后的画面。地图贴图在同一页面中正常显示。

该结果推翻了「GLTFLoader 解析成功即可作为模型视觉正确依据」的假设。此前的 `models-three-loader.json` 明确使用 PNG 尺寸替身，只验证了骨骼和动画对象构建，没有验证浏览器实际纹理解码、CSP 或 GPU 画面。

## 已定位的加载问题

服务器当前 CSP 的 `img-src` 和 `connect-src` 不包含 `blob:`。Three.js 的标准 GLTFLoader 对 GLB 内嵌 PNG 创建 Blob URL，随后交给 ImageBitmapLoader 发起读取；受 CSP 限制时该纹理读取会失败，而 GLTFLoader 允许返回空纹理并继续完成模型解析。这能够解释白色材质，尚不能单独解释所有屏幕占比与轮廓问题。

修复方向是在游戏加载器中直接读取 GLB image bufferView，验证 PNG 后通过 `createImageBitmap(Blob)` 解码，并创建真实 Texture，避免发起 Blob URL 读取。加载就绪前还须核对所有被引用的贴图实际存在，不能把无贴图白模作为成功。

## 姿态核验与准入

模型代理已独立读取原 MDL 的骨骼局部顶点和动画 RLE，在相同序列／帧构造源姿态，逐点对照 Three.js 的 GLTFLoader、SkeletonUtils.clone 和 AnimationMixer 计算后的蒙皮顶点。48 个场景覆盖 Glock、AK 和 M4 的 idle、draw、reload 首／中／末帧，并纳入实际父坐标变换；最大顶点误差为 0.00000323 个原单位。该范围内未发现 inverseBind、骨骼局部变换或动画转换错误，未修改转换器、GLB、客户端清单或 Go 规则数据。结果位于 `.local/cs16/verification/model-pose-glb.json`。

Glock 的原 idle 和 draw 末帧包围盒为 X -0.4313 至 17.0849、Y -2.4670 至 7.4085、Z -11.5517 至 -2.4661。原模型本身包含接近相机的手臂顶点。当前窄屏使用固定 75 度垂直视场角时，约 0.7 的宽高比对应约 54 度水平视场角，会显著放大这些近相机部位；该投影行为须与模型转换问题分开验证。

浏览器加载路径修复后，再核对真实纹理、轮廓、动画、宽屏和窄屏下的投影。当前武器相机的近裁剪面和视场角也须与原版证据核对。整体缩小、隐藏手臂或换成替代模型不能作为本次失败的修复依据。最终准入需要真实 GPU 截图，数值对照只负责缩小骨骼转换的排查范围。

## 已确认的三角形约定缺口

Valve 官方 Half-Life SDK 的 `utils/mdlviewer/studio_render.cpp` 使用 `glCullFace(GL_FRONT)`，并直接提交 MDL 原始 triangle fan／strip 命令，证明该格式采用与 glTF 默认前面约定相反的绕序。初版转换器保留原绕序并把所有材质设为双面，这掩盖了格式差异。它不改变顶点位置，因此不会被上述姿态数值测试发现，却会影响正反面和照明。

修正应统一反转输出索引，使 glTF 使用逆时针前面，再按原渲染方式保留材质的遮罩和混合语义。各面不按法线猜测单独翻转，手臂也不单独隐藏。CS 兼容客户端的视模右手模式另对原 Y 轴作反射，属于显示层根变换；这与格式绕序转换分开处理，避免双重翻转。视模反射必须使用缩放反射矩阵，不能把负行列式矩阵当作旋转四元数。

模型单独重生入口为 `python3 -B scripts/cs16/build_assets.py --models-only`。它只刷新模型、主清单和资源登记候选；地图和 Go 规则的摘要必须保持不变。该入口具有连续两次执行一致性的 E2E 验证。
