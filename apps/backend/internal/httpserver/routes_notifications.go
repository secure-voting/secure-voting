package httpserver

func registerNotificationRoutes(r *routeCtx) {
	r.mux.Handle("GET /api/v1/notifications", r.RequireAuth(r.notificationsH.List))
	r.mux.Handle("POST /api/v1/notifications", r.LimitWrite(r.RequireAuth(r.notificationsH.Create)))
	r.mux.Handle("POST /api/v1/notifications/{id}/read", r.LimitWrite(r.RequireAuth(r.notificationsH.MarkRead)))
	r.mux.Handle("POST /api/v1/notifications/read-all", r.LimitWrite(r.RequireAuth(r.notificationsH.MarkAllRead)))
	r.mux.Handle("DELETE /api/v1/notifications/{id}", r.LimitWrite(r.RequireAuth(r.notificationsH.Delete)))
	r.mux.Handle("DELETE /api/v1/notifications", r.LimitWrite(r.RequireAuth(r.notificationsH.ClearAll)))
}
