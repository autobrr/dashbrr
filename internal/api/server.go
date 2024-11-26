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
	"github.com/autobrr/dashbrr/internal/services"
	"github.com/autobrr/dashbrr/internal/services/cache"
	"github.com/autobrr/dashbrr/internal/types"
	"github.com/autobrr/dashbrr/web"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type Server struct {
	cfg        *config.Config
	db         *database.DB
	cache      cache.Store
	healthSvc  *services.HealthService
	httpServer *http.Server
}

func NewServer(cfg *config.Config, db *database.DB, cache cache.Store, healthSvc *services.HealthService) *Server {
	return &Server{
		cfg:       cfg,
		db:        db,
		cache:     cache,
		healthSvc: healthSvc,
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
		Addr:         s.cfg.Server.ListenAddr,
		Handler:      s.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.httpServer.Serve(listener)
}

func (s *Server) Shutdown(ctx context.Context) error {
	//s.cache.Close()
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

	r.Use(middleware.SetupCORS())
	// TODO: Enable secure headers?
	//r.Use(middleware.Secure(nil))
	r.Use(middleware.CSRF(nil)) // Added CSRF middleware with default config that includes OAuth callback exemption

	// Create rate limiters with different configurations
	apiRateLimiter := middleware.NewRateLimiter(s.cache, time.Minute, 60, "api:")       // 60 requests per minute for API
	healthRateLimiter := middleware.NewRateLimiter(s.cache, time.Minute, 30, "health:") // 30 health checks per minute
	authRateLimiter := middleware.NewRateLimiter(s.cache, time.Minute, 30, "auth:")     // 30 auth requests per minute

	// Special rate limiter for Tailscale services
	tailscaleRateLimiter := middleware.NewRateLimiter(s.cache, 2*time.Minute, 20, "tailscale:") // 20 requests per 2 minutes

	// Create cache middleware (now handles TTLs internally)
	cacheMiddleware := middleware.NewCacheMiddleware(s.cache)

	// Initialize handlers with cache
	settingsHandler := handlers.NewSettingsHandler(s.db, s.healthSvc, s.cache)
	//serviceHandler := handlers.NewServiceHandler(db, health, store)
	healthHandler := handlers.NewHealthHandler(s.db, s.healthSvc)
	eventsHandler := handlers.NewEventsHandler(s.db, s.healthSvc)
	autobrrHandler := handlers.NewAutobrrHandler(s.db, s.cache)
	omegabrrHandler := handlers.NewOmegabrrHandler(s.db, s.cache)
	maintainerrHandler := handlers.NewMaintainerrHandler(s.db, s.cache)
	plexHandler := handlers.NewPlexHandler(s.db, s.cache)
	tailscaleHandler := handlers.NewTailscaleHandler(s.db, s.cache)
	overseerrHandler := handlers.NewOverseerrHandler(s.db, s.cache)
	sonarrHandler := handlers.NewSonarrHandler(s.db, s.cache)
	radarrHandler := handlers.NewRadarrHandler(s.db, s.cache)
	prowlarrHandler := handlers.NewProwlarrHandler(s.db, s.cache)

	// Initialize auth handlers and middleware
	var oidcAuthHandler *handlers.AuthHandler
	builtinAuthHandler := handlers.NewBuiltinAuthHandler(s.db, s.cache)
	authMiddleware := middleware.NewAuthMiddleware(s.cache)

	// Initialize OIDC if configuration is provided
	if hasOIDCConfig() {
		authConfig := &types.AuthConfig{
			Issuer:       getEnvOrDefault("OIDC_ISSUER", ""),
			ClientID:     getEnvOrDefault("OIDC_CLIENT_ID", ""),
			ClientSecret: getEnvOrDefault("OIDC_CLIENT_SECRET", ""),
			RedirectURL:  getEnvOrDefault("OIDC_REDIRECT_URL", "http://localhost:3000/api/auth/callback"),
		}
		oidcAuthHandler = handlers.NewAuthHandler(authConfig, s.cache)
	}

	// Start the health monitor
	eventsHandler.StartHealthMonitor()

	// Public routes (no auth required)
	public := r.Group("")
	{
		// Health check endpoint
		public.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// Auth configuration endpoint
		public.GET("/api/auth/config", handlers.GetAuthConfig)

		// OIDC auth endpoints (only if OIDC is configured)
		if oidcAuthHandler != nil {
			public.GET("/api/auth/callback", oidcAuthHandler.Callback)
			oidcAuth := public.Group("/api/auth/oidc")
			oidcAuth.Use(authRateLimiter.RateLimit())
			{
				oidcAuth.GET("/login", oidcAuthHandler.Login)
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
				oidc.POST("/refresh", oidcAuthHandler.RefreshToken)
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

		// Health check endpoints (no cache for SSE)
		health := api.Group("/health")
		health.Use(healthRateLimiter.RateLimit())
		{
			health.GET("/:service", healthHandler.CheckHealth)
			health.GET("/events", eventsHandler.StreamHealth)
		}

		//serviceRoutes := api.Group("/services")
		//serviceRoutes.Use(cacheMiddleware.Cache())
		//{
		//
		//	serviceRoutes.POST("/", serviceHandler.Create)
		//
		//	autobrr := serviceRoutes.Group("/autobrr/:id")
		//	autobrr.GET("/stats", autobrrHandler.GetAutobrrReleaseStats)
		//	autobrr.GET("/irc", autobrrHandler.GetAutobrrIRCStatus)
		//	autobrr.GET("/releases", autobrrHandler.GetAutobrrReleases)
		//}

		// Service endpoints with specific rate limits and caches
		services := api.Group("")
		{
			// Regular services with standard rate limit
			regularServices := services.Group("")
			regularServices.Use(apiRateLimiter.RateLimit())
			regularServices.Use(cacheMiddleware.Cache())
			{
				regularServices.GET("/autobrr/stats", autobrrHandler.GetAutobrrReleaseStats)
				regularServices.GET("/autobrr/irc", autobrrHandler.GetAutobrrIRCStatus)
				regularServices.GET("/autobrr/releases", autobrrHandler.GetAutobrrReleases)
				regularServices.GET("/plex/sessions", plexHandler.GetPlexSessions)
				regularServices.GET("/maintainerr/collections", maintainerrHandler.GetMaintainerrCollections)

				// Overseerr endpoints
				overseerr := regularServices.Group("/overseerr")
				{
					overseerr.GET("/requests", overseerrHandler.GetRequests)
				}

				// Sonarr endpoints
				sonarr := regularServices.Group("/sonarr")
				{
					sonarr.GET("/queue", sonarrHandler.GetQueue)
					sonarr.GET("/stats", sonarrHandler.GetStats)
					sonarr.DELETE("/queue/:id", sonarrHandler.DeleteQueueItem)
				}

				// Radarr endpoints
				radarr := regularServices.Group("/radarr")
				{
					radarr.GET("/queue", radarrHandler.GetQueue)
					radarr.DELETE("/queue/:id", radarrHandler.DeleteQueueItem)
				}

				// Prowlarr endpoints
				prowlarr := regularServices.Group("/prowlarr")
				{
					prowlarr.GET("/stats", prowlarrHandler.GetStats)
					prowlarr.GET("/indexers", prowlarrHandler.GetIndexers)
				}

				// Omegabrr endpoints
				omegabrr := regularServices.Group("/omegabrr")
				{
					omegabrr.GET("/status", omegabrrHandler.GetOmegabrrStatus)
					webhook := omegabrr.Group("/webhook")
					{
						webhook.POST("/arrs", omegabrrHandler.TriggerWebhookArrs)
						webhook.POST("/lists", omegabrrHandler.TriggerWebhookLists)
						webhook.POST("/all", omegabrrHandler.TriggerWebhookAll)
					}
				}
			}

			// Tailscale services with special rate limit
			tailscaleServices := services.Group("")
			tailscaleServices.Use(tailscaleRateLimiter.RateLimit())
			tailscaleServices.Use(cacheMiddleware.Cache())
			{
				tailscaleServices.GET("/tailscale/devices", tailscaleHandler.GetTailscaleDevices)
			}

			// Service action endpoints that require instanceId
			serviceActions := services.Group("/services/:instanceId")
			serviceActions.Use(apiRateLimiter.RateLimit())
			{
				// Overseerr action endpoints
				overseerrActions := serviceActions.Group("/overseerr")
				{
					overseerrActions.POST("/request/:requestId/:status", overseerrHandler.UpdateRequestStatus)
				}
			}
		}
	}

	web.ServeStatic(r)

	return r
}

// hasOIDCConfig checks if all required OIDC configuration is provided
func hasOIDCConfig() bool {
	return os.Getenv("OIDC_ISSUER") != "" &&
		os.Getenv("OIDC_CLIENT_ID") != "" &&
		os.Getenv("OIDC_CLIENT_SECRET") != ""
}

// getEnvOrDefault returns the value of an environment variable or a default value if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
