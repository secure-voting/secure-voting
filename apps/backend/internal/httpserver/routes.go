package httpserver

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/ballots"
	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/elections"
	"secure-voting/apps/backend/internal/results"

	ah "secure-voting/apps/backend/internal/httpserver/auth"
	bh "secure-voting/apps/backend/internal/httpserver/ballots"
	eh "secure-voting/apps/backend/internal/httpserver/elections"
	rh "secure-voting/apps/backend/internal/httpserver/results"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

func Routes(cfg config.Config, db *pgxpool.Pool, rdb *redis.Client) http.Handler {
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

	// Handlers
	authH := ah.NewHandlers(authSvc)
	electionsH := eh.NewHandlers(electionsSvc)
	ballotsH := bh.NewHandlers(ballotsSvc)
	resultsH := rh.NewHandlers(resultsSvc)

	// Auth (public)
	mux.Handle("POST /api/v1/auth/register", http.HandlerFunc(authH.Register))
	mux.Handle("POST /api/v1/auth/login", http.HandlerFunc(authH.Login))

	// Auth (protected)
	mux.Handle("GET /api/v1/auth/me", middleware.RequireAuth(authSvc, http.HandlerFunc(authH.Me)))
	mux.Handle("POST /api/v1/auth/logout", middleware.RequireAuth(authSvc, http.HandlerFunc(authH.Logout)))

	// Elections
	mux.Handle("GET /api/v1/elections", middleware.RequireAuth(authSvc, http.HandlerFunc(electionsH.List)))
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

	// Actions (schedule/open/pause/resume/close/publish)
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

	return mux
}
