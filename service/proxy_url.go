package service

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

var errUnrecognizedProxyFormat = errors.New("unrecognized proxy format, expected scheme://[user:pass@]host:port, host:port, host:port:user:pass or user:pass@host:port")

func isSupportedProxyScheme(scheme string) bool {
	switch scheme {
	case "http", "https", "socks5", "socks5h":
		return true
	}
	return false
}

func isValidProxyPort(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}

// isValidProxyHost 校验主机名：IPv6 字面量放行，其余不得含冒号、空白或 URL 分隔符。
func isValidProxyHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.Contains(host, ":") {
		return net.ParseIP(host) != nil
	}
	return !strings.ContainsAny(host, " \t\r\n/@?#%[]")
}

// looksLikeProxyHost 用于消歧 host:port:user:pass 与 user:pass:host:port 两端都像端口的情况。
func looksLikeProxyHost(s string) bool {
	return strings.Contains(s, ".") || s == "localhost" || net.ParseIP(s) != nil
}

func buildProxyURL(scheme, host, port, user, pass string) (string, error) {
	if !isValidProxyHost(host) || !isValidProxyPort(port) {
		return "", errUnrecognizedProxyFormat
	}
	if user == "" && pass != "" {
		return "", errUnrecognizedProxyFormat
	}
	u := &url.URL{Scheme: scheme, Host: net.JoinHostPort(host, port)}
	if user != "" {
		if pass != "" {
			u.User = url.UserPassword(user, pass)
		} else {
			u.User = url.User(user)
		}
	}
	return u.String(), nil
}

// parseColonProxy 解析冒号分隔格式：host:port、host:port:user:pass、user:pass:host:port。
func parseColonProxy(scheme, rest string) (string, error) {
	if strings.HasPrefix(rest, "[") {
		host, port, err := net.SplitHostPort(rest)
		if err != nil {
			return "", errUnrecognizedProxyFormat
		}
		return buildProxyURL(scheme, host, port, "", "")
	}
	parts := strings.Split(rest, ":")
	switch {
	case len(parts) == 2:
		return buildProxyURL(scheme, parts[0], parts[1], "", "")
	case len(parts) >= 4:
		hostFirst := isValidProxyPort(parts[1])
		userFirst := len(parts) == 4 && isValidProxyPort(parts[3])
		if hostFirst && userFirst && !looksLikeProxyHost(parts[0]) && looksLikeProxyHost(parts[2]) {
			hostFirst = false
		}
		if hostFirst {
			// 密码允许含冒号：第 4 段及以后全部视为密码
			return buildProxyURL(scheme, parts[0], parts[1], parts[2], strings.Join(parts[3:], ":"))
		}
		if userFirst {
			return buildProxyURL(scheme, parts[2], parts[3], parts[0], parts[1])
		}
	}
	return "", errUnrecognizedProxyFormat
}

// parseUserinfoProxy 解析 user:pass@host:port；以最后一个 @ 分割，密码允许含 @。
func parseUserinfoProxy(scheme, rest string) (string, error) {
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return "", errUnrecognizedProxyFormat
	}
	host, port, err := net.SplitHostPort(rest[at+1:])
	if err != nil {
		return "", errUnrecognizedProxyFormat
	}
	user, pass, _ := strings.Cut(rest[:at], ":")
	return buildProxyURL(scheme, host, port, user, pass)
}

// NormalizeProxyURL 把多种常见代理写法规范化为 scheme://[user:pass@]host:port。
//
// 支持：
//   - 标准 URL：http/https/socks5/socks5h://[user:pass@]host[:port]（合法时原样返回，保持向后兼容）
//   - host:port
//   - host:port:user:pass（代理商常见导出格式）
//   - user:pass:host:port
//   - user:pass@host:port
//
// 以上非标准写法均可带协议前缀（如 socks5://host:port:user:pass），未写协议默认 http。
// 空串（含纯空白）返回 ""，表示不使用代理。
func NormalizeProxyURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}

	scheme := "http"
	rest := s
	hasScheme := false
	if idx := strings.Index(s, "://"); idx >= 0 {
		scheme = strings.ToLower(s[:idx])
		rest = s[idx+len("://"):]
		hasScheme = true
	}
	if !isSupportedProxyScheme(scheme) {
		return "", fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", scheme)
	}

	if hasScheme {
		if u, err := url.Parse(s); err == nil && isValidProxyHost(u.Hostname()) &&
			(u.Port() == "" || isValidProxyPort(u.Port())) {
			return s, nil
		}
	}

	rest = strings.TrimSuffix(rest, "/")
	if rest == "" {
		return "", errUnrecognizedProxyFormat
	}
	// 冒号格式优先：避免 host:port:user:p@ss 这类密码含 @ 的写法被误判为 userinfo 格式
	if normalized, err := parseColonProxy(scheme, rest); err == nil {
		return normalized, nil
	}
	return parseUserinfoProxy(scheme, rest)
}
