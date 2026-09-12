# 游乐场配置

## 参数与存储

两主题首次使用时，Temperature、Top P、Max Tokens、Frequency Penalty、Presence Penalty、Seed 均未设置且关闭；请求只发送已开启并填写有效数值的参数。清空不是零，显式填写的 `0` 会保留。重置恢复未设置状态，模型、分组、接口和流式等运行选项仍有默认值。

配置分别保存到 `playground_config_default`、`playground_config_classic`。没有主题独立配置时，兼容读取旧的 `playground_config`；不删除旧键或聊天记录。已保存的数值与开关优先，缺失字段不再补入采样默认值。新版同时保存接口、思考等级、搜索和代码执行偏好。

## 内置工具

Web 搜索与代码执行独立启用。旧版 `toolsEnabled` 仅用于迁移缺失的新字段，新字段显式 `false` 不会被旧聚合开关覆盖。

| 接口 | Web 搜索 | 代码执行 |
| --- | --- | --- |
| OpenAI Chat | `web_search_options` | 不支持，控件禁用且请求不发送 |
| Responses | `web_search` | `code_interpreter` |
| Anthropic | `web_search_20250305` | `code_execution_20250825` |
| Gemini | `googleSearch` | `codeExecution` |

工具开关表达允许使用，不保证模型或渠道实际支持。候选模型按已声明的接口能力过滤，未知能力保留；跨协议中转是否保留工具仍取决于渠道适配器。自定义请求正文优先，不由开关隐式改写。

## 输入交互

新版桌面模型选择器打开后聚焦搜索框，切换分组不反复抢焦点；移动端保留主动点击搜索唤起键盘的行为。弹层点击不会触发底栏的自动聚焦，也不会聚焦隐藏字段。输入区整体禁用样式只跟随真正的输入控件，禁用单个工具按钮不使整块输入区变淡。
