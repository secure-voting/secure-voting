package middleware

import (
	"net"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/httpserver/httputil"
)

func RequireTrustedCIDRs(cidrs []string, next http.Handler) http.Handler {
	cidrs = normalizeCIDRs(cidrs)
	if len(cidrs) == 0 {
		return next
	}

	nets := mustParseCIDRs(cidrs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ipStr := clientIP(r)
		if ipStr == "" {
			httputil.WriteError(w, http.StatusForbidden, "forbidden_network", "request is not from a trusted network")
			return
		}

		ip := net.ParseIP(strings.TrimSpace(ipStr))
		if ip == nil {
			httputil.WriteError(w, http.StatusForbidden, "forbidden_network", "request is not from a trusted network")
			return
		}

		for _, n := range nets {
			if n.Contains(ip) {
				next.ServeHTTP(w, r)
				return
			}
		}

		httputil.WriteError(w, http.StatusForbidden, "forbidden_network", "request is not from a trusted network")
	})
}

func normalizeCIDRs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func mustParseCIDRs(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		_, netw, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err != nil {
			panic("invalid ADMIN_TRUSTED_CIDRS entry: " + raw)
		}
		out = append(out, netw)
	}
	return out
}
