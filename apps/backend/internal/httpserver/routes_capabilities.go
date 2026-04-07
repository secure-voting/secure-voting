package httpserver

func registerCapabilitiesRoutes(r *routeCtx) {
	r.mux.Handle("GET /api/v1/capabilities/tally-rules", r.RequireAuth(r.capabilitiesH.ListTallyRules))
}