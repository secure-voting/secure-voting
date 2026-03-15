package httpserver

func registerAuthRoutes(r *routeCtx) {
	r.mux.Handle("POST /api/v1/auth/register", r.Wrap(r.authH.Register))
	r.mux.Handle("POST /api/v1/auth/login", r.Wrap(r.authH.Login))

	r.mux.Handle("GET /api/v1/auth/me", r.RequireAuth(r.authH.Me))
	r.mux.Handle("POST /api/v1/auth/logout", r.RequireAuth(r.authH.Logout))
}
