package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// proxyProbeURL 返回纯文本 key=value，含出口 ip 与国家码 loc，无需鉴权、无频率限制
	proxyProbeURL           = "https://www.cloudflare.com/cdn-cgi/trace"
	proxyProbeTimeout       = 10 * time.Second
	proxyProbeBodyMaxBytes  = 4096
	proxyProbeMessageMaxLen = 200
)

// ProxyProbeResult 代理连通性测试结果。Proxy 为规范化后的地址（密码已打码）。
type ProxyProbeResult struct {
	Ok         bool   `json:"ok"`
	Proxy      string `json:"proxy"`
	StatusCode int    `json:"status_code"`
	LatencyMs  int64  `json:"latency_ms"`
	ExitIP     string `json:"exit_ip"`
	Country    string `json:"country"`
	Message    string `json:"message"`
	// SuggestedProxy 未写协议、按 http 不通但按 socks5 可用时，建议改写成的带协议地址（保持用户原写法，仅补前缀）
	SuggestedProxy string `json:"suggested_proxy,omitempty"`

	// httpReply 代理以 HTTP 协议作出了应答（含 CONNECT 被拒），据此可断定是 HTTP 代理
	httpReply bool
}

// ProbeProxy 通过代理访问探测地址，验证连通性并解析出口 IP / 国家。
// 返回 error 仅表示代理地址为空或格式无法识别；连通失败体现在 result.Ok=false。
func ProbeProxy(ctx context.Context, rawProxy string) (*ProxyProbeResult, error) {
	return probeProxy(ctx, rawProxy, proxyProbeURL, proxyProbeTimeout)
}

// probeProxy 未写协议时并行按 http 与 socks5 探测（http 优先），
// 代理商 host:port:user:pass 写法常对应 SOCKS5 端口，按默认 http 会连不通。
// 写明协议时只按该协议探测，不擅自切换，仅在疑似连到 SOCKS 端口时给出提示。
func probeProxy(ctx context.Context, rawProxy, targetURL string, timeout time.Duration) (*ProxyProbeResult, error) {
	trimmed := strings.TrimSpace(rawProxy)
	normalized, err := NormalizeProxyURL(trimmed)
	if err != nil {
		return nil, err
	}
	if normalized == "" {
		return nil, errors.New("proxy is empty")
	}

	if strings.Contains(trimmed, "://") {
		result := probeNormalizedProxy(ctx, normalized, targetURL, timeout)
		if !result.Ok && looksLikeSocksReply(result.Message) {
			result.Message += "（该端口疑似 SOCKS 代理，可改用 socks5:// 前缀）"
		}
		return result, nil
	}

	socksRaw := "socks5://" + trimmed
	socksNormalized, err := NormalizeProxyURL(socksRaw)
	if err != nil {
		return probeNormalizedProxy(ctx, normalized, targetURL, timeout), nil
	}

	// 两路并行；任一路成功、或 HTTP 代理已给出明确应答，立即返回并取消另一路
	// （SOCKS 握手打到 HTTP 代理上通常会挂到超时，不能干等）
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type outcome struct {
		socks  bool
		result *ProxyProbeResult
	}
	outcomes := make(chan outcome, 2)
	go func() { outcomes <- outcome{false, probeNormalizedProxy(ctx, normalized, targetURL, timeout)} }()
	go func() { outcomes <- outcome{true, probeNormalizedProxy(ctx, socksNormalized, targetURL, timeout)} }()

	var httpResult, socksResult *ProxyProbeResult
	for i := 0; i < 2; i++ {
		o := <-outcomes
		if o.result.Ok {
			if o.socks {
				o.result.SuggestedProxy = socksRaw
			}
			return o.result, nil
		}
		if o.socks {
			socksResult = o.result
			continue
		}
		httpResult = o.result
		if httpResult.httpReply {
			return httpResult, nil
		}
	}
	httpResult.Message = fmt.Sprintf("HTTP：%s；SOCKS5：%s", httpResult.Message, socksResult.Message)
	return httpResult, nil
}

// probeNormalizedProxy 经规范化后的代理访问探测地址一次。
func probeNormalizedProxy(ctx context.Context, normalized, targetURL string, timeout time.Duration) *ProxyProbeResult {
	result := &ProxyProbeResult{}
	parsed, err := url.Parse(normalized)
	if err != nil {
		result.Message = "代理地址解析失败"
		return result
	}
	result.Proxy = parsed.Redacted()

	// 独立 Transport、不进缓存：测试任意地址不会污染 relay 用的代理客户端缓存
	transport, err := newProxyTransport(parsed)
	if err != nil {
		result.Message = summarizeProxyProbeError(err, parsed, timeout)
		return result
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: checkRedirect}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		result.Message = summarizeProxyProbeError(err, parsed, timeout)
		return result
	}

	start := time.Now()
	resp, err := client.Do(req)
	result.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		result.httpReply = isHTTPConnectRejection(err)
		result.Message = summarizeProxyProbeError(err, parsed, timeout)
		return result
	}
	defer resp.Body.Close()
	result.httpReply = true
	result.StatusCode = resp.StatusCode
	body, _ := io.ReadAll(io.LimitReader(resp.Body, proxyProbeBodyMaxBytes))

	switch {
	case resp.StatusCode == http.StatusProxyAuthRequired:
		result.Message = "代理认证失败（407），请检查用户名和密码"
	case resp.StatusCode >= http.StatusInternalServerError:
		// 正向代理连不上目标时通常回 502/503/504
		result.Message = fmt.Sprintf("代理返回 HTTP %d，无法访问外网", resp.StatusCode)
	default:
		result.Ok = true
		if resp.StatusCode == http.StatusOK {
			result.ExitIP, result.Country = parseCloudflareTrace(body)
		}
	}
	return result
}

// isHTTPConnectRejection HTTP 代理拒绝 CONNECT 时，Go 以状态文本（如 "Proxy Authentication Required"、"Forbidden"）作为错误返回。
func isHTTPConnectRejection(err error) bool {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		err = urlErr.Err
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return false
	}
	msg := err.Error()
	for code := 400; code < 600; code++ {
		if text := http.StatusText(code); text != "" && text == msg {
			return true
		}
	}
	return false
}

// looksLikeSocksReply 按 HTTP 协议连到 SOCKS 端口时，Go 报 malformed HTTP response "\x00..."（SOCKS4 回包）或 "\x05..."（SOCKS5）。
func looksLikeSocksReply(msg string) bool {
	return strings.Contains(msg, `malformed HTTP response "\x0`)
}

// parseCloudflareTrace 从 /cdn-cgi/trace 响应体中取出 ip 与 loc。
func parseCloudflareTrace(body []byte) (ip, country string) {
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if !ok {
			continue
		}
		switch key {
		case "ip":
			ip = value
		case "loc":
			country = value
		}
	}
	return ip, country
}

// summarizeProxyProbeError 把网络错误压成单行、限长、不含代理密码的提示。
func summarizeProxyProbeError(err error, proxyURL *url.URL, timeout time.Duration) string {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Sprintf("连接超时（%ds）", int(timeout.Seconds()))
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		err = urlErr.Err // 去掉 `Get "https://...": ` 前缀
	}
	msg := strings.ReplaceAll(strings.TrimSpace(err.Error()), "\n", " ")
	if strings.Contains(msg, "Proxy Authentication Required") {
		return "代理认证失败（407），请检查用户名和密码"
	}
	if proxyURL.User != nil {
		if pass, ok := proxyURL.User.Password(); ok && len(pass) >= 4 {
			msg = strings.ReplaceAll(msg, pass, "***")
		}
	}
	if len(msg) > proxyProbeMessageMaxLen {
		msg = msg[:proxyProbeMessageMaxLen] + "..."
	}
	return msg
}
