package ai

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrAPIBaseNotAllowed 表示模型接口地址不被允许（SSRF 防护）。
var ErrAPIBaseNotAllowed = errors.New("模型接口地址不被允许")

// ValidateAPIBase 校验模型 api_base（安全：SSRF 收敛）。
//
// 为什么必须校验：api_base 是使用者可改的字段，服务端会拿着**已入库的真实 API Key**
// 向这个地址发请求。若不收敛，攻击者把 api_base 指向内网就是端口扫描器 /
// 元数据服务读取器，指向自己的服务器则是"一条调用链把受害者的 Key 发出去"。
//
// 规则：
//  1. 只允许 http / https；
//  2. 解析主机名后逐个地址检查，拒绝环回 / 私网 / 链路本地 / 未指定 / 组播 / 保留地址；
//  3. allowPrivate=true 时只跳过第 2 条（本地冒烟要连 127.0.0.1 的假模型服务，需显式开启）。
func ValidateAPIBase(raw string, allowPrivate bool) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("%w：地址为空", ErrAPIBaseNotAllowed)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("%w：%s", ErrAPIBaseNotAllowed, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w：只允许 http/https，收到 %q", ErrAPIBaseNotAllowed, parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%w：缺少主机名", ErrAPIBaseNotAllowed)
	}
	if allowPrivate {
		return nil
	}

	// 字面量 IP 先按字面判，避免依赖 DNS
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w：%s 属于内网/保留地址", ErrAPIBaseNotAllowed, host)
		}
		return nil
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("%w：无法解析主机名 %s（%s）", ErrAPIBaseNotAllowed, host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("%w：主机名 %s 没有解析结果", ErrAPIBaseNotAllowed, host)
	}
	for _, ip := range addrs {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w：%s 解析到内网/保留地址 %s", ErrAPIBaseNotAllowed, host, ip)
		}
	}
	return nil
}

// isBlockedIP 判断地址是否属于"不该让服务端主动去连"的范围。
func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 100.64.0.0/10（CGNAT，含 Tailscale 等内网覆盖网）、192.0.0.0/24
		//
		// 刻意**不拦** 198.18.0.0/15：它是代理/加速软件常用的 fake-IP 段
		// （本机实测 api.openai.com 就解析到 198.19.x），拦了会误伤正常公网域名，
		// 而它本身路由不到真正的内网服务，安全收益接近零。
		switch {
		case ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127:
			return true
		case ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0:
			return true
		}
	}
	// IPv6 唯一本地地址 fc00::/7、以及 IPv4 映射地址按 IPv4 判定
	if len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc {
		return true
	}
	return false
}
