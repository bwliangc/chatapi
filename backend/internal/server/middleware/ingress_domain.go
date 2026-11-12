package middleware

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const ingressDomainHeader = "X-ChatAPI-Ingress-Domain"

// IngressDomain prefers the marker from a trusted immediate peer. Otherwise,
// it records the request's own Host only if it matches a configured API domain.
// The Host fallback identifies the requested domain for logs, not a trusted
// peer. The callback returns a read-only snapshot of the configured domains.
func IngressDomain(trustedProxies []string, configuredDomains func() map[string]struct{}) gin.HandlerFunc {
	proxies := ingressProxyNetworks(trustedProxies)
	return func(c *gin.Context) {
		req := c.Request
		values := req.Header.Values(ingressDomainHeader)
		if len(values) > 0 {
			// This is internal metadata and must never be forwarded upstream.
			req.Header.Del(ingressDomainHeader)
		}
		var domain string
		if len(values) == 1 && len(proxies) > 0 {
			peer := net.ParseIP(c.RemoteIP())
			for _, proxy := range proxies {
				if proxy.Contains(peer) {
					domain = normalizeIngressDomain(values[0])
					break
				}
			}
		}
		if domain == "" && configuredDomains != nil {
			if allowed := configuredDomains(); len(allowed) > 0 {
				if host := ingressDomainFromHost(req.Host); host != "" {
					if _, ok := allowed[host]; ok {
						domain = host
					}
				}
			}
		}
		if domain != "" {
			c.Request = req.WithContext(context.WithValue(req.Context(), ctxkey.IngressDomain, domain))
		}
		c.Next()
	}
}

// IngressDomainsFromURLs builds an exact domain allowlist from configured API URLs.
func IngressDomainsFromURLs(endpoints []string) map[string]struct{} {
	domains := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		u, err := url.Parse(strings.TrimSpace(endpoint))
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
			continue
		}
		if domain := ingressDomainFromHost(u.Host); domain != "" {
			domains[domain] = struct{}{}
		}
	}
	return domains
}

func ingressDomainFromHost(host string) string {
	domain, port, hasPort := strings.Cut(host, ":")
	if hasPort {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return ""
		}
	}
	return normalizeIngressDomain(domain)
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
