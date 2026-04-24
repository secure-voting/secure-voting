package httpserver

func registerSettingsRoutes(r *routeCtx) {
	r.mux.Handle("GET /api/v1/admin/settings", r.RequireAdminTrusted(r.adminSettingsH.Get))
	r.mux.Handle("PUT /api/v1/admin/settings", r.LimitWrite(r.RequireAdminTrusted(r.adminSettingsH.Update)))
}
