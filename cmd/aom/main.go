package main

import (
	"aom/internal/server"
	"context"
	"crypto/tls"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

//go:embed all:web
var assets embed.FS

func main() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	addr := flag.String("addr", "127.0.0.1:8080", "监听地址；局域网使用0.0.0.0:8080")
	dataDir := flag.String("data-dir", filepath.Join(filepath.Dir(exe), "data"), "数据目录，默认位于可执行文件旁")
	autoTLS := flag.Bool("https", false, "使用data目录中的证书启动HTTPS，缺失时生成本机自签名证书")
	tlsCert := flag.String("tls-cert", "", "HTTPS证书PEM路径")
	tlsKey := flag.String("tls-key", "", "HTTPS私钥PEM路径")
	flag.Parse()
	if *autoTLS && *tlsCert == "" && *tlsKey == "" {
		cert, key, e := server.EnsureLocalCertificate(*dataDir)
		if e != nil {
			log.Fatal(e)
		}
		*tlsCert = cert
		*tlsKey = key
		fmt.Println("HTTPS使用本地证书。客户端需核对并信任server.crt；正式内网可使用组织签发的证书。")
	}
	if (*tlsCert == "") != (*tlsKey == "") {
		log.Fatal("HTTPS需要同时指定--tls-cert和--tls-key")
	}
	web, err := fs.Sub(assets, "web")
	if err != nil {
		log.Fatal(err)
	}
	if _, err = fs.Stat(web, "index.html"); err != nil {
		log.Fatal("前端资源缺失，请先执行 npm run build")
	}
	auth, err := server.NewAuthServer(*dataDir, web, os.Getenv("AOM_SETUP_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	defer auth.Close()
	if token := auth.SetupToken(); token != "" {
		fmt.Printf("\n首次设置账号需要初始化令牌（请勿分享）：\n%s\n", token)
	}
	scheme := "http"
	if *tlsCert != "" {
		scheme = "https"
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Printf("监听失败（可能端口已占用）：%v", err)
		return
	}
	srv := &http.Server{Handler: auth, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	fmt.Printf("\n AOM · 机房数字孪生与运维系统\n 浏览器访问：%s://%s\n 数据目录：%s\n 按 Ctrl+C 安全退出。\n\n", scheme, *addr, *dataDir)
	if host, port, e := net.SplitHostPort(*addr); e == nil && (host == "0.0.0.0" || host == "::") {
		fmt.Printf(" 本机访问：%s://localhost:%s\n", scheme, port)
		if *tlsCert == "" {
			fmt.Println(" 局域网登录需要HTTPS，请用--tls-cert和--tls-key指定证书。")
		} else {
			fmt.Printf(" 局域网访问：https://服务器IP:%s\n", port)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if *tlsCert != "" {
		err = srv.ServeTLS(listener, *tlsCert, *tlsKey)
	} else {
		err = srv.Serve(listener)
	}
	if err != nil && err != http.ErrServerClosed {
		log.Printf("服务停止：%v", err)
	}
}
