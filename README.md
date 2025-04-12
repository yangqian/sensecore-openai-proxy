# SenseCore OpenAI Proxy

这是一个用于代理 SenseCore API 到 OpenAI API 请求的服务。

## 相关信息

项目基于，由AI转换。
1. https://linux.do/t/topic/67473
2. https://github.com/MurphyLo/cf-openai-sensechat-proxy

## 功能

- 转发 `/v1/chat/completions` 请求到目标 URL。
- 修改请求体中的字段，例如 `max_tokens` 转换为 `max_new_tokens`。
- 支持 SSE 响应的处理和数据转换。
- 自动生成 JWT Token 用于授权。

## 项目结构

```
main.go
```

- `main.go`：项目的主入口，包含所有核心逻辑。

## 环境要求

- Go 1.18 或更高版本

## 安装与运行

1. 克隆项目到本地：

   ```bash
   git clone https://github.com/your-repo/sensecore-openai-proxy.git
   cd sensecore-openai-proxy
   ```

2. 构建并运行服务：

   ```bash
   go run main.go
   ```

3. 服务将运行在 `http://localhost:8089`。

## 使用方法

### 转发请求

将请求发送到 `http://localhost:8089/v1/chat/completions`，服务会将其转发到目标 URL `https://api.sensenova.cn/v1/llm/chat-completions`。

### 修改请求体

服务会自动修改以下字段：

- `max_tokens` -> `max_new_tokens`
- `frequency_penalty` -> `repetition_penalty`
- 限制 `top_p` 的范围在 `(0.000001, 0.999999)`。

### SSE 响应处理

如果目标服务返回 `text/event-stream` 响应，服务会对数据进行转换并返回。

### JWT Token 生成

如果请求头中包含 `Authorization`，并且格式为 `Bearer <ak|sk>`，服务会自动生成 JWT Token 并替换原有的 `Authorization`。

## 配置

修改 `main.go` 中的常量 `TELEGRAPH_URL` 以更改目标服务的 URL：

```go
const TELEGRAPH_URL = "https://api.sensenova.cn"
```

## 示例请求

### 请求示例

```bash
curl -X POST http://localhost:8089/v1/chat/completions \
-H "Content-Type: application/json" \
-H "Authorization: Bearer <ak|sk>" \
-d '{
  "max_tokens": 100,
  "frequency_penalty": 0.5,
  "top_p": 1.0,
  "model": "gpt-3.5-turbo"
}'
```

### 响应示例

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion.chunk",
  "created": 1690000000,
  "model": "gpt-3.5-turbo",
  "system_fingerprint": "cf-openai-sensechat-proxy-123",
  "choices": [
    {
      "index": 0,
      "delta": {
        "content": "Hello, world!"
      },
      "finish_reason": null
    }
  ]
}
```

## 许可证

本项目使用 [MIT License](LICENSE)。