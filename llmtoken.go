package llmtoken

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config 配置结构体
type Config struct {
	Apiurl string `json:"Apiurl,omitempty"` // 注意这里的命名不要驼峰，否则插件加载有问题
}

// CreateConfig 创建默认配置
func CreateConfig() *Config {
	return &Config{}
}

// TokenPlugin 插件结构体
type TokenPlugin struct {
	next   http.Handler
	name   string
	Apiurl string
}

// New 插件实例化
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.Apiurl == "" {
		return nil, fmt.Errorf("Apiurl is required")
	}
	return &TokenPlugin{
		next:   next,
		name:   name,
		Apiurl: config.Apiurl,
	}, nil
}

// 请求拦截处理
func (p *TokenPlugin) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	start := time.Now()

	// 读取请求体
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(rw, "failed to read request body", http.StatusInternalServerError)
		return
	}
	req.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	// 每个请求生成一个 UUID 作为 request_id
	requestID := GenerateRequestID()

	// 捕获响应体
	rec := &responseRecorder{ResponseWriter: rw, body: &bytes.Buffer{}}
	p.next.ServeHTTP(rec, req)

	// 异步发送到 FastAPI
	go p.sendToFastAPI(req, reqBody, rec.body.Bytes(), time.Since(start).Seconds(), requestID)
}

// 响应体捕获器
type responseRecorder struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// GenerateRequestID 生成唯一的 request_id，适合高并发环境
func GenerateRequestID() string {
	// 当前纳秒时间
	nanoTime := time.Now().UnixNano()

	// 生成8字节随机数
	randomBytes := make([]byte, 8)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// 如果随机数生成失败，就只用时间戳
		return fmt.Sprintf("%d", nanoTime)
	}

	// 转成16进制字符串
	randomHex := hex.EncodeToString(randomBytes)

	// 组合：时间戳 + 随机串，保证全局唯一
	return fmt.Sprintf("%d-%s", nanoTime, randomHex)
}

// RawPayload 上报的数据结构
type RawPayload struct {
	RequestID    string  `json:"request_id"` // 每请求的唯一标识
	RequestBody  string  `json:"request_body"`
	ResponseBody string  `json:"response_body"`
	ElapsedTime  float64 `json:"elapsed_time"`
	Path         string  `json:"path"`
	Timestamp    string  `json:"timestamp"`
}

// 异步发送请求
func (p *TokenPlugin) sendToFastAPI(req *http.Request, reqBody, resBody []byte, elapsed float64, requestID string) {
	payload := RawPayload{
		RequestID:    requestID, // 传入
		RequestBody:  string(reqBody),
		ResponseBody: string(resBody),
		ElapsedTime:  elapsed,
		Path:         req.URL.Path,
		Timestamp:    time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("failed to marshal payload: %v\n", err)
		return
	}

	resp, err := http.Post(p.Apiurl, "application/json", bytes.NewBuffer(data))
	if err != nil {
		fmt.Printf("failed to send payload: %v\n", err)
		return
	}
	defer resp.Body.Close()
}
