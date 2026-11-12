package middleware

import (
	"context"
	"net"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const ingressDomainHeader = "X-ChatAPI-Ingress-Domain"

// IngressDomain reads the domain set by the reverse proxy that accepted the
// request. Only the immediate peer can vouch for it; client IP headers and the
// client IP compatibility switch have no effect here.
func IngressDomain(trustedProxies []string) gin.HandlerFunc {
	proxies := ingressProxyNetworks(trustedProxies)
	return func(c *gin.Context) {
		req := c.Request
		values := req.Header.Values(ingressDomainHeader)
		if len(values) > 0 {
			// This is internal metadata and must never be forwarded upstream.
			req.Header.Del(ingressDomainHeader)
		}
		if len(values) == 1 && len(proxies) > 0 {
			peer := net.ParseIP(c.RemoteIP())
			for _, proxy := range proxies {
				if proxy.Contains(peer) {
					if domain := normalizeIngressDomain(values[0]); domain != "" {
						c.Request = req.WithContext(context.WithValue(req.Context(), ctxkey.IngressDomain, domain))
					}
					break
				}
			}
		}
		c.Next()
	}
}

func ingressProxyNetworks(proxies []string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(proxies))
	for _, proxy := range proxies {
		if ip := net.ParseIP(proxy); ip != nil {
			if ipv4 := ip.To4(); ipv4 != nil {
				proxy = ipv4.String() + "/32"
			} else {
				proxy = ip.String() + "/128"
			}
		}
		_, network, err := net.ParseCIDR(proxy)
		if err != nil {
			return nil // Match Gin's fail-closed handling of an invalid list.
		}
		networks = append(networks, network)
	}
	return networks
}

func normalizeIngressDomain(value string) string {
	value = strings.TrimSuffix(strings.TrimSpace(value), ".")
	if len(value) == 0 || len(value) > 253 || net.ParseIP(value) != nil {
		return ""
	}
	for label := range strings.SplitSeq(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return ""
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if ch != '-' && !(ch >= 'A' && ch <= 'Z') && !(ch >= 'a' && ch <= 'z') && !(ch >= '0' && ch <= '9') {
				return ""
			}
		}
	}
	return strings.ToLower(value)
}
