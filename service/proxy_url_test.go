package service

import (
	"net/url"
	"testing"
)

func TestNormalizeProxyURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"blank", "   ", ""},

		// 已是标准 URL：原样保留（向后兼容已存的渠道配置，代理客户端缓存 key 不变）
		{"http url with auth unchanged", "http://user:pass@1.2.3.4:8080", "http://user:pass@1.2.3.4:8080"},
		{"socks5 url unchanged", "socks5://user:pass@proxy.example.com:1080", "socks5://user:pass@proxy.example.com:1080"},
		{"socks5h url unchanged", "socks5h://proxy.example.com:1080", "socks5h://proxy.example.com:1080"},
		{"https url without port unchanged", "https://proxy.example.com", "https://proxy.example.com"},
		{"surrounding spaces trimmed", "  http://1.2.3.4:8080  ", "http://1.2.3.4:8080"},

		// 无协议：默认 http
		{"host:port", "pw164.datte.org:23031", "http://pw164.datte.org:23031"},
		{"host:port:user:pass", "pw164.datte.org:23031:me8bsitx:z6ysax5f", "http://me8bsitx:z6ysax5f@pw164.datte.org:23031"},
		{"user:pass@host:port", "me8bsitx:z6ysax5f@pw164.datte.org:23031", "http://me8bsitx:z6ysax5f@pw164.datte.org:23031"},
		{"user:pass:host:port", "me8bsitx:z6ysax5f:pw164.datte.org:23031", "http://me8bsitx:z6ysax5f@pw164.datte.org:23031"},
		{"ipv4 host:port:user:pass", "1.2.3.4:8080:user:pass", "http://user:pass@1.2.3.4:8080"},
		{"ipv6 host:port", "[2001:db8::1]:1080", "http://[2001:db8::1]:1080"},

		// 带协议的冒号分隔格式（常见于代理商导出）
		{"socks5 host:port:user:pass", "socks5://pw164.datte.org:23031:me8bsitx:z6ysax5f", "socks5://me8bsitx:z6ysax5f@pw164.datte.org:23031"},
		{"uppercase scheme", "SOCKS5H://pw164.datte.org:23031:u:p", "socks5h://u:p@pw164.datte.org:23031"},
		{"trailing slash", "http://pw164.datte.org:23031:u:p/", "http://u:p@pw164.datte.org:23031"},

		// 密码含特殊字符：正确转义，解析后能还原
		{"password with colon", "1.2.3.4:8080:user:pa:ss", "http://user:pa%3Ass@1.2.3.4:8080"},
		{"password with at", "1.2.3.4:8080:user:p@ss", "http://user:p%40ss@1.2.3.4:8080"},
		{"userinfo password with at", "user:p@ss@1.2.3.4:8080", "http://user:p%40ss@1.2.3.4:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeProxyURL(tc.in)
			if err != nil {
				t.Fatalf("NormalizeProxyURL(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeProxyURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeProxyURLPasswordRoundTrip(t *testing.T) {
	got, err := NormalizeProxyURL("1.2.3.4:8080:user:p@ss:w/rd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("normalized url %q must parse: %v", got, err)
	}
	if u.User.Username() != "user" {
		t.Fatalf("username = %q, want user", u.User.Username())
	}
	if pass, _ := u.User.Password(); pass != "p@ss:w/rd" {
		t.Fatalf("password = %q, want p@ss:w/rd", pass)
	}
	if u.Host != "1.2.3.4:8080" {
		t.Fatalf("host = %q, want 1.2.3.4:8080", u.Host)
	}
}

func TestNormalizeProxyURLRejectsInvalid(t *testing.T) {
	cases := []string{
		"ftp://1.2.3.4:21",         // 不支持的协议
		"justahost",                // 缺端口
		"1.2.3.4:notaport",         // 端口非数字
		"1.2.3.4:0",                // 端口越界
		"1.2.3.4:65536",            // 端口越界
		"1.2.3.4:8080:user",        // 三段无法判定
		"a:b:c:d",                  // 四段但两端都不是端口
		"socks5://",                // 只有协议
		"http://bad host:8080:u:p", // 主机含空格
		"user:pass@1.2.3.4",        // userinfo 形式缺端口
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if got, err := NormalizeProxyURL(in); err == nil {
				t.Fatalf("NormalizeProxyURL(%q) = %q, want error", in, got)
			}
		})
	}
}

// 回归：NewProxyHttpClient 必须接受新格式，并与等价标准 URL 共用同一个缓存客户端。
func TestNewProxyHttpClientAcceptsColonFormat(t *testing.T) {
	ResetProxyClientCache()
	t.Cleanup(ResetProxyClientCache)

	a, err := NewProxyHttpClient("pw164.datte.org:23031:me8bsitx:z6ysax5f")
	if err != nil {
		t.Fatalf("colon format must be accepted: %v", err)
	}
	b, err := NewProxyHttpClient("http://me8bsitx:z6ysax5f@pw164.datte.org:23031")
	if err != nil {
		t.Fatalf("standard url must be accepted: %v", err)
	}
	if a != b {
		t.Fatalf("equivalent proxy formats should share one cached client")
	}

	if _, err := NewProxyHttpClient("socks5://pw164.datte.org:23031:u:p"); err != nil {
		t.Fatalf("socks5 colon format must be accepted: %v", err)
	}
	if _, err := NewProxyHttpClient("ftp://1.2.3.4:21"); err == nil {
		t.Fatalf("unsupported scheme must still be rejected")
	}
}
