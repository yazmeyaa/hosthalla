package web

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	authentication_repository "github.com/yazmeyaa/hosthalla/internal/authentication/storage"
	"github.com/yazmeyaa/hosthalla/internal/events"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/web/handlers"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	ui_assets "github.com/yazmeyaa/hosthalla/ui/app/assets"
	administration_page "github.com/yazmeyaa/hosthalla/ui/pages/administration_page"
	dashboard_page "github.com/yazmeyaa/hosthalla/ui/pages/dashboard"
	help_page "github.com/yazmeyaa/hosthalla/ui/pages/help_page"
	hosts_page "github.com/yazmeyaa/hosthalla/ui/pages/hosts_page"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/layout"
)

type NewRouterParams struct {
	HostService       *host.Service
	AgentService      *agent.Service
	SessionRepository authentication_repository.SessionRepository
	AuthService       *auth_service.Service
	Logger            *slog.Logger
	EventBus          events.EventBus
	WebOrigin         string
}

func NewRouter(params NewRouterParams) http.Handler {
	indexHandler := handlers.NewIndexHandler(params.HostService, params.Logger, params.AuthService)
	authHandler := handlers.NewAuthHandler(params.Logger, params.AuthService)
	hostHandler := handlers.NewHostsHandler(params.HostService, params.AuthService, params.Logger, params.WebOrigin)
	administrationHandler := handlers.NewAdministrationHandler(handlers.NewAdministrationHandlerParams{
		AuthService:  params.AuthService,
		AgentService: params.AgentService,
		HostService:  params.HostService,
		Logger:       params.Logger,
	})
	dashboardHandler := handlers.NewDashboardHandler(handlers.DashboardHandlerParams{
		Logger:         params.Logger,
		HostService:    params.HostService,
		ProfileService: params.AuthService,
		EventBus:       params.EventBus,
	})
	helpHandler := handlers.NewHelpHandler(params.AuthService, params.Logger)

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", staticAssetsHandler(http.StripPrefix("/assets/", http.FileServer(http.FS(ui_assets.Files)))))

	mux.Handle("GET /", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(indexHandler.Index)))
	mux.Handle("GET /dashboard", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(dashboardHandler.Dashboard)))
	mux.Handle("GET /help", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(helpHandler.Help)))
	mux.Handle("GET /help/{topic}", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(helpHandler.Help)))

	mux.HandleFunc("GET /auth", authHandler.Auth)
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.Handle("POST /auth/logout", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(authHandler.Logout)))

	mux.Handle("GET /hosts", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.ListHosts)))
	mux.Handle("GET /hosts/export", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.ExportHosts)))
	mux.Handle("POST /hosts/create", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.CreateHost)))
	mux.Handle("POST /hosts/import", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.ImportHosts)))
	mux.Handle("POST /hosts/{id}/update", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.UpdateHost)))
	mux.Handle("POST /hosts/{id}/delete", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.DeleteHost)))
	mux.Handle("POST /hosts/{id}/ping", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.PingHost)))
	mux.Handle("POST /hosts/{id}/management-methods/create", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.CreateHostManagementMethod)))
	mux.Handle("POST /hosts/{id}/management-methods/{methodID}/secret", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.GetHostManagementMethodSecret)))
	mux.Handle("POST /hosts/{id}/agent/register-command", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.CreateAgentRegisterCommand)))
	mux.Handle("POST /hosts/ping-all", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(hostHandler.PingAllHosts)))

	mux.Handle("GET /administration", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.Administration)))
	mux.Handle("GET /administration/{section}", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.Administration)))
	mux.Handle("POST /administration/users/create", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.CreateUser)))
	mux.Handle("POST /administration/users/{id}/update", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.UpdateUser)))
	mux.Handle("POST /administration/users/{id}/password", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.SetUserPassword)))
	mux.Handle("POST /administration/users/{id}/delete", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.DeleteUser)))
	mux.Handle("POST /administration/api-tokens/create", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.CreateAPIToken)))
	mux.Handle("POST /administration/api-tokens/{id}/revoke", middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(administrationHandler.RevokeAPIToken)))

	csrfProtection := http.NewCrossOriginProtection()
	protectedRoutes := csrfProtection.Handler(middlewares.RequestLoggingMiddleware(params.Logger, mux))

	rootMux := http.NewServeMux()
	rootMux.Handle(
		"GET /dashboard/subscribe",
		middlewares.RequestLoggingMiddleware(
			params.Logger,
			middlewares.AuthMiddleware(params.SessionRepository, http.HandlerFunc(dashboardHandler.SubscribeToDashboard)),
		),
	)
	rootMux.Handle("/", protectedRoutes)

	return templ.NewCSSMiddleware(rootMux, cssClasses()...)
}

func cssClasses() []templ.CSSClass {
	classes := layout.CSSClasses()
	classes = append(classes, dashboard_page.CSSClasses()...)
	classes = append(classes, hosts_page.CSSClasses()...)
	classes = append(classes, administration_page.CSSClasses()...)
	classes = append(classes, help_page.CSSClasses()...)
	return classes
}

func staticAssetsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/assets/")
		if strings.HasPrefix(path, "fonts/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		next.ServeHTTP(w, r)
	})
}
