package llmtoken

import (
	"bytes"
	"context"
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

	// 捕获响应体
	rec := &responseRecorder{ResponseWriter: rw, body: &bytes.Buffer{}}
	p.next.ServeHTTP(rec, req)

	// 异步发送到 FastAPI
	go p.sendToFastAPI(req, reqBody, rec.body.Bytes(), time.Since(start).Seconds())
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

// RawPayload 上报的数据结构
type RawPayload struct {
	RequestBody  string  `json:"request_body"`
	ResponseBody string  `json:"response_body"`
	ElapsedTime  float64 `json:"elapsed_time"`
	Path         string  `json:"path"`
	Timestamp    string  `json:"timestamp"`
}

// 异步发送请求
func (p *TokenPlugin) sendToFastAPI(req *http.Request, reqBody, resBody []byte, elapsed float64) {
	payload := RawPayload{
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
