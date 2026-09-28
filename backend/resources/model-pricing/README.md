# 模型价格数据

`model_prices_and_context_window.json` 是服务读取的价格文件（配置项 `pricing.fallback_file`，默认指向本文件），格式与 LiteLLM 的同名文件一致。服务不做任何远程同步，也不读数据目录里的价格文件。

## 加载与生效

- 启动时读取本文件，再叠加可选的 override 补丁文件（`pricing.override_file`，按字段浅合并，优先级最高）。
- 运行中每隔 `pricing.hash_check_interval_minutes`（默认 10 分钟）比对这两个文件的内容指纹，变了就热重载，并重新播种模型目录；文件半写或不是 JSON 对象时保留当前价格，下一轮再试。

## 更新

需要更新时直接替换本文件并随代码发布，例如从 LiteLLM 取最新版后人工核对差异：

```bash
curl -s https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json -o model_prices_and_context_window.json
```
