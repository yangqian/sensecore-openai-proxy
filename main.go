package main

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const TELEGRAPH_URL = "https://api.sensenova.cn"

var buffer string

type ChatRequest struct {
	MaxTokens         int     `json:"max_tokens,omitempty"`
	MaxNewTokens      int     `json:"max_new_tokens,omitempty"`
	FrequencyPenalty  float64 `json:"frequency_penalty,omitempty"`
	RepetitionPenalty float64 `json:"repetition_penalty,omitempty"`
	TopP              float64 `json:"top_p,omitempty"`
	Model             string  `json:"model,omitempty"`
}

type SenseAPIResponse struct {
	Data struct {
		ID    string `json:"id"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			KnowledgeTokens  int `json:"knowledge_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Choices []struct {
			Index            int    `json:"index"`
			Role             string `json:"role"`
			Delta            string `json:"delta"`
			ReasoningContent string `json:"reasoning_content,omitempty"` // Make reasoning_content optional
			FinishReason     string `json:"finish_reason"`
		} `json:"choices"`
		Plugins map[string]interface{} `json:"plugins"`
	} `json:"data"`
	Status struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
}

type OpenAIResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Choices           []Choice `json:"choices"`
}

type Choice struct {
	Index        int         `json:"index"`
	Delta        interface{} `json:"delta"`
	Logprobs     interface{} `json:"logprobs"`
	FinishReason interface{} `json:"finish_reason"`
}

type JWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type JWTPayload struct {
	Iss string `json:"iss"`
	Exp int64  `json:"exp"`
	Nbf int64  `json:"nbf"`
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	parsedURL, err := url.Parse(r.URL.String())
	if err != nil {
		http.Error(w, "Failed to parse URL", http.StatusInternalServerError)
		return
	}

	headersOrigin := r.Header.Get("Access-Control-Allow-Origin")
	if headersOrigin == "" {
		headersOrigin = "*"
	}

	if parsedURL.Path == "/v1/chat/completions" {
		targetURL, err := url.Parse(TELEGRAPH_URL)
		if err != nil {
			http.Error(w, "Failed to parse target URL", http.StatusInternalServerError)
			return
		}

		targetURL.Path = "/v1/llm/chat-completions"

		modifiedBody, err := modifyRequestBody(r)
		if err != nil {
			http.Error(w, "Failed to modify request body", http.StatusInternalServerError)
			return
		}

		modifiedRequest, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewBuffer(modifiedBody))
		if err != nil {
			http.Error(w, "Failed to create new request", http.StatusInternalServerError)
			return
		}

		// Copy headers
		for key, values := range r.Header {
			for _, value := range values {
				modifiedRequest.Header.Add(key, value)
			}
		}

		// Handle Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) > 1 {
				aksk := parts[1]
				if strings.Contains(aksk, "|") {
					akskParts := strings.Split(aksk, "|")
					if len(akskParts) == 2 {
						ak, sk := akskParts[0], akskParts[1]
						apiToken, err := encodeJWTToken(ak, sk)
						if err == nil {
							modifiedRequest.Header.Set("Authorization", "Bearer "+apiToken)
						}
					}
				}
			}
		}

		client := &http.Client{}
		resp, err := client.Do(modifiedRequest)
		if err != nil {
			http.Error(w, "Failed to forward request", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		w.Header().Set("Access-Control-Allow-Origin", headersOrigin)
		w.WriteHeader(resp.StatusCode)

		if resp.Header.Get("Content-Type") == "text/event-stream" {
			// Handle SSE
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			var originalModel string
			if modifiedBody != nil {
				var chatReq ChatRequest
				if err := json.Unmarshal(modifiedBody, &chatReq); err == nil {
					originalModel = chatReq.Model
				}
			}

			buffer = ""
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				buffer += line + "\n"

				if line == "" {
					transformedData := transformSSEChunk(buffer, originalModel)
					buffer = ""
					w.Write([]byte(transformedData))
					w.(http.Flusher).Flush()
				}
			}
		} else {
			// For non-SSE responses, just copy the body
			io.Copy(w, resp.Body)
		}
	} else {
		// Forward other requests directly
		targetURL, err := url.Parse(r.URL.String())
		if err != nil {
			http.Error(w, "Failed to parse URL", http.StatusInternalServerError)
			return
		}

		client := &http.Client{}
		req, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		// Copy headers
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Failed to forward request", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		w.Header().Set("Access-Control-Allow-Origin", headersOrigin)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func modifyRequestBody(r *http.Request) ([]byte, error) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && strings.Contains(contentType, "application/json") {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}

		if maxTokens, ok := body["max_tokens"]; ok {
			body["max_new_tokens"] = maxTokens
			delete(body, "max_tokens")
		}

		if frequencyPenalty, ok := body["frequency_penalty"].(float64); ok {
			body["repetition_penalty"] = (frequencyPenalty + 2) / 2
			delete(body, "frequency_penalty")
		}

		if topP, ok := body["top_p"].(float64); ok {
			if topP <= 0 {
				body["top_p"] = 0.000001
			} else if topP >= 1 {
				body["top_p"] = 0.999999
			}
		}

		return json.Marshal(body)
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

func transformSSEChunk(chunkStr, originalModel string) string {
	result := ""
	lines := strings.Split(chunkStr, "\n")

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "data:") {
			dataStr := strings.TrimSpace(line[5:])

			if dataStr == "[DONE]" {
				result += "data: [DONE]\n\n"
			} else {
				var data SenseAPIResponse
				err := json.Unmarshal([]byte(dataStr), &data)
				if err != nil {
					// Try original parsing method if the new structure fails
					continue
				}

				transformedData := OpenAIResponse{
					ID:                data.Data.ID,
					Object:            "chat.completion.chunk",
					Created:           time.Now().Unix(),
					Model:             originalModel,
					SystemFingerprint: "cf-openai-sensechat-proxy-123",
					Choices:           make([]Choice, len(data.Data.Choices)),
				}

				for j, choice := range data.Data.Choices {
					// Create delta object based on the content and reasoning_content
					deltaMap := make(map[string]interface{})

					// Add content if delta is not empty
					if choice.Delta != "" {
						deltaMap["content"] = choice.Delta
					}

					// Add reasoning_content only if it exists and is not empty
					if choice.ReasoningContent != "" {
						deltaMap["reasoning_content"] = choice.ReasoningContent
					}

					// If both are empty, use empty object
					if len(deltaMap) == 0 {
						deltaMap = map[string]interface{}{}
					}

					var finishReason interface{} = nil
					if choice.FinishReason != "" {
						finishReason = choice.FinishReason
					}

					transformedData.Choices[j] = Choice{
						Index:        choice.Index,
						Delta:        deltaMap,
						Logprobs:     nil,
						FinishReason: finishReason,
					}
				}

				jsonData, err := json.Marshal(transformedData)
				if err == nil {
					result += "data: " + string(jsonData) + "\n\n"
				}
			}
		} else if line != "" {
			result += line + "\n\n"
		}
	}

	return result
}

func transformDelta(delta interface{}, role string) interface{} {
	switch v := delta.(type) {
	case string:
		return map[string]interface{}{
			"role":    role,
			"content": v,
		}
	case map[string]interface{}:
		if len(v) == 0 {
			return map[string]interface{}{}
		}

		// Create a result map that will hold all fields
		result := make(map[string]interface{})

		// Copy all fields from the delta to preserve everything
		for k, val := range v {
			result[k] = val
		}

		return result
	default:
		// Check if it might be a reasoning_content structure
		if reasoningMap, ok := delta.(map[string]interface{}); ok && reasoningMap != nil {
			// If there's a reasoning_content field, preserve the structure
			return reasoningMap
		}
		return map[string]interface{}{}
	}
}

func encodeJWTToken(ak, sk string) (string, error) {
	header := JWTHeader{
		Alg: "HS256",
		Typ: "JWT",
	}

	now := time.Now().Unix()
	payload := JWTPayload{
		Iss: ak,
		Exp: now + 120, // Current time + 120 seconds
		Nbf: now - 5,   // Current time - 5 seconds
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	headerEncoded := base64UrlEncode(headerJSON)
	payloadEncoded := base64UrlEncode(payloadJSON)

	unsignedToken := headerEncoded + "." + payloadEncoded

	h := hmac.New(sha256.New, []byte(sk))
	h.Write([]byte(unsignedToken))
	signature := h.Sum(nil)

	signatureEncoded := base64UrlEncode(signature)

	return unsignedToken + "." + signatureEncoded, nil
}

func base64UrlEncode(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.TrimRight(encoded, "=")
	return encoded
}

func main() {
	http.HandleFunc("/", handleRequest)
	log.Println("Server starting on port 8089...")
	if err := http.ListenAndServe(":8089", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
