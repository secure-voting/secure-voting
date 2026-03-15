package httpserver

func registerBallotRoutes(r *routeCtx) {
	r.mux.Handle("POST /api/v1/elections/{id}/ballots/submit", r.RequireRole("voter", r.ballotsH.Submit))
	r.mux.Handle("GET /api/v1/elections/{id}/ballots/me", r.RequireRole("voter", r.ballotsH.Me))
}
