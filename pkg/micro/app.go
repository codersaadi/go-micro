package micro

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// CORSConfig represents configuration for CORS middleware
type CORSConfig struct {
	Enabled          bool     `envconfig:"CORS_ENABLED" default:"true"`
	AllowedOrigins   []string `envconfig:"CORS_ALLOWED_ORIGINS" default:"*"`
	AllowedMethods   []string `envconfig:"CORS_ALLOWED_METHODS" default:"GET,POST,PUT,DELETE,OPTIONS,HEAD"`
	AllowedHeaders   []string `envconfig:"CORS_ALLOWED_HEADERS" default:"Content-Type,Authorization,X-Requested-With"`
	ExposedHeaders   []string `envconfig:"CORS_EXPOSED_HEADERS" default:""`
	AllowCredentials bool     `envconfig:"CORS_ALLOW_CREDENTIALS" default:"false"`
	MaxAge           int      `envconfig:"CORS_MAX_AGE" default:"300"` // In seconds
}

// Update the App struct to include the rate limiter
type App struct {
	Config *Config
	Router *mux.Router
	Logger Logger

	Validator    *validator.Validate
	middleware   []mux.MiddlewareFunc
	server       *http.Server
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	healthChecks map[string]HealthCheck
	rateLimiter  *rateLimiter // Add this field
}

// Update Config struct to include the new CORS config
type Config struct {
	AppName         string        `envconfig:"APP_NAME" default:"micro-service"`
	Port            int           `envconfig:"PORT" default:"8080" validate:"required,min=1,max=65535"`
	LogLevel        string        `envconfig:"LOG_LEVEL" default:"info" validate:"oneof=debug info warn error"`
	DBDSN           string        `envconfig:"DB_DSN" required:"true"`
	ReadTimeout     time.Duration `envconfig:"READ_TIMEOUT" default:"5s"`
	WriteTimeout    time.Duration `envconfig:"WRITE_TIMEOUT" default:"10s"`
	MetricsEnabled  bool          `envconfig:"METRICS_ENABLED" default:"true"`
	HandlerTimeout  time.Duration `envconfig:"HANDLER_TIMEOUT" default:"30s"`
	CertFile        string        `envconfig:"CERT_FILE"`
	KeyFile         string        `envconfig:"KEY_FILE"`
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"10s"`
	RateLimiter     RateLimiterConfig
	CORS            CORSConfig // New detailed CORS configuration
}

// Handler is a function that processes requests with context
type Handler func(ctx context.Context, w http.ResponseWriter, r *http.Request) error

// HealthCheck represents a health check function with metadata
type HealthCheck struct {
	Name        string
	Description string
	Check       func(context.Context) error
}

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpDuration)
}

// Update NewApp to initialize the rate limiter
func NewApp(config *Config) (*App, error) {
	if config == nil {
		config = &Config{}
		if err := envconfig.Process("", config); err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	validate := validator.New()
	if err := validate.Struct(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	logger, err := NewLogger(config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		Config:       config,
		Router:       mux.NewRouter(),
		Logger:       logger,
		Validator:    validate,
		ctx:          ctx,
		cancel:       cancel,
		healthChecks: make(map[string]HealthCheck),
	}

	// Initialize rate limiter
	if app.Config.RateLimiter.Enabled {
		app.rateLimiter = newRateLimiter(app.Config.RateLimiter)
	}

	app.setupDefaultMiddleware()
	app.registerSystemEndpoints()

	return app, nil
}

// Update setupDefaultMiddleware to use the new CORS config
func (a *App) setupDefaultMiddleware() {
	a.Use(a.requestIDMiddleware)
	a.Use(a.securityHeadersMiddleware)

	if a.Config.RateLimiter.Enabled {
		a.Use(a.rateLimiterMiddleware)
	}

	if a.Config.MetricsEnabled {
		a.Use(a.metricsMiddleware)
	}

	a.Use(a.logMiddleware)
	a.Use(a.recoveryMiddleware)
	a.Use(a.timeoutMiddleware(a.Config.HandlerTimeout))

	// Enhanced CORS configuration
	if a.Config.CORS.Enabled {
		corsOptions := []handlers.CORSOption{}

		// Configure allowed origins
		if len(a.Config.CORS.AllowedOrigins) > 0 {
			corsOptions = append(corsOptions, handlers.AllowedOrigins(a.Config.CORS.AllowedOrigins))
		}

		// Configure allowed methods
		if len(a.Config.CORS.AllowedMethods) > 0 {
			corsOptions = append(corsOptions, handlers.AllowedMethods(a.Config.CORS.AllowedMethods))
		}

		// Configure allowed headers
		if len(a.Config.CORS.AllowedHeaders) > 0 {
			corsOptions = append(corsOptions, handlers.AllowedHeaders(a.Config.CORS.AllowedHeaders))
		}

		// Configure exposed headers
		if len(a.Config.CORS.ExposedHeaders) > 0 {
			corsOptions = append(corsOptions, handlers.ExposedHeaders(a.Config.CORS.ExposedHeaders))
		}

		// Configure credentials
		if a.Config.CORS.AllowCredentials {
			corsOptions = append(corsOptions, handlers.AllowCredentials())
		}

		// Configure max age
		if a.Config.CORS.MaxAge > 0 {
			corsOptions = append(corsOptions, handlers.MaxAge(a.Config.CORS.MaxAge))
		}

		a.Router.Use(handlers.CORS(corsOptions...))
	}
}
func (a *App) registerSystemEndpoints() {
	if a.Config.MetricsEnabled {
		a.Router.Handle("/metrics", promhttp.Handler())
	}

	a.Router.HandleFunc("/health", a.healthHandler)
}

// Start starts the application server
func (a *App) Start() error {
	a.applyMiddleware()

	a.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", a.Config.Port),
		Handler:      a.Router,
		ReadTimeout:  a.Config.ReadTimeout,
		WriteTimeout: a.Config.WriteTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		a.Logger.Info("server starting", zap.String("addr", a.server.Addr))

		var err error
		if a.Config.CertFile != "" && a.Config.KeyFile != "" {
			err = a.server.ListenAndServeTLS(a.Config.CertFile, a.Config.KeyFile)
		} else {
			err = a.server.ListenAndServe()
		}

		serverErrors <- err
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case <-shutdown:
		a.Logger.Info("server shutdown initiated")
		return a.gracefulShutdown()
	}
}

func (a *App) applyMiddleware() {
	for _, m := range a.middleware {
		a.Router.Use(m)
	}
}

// Update gracefulShutdown to clean up the rate limiter
func (a *App) gracefulShutdown() error {
	a.cancel()

	// Stop the rate limiter's cleanup goroutine
	if a.rateLimiter != nil {
		a.rateLimiter.stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.Config.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		a.Logger.Error("graceful shutdown failed", zap.Error(err))

		if closeErr := a.server.Close(); closeErr != nil {
			return fmt.Errorf("forced shutdown error: %w", closeErr)
		}
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	a.wg.Wait()
	a.Logger.Info("server shutdown complete")
	return nil
}

// Parameter handling functions
func (a *App) URLParam(r *http.Request, name string) string {
	return mux.Vars(r)[name]
}

func (a *App) URLParamInt(r *http.Request, name string) (int, error) {
	val := a.URLParam(r, name)
	result, err := strconv.Atoi(val)
	if err != nil {
		return 0, NewAPIError(http.StatusBadRequest, "invalid path parameter", map[string]string{
			"parameter": name,
			"value":     val,
		})
	}
	return result, nil
}

func (a *App) QueryParam(r *http.Request, name string) string {
	return r.URL.Query().Get(name)
}

func (a *App) QueryParamInt(r *http.Request, name string) (int, error) {
	val := a.QueryParam(r, name)
	result, err := strconv.Atoi(val)
	if err != nil {
		return 0, NewAPIError(http.StatusBadRequest, "invalid query parameter", map[string]string{
			"parameter": name,
			"value":     val,
		})
	}
	return result, nil
}

func (a *App) QueryParams(r *http.Request) url.Values {
	return r.URL.Query()
}

// JSON response helpers
func (a *App) JSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func (a *App) JSONError(w http.ResponseWriter, err error) {
	a.handleError(w, err)
}

// Decode request body with validation
func (a *App) Decode(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return NewAPIError(http.StatusBadRequest, "invalid request body")
	}
	defer r.Body.Close()

	if err := a.Validator.Struct(v); err != nil {
		validationErrors := make(map[string]string)
		if ve, ok := err.(validator.ValidationErrors); ok {
			for _, fe := range ve {
				validationErrors[fe.Field()] = fe.Tag()
			}
		}

		apiError := NewAPIError(http.StatusBadRequest, "validation failed")
		if a.Config.LogLevel == "debug" {
			apiError.Details = validationErrors
		}
		return apiError
	}

	return nil
}

func getRequestIDFromContext(w http.ResponseWriter) string {
	if ctx := w.(*loggingResponseWriter).context; ctx != nil {
		if reqID, ok := ctx.Value("request_id").(string); ok {
			return reqID
		}
	}
	return ""
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Use adds middleware to the application
func (a *App) Use(middleware mux.MiddlewareFunc) {
	a.middleware = append(a.middleware, middleware)
}

// HTTP method shortcuts
func (a *App) GET(path string, handler Handler)    { a.Handle(http.MethodGet, path, handler) }
func (a *App) POST(path string, handler Handler)   { a.Handle(http.MethodPost, path, handler) }
func (a *App) PUT(path string, handler Handler)    { a.Handle(http.MethodPut, path, handler) }
func (a *App) DELETE(path string, handler Handler) { a.Handle(http.MethodDelete, path, handler) }

func (a *App) Handle(method, path string, handler Handler) {
	a.Router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := handler(ctx, w, r); err != nil {
			a.handleError(w, err)
		}
	}).Methods(method)
}

// RouterGroup represents a group of routes with shared prefix and middleware
type RouterGroup struct {
	prefix     string
	middleware []mux.MiddlewareFunc
	app        *App
	router     *mux.Router
}

// Group creates a new router group with the given prefix
func (a *App) Group(prefix string) *RouterGroup {
	subRouter := a.Router.PathPrefix(prefix).Subrouter()
	return &RouterGroup{
		prefix:     prefix,
		middleware: []mux.MiddlewareFunc{},
		app:        a,
		router:     subRouter,
	}
}

// WithMiddleware adds middleware to the router group
func (g *RouterGroup) WithMiddleware(middleware mux.MiddlewareFunc) *RouterGroup {
	g.middleware = append(g.middleware, middleware)
	g.router.Use(middleware)
	return g
}

// Group creates a nested group
func (g *RouterGroup) Group(prefix string) *RouterGroup {
	subRouter := g.router.PathPrefix(prefix).Subrouter()

	// Apply parent middlewares to subgroup
	for _, m := range g.middleware {
		subRouter.Use(m)
	}

	return &RouterGroup{
		prefix:     g.prefix + prefix,
		middleware: g.middleware,
		app:        g.app,
		router:     subRouter,
	}
}

// GET adds a GET route to the group
func (g *RouterGroup) GET(path string, handler Handler) *RouterGroup {
	g.HandleMethod(http.MethodGet, path, handler)
	return g
}

// POST adds a POST route to the group
func (g *RouterGroup) POST(path string, handler Handler) *RouterGroup {
	g.HandleMethod(http.MethodPost, path, handler)
	return g
}

// PUT adds a PUT route to the group
func (g *RouterGroup) PUT(path string, handler Handler) *RouterGroup {
	g.HandleMethod(http.MethodPut, path, handler)
	return g
}

// DELETE adds a DELETE route to the group
func (g *RouterGroup) DELETE(path string, handler Handler) *RouterGroup {
	g.HandleMethod(http.MethodDelete, path, handler)
	return g
}

// HandleMethod adds a route with the specified method to the group
// Using a different name than Handle to avoid conflicts with App.Handle
func (g *RouterGroup) HandleMethod(method, path string, handler Handler) *RouterGroup {
	g.router.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := handler(ctx, w, r); err != nil {
			g.app.handleError(w, err)
		}
	}).Methods(method)
	return g
}
