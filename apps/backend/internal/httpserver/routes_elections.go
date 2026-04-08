package httpserver

func registerElectionRoutes(r *routeCtx) {
	r.mux.Handle("GET /api/v1/elections", r.RequireAuth(r.electionsH.List))
	r.mux.Handle("GET /api/v1/elections/{id}", r.RequireAuth(r.electionsH.Get))
	r.mux.Handle("GET /api/v1/elections/{id}/ballot", r.RequireAuth(r.electionsH.BallotMeta))

	r.mux.Handle("POST /api/v1/elections", r.RequireRole("admin", r.electionsH.Create))
	r.mux.Handle("PUT /api/v1/elections/{id}/rules", r.RequireRole("admin", r.electionsH.UpdateRules))
	r.mux.Handle("POST /api/v1/elections/{id}/actions/{action}", r.RequireRole("admin", r.electionsH.Action))

	r.mux.Handle("POST /api/v1/elections/{id}/invites", r.RequireRole("admin", r.electionsH.CreateInvite))
	r.mux.Handle("GET /api/v1/elections/{id}/invites", r.RequireRole("admin", r.electionsH.ListInvites))

	r.mux.Handle("POST /api/v1/elections/candidates/import", r.RequireRole("admin", r.electionsH.ImportCandidates))
	r.mux.Handle("POST /api/v1/elections/{id}/invites/import", r.RequireRole("admin", r.electionsH.ImportInvites))
}