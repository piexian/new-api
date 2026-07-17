# Plan

1. 在匿名、清空 Cookie 和不同 User-Agent 上获取公网首页与响应头。
2. 使用浏览器自动化记录主题切换前后 Cookie、localStorage、资源 URL、控制台和网络失败。
3. 用 CodeGraph 追踪服务端前端选择、主题设置 API 和两套前端切换控件的调用路径。
4. 将公网证据与当前代码、构建产物和部署缓存行为交叉比对。
5. 形成 Critical/Warning/Info 审查结论、复现步骤和建议修复顺序。
6. 归档并提交 CCG 审查记录。
