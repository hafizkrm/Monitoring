package middleware

import (
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipVisitor tracks the limiter and last seen time
type ipVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter structure
type IPRateLimiter struct {
	ips map[string]*ipVisitor
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

// NewIPRateLimiter creates a new rate limiter
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		ips: make(map[string]*ipVisitor),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}

	// Start cleanup background worker
	go limiter.cleanupVisitors()
	return limiter
}

func (i *IPRateLimiter) cleanupVisitors() {
	for {
		time.Sleep(10 * time.Minute)
		i.mu.Lock()
		for ip, v := range i.ips {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(i.ips, ip)
			}
		}
		i.mu.Unlock()
	}
}

// AddIP adds an IP to the map
func (i *IPRateLimiter) AddIP(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter := rate.NewLimiter(i.r, i.b)
	i.ips[ip] = &ipVisitor{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

// GetLimiter gets the rate limiter for a specific IP
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	v, exists := i.ips[ip]

	if !exists {
		limiter := rate.NewLimiter(i.r, i.b)
		i.ips[ip] = &ipVisitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

var trustedProxies []*net.IPNet

func init() {
	proxies := os.Getenv("TRUSTED_PROXIES")
	if proxies == "" {
		// Default: only trust localhost
		proxies = "127.0.0.1/32,::1/128"
	}
	for _, cidr := range strings.Split(proxies, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			trustedProxies = append(trustedProxies, ipNet)
		}
	}
}

func isTrustedProxy(ip net.IP) bool {
	for _, network := range trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// GetRealIP extracts the real IP from request, trusting X-Forwarded-For only for configured Trusted Proxies
func GetRealIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP != nil && isTrustedProxy(parsedIP) {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			// X-Forwarded-For can contain multiple IPs. The client's original IP is the first one.
			ips := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	return ip
}

// RateLimitMiddleware applies rate limiting per IP address
func RateLimitMiddleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetRealIP(r)

			l := limiter.GetLimiter(ip)
			if !l.Allow() {
				log.Printf("[WARN] Rate limit exceeded for IP: %s, User-Agent: %s, Path: %s", ip, r.UserAgent(), r.URL.Path)
				w.Header().Set("Retry-After", "1")
				http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
