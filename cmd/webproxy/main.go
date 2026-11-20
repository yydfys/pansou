// PanSou Web 前端托管 + API 反代（零依赖，仅标准库）
//
// 用途：手机 / WebHTV 实验室里，PanSou 本体（pansou-arm64，默认 :8888）只提供 /api，
// 这个 webproxy 负责把前端静态文件发出去，并把 /api/* 反代到本体。
//
// 环境变量：
//   WEB_ROOT    静态目录（默认 ./dist）
//   LISTEN      监听地址（默认 :8080）
//   API_UPSTREAM 后端地址（默认 http://127.0.0.1:8888）
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func main() {
	root := env("WEB_ROOT", "./dist")
	listen := env("LISTEN", ":8080")
	upstream := env("API_UPSTREAM", "http://127.0.0.1:8888")

	target, err := url.Parse(upstream)
	if err != nil {
		log.Fatalf("API_UPSTREAM 非法: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		log.Printf("反代失败 %s: %v", r.URL.Path, e)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":502,"message":"backend unavailable"}`))
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		log.Fatalf("WEB_ROOT 非法: %v", err)
	}
	if st, err := os.Stat(absRoot); err != nil || !st.IsDir() {
		log.Fatalf("WEB_ROOT 不存在或不是目录: %s", absRoot)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// API 全部走反代
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			proxy.ServeHTTP(w, r)
			return
		}
		// 静态文件；不存在则回退 index.html（SPA 路由）
		p := filepath.Join(absRoot, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			http.ServeFile(w, r, p)
			return
		}
		index := filepath.Join(absRoot, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})

	log.Printf("webproxy 启动: 静态 %s | 反代 %s | 监听 %s", absRoot, upstream, listen)
	if err := http.ListenAndServe(listen, mux); err != nil {
		log.Fatalf("监听失败: %v", err)
	}
}
