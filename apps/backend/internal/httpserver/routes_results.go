package httpserver

func registerResultsRoutes(r *routeCtx) {
	r.mux.Handle("GET /api/v1/elections/{id}/results", r.RequireAuth(r.resultsH.Get))
}
