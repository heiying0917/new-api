package service

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newFakeAuthProxy 起一个要求 Basic 认证的 HTTP 正向代理，直接回 Cloudflare trace 格式的响应体。
func newFakeAuthProxy(t *testing.T, user, pass string) *httptest.Server {
	t.Helper()
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != wantAuth {
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		// 经代理转发的请求是 absolute-form，Host 应为探测目标而非代理自身
		if r.URL.Host != "probe.test" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("fl=1\nh=probe.test\nip=203.0.113.7\nts=1\nloc=JP\n"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProbeProxyColonFormatSuccess(t *testing.T) {
	srv := newFakeAuthProxy(t, "me8bsitx", "z6ysax5f")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	res, err := probeProxy(context.Background(), host+":"+port+":me8bsitx:z6ysax5f", "http://probe.test/cdn-cgi/trace", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("probe should succeed, got %+v", res)
	}
	if res.ExitIP != "203.0.113.7" || res.Country != "JP" {
		t.Fatalf("exit ip/country = %q/%q, want 203.0.113.7/JP", res.ExitIP, res.Country)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if strings.Contains(res.Proxy, "z6ysax5f") {
		t.Fatalf("returned proxy must redact password, got %q", res.Proxy)
	}
	if res.Proxy != "http://me8bsitx:xxxxx@"+host+":"+port {
		t.Fatalf("returned proxy should show normalized user@host:port, got %q", res.Proxy)
	}
}

func TestProbeProxyWrongPassword(t *testing.T) {
	srv := newFakeAuthProxy(t, "user", "right-pass")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	res, err := probeProxy(context.Background(), host+":"+port+":user:wrong-pass", "http://probe.test/cdn-cgi/trace", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ok {
		t.Fatalf("407 from proxy must be reported as failure, got %+v", res)
	}
	if res.StatusCode != http.StatusProxyAuthRequired || res.Message == "" {
		t.Fatalf("want 407 with message, got %+v", res)
	}
}

func TestProbeProxyUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // 端口立即关闭，连接会被拒绝

	const secret = "s3cretPassw0rd"
	res, err := probeProxy(context.Background(), "socks5://"+addr+":user:"+secret, "http://probe.test/cdn-cgi/trace", 3*time.Second)
	if err != nil {
		t.Fatalf("connect failure must not be a format error: %v", err)
	}
	if res.Ok || res.Message == "" {
		t.Fatalf("unreachable proxy must fail with a message, got %+v", res)
	}
	if strings.Contains(res.Message, secret) || strings.Contains(res.Proxy, secret) {
		t.Fatalf("password leaked in result: %+v", res)
	}
}

func TestProbeProxyInvalidFormat(t *testing.T) {
	if _, err := probeProxy(context.Background(), "not a proxy", "http://probe.test/", time.Second); err == nil {
		t.Fatalf("invalid proxy format must return error")
	}
	if _, err := probeProxy(context.Background(), "   ", "http://probe.test/", time.Second); err == nil {
		t.Fatalf("empty proxy must return error")
	}
}

// newFakeSocks5Proxy 起一个只支持 SOCKS5（用户名/密码认证）的代理：
// 收到非 SOCKS5 握手（如 HTTP 请求）时回 SOCKS4 拒绝包并断开，复现真实代理商端口的行为；
// CONNECT 成功后直接以 HTTP 回 Cloudflare trace 格式的响应体。
func newFakeSocks5Proxy(t *testing.T, user, pass string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveFakeSocks5(conn, user, pass)
		}
	}()
	return ln.Addr().String()
}

func serveFakeSocks5(conn net.Conn, user, pass string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 512)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	if buf[0] != 0x05 {
		_, _ = conn.Write([]byte{0x00, 0x5b, 0, 0, 0, 0, 0, 0})
		return
	}
	if _, err := io.ReadFull(conn, buf[:buf[1]]); err != nil {
		return
	}
	_, _ = conn.Write([]byte{0x05, 0x02}) // 要求用户名/密码认证
	// RFC 1929: VER ULEN UNAME PLEN PASSWD
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	uname := make([]byte, buf[1])
	if _, err := io.ReadFull(conn, uname); err != nil {
		return
	}
	if _, err := io.ReadFull(conn, buf[:1]); err != nil {
		return
	}
	passwd := make([]byte, buf[0])
	if _, err := io.ReadFull(conn, passwd); err != nil {
		return
	}
	if string(uname) != user || string(passwd) != pass {
		_, _ = conn.Write([]byte{0x01, 0x01})
		return
	}
	_, _ = conn.Write([]byte{0x01, 0x00})
	// CONNECT: VER CMD RSV ATYP DST.ADDR DST.PORT
	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return
	}
	switch buf[3] {
	case 0x01:
		_, _ = io.ReadFull(conn, buf[:4+2])
	case 0x03:
		_, _ = io.ReadFull(conn, buf[:1])
		_, _ = io.ReadFull(conn, buf[:int(buf[0])+2])
	case 0x04:
		_, _ = io.ReadFull(conn, buf[:16+2])
	}
	_, _ = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return
	}
	_ = req.Body.Close()
	body := "ip=198.51.100.9\nloc=VN\n"
	_, _ = fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
}

// 回归：代理商 host:port:user:pass 常是 SOCKS5 端口。未写协议时应自动识别出 SOCKS5，并给出带协议的建议写法。
func TestProbeProxySchemelessDetectsSocks5(t *testing.T) {
	addr := newFakeSocks5Proxy(t, "me8bsitx", "z6ysax5f")
	raw := addr + ":me8bsitx:z6ysax5f"

	res, err := probeProxy(context.Background(), raw, "http://probe.test/cdn-cgi/trace", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("socks5-only proxy should be detected and succeed, got %+v", res)
	}
	if res.SuggestedProxy != "socks5://"+raw {
		t.Fatalf("suggested proxy = %q, want %q", res.SuggestedProxy, "socks5://"+raw)
	}
	if res.ExitIP != "198.51.100.9" || res.Country != "VN" {
		t.Fatalf("exit ip/country = %q/%q", res.ExitIP, res.Country)
	}
	if !strings.HasPrefix(res.Proxy, "socks5://") || strings.Contains(res.Proxy, "z6ysax5f") {
		t.Fatalf("returned proxy should be redacted socks5 url, got %q", res.Proxy)
	}
}

// 写明 http:// 却连到 SOCKS 端口：不擅自改协议，但要提示改用 socks5://。
func TestProbeProxyExplicitHttpOnSocksPortHints(t *testing.T) {
	addr := newFakeSocks5Proxy(t, "u", "p")

	res, err := probeProxy(context.Background(), "http://"+addr+":u:p", "http://probe.test/cdn-cgi/trace", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ok || res.SuggestedProxy != "" {
		t.Fatalf("explicit scheme must not be switched, got %+v", res)
	}
	if !strings.Contains(res.Message, "socks5://") {
		t.Fatalf("message should hint socks5://, got %q", res.Message)
	}
}

// 未写协议且是 HTTP 代理：直接按 http 成功，不给建议。
func TestProbeProxySchemelessHttpNoSuggestion(t *testing.T) {
	srv := newFakeAuthProxy(t, "u", "p")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	res, err := probeProxy(context.Background(), host+":"+port+":u:p", "http://probe.test/cdn-cgi/trace", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok || res.SuggestedProxy != "" || !strings.HasPrefix(res.Proxy, "http://") {
		t.Fatalf("http proxy should succeed without suggestion, got %+v", res)
	}
}

// 未写协议且两种协议都不通：错误信息同时给出 HTTP 与 SOCKS5 的失败原因。
func TestProbeProxySchemelessBothFail(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	res, err := probeProxy(context.Background(), addr+":u:p", "http://probe.test/cdn-cgi/trace", 3*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ok || !strings.Contains(res.Message, "HTTP") || !strings.Contains(res.Message, "SOCKS5") {
		t.Fatalf("want failure mentioning both protocols, got %+v", res)
	}
}

// 生产探测目标是 https（走 CONNECT）。HTTP 代理拒绝 CONNECT（如 407）已能断定是 HTTP 代理，
// 必须立即返回，不能等并行的 SOCKS5 探测挂到超时。
func TestProbeProxySchemelessConnectRejectedReturnsFast(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	start := time.Now()
	res, err := probeProxy(context.Background(), host+":"+port+":u:wrong", "https://probe.test/cdn-cgi/trace", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("should return as soon as the HTTP proxy rejects CONNECT, took %v", elapsed)
	}
	if res.Ok || !strings.Contains(res.Message, "407") || strings.Contains(res.Message, "SOCKS5") {
		t.Fatalf("want HTTP 407 failure only, got %+v", res)
	}
}
