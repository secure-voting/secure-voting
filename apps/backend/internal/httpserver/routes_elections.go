package httpserver

func registerElectionRoutes(r *routeCtx) {
	r.mux.Handle("GET /api/v1/elections", r.RequireAuth(r.electionsH.List))
	r.mux.Handle("GET /api/v1/elections/{id}", r.RequireAuth(r.electionsH.Get))
	r.mux.Handle("GET /api/v1/elections/{id}/ballot", r.RequireAuth(r.electionsH.BallotMeta))

	r.mux.Handle("POST /api/v1/elections", r.RequireAdminTrusted(r.electionsH.Create))
	r.mux.Handle("PUT /api/v1/elections/{id}/rules", r.RequireAdminTrusted(r.electionsH.UpdateRules))
	r.mux.Handle("POST /api/v1/elections/{id}/actions/{action}", r.RequireAdminTrusted(r.electionsH.Action))

	r.mux.Handle("POST /api/v1/elections/{id}/invites", r.RequireAdminTrusted(r.electionsH.CreateInvite))
	r.mux.Handle("GET /api/v1/elections/{id}/invites", r.RequireAdminTrusted(r.electionsH.ListInvites))

	r.mux.Handle("POST /api/v1/elections/candidates/import", r.RequireAdminTrusted(r.electionsH.ImportCandidates))
	r.mux.Handle("POST /api/v1/elections/{id}/invites/import", r.RequireAdminTrusted(r.electionsH.ImportInvites))
}
