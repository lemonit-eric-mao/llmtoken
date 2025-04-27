package llmtoken_test

import (
	"context"
	"github.com/lemonit-eric-mao/llmtoken"
	"net/http"
	"testing"
)

// TokenPlugin 插件结构体
type TokenPlugin struct {
	next   http.Handler
	name   string
	Apiurl string
}

func TestUrlPassed(t *testing.T) {
	// 配置插件，Url指向假服务器
	cfg := llmtoken.CreateConfig()
	cfg.Apiurl = "http://example.com:8000/api/report"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("hello from backend"))
	})

	handler, err := llmtoken.New(ctx, next, cfg, "llm-token-plugin")
	if err != nil {
		t.Fatal(err)
	}

	// 类型断言，把 handler 还原成 *TokenPlugin
	tokenPlugin, ok := handler.(*llmtoken.TokenPlugin)
	if !ok {
		t.Fatal("handler is not a TokenPlugin")
	}

	// 然后就可以直接访问 Apiurl 字段了
	if tokenPlugin.Apiurl != cfg.Apiurl {
		t.Errorf("expected Apiurl %s, got %s", cfg.Apiurl, tokenPlugin.Apiurl)
	} else {
		t.Logf("PASS Apiurl: %s == cfg.Url: %s", cfg.Apiurl, tokenPlugin.Apiurl)
	}
}
