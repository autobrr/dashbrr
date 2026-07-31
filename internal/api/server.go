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

	// Special rate limiter for Tailscale services
	tailscaleRateLimiter := middleware.NewRateLimiter(s.cache, 2*time.Minute, 20, "tailscale:") // 20 requests per 2 minutes

	// Create cache middleware (now handles TTLs internally)
	cacheMiddleware := middleware.NewCacheMiddleware(s.cache)

	// Initialize handlers with cache
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
	autobrrHandler := handlers.NewAutobrrHandler(s.db, s.cache, bc)
	maintainerrHandler := handlers.NewMaintainerrHandler(s.db, s.cache, bc)
	plexHandler := handlers.NewPlexHandler(s.db, s.cache, bc)
	jellyfinHandler := handlers.NewJellyfinHandler(s.db, s.cache, bc)
	uptimeKumaHandler := handlers.NewUptimeKumaHandler(s.db, s.cache, bc)
	plexAuthHandler := handlers.NewPlexAuthHandler()
	tailscaleHandler := handlers.NewTailscaleHandler(s.db, s.cache)
	overseerrHandler := handlers.NewOverseerrHandler(s.db, s.cache, bc)
	sonarrHandler := handlers.NewSonarrHandler(s.db, s.cache, bc)
	radarrHandler := handlers.NewRadarrHandler(s.db, s.cache, bc)
	lidarrHandler := handlers.NewLidarrHandler(s.db, s.cache, bc)
	readarrHandler := handlers.NewReadarrHandler(s.db, s.cache, bc)
	prowlarrHandler := handlers.NewProwlarrHandler(s.db, s.cache, bc)
	traefikHandler := handlers.NewTraefikHandler(s.db, s.cache, bc)
	bazarrHandler := handlers.NewBazarrHandler(s.db, s.cache, bc)
	sabnzbdHandler := handlers.NewSabnzbdHandler(s.db, s.cache, bc)
	nzbgetHandler := handlers.NewNzbgetHandler(s.db, s.cache, bc)
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
				regularServices.GET("/jellyfin/summary", jellyfinHandler.GetSummary)
				regularServices.GET("/uptimekuma/summary", uptimeKumaHandler.GetSummary)
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

				// Lidarr endpoints
				lidarr := regularServices.Group("/lidarr")
				{
					lidarr.GET("/queue", lidarrHandler.GetQueue)
					lidarr.DELETE("/queue/:id", lidarrHandler.DeleteQueueItem)
				}

				// Readarr endpoints
				readarr := regularServices.Group("/readarr")
				{
					readarr.GET("/queue", readarrHandler.GetQueue)
					readarr.DELETE("/queue/:id", readarrHandler.DeleteQueueItem)
				}

				// Prowlarr endpoints
				prowlarr := regularServices.Group("/prowlarr")
				{
					prowlarr.GET("/stats", prowlarrHandler.GetStats)
					prowlarr.GET("/indexers", prowlarrHandler.GetIndexers)
				}

				// Traefik endpoints
				traefik := regularServices.Group("/traefik")
				{
					traefik.GET("/summary", traefikHandler.GetSummary)
				}

				// Bazarr endpoints
				bazarr := regularServices.Group("/bazarr")
				{
					bazarr.GET("/summary", bazarrHandler.GetSummary)
				}

				// SABnzbd endpoints
				sabnzbd := regularServices.Group("/sabnzbd")
				{
					sabnzbd.GET("/summary", sabnzbdHandler.GetSummary)
				}

				// NZBGet endpoints
				nzbget := regularServices.Group("/nzbget")
				{
					nzbget.GET("/summary", nzbgetHandler.GetSummary)
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
