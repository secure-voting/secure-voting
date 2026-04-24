package httpserver

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

func Routes(cfg config.Config, db *pgxpool.Pool, rdb *redis.Client, mdb *mongo.Database) http.Handler {
	r := newRouteCtx(cfg, db, rdb, mdb)

	registerHealth(r)
	registerAuthRoutes(r)
	registerAdminRoutes(r)
	registerSettingsRoutes(r)
	registerElectionRoutes(r)
	registerBallotRoutes(r)
	registerResultsRoutes(r)
	registerResearchRoutes(r)
	registerCapabilitiesRoutes(r)
	registerNotificationRoutes(r)

	return middleware.SecurityHeaders(r.mux)
}
