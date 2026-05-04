package httpserver

func registerAuthRoutes(r *routeCtx) {
	r.mux.Handle("POST /api/v1/auth/register", r.WrapAuthLimited(r.authH.Register))
	r.mux.Handle("POST /api/v1/auth/login", r.WrapAuthLimited(r.authH.Login))
	r.mux.Handle("POST /api/v1/auth/refresh", r.WrapAuthLimited(r.authH.Refresh))
	r.mux.Handle("POST /api/v1/auth/email/verification/confirm", r.RequireAuthLimited(r.authH.ConfirmEmailVerification))

	r.mux.Handle("GET /api/v1/auth/me", r.RequireAuth(r.authH.Me))
	r.mux.Handle("POST /api/v1/auth/logout", r.RequireAuth(r.authH.Logout))

	r.mux.Handle("POST /api/v1/auth/change-password", r.RequireAuthLimited(r.authH.ChangePassword))
	r.mux.Handle("PATCH /api/v1/auth/profile", r.RequireAuthLimited(r.authH.UpdateProfile))
	r.mux.Handle("POST /api/v1/auth/email/verification/request", r.RequireAuthLimited(r.authH.RequestEmailVerification))
	r.mux.Handle("POST /api/v1/auth/invite/accept", r.RequireAuthLimited(r.authH.AcceptInvite))
}
