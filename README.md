# SenseCore OpenAI Proxy

这是一个用于代理 SenseCore API 到 OpenAI API 请求的服务。

## 相关信息

项目由AI转换基于
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
   git clone https://github.com/yangqian/sensecore-openai-proxy.git
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

## 许可证

本项目使用 [MIT License](LICENSE)。