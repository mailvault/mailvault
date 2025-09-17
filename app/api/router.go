package api

import (
	"fmt"
	"mailvault/app/api/middleware"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func Router() *chi.Mux {
	r := chi.NewRouter()
	r.Use(
		chiMiddleware.RedirectSlashes,
		cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: false,
			MaxAge:           300,
		}),
		chiMiddleware.Logger,
		chiMiddleware.Recoverer,
	)

	// Initialize audit middleware
	auditMw := middleware.NewAuditMiddleware(middleware.DefaultAuditConfig())
	securityMw := middleware.NewSecurityMiddleware(middleware.DefaultSecurityConfig())
	metricsMw := middleware.NewMetricsMiddleware(middleware.DefaultMetricsConfig())

	r.Use(securityMw.SecurityHeaders())
	r.Use(securityMw.CORS())
	r.Use(auditMw.AuditLog())
	r.Use(metricsMw.MetricsHandler())

	// Swagger documentation routes
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL(fmt.Sprintf("http://%s/swagger/doc.json", "localhost:3000")),
	))

	return r
}
