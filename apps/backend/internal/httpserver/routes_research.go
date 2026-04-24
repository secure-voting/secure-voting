package httpserver

import (
	"net/http"

	"secure-voting/apps/backend/internal/httpserver/middleware"
)

func registerResearchRoutes(r *routeCtx) {
	r.mux.Handle("GET /api/v1/jobs",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.jobsH.List)),
		),
	)
	r.mux.Handle("GET /api/v1/jobs/{id}",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.jobsH.Get)),
		),
	)

	r.mux.Handle("GET /api/v1/audit-log",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher", "voter"}, http.HandlerFunc(r.auditH.List)),
		),
	)

	r.mux.Handle("POST /api/v1/datasets/import",
		r.LimitWrite(
			middleware.RequireAuth(r.authSvc,
				middleware.RequireRole("researcher", http.HandlerFunc(r.datasetsH.Import)),
			),
		),
	)
	r.mux.Handle("POST /api/v1/datasets/generate",
		r.LimitWrite(
			middleware.RequireAuth(r.authSvc,
				middleware.RequireRole("researcher", http.HandlerFunc(r.datasetsH.Generate)),
			),
		),
	)
	r.mux.Handle("GET /api/v1/datasets",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(r.datasetsH.List)),
		),
	)
	r.mux.Handle("GET /api/v1/datasets/{id}",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(r.datasetsH.Get)),
		),
	)
	r.mux.Handle("GET /api/v1/datasets/{id}/download",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireRole("researcher", http.HandlerFunc(r.datasetsH.Download)),
		),
	)

	r.mux.Handle("POST /api/v1/experiments",
		r.LimitWrite(
			middleware.RequireAuth(r.authSvc,
				middleware.RequireRole("researcher", http.HandlerFunc(r.experimentsH.Create)),
			),
		),
	)
	r.mux.Handle("GET /api/v1/experiments",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.experimentsH.List)),
		),
	)
	r.mux.Handle("GET /api/v1/experiments/{id}",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.experimentsH.Get)),
		),
	)

	r.mux.Handle("POST /api/v1/experiment-runs/batch",
		r.LimitWrite(
			middleware.RequireAuth(r.authSvc,
				middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.runsH.Batch)),
			),
		),
	)
	r.mux.Handle("GET /api/v1/experiment-runs",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.runsH.List)),
		),
	)
	r.mux.Handle("GET /api/v1/experiment-runs/{id}",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.runsH.Get)),
		),
	)
	r.mux.Handle("GET /api/v1/experiment-runs/{id}/result",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.runsH.Result)),
		),
	)
	r.mux.Handle("GET /api/v1/experiment-runs/{id}/download",
		middleware.RequireAuth(r.authSvc,
			middleware.RequireAnyRole([]string{"admin", "researcher"}, http.HandlerFunc(r.runsH.Download)),
		),
	)
}
