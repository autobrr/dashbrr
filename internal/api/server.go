package api

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/autobrr/dashbrr/internal/api/handlers"
	"github.com/autobrr/dashbrr/internal/api/middleware"
	"github.com/autobrr/dashbrr/internal/config"
	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/sse"
	"github.com/autobrr/dashbrr/internal/types"
	"github.com/autobrr/dashbrr/web"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// defaultOIDCCallbackURL is used when the config does not set a redirect URL.
const defaultOIDCCallbackURL = "http://localhost:3000/api/auth/oidc/callback"

type Server struct {
	cfg        *config.Config
	db         *database.DB
	cache      cache.Store
	hub        *sse.Hub
	poller     *handlers.Poller
	pollerStop context.CancelFunc
	httpServer *http.Server
}

func NewServer(cfg *config.Config, db *database.DB, cache cache.Store) *Server {
	return &Server{
		cfg:   cfg,
		db:    db,
		cache: cache,
		hub:   sse.NewHub(),
	}
}

func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.cfg.Server.ListenAddr)
	if err != nil {
		return err
	}

	log.Info().
		Str("address", listener.Addr().String()).
		Str("mode", gin.Mode()).
		Str("database", s.cfg.Database.Path).
		Msg("Starting server")

	s.httpServer = &http.Server{
		Addr:        s.cfg.Server.ListenAddr,
		Handler:     s.Handler(),
		ReadTimeout: 15 * time.Second,
		// Keep disabled to support long-lived streaming responses (SSE).
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	return s.httpServer.Serve(listener)
}

func (s *Server) Shutdown(ctx context.Context) error {
	//s.cache.Close()
	if s.pollerStop != nil {
		s.pollerStop()
	}
	if s.hub != nil {
		s.hub.Close()
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	ginMode := gin.ReleaseMode
	if os.Getenv("GIN_MODE") == "debug" {
		ginMode = gin.DebugMode
	}
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())

	trustedProxies := []string{"127.0.0.1", "::1"}
	if gin.Mode() == gin.DebugMode {
		trustedProxies = []string{}
	}

	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		log.Error().Err(err).Msg("Failed to set trusted proxies")
	}

	r.Use(middleware.SetupCORS(
		s.cfg.Server.CORSOrigins,
		s.cfg.Server.CORSHeaders,
		s.cfg.Server.CORSMethods,
		time.Duration(s.cfg.Server.CORSMaxAgeH)*time.Hour,
		s.cfg.Server.CORSCreds,
	))

	// Create rate limiters with different configurations
	apiRateLimiter := middleware.NewRateLimiter(s.cache, time.Minute, 60, "api:")       // 60 requests per minute for API
	healthRateLimiter := middleware.NewRateLimiter(s.cache, time.Minute, 30, "health:") // 30 health checks per minute
	authRateLimiter := middleware.NewRateLimiter(s.cache, time.Minute, 30, "auth:")     // 30 auth requests per minute

	bc := handlers.NewBroadcaster(s.hub)
	// Background polling will publish SSE updates.
	if s.poller == nil {
		s.poller = handlers.NewPoller(s.db, bc)
		pctx, cancel := context.WithCancel(context.Background())
		s.pollerStop = cancel
		s.poller.Start(pctx)
	}

	settingsHandler := handlers.NewSettingsHandler(s.db, s.cache, s.poller)
	//serviceHandler := handlers.NewServiceHandler(db, health, store)
	healthHandler := handlers.NewHealthHandler(s.db)
	eventsHandler := handlers.NewEventsHandler(s.hub, bc)
	plexAuthHandler := handlers.NewPlexAuthHandler()
	overseerrHandler := handlers.NewOverseerrHandler(s.db, s.poller)
	sonarrHandler := handlers.NewSonarrHandler(s.db, s.poller)
	radarrHandler := handlers.NewRadarrHandler(s.db, s.poller)
	lidarrHandler := handlers.NewLidarrHandler(s.db, s.poller)
	readarrHandler := handlers.NewReadarrHandler(s.db, s.poller)
	whisparrHandler := handlers.NewWhisparrHandler(s.db, s.poller)
	uiPreferencesHandler := handlers.NewUIPreferencesHandler(s.db)

	// Initialize auth handlers and middleware
	var oidcAuthHandler *handlers.AuthHandler
	builtinAuthHandler := handlers.NewBuiltinAuthHandler(s.db, s.cache)
	authMiddleware := middleware.NewAuthMiddleware(s.cache)

	// Initialize OIDC if configuration is provided
	oidc := s.cfg.Auth.OIDC
	if oidc.IsConfigured() {
		redirectURL := oidc.RedirectURL
		if redirectURL == "" {
			redirectURL = defaultOIDCCallbackURL
		}
		authConfig := &types.AuthConfig{
			Issuer:       oidc.Issuer,
			ClientID:     oidc.ClientID,
			ClientSecret: oidc.ClientSecret,
			RedirectURL:  redirectURL,
		}
		oidcAuthHandler = handlers.NewAuthHandler(authConfig, s.cache)
	}

	// Public routes (no auth required)
	public := r.Group("")
	{
		// Health check endpoint
		public.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// Auth configuration endpoint
		public.GET("/api/auth/config", handlers.AuthConfig(oidc.IsConfigured()))

		// OIDC auth endpoints (only if OIDC is configured)
		if oidcAuthHandler != nil {
			public.GET("/api/auth/callback", oidcAuthHandler.Callback)
			// Alias: keep callback under the OIDC path for consistency.
			public.GET("/api/auth/oidc/callback", oidcAuthHandler.Callback)
			oidcAuth := public.Group("/api/auth/oidc")
			oidcAuth.Use(authRateLimiter.RateLimit())
			{
				oidcAuth.GET("/login", oidcAuthHandler.Login)
				// Support top-level browser navigation (GET) and programmatic (POST).
				oidcAuth.GET("/logout", oidcAuthHandler.Logout)
				oidcAuth.POST("/logout", oidcAuthHandler.Logout)
			}
		}

		// Built-in auth endpoints
		builtinAuth := public.Group("/api/auth")
		builtinAuth.Use(authRateLimiter.RateLimit())
		{
			builtinAuth.GET("/registration-status", builtinAuthHandler.CheckRegistrationStatus)
			builtinAuth.POST("/register", builtinAuthHandler.Register)
			builtinAuth.POST("/login", builtinAuthHandler.Login)
			builtinAuth.POST("/logout", builtinAuthHandler.Logout)
			builtinAuth.GET("/verify", builtinAuthHandler.Verify)
		}
	}

	// Protected auth routes
	protectedAuth := r.Group("/api/auth")
	protectedAuth.Use(authMiddleware.RequireAuth())
	protectedAuth.Use(authRateLimiter.RateLimit())
	{
		if oidcAuthHandler != nil {
			oidc := protectedAuth.Group("/oidc")
			{
				oidc.GET("/verify", oidcAuthHandler.VerifyToken)
				oidc.GET("/userinfo", oidcAuthHandler.UserInfo)
			}
		}
		protectedAuth.GET("/userinfo", builtinAuthHandler.GetUserInfo)
	}

	// API routes group with auth middleware
	api := r.Group("/api")
	api.Use(authMiddleware.RequireAuth())
	{
		// Settings endpoints - no caching to ensure fresh data
		settings := api.Group("/settings")
		{
			settings.GET("", settingsHandler.GetSettings)
			settings.POST("/:instance", settingsHandler.SaveSettings)
			settings.DELETE("/:instance", settingsHandler.DeleteSettings)
		}

		uiPreferences := api.Group("/ui/preferences")
		{
			uiPreferences.GET("/collapse", uiPreferencesHandler.GetCollapsePreferences)
			uiPreferences.PUT("/collapse", uiPreferencesHandler.UpsertCollapsePreference)
		}

		plexAuth := api.Group("/plex/auth")
		plexAuth.Use(apiRateLimiter.RateLimit())
		{
			plexAuth.POST("/pin", plexAuthHandler.CreatePIN)
			plexAuth.GET("/pin/:pinId", plexAuthHandler.GetPIN)
		}

		// Health check endpoints
		health := api.Group("/health")
		health.Use(healthRateLimiter.RateLimit())
		{
			health.GET("/:service", healthHandler.CheckHealth)
		}

		// SSE events (preferred)
		api.GET("/events", eventsHandler.Stream)

		// Service actions. Each action asks the poller for fresh data; SSE carries the result.
		actions := api.Group("")
		actions.Use(apiRateLimiter.RateLimit())
		{
			actions.DELETE("/sonarr/queue/:id", sonarrHandler.DeleteQueueItem)
			actions.DELETE("/radarr/queue/:id", radarrHandler.DeleteQueueItem)
			actions.DELETE("/lidarr/queue/:id", lidarrHandler.DeleteQueueItem)
			actions.DELETE("/readarr/queue/:id", readarrHandler.DeleteQueueItem)
			actions.DELETE("/whisparr/queue/:id", whisparrHandler.DeleteQueueItem)
			actions.POST("/services/:instanceId/overseerr/request/:requestId/:status", overseerrHandler.UpdateRequestStatus)
		}
	}

	web.ServeStatic(r)

	return r
}
