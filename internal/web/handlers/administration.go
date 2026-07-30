package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	"github.com/yazmeyaa/hosthalla/internal/authentication"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/version"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/pages/administration_page"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/layout"
)

type AdministrationHandler struct {
	authService  *auth_service.Service
	agentService *agent.Service
	hostService  *host.Service
	logger       *slog.Logger
}

type NewAdministrationHandlerParams struct {
	AuthService  *auth_service.Service
	AgentService *agent.Service
	HostService  *host.Service
	Logger       *slog.Logger
}

func NewAdministrationHandler(params NewAdministrationHandlerParams) *AdministrationHandler {
	return &AdministrationHandler{authService: params.AuthService, agentService: params.AgentService, hostService: params.HostService, logger: params.Logger}
}

func (h *AdministrationHandler) Administration(w http.ResponseWriter, r *http.Request) {
	section := r.PathValue("section")
	props, ok := h.administrationPageProps(w, r, section)
	if !ok {
		return
	}
	props.Notice = administrationNotice(r)

	h.renderAdministrationPage(w, r, props)
}

func (h *AdministrationHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderAdministrationFormError(w, r, "users", err)
		return
	}

	if _, err := h.authService.CreateUser(r.Context(), auth_service.CreateUserDTO{
		Username: r.FormValue("username"),
		Password: r.FormValue("password"),
	}); err != nil {
		h.renderAdministrationFormError(w, r, "users", err)
		return
	}

	http.Redirect(w, r, "/administration/users?created=1", http.StatusSeeOther)
}

func (h *AdministrationHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderAdministrationFormError(w, r, "users", err)
		return
	}

	if _, err := h.authService.UpdateUser(r.Context(), auth_service.UpdateUserDTO{
		ID:       r.PathValue("id"),
		Username: r.FormValue("username"),
	}); err != nil {
		h.renderAdministrationFormError(w, r, "users", err)
		return
	}

	http.Redirect(w, r, "/administration/users?updated=1", http.StatusSeeOther)
}

func (h *AdministrationHandler) SetUserPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderAdministrationFormError(w, r, "users", err)
		return
	}

	if _, err := h.authService.SetPassword(r.Context(), auth_service.SetPasswordDTO{
		ProfileID: r.PathValue("id"),
		Password:  r.FormValue("password"),
	}); err != nil {
		h.renderAdministrationFormError(w, r, "users", err)
		return
	}

	http.Redirect(w, r, "/administration/users?password=1", http.StatusSeeOther)
}

func (h *AdministrationHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session for delete user", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	userID := r.PathValue("id")
	if userID == session.ProfileID {
		h.renderAdministrationFormError(w, r, "users", errors.New("you cannot delete your own user"))
		return
	}
	if err := h.authService.DeleteUser(r.Context(), userID); err != nil {
		h.renderAdministrationFormError(w, r, "users", err)
		return
	}

	http.Redirect(w, r, "/administration/users?deleted=1", http.StatusSeeOther)
}

func (h *AdministrationHandler) administrationPageProps(w http.ResponseWriter, r *http.Request, section string) (administration_page.AdministrationPageProps, bool) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session for administration page", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return administration_page.AdministrationPageProps{}, false
	}

	profile, err := h.authService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		h.logger.Error("failed to load profile for administration page", slog.String("profile_id", session.ProfileID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return administration_page.AdministrationPageProps{}, false
	}

	pageProps := administration_page.AdministrationPageProps{
		Profile:                   profile,
		CurrentProfileID:          profile.ID,
		ActiveSection:             section,
		AgentStatusFilter:         r.URL.Query().Get("agent_status"),
		AgentSearchQuery:          r.URL.Query().Get("agent_q"),
		AppVersion:                version.VersionString(),
		LatestSessionsByProfileID: map[string]authentication.Session{},
	}

	pageProps.Users, err = h.authService.ListUsers(r.Context())
	if err != nil {
		h.logger.Warn("failed to list users for administration page", slog.String("error", err.Error()))
		pageProps.DataErrors = append(pageProps.DataErrors, "Users could not be loaded.")
	}

	pageProps.APITokens, err = h.authService.ListAPITokens(r.Context())
	if err != nil {
		h.logger.Warn("failed to list api tokens for administration page", slog.String("error", err.Error()))
		pageProps.DataErrors = append(pageProps.DataErrors, "API tokens could not be loaded.")
	}

	if h.agentService != nil {
		pageProps.Agents, err = h.agentService.ListAgents(r.Context())
		if err != nil {
			h.logger.Warn("failed to list agents for administration page", slog.String("error", err.Error()))
			pageProps.DataErrors = append(pageProps.DataErrors, "Agents could not be loaded.")
		}
		pageProps.AgentConfigs = make(map[string]agent.AgentConfig, len(pageProps.Agents))
		for _, currentAgent := range pageProps.Agents {
			config, err := h.agentService.GetConfigByAgentID(r.Context(), currentAgent.ID)
			if err == nil {
				pageProps.AgentConfigs[currentAgent.ID.String()] = config
				continue
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				h.logger.Warn("failed to load agent config for administration page", slog.String("agent_id", currentAgent.ID.String()), slog.String("error", err.Error()))
			}
		}
	}

	if h.hostService != nil {
		pageProps.Hosts, err = h.hostService.ListHosts(r.Context(), host.ListHostsFilter{})
		if err != nil {
			h.logger.Warn("failed to list hosts for administration page", slog.String("error", err.Error()))
			pageProps.DataErrors = append(pageProps.DataErrors, "Hosts could not be loaded.")
		}
		pageProps.HostsByID = make(map[string]host.Host, len(pageProps.Hosts))
		hostIDs := make([]uuid.UUID, 0, len(pageProps.Hosts))
		for _, currentHost := range pageProps.Hosts {
			pageProps.HostsByID[currentHost.ID.String()] = currentHost
			hostIDs = append(hostIDs, currentHost.ID)
		}
		latestSnapshots, err := h.hostService.ListLatestHostMetricSnapshotsByHostIDs(r.Context(), hostIDs)
		if err == nil {
			pageProps.LatestMetricSnapshots = make(map[string]host.HostMetricSnapshot, len(latestSnapshots))
			for hostID, snapshot := range latestSnapshots {
				pageProps.LatestMetricSnapshots[hostID.String()] = snapshot
			}
		} else {
			h.logger.Warn("failed to list latest metric snapshots for administration page", slog.String("error", err.Error()))
			pageProps.DataErrors = append(pageProps.DataErrors, "Metrics could not be loaded.")
		}
	}

	for _, user := range pageProps.Users {
		latestSession, err := h.authService.GetSessionByProfileID(r.Context(), user.ID)
		if err == nil {
			pageProps.LatestSessionsByProfileID[user.ID] = latestSession
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			h.logger.Warn("failed to load latest user session for administration page", slog.String("profile_id", user.ID), slog.String("error", err.Error()))
		}
	}

	h.logger.Debug("rendering administration page", slog.String("profile_id", profile.ID), slog.String("section", section))
	return pageProps, true
}

func (h *AdministrationHandler) renderAdministrationPage(w http.ResponseWriter, r *http.Request, pageProps administration_page.AdministrationPageProps) {
	if isHTMXBoostedNavigationRequest(r) {
		layout.AppContent().Render(templ.WithChildren(r.Context(), administration_page.AdministrationPageContent(pageProps)), w)
		return
	}

	administration_page.AdministrationPage(pageProps).Render(r.Context(), w)
}

func (h *AdministrationHandler) renderAdministrationFormError(w http.ResponseWriter, r *http.Request, section string, err error) {
	props, ok := h.administrationPageProps(w, r, section)
	if !ok {
		return
	}
	props.Notice = administration_page.AdministrationNotice{Variant: "danger", Message: err.Error()}
	w.WriteHeader(http.StatusBadRequest)
	h.renderAdministrationPage(w, r, props)
}

func (h *AdministrationHandler) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session for create api token", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.authService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		h.logger.Error("failed to load profile for create api token", slog.String("profile_id", session.ProfileID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Warn("invalid create api token payload", slog.String("profile_id", profile.ID), slog.String("error", err.Error()))
		h.renderAdministrationFormError(w, r, "tokens", err)
		return
	}

	expiresIn, err := parseTokenExpiresInDays(r.FormValue("expiresInDays"))
	if err != nil {
		h.logger.Warn("invalid expiresInDays value", slog.String("profile_id", profile.ID), slog.String("error", err.Error()))
		h.renderAdministrationFormError(w, r, "tokens", err)
		return
	}

	createdToken, err := h.authService.CreateAPIToken(r.Context(), auth_service.CreateAPITokenDTO{
		ProfileID: profile.ID,
		Name:      r.FormValue("name"),
		Scopes:    parseScopes(r.Form["scope"]),
		ExpiresIn: expiresIn,
	})
	if err != nil {
		h.logger.Warn("failed to create api token", slog.String("profile_id", profile.ID), slog.String("error", err.Error()))
		h.renderAdministrationFormError(w, r, "tokens", err)
		return
	}

	h.logger.Info("api token created", slog.String("profile_id", profile.ID), slog.String("token_id", createdToken.Token.ID))

	props, ok := h.administrationPageProps(w, r, "tokens")
	if !ok {
		return
	}
	props.CreatedAPIToken = createdToken.PlainToken
	props.Notice = administration_page.AdministrationNotice{Variant: "success", Message: "API token created."}
	h.renderAdministrationPage(w, r, props)
}

func (h *AdministrationHandler) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.authService.GetAPITokenByID(r.Context(), r.PathValue("id"))
	if err != nil {
		h.logger.Warn("api token not found for revoke", slog.String("token_id", r.PathValue("id")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := h.authService.RevokeAPIToken(r.Context(), token.ID); err != nil {
		h.logger.Warn("failed to revoke api token", slog.String("token_id", token.ID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.logger.Warn("api token revoked", slog.String("token_id", token.ID), slog.String("profile_id", token.ProfileID))

	http.Redirect(w, r, "/administration/tokens?revoked=1", http.StatusSeeOther)
}

func parseScopes(rawScopes []string) []string {
	return rawScopes
}

func parseTokenExpiresInDays(rawValue string) (time.Duration, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return 0, nil
	}

	days, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if days < 0 {
		return 0, errors.New("expiresInDays must be greater or equal to 0")
	}
	return time.Duration(days) * 24 * time.Hour, nil
}

func administrationNotice(r *http.Request) administration_page.AdministrationNotice {
	query := r.URL.Query()
	switch {
	case query.Get("created") == "1":
		return administration_page.AdministrationNotice{Variant: "success", Message: "User created."}
	case query.Get("updated") == "1":
		return administration_page.AdministrationNotice{Variant: "success", Message: "User updated."}
	case query.Get("password") == "1":
		return administration_page.AdministrationNotice{Variant: "success", Message: "Password updated."}
	case query.Get("deleted") == "1":
		return administration_page.AdministrationNotice{Variant: "success", Message: "User deleted."}
	case query.Get("revoked") == "1":
		return administration_page.AdministrationNotice{Variant: "success", Message: "API token revoked."}
	default:
		return administration_page.AdministrationNotice{}
	}
}
