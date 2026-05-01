package httpserver

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"secure-voting/apps/backend/internal/audit"
	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/ballots"
	"secure-voting/apps/backend/internal/capabilities"
	"secure-voting/apps/backend/internal/computeclient"
	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/datasets"
	"secure-voting/apps/backend/internal/elections"
	"secure-voting/apps/backend/internal/experimentruns"
	"secure-voting/apps/backend/internal/experiments"
	"secure-voting/apps/backend/internal/jobs"
	"secure-voting/apps/backend/internal/notifications"
	"secure-voting/apps/backend/internal/results"

	"secure-voting/apps/backend/internal/adminsettings"
	ash "secure-voting/apps/backend/internal/httpserver/adminsettings"
	auusersh "secure-voting/apps/backend/internal/httpserver/adminusers"
	auh "secure-voting/apps/backend/internal/httpserver/audit"
	ah "secure-voting/apps/backend/internal/httpserver/auth"
	bh "secure-voting/apps/backend/internal/httpserver/ballots"
	caph "secure-voting/apps/backend/internal/httpserver/capabilities"
	dsh "secure-voting/apps/backend/internal/httpserver/datasets"
	eh "secure-voting/apps/backend/internal/httpserver/elections"
	erh "secure-voting/apps/backend/internal/httpserver/experimentruns"
	exh "secure-voting/apps/backend/internal/httpserver/experiments"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	jh "secure-voting/apps/backend/internal/httpserver/jobs"
	"secure-voting/apps/backend/internal/httpserver/middleware"
	nh "secure-voting/apps/backend/internal/httpserver/notifications"
	rh "secure-voting/apps/backend/internal/httpserver/results"
)

type routeCtx struct {
	cfg config.Config
	mux *http.ServeMux

	authSvc *asvc.Service

	computeClient *computeclient.Client

	authH           *ah.Handlers
	electionsH      *eh.Handlers
	ballotsH        *bh.Handlers
	resultsH        *rh.Handlers
	jobsH           *jh.Handlers
	auditH          *auh.Handlers
	datasetsH       *dsh.Handlers
	experimentsH    *exh.Handlers
	runsH           *erh.Handlers
	capabilitiesH   *caph.Handlers
	notificationsH  *nh.Handlers
	adminUsersH     *auusersh.Handlers
	authRateLimiter *middleware.RateLimiter
	redisClient     *redis.Client

	writeRateLimiter *middleware.RateLimiter
	adminSettingsH   *ash.Handlers
}

func newRouteCtx(cfg config.Config, db *pgxpool.Pool, rdb *redis.Client, mdb *mongo.Database) *routeCtx {
	mux := http.NewServeMux()

	ctx := context.Background()

	var computeClient *computeclient.Client
	if cfg.ComputeGRPCAddr != "" {
		cc, err := computeclient.New(ctx, computeclient.Config{
			Addr:       cfg.ComputeGRPCAddr,
			UseTLS:     cfg.ComputeTLS,
			CACertPath: cfg.ComputeTLSCA,
			ServerName: cfg.ComputeTLSServerName,
		})
		if err != nil {
			log.Printf("compute client unavailable: addr=%s tls=%v err=%v", cfg.ComputeGRPCAddr, cfg.ComputeTLS, err)
		} else {
			computeClient = cc
		}
	}

	authSvc := asvc.NewServiceWithRefreshTTL(db, cfg.TokenTTL, cfg.RefreshTokenTTL).
		WithEmailVerificationSender(newEmailVerificationSender(cfg))
	electionsSvc := elections.NewService(db, computeClient)
	ballotsSvc := ballots.NewService(db, rdb, cfg.IdempotencyTTL)
	resultsSvc := results.NewService(db)

	jobsSvc := jobs.NewService(db)
	auditSvc := audit.NewService(db)
	datasetsSvc := datasets.NewService(mdb, db)
	experimentsSvc := experiments.NewService(db)
	runsSvc := experimentruns.NewService(db, mdb)
	capabilitiesSvc := capabilities.NewService(computeClient)
	notificationsSvc := notifications.NewService(db)

	authRateLimiter := middleware.NewRateLimiter(
		rdb,
		"auth_rl",
		cfg.AuthRateLimit,
		cfg.AuthRateLimitTTL,
		[]string{"POST", "PATCH"},
		[]string{"/api/v1/auth"},
	)

	writeRateLimiter := middleware.NewRateLimiter(
		rdb,
		"write_rl",
		cfg.WriteRateLimit,
		cfg.WriteRateLimitTTL,
		[]string{"POST", "PUT", "PATCH", "DELETE"},
		nil,
	)

	adminSettingsSvc := adminsettings.NewService(db)

	return &routeCtx{
		cfg: cfg,
		mux: mux,

		authSvc: authSvc,

		computeClient: computeClient,

		authH:            ah.NewHandlers(authSvc),
		electionsH:       eh.NewHandlers(electionsSvc),
		ballotsH:         bh.NewHandlers(ballotsSvc),
		resultsH:         rh.NewHandlers(resultsSvc),
		jobsH:            jh.NewHandlers(jobsSvc),
		auditH:           auh.NewHandlers(auditSvc),
		datasetsH:        dsh.NewHandlers(datasetsSvc, cfg),
		experimentsH:     exh.NewHandlers(experimentsSvc),
		runsH:            erh.NewHandlers(runsSvc),
		capabilitiesH:    caph.NewHandlers(capabilitiesSvc),
		notificationsH:   nh.NewHandlers(notificationsSvc),
		adminUsersH:      auusersh.NewHandlers(authSvc),
		authRateLimiter:  authRateLimiter,
		writeRateLimiter: writeRateLimiter,
		adminSettingsH:   ash.NewHandlers(adminSettingsSvc),
		redisClient:      rdb,
	}
}

func (c *routeCtx) WrapAuthLimited(fn httputil.HandlerFunc) http.Handler {
	if c.authRateLimiter == nil {
		return httputil.Wrap(fn)
	}
	return c.authRateLimiter.Middleware(httputil.Wrap(fn))
}

func (c *routeCtx) RequireAuthLimited(fn httputil.HandlerFunc) http.Handler {
	base := middleware.RequireAuth(c.authSvc, httputil.Wrap(fn))
	if c.authRateLimiter == nil {
		return base
	}
	return c.authRateLimiter.Middleware(base)
}

func (c *routeCtx) Wrap(fn httputil.HandlerFunc) http.Handler {
	return httputil.Wrap(fn)
}

func (c *routeCtx) RequireAuth(fn httputil.HandlerFunc) http.Handler {
	return middleware.RequireAuth(c.authSvc, httputil.Wrap(fn))
}

func (c *routeCtx) RequireRole(role string, fn httputil.HandlerFunc) http.Handler {
	return middleware.RequireAuth(c.authSvc, middleware.RequireRole(role, httputil.Wrap(fn)))
}

func (c *routeCtx) LimitWrite(next http.Handler) http.Handler {
	if c.writeRateLimiter == nil {
		return next
	}
	return c.writeRateLimiter.Middleware(next)
}

func (c *routeCtx) RequireAdminTrusted(fn httputil.HandlerFunc) http.Handler {
	protected := middleware.RequireRole(
		"admin",
		middleware.RequireTrustedCIDRs(c.cfg.AdminTrustedCIDRs, httputil.Wrap(fn)),
	)
	return middleware.RequireAuth(c.authSvc, protected)
}

func newEmailVerificationSender(cfg config.Config) asvc.EmailVerificationSender {
	switch cfg.EmailVerificationMode {
	case "smtp":
		return asvc.NewSMTPEmailVerificationSender(asvc.EmailVerificationSenderConfig{
			Mode:      cfg.EmailVerificationMode,
			Host:      cfg.SMTPHost,
			Port:      cfg.SMTPPort,
			Username:  cfg.SMTPUsername,
			Password:  cfg.SMTPPassword,
			FromEmail: cfg.SMTPFromEmail,
			FromName:  cfg.SMTPFromName,
			TLSMode:   cfg.SMTPTLSMode,
		})
	case "disabled":
		return asvc.NewDisabledEmailVerificationSender()
	default:
		return asvc.NewDevEmailVerificationSender()
	}
}
