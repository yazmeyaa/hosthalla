package handlers

import (
	"net/http"

	"github.com/yazmeyaa/hosthalla/internal/host/storage/postgres"
	"github.com/yazmeyaa/hosthalla/ui/pages/index_page"
)

type IndexHandler struct {
	r *postgres.HostRepositoryPostgresImpl
}

func NewIndexHandler(r *postgres.HostRepositoryPostgresImpl) *IndexHandler {
	return &IndexHandler{r}
}

func (h *IndexHandler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hosts, err := h.r.ListHosts(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index_page.IndexPage(hosts).Render(ctx, w)
}
