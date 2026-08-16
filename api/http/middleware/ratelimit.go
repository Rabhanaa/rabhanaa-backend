package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Public browsing made these the only unauthenticated endpoints that read
// business data, and the project has no rate limiting anywhere, so this is it.
//
// Deliberately in-process: a shared limiter needs Redis, and the cron already
// assumes a single API instance. It resets on deploy and would not coordinate
// across replicas — enough to stop a careless scraper, not a distributed one.
//
// The limit is deliberately generous. Egyptian mobile carriers put large
// numbers of subscribers behind one CGNAT address, so an IP is a poor proxy for
// a person: blocking real customers costs far more than a scraper collecting
// data the client has decided to publish anyway.
const (
	defaultPublicRatePerMinute = 300
	publicRateWindow           = time.Minute
	bucketIdleTTL              = 10 * time.Minute
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type ipLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func newIPLimiter() *ipLimiter {
	l := &ipLimiter{buckets: make(map[string]*bucket)}
	go l.reapIdle()
	return l
}

// allow spends one token, refilling continuously rather than on a fixed window
// boundary so a caller cannot burst twice by straddling one.
func (l *ipLimiter) allow(ip string, burst float64) bool {
	now := time.Now()
	refillPerSecond := burst / publicRateWindow.Seconds()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[ip]
	if !ok {
		l.buckets[ip] = &bucket{tokens: burst - 1, lastSeen: now}
		return true
	}

	b.tokens += now.Sub(b.lastSeen).Seconds() * refillPerSecond
	if b.tokens > burst {
		b.tokens = burst
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// reapIdle keeps the map from growing without bound as addresses come and go.
func (l *ipLimiter) reapIdle() {
	for range time.Tick(bucketIdleTTL) {
		cutoff := time.Now().Add(-bucketIdleTTL)
		l.mu.Lock()
		for ip, b := range l.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(l.buckets, ip)
			}
		}
		l.mu.Unlock()
	}
}

// PublicRateLimit throttles anonymous reads per client IP.
//
// Must be registered AFTER OptionalAuth: a signed-in caller is identified by
// their token, so throttling them by address is the wrong axis and would punish
// every member sharing a carrier NAT for one noisy neighbour.
func PublicRateLimit(perMinute int) gin.HandlerFunc {
	limiter := newIPLimiter()
	if perMinute <= 0 {
		perMinute = defaultPublicRatePerMinute
	}
	burst := float64(perMinute)

	return func(c *gin.Context) {
		if c.GetInt("userID") != 0 {
			c.Next()
			return
		}

		// ClientIP honours the proxy headers Gin is configured to trust, which
		// matters behind coolify-proxy — otherwise every request would share the
		// proxy's own address and one visitor could throttle everyone.
		if !limiter.allow(c.ClientIP(), burst) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "RATE_LIMITED",
				"message": "عدد كبير من الطلبات — حاول بعد قليل",
			})
			return
		}
		c.Next()
	}
}
