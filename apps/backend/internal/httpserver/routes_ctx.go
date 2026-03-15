package httpserver

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"secure-voting/apps/backend/internal/audit"
	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/ballots"
	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/datasets"
	"secure-voting/apps/backend/internal/elections"
	"secure-voting/apps/backend/internal/experimentruns"
	"secure-voting/apps/backend/internal/experiments"
	"secure-voting/apps/backend/internal/jobs"
	"secure-voting/apps/backend/internal/results"

	auh "secure-voting/apps/backend/internal/httpserver/audit"
	ah "secure-voting/apps/backend/internal/httpserver/auth"
	bh "secure-voting/apps/backend/internal/httpserver/ballots"
	dsh "secure-voting/apps/backend/internal/httpserver/datasets"
	eh "secure-voting/apps/backend/internal/httpserver/elections"
	erh "secure-voting/apps/backend/internal/httpserver/experimentruns"
	exh "secure-voting/apps/backend/internal/httpserver/experiments"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	jh "secure-voting/apps/backend/internal/httpserver/jobs"
	"secure-voting/apps/backend/internal/httpserver/middleware"
	rh "secure-voting/apps/backend/internal/httpserver/results"
)

type routeCtx struct {
	cfg config.Config
	mux *http.ServeMux

	authSvc *asvc.Service

	authH      *ah.Handlers
	electionsH *eh.Handlers
	ballotsH   *bh.Handlers
	resultsH   *rh.Handlers

	jobsH        *jh.Handlers
	auditH       *auh.Handlers
	datasetsH    *dsh.Handlers
	experimentsH *exh.Handlers
	runsH        *erh.Handlers
}

func newRouteCtx(cfg config.Config, db *pgxpool.Pool, rdb *redis.Client, mdb *mongo.Database) *routeCtx {
	mux := http.NewServeMux()

	authSvc := asvc.NewService(db, cfg.TokenTTL)
	electionsSvc := elections.NewService(db)
	ballotsSvc := ballots.NewService(db, rdb, cfg.IdempotencyTTL)
	resultsSvc := results.NewService(db)

	jobsSvc := jobs.NewService(db)
	auditSvc := audit.NewService(db)
	datasetsSvc := datasets.NewService(mdb)
	experimentsSvc := experiments.NewService(db)
	runsSvc := experimentruns.NewService(db, mdb)

	return &routeCtx{
		cfg: cfg,
		mux: mux,

		authSvc: authSvc,

		authH:      ah.NewHandlers(authSvc),
		electionsH: eh.NewHandlers(electionsSvc),
		ballotsH:   bh.NewHandlers(ballotsSvc),
		resultsH:   rh.NewHandlers(resultsSvc),

		jobsH:        jh.NewHandlers(jobsSvc),
		auditH:       auh.NewHandlers(auditSvc),
		datasetsH:    dsh.NewHandlers(datasetsSvc, cfg),
		experimentsH: exh.NewHandlers(experimentsSvc),
		runsH:        erh.NewHandlers(runsSvc),
	}
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
