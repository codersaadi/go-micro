package micro

import (
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimiterConfig represents the configuration for rate limiting
type RateLimiterConfig struct {
	Enabled      bool          `envconfig:"RATE_LIMITER_ENABLED" default:"true"`
	RequestsPerS float64       `envconfig:"RATE_LIMITER_REQUESTS_PER_SECOND" default:"100"`
	Burst        int           `envconfig:"RATE_LIMITER_BURST" default:"50"`
	TTL          time.Duration `envconfig:"RATE_LIMITER_TTL" default:"1h"`
	// Strategy can be "ip", "token" or "global"
	Strategy string `envconfig:"RATE_LIMITER_STRATEGY" default:"ip" validate:"oneof=ip token global"`
}

// rateLimiter handles rate limiting functionality
type rateLimiter struct {
	config   RateLimiterConfig
	limiters map[string]*visitorLimiter
	mu       sync.Mutex
	cleanup  *time.Ticker
}

type visitorLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newRateLimiter creates a new rate limiter instance
func newRateLimiter(config RateLimiterConfig) *rateLimiter {
	rl := &rateLimiter{
		config:   config,
		limiters: make(map[string]*visitorLimiter),
		cleanup:  time.NewTicker(10 * time.Minute),
	}

	// Start cleanup goroutine
	go rl.cleanupStaleVisitors()

	return rl
}

// getLimiter returns a rate limiter for a particular visitor
func (rl *rateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.limiters[key]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(rl.config.RequestsPerS), rl.config.Burst)
		rl.limiters[key] = &visitorLimiter{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}

	// Update the last seen time
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupStaleVisitors removes visitors that haven't been seen for a while
func (rl *rateLimiter) cleanupStaleVisitors() {
	for range rl.cleanup.C {
		rl.mu.Lock()
		for key, v := range rl.limiters {
			if time.Since(v.lastSeen) > rl.config.TTL {
				delete(rl.limiters, key)
			}
		}
		rl.mu.Unlock()
	}
}

// stop stops the cleanup goroutine
func (rl *rateLimiter) stop() {
	rl.cleanup.Stop()
}

// Update the App struct to include the rate limiter
func (app *App) initRateLimiter() {
	// Add the RateLimiterConfig to the main Config struct
	if app.Config.RateLimiter.Enabled {
		app.rateLimiter = newRateLimiter(app.Config.RateLimiter)
		// Register the rate limiting middleware
		app.Use(app.rateLimiterMiddleware)
	}
}

// getClientIdentifier extracts the client identifier based on the strategy
func (a *App) getClientIdentifier(r *http.Request) string {
	switch a.Config.RateLimiter.Strategy {
	case "ip":
		// Extract IP from X-Forwarded-For or RemoteAddr
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
		return ip
	case "token":
		// Use Authorization header token
		return r.Header.Get("Authorization")
	case "global":
		// Global rate limiting uses a constant key
		return "global"
	default:
		// Default to IP-based
		return r.RemoteAddr
	}
}

// rateLimiterMiddleware implements the rate limiting logic
func (a *App) rateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Config.RateLimiter.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get client identifier based on strategy
		clientID := a.getClientIdentifier(r)

		// Skip rate limiting if no valid client identifier
		if clientID == "" && a.Config.RateLimiter.Strategy != "global" {
			next.ServeHTTP(w, r)
			return
		}

		// Get the limiter for this client
		limiter := a.rateLimiter.getLimiter(clientID)

		// Check if this request is allowed
		if !limiter.Allow() {
			requestID := r.Context().Value(contextKeyRequestID).(string)
			a.Logger.Warn("rate limit exceeded",
				zap.String("client_id", clientID),
				zap.String("path", r.URL.Path),
				zap.String("request_id", requestID),
			)

			apiErr := NewAPIError(http.StatusTooManyRequests, "Rate limit exceeded")
			w.Header().Set("Retry-After", "60") // Suggest retry after 60 seconds
			a.JSONError(w, apiErr)
			return
		}

		// Request allowed, proceed to next handler
		next.ServeHTTP(w, r)
	})
}
