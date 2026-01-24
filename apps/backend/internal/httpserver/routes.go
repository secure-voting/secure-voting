package httpserver

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/ballots"
	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/elections"
	"secure-voting/apps/backend/internal/results"

	"secure-voting/apps/backend/internal/audit"
	"secure-voting/apps/backend/internal/datasets"
	"secure-voting/apps/backend/internal/experimentruns"
	"secure-voting/apps/backend/internal/experiments"
	"secure-voting/apps/backend/internal/jobs"

	ah "secure-voting/apps/backend/internal/httpserver/auth"
	bh "secure-voting/apps/backend/internal/httpserver/ballots"
	eh "secure-voting/apps/backend/internal/httpserver/elections"
	rh "secure-voting/apps/backend/internal/httpserver/results"

	auh "secure-voting/apps/backend/internal/httpserver/audit"
	dsh "secure-voting/apps/backend/internal/httpserver/datasets"
	exh "secure-voting/apps/backend/internal/httpserver/experiments"
	erh "secure-voting/apps/backend/internal/httpserver/experimentruns"
	jh "secure-voting/apps/backend/internal/httpserver/jobs"

	"secure-voting/apps/backend/internal/httpserver/middleware"
)

func Routes(cfg config.Config, db *pgxpool.Pool, rdb *redis.Client, mdb *mongo.Database) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	// Services
	authSvc := asvc.NewService(db, cfg.TokenTTL)
	electionsSvc := elections.NewService(db)
	ballotsSvc := ballots.NewService(db, rdb, cfg.IdempotencyTTL)
	resultsSvc := results.NewService(db)

	// New services
	jobsSvc := jobs.NewService(db)
	auditSvc := audit.NewService(db)
	datasetsSvc := datasets.NewService(mdb)
	experimentsSvc := experiments.NewService(db)
	runsSvc := experimentruns.NewService(db, mdb)

	// Handlers
	authH := ah.NewHandlers(authSvc)
	electionsH := eh.NewHandlers(electionsSvc)
	ballotsH := bh.NewHandlers(ballotsSvc)
	resultsH := rh.NewHandlers(resultsSvc)

	jobsH := jh.NewHandlers(jobsSvc)
	auditH := auh.NewHandlers(auditSvc)
	datasetsH := dsh.NewHandlers(datasetsSvc, cfg)
	experimentsH := exh.NewHandlers(experimentsSvc)
	runsH := erh.NewHandlers(runsSvc)

	// Auth (public)
	mux.Handle("POST /api/v1/auth/register", http.HandlerFunc(authH.Register))
	mux.Handle("POST /api/v1/auth/login", http.HandlerFunc(authH.Login))

	// Auth (protected)
	mux.Handle("GET /api/v1/auth/me", middleware.RequireAuth(authSvc, http.HandlerFunc(authH.Me)))
	mux.Handle("POST /api/v1/auth/logout", middleware.RequireAuth(authSvc, http.HandlerFunc(authH.Logout)))

	// Elections
	mux.Handle("GET /api/v1/elections", middleware.RequireAuth(authSvc, http.HandlerFunc(electionsH.List)))
	mux.Handle("GET /api/v1/elections/{id}", middleware.RequireAuth(authSvc, http.HandlerFunc(electionsH.Get)))
	mux.Handle("POST /api/v1/elections",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("admin", http.HandlerFunc(electionsH.Create)),
		),
	)
	mux.Handle("GET /api/v1/elections/{id}/ballot", middleware.RequireAuth(authSvc, http.HandlerFunc(electionsH.BallotMeta)))

	// Rules
	mux.Handle("PUT /api/v1/elections/{id}/rules",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("admin", http.HandlerFunc(electionsH.UpdateRules)),
		),
	)

	// Actions
	mux.Handle("POST /api/v1/elections/{id}/actions/{action}",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("admin", http.HandlerFunc(electionsH.Action)),
		),
	)

	// Invites
	mux.Handle("POST /api/v1/elections/{id}/invites",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("admin", http.HandlerFunc(electionsH.CreateInvite)),
		),
	)
	mux.Handle("GET /api/v1/elections/{id}/invites",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("admin", http.HandlerFunc(electionsH.ListInvites)),
		),
	)

	// Ballots
	mux.Handle("POST /api/v1/elections/{id}/ballots/submit",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("voter", http.HandlerFunc(ballotsH.Submit)),
		),
	)
	mux.Handle("GET /api/v1/elections/{id}/ballots/me",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("voter", http.HandlerFunc(ballotsH.Me)),
		),
	)

	// Results
	mux.Handle("GET /api/v1/elections/{id}/results", middleware.RequireAuth(authSvc, http.HandlerFunc(resultsH.Get)))

	// Jobs
	mux.Handle("GET /api/v1/jobs",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(jobsH.List)),
		),
	)
	mux.Handle("GET /api/v1/jobs/{id}",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(jobsH.Get)),
		),
	)

	// Audit-log
	mux.Handle("GET /api/v1/audit-log",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher", "voter"}, http.HandlerFunc(auditH.List)),
		),
	)

	// Datasets (researcher)
	mux.Handle("POST /api/v1/datasets/import",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(datasetsH.Import)),
		),
	)
	mux.Handle("POST /api/v1/datasets/generate",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(datasetsH.Generate)),
		),
	)
	mux.Handle("GET /api/v1/datasets",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(datasetsH.List)),
		),
	)
	mux.Handle("GET /api/v1/datasets/{id}",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(datasetsH.Get)),
		),
	)
	mux.Handle("GET /api/v1/datasets/{id}/download",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(datasetsH.Download)),
		),
	)

	// Experiments (researcher/admin)
	mux.Handle("POST /api/v1/experiments",
		middleware.RequireAuth(authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(experimentsH.Create)),
		),
	)
	mux.Handle("GET /api/v1/experiments",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(experimentsH.List)),
		),
	)
	mux.Handle("GET /api/v1/experiments/{id}",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(experimentsH.Get)),
		),
	)

	// Experiment-runs
	mux.Handle("POST /api/v1/experiment-runs/batch",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(runsH.Batch)),
		),
	)
	mux.Handle("GET /api/v1/experiment-runs",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(runsH.List)),
		),
	)
	mux.Handle("GET /api/v1/experiment-runs/{id}",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(runsH.Get)),
		),
	)
	mux.Handle("GET /api/v1/experiment-runs/{id}/result",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(runsH.Result)),
		),
	)
	mux.Handle("GET /api/v1/experiment-runs/{id}/download",
		middleware.RequireAuth(authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(runsH.Download)),
		),
	)

	return mux
}
