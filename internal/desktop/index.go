package desktop

import (
	"net/http"

	"github.com/bornholm/corpus/internal/desktop/settings"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"

	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
)

func (h *Handler) handleIndexPage(w http.ResponseWriter, r *http.Request) {
	st, err := h.store.Get(true)
	if err != nil {
		common.HandleError(w, r, err)
		return
	}

	ctx := r.Context()

	var redirectURL string

	if len(st.Servers) > 0 {
		var preferredServer *settings.Server
		for _, s := range st.Servers {
			if !s.Preferred {
				continue
			}

			preferredServer = &s
			break
		}

		if preferredServer == nil {
			redirectURL = string(commonComp.BaseURL(ctx, commonComp.WithPath("/servers")))
		} else {
			h.setCurrentServer(preferredServer)

			redirectURL = string(commonComp.BaseURL(ctx, commonComp.WithPath("/portal")))
		}

	} else {
		redirectURL = string(commonComp.BaseURL(ctx, commonComp.WithPath("/servers")))
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
