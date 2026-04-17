package httpserver

func registerAdminRoutes(r *routeCtx) {
	r.mux.Handle("GET /api/v1/admin/users", r.RequireAdminTrusted(r.adminUsersH.List))
	r.mux.Handle("PATCH /api/v1/admin/users/{id}/role", r.RequireAdminTrusted(r.adminUsersH.UpdateRole))
}
