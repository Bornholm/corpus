package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/a-h/templ"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/admin/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
	documentTask "github.com/bornholm/corpus/internal/task/document"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/pkg/errors"
)

// --- Page handlers ---

func (h *Handler) getFilesystemSourcesPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillFilesystemSourcesPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}
	templ.Handler(component.FilesystemSourcesPage(*vmodel)).ServeHTTP(w, r)
}

func (h *Handler) getNewFilesystemSourcePage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillNewFilesystemSourcePageViewModel(r, "")
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}
	templ.Handler(component.NewFilesystemSourcePage(*vmodel)).ServeHTTP(w, r)
}

// getBackendFormPartial serves the per-backend form fields as an HTMX partial.
func (h *Handler) getBackendFormPartial(w http.ResponseWriter, r *http.Request) {
	backendType := r.URL.Query().Get("backend_type")
	vmodel := component.BackendFormVModel{BackendType: backendType}
	templ.Handler(component.BackendFormFields(vmodel)).ServeHTTP(w, r)
}

func (h *Handler) postFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	label := r.FormValue("label")
	backendType := r.FormValue("backend_type")

	if label == "" || backendType == "" {
		vmodel, _ := h.fillNewFilesystemSourcePageViewModel(r, "Le libellé et le type de backend sont obligatoires.")
		if vmodel != nil {
			templ.Handler(component.NewFilesystemSourcePage(*vmodel)).ServeHTTP(w, r)
		}
		return
	}

	backendConfig, err := parseBackendConfigFromForm(r, backendType)
	if err != nil {
		vmodel, _ := h.fillNewFilesystemSourcePageViewModel(r, "Erreur de configuration du backend : "+err.Error())
		if vmodel != nil {
			templ.Handler(component.NewFilesystemSourcePage(*vmodel)).ServeHTTP(w, r)
		}
		return
	}

	collections := r.Form["collections"]
	collectionIDs := make([]model.CollectionID, len(collections))
	for i, c := range collections {
		collectionIDs[i] = model.CollectionID(c)
	}

	opts := parseFilesystemSourceOptions(r)
	syncInterval := parseFilesystemSourceSyncInterval(r)

	if _, err := h.filesystemSourceStore.CreateFilesystemSource(ctx, label, backendType, backendConfig, collectionIDs, opts, syncInterval); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/filesystem-sources"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) getFilesystemSourcePage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillFilesystemSourcePageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}
	templ.Handler(component.FilesystemSourcePage(*vmodel)).ServeHTTP(w, r)
}

func (h *Handler) getEditFilesystemSourcePage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillEditFilesystemSourcePageViewModel(r, "")
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}
	templ.Handler(component.EditFilesystemSourcePage(*vmodel)).ServeHTTP(w, r)
}

func (h *Handler) postEditFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	id := model.FilesystemSourceID(r.PathValue("id"))
	updates := port.FilesystemSourceUpdates{}

	labelVal := r.FormValue("label")
	if labelVal != "" {
		updates.Label = &labelVal
	}

	backendTypeVal := r.FormValue("backend_type")
	if backendTypeVal != "" {
		updates.BackendType = &backendTypeVal
		backendConfig, err := parseBackendConfigFromForm(r, backendTypeVal)
		if err != nil {
			vmodel, _ := h.fillEditFilesystemSourcePageViewModel(r, "Erreur de configuration du backend : "+err.Error())
			if vmodel != nil {
				templ.Handler(component.EditFilesystemSourcePage(*vmodel)).ServeHTTP(w, r)
			}
			return
		}
		updates.BackendConfig = &backendConfig
	}

	collections := r.Form["collections"]
	collIDs := make([]model.CollectionID, len(collections))
	for i, c := range collections {
		collIDs[i] = model.CollectionID(c)
	}
	updates.CollectionIDs = collIDs

	opts := parseFilesystemSourceOptions(r)
	updates.Options = &opts

	syncInterval := parseFilesystemSourceSyncInterval(r)
	updates.SyncInterval = &syncInterval

	if _, err := h.filesystemSourceStore.UpdateFilesystemSource(ctx, id, updates); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, common.NewHTTPError(http.StatusNotFound))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/filesystem-sources", string(id)))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) postDeleteFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := model.FilesystemSourceID(r.PathValue("id"))

	if err := h.filesystemSourceStore.DeleteFilesystemSource(ctx, id); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, common.NewHTTPError(http.StatusNotFound))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/filesystem-sources"))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

func (h *Handler) postSyncFilesystemSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := model.FilesystemSourceID(r.PathValue("id"))

	if _, err := h.filesystemSourceStore.GetFilesystemSourceByID(ctx, id); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			common.HandleError(w, r, common.NewHTTPError(http.StatusNotFound))
			return
		}
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	user := httpCtx.User(ctx)
	syncTask := documentTask.NewSyncFilesystemSourceTask(user, id)

	if err := h.taskRunner.ScheduleTask(ctx, syncTask); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	redirectURL := commonComp.BaseURL(r.Context(), commonComp.WithPath("/admin/tasks", string(syncTask.ID())))
	http.Redirect(w, r, string(redirectURL), http.StatusSeeOther)
}

// --- ViewModel builders ---

func (h *Handler) fillFilesystemSourcesPageViewModel(r *http.Request) (*component.FilesystemSourcesPageVModel, error) {
	vmodel := &component.FilesystemSourcesPageVModel{}
	ctx := r.Context()

	if err := common.FillViewModel(ctx, vmodel, r,
		h.fillFilesystemSourcesAppLayout,
		h.fillFilesystemSourcesList,
	); err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillFilesystemSourcesAppLayout(ctx context.Context, vmodel *component.FilesystemSourcesPageVModel, r *http.Request) error {
	return fillAdminAppLayout(ctx, &vmodel.AppLayoutVModel, "filesystem-sources")
}

func (h *Handler) fillFilesystemSourcesList(ctx context.Context, vmodel *component.FilesystemSourcesPageVModel, r *http.Request) error {
	page := 0
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n - 1
		}
	}
	limit := 20

	sources, total, err := h.filesystemSourceStore.QueryFilesystemSources(ctx, page, limit)
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.Sources = sources
	vmodel.CurrentPage = page + 1
	vmodel.PageSize = limit
	vmodel.TotalSources = int(total)

	return nil
}

func (h *Handler) fillFilesystemSourcePageViewModel(r *http.Request) (*component.FilesystemSourcePageVModel, error) {
	vmodel := &component.FilesystemSourcePageVModel{}
	ctx := r.Context()

	if err := common.FillViewModel(ctx, vmodel, r,
		h.fillFilesystemSourceAppLayout,
		h.fillFilesystemSourceDetail,
	); err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillFilesystemSourceAppLayout(ctx context.Context, vmodel *component.FilesystemSourcePageVModel, r *http.Request) error {
	return fillAdminAppLayout(ctx, &vmodel.AppLayoutVModel, "filesystem-sources")
}

func (h *Handler) fillFilesystemSourceDetail(ctx context.Context, vmodel *component.FilesystemSourcePageVModel, r *http.Request) error {
	id := model.FilesystemSourceID(r.PathValue("id"))
	src, err := h.filesystemSourceStore.GetFilesystemSourceByID(ctx, id)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return common.NewHTTPError(http.StatusNotFound)
		}
		return errors.WithStack(err)
	}
	vmodel.Source = src
	return nil
}

func (h *Handler) fillNewFilesystemSourcePageViewModel(r *http.Request, errMsg string) (*component.NewFilesystemSourcePageVModel, error) {
	vmodel := &component.NewFilesystemSourcePageVModel{Error: errMsg}
	ctx := r.Context()

	if err := common.FillViewModel(ctx, vmodel, r,
		h.fillNewFilesystemSourceAppLayout,
		h.fillNewFilesystemSourceCollections,
	); err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillNewFilesystemSourceAppLayout(ctx context.Context, vmodel *component.NewFilesystemSourcePageVModel, r *http.Request) error {
	return fillAdminAppLayout(ctx, &vmodel.AppLayoutVModel, "filesystem-sources")
}

func (h *Handler) fillNewFilesystemSourceCollections(ctx context.Context, vmodel *component.NewFilesystemSourcePageVModel, r *http.Request) error {
	colls, err := h.documentStore.QueryCollections(ctx, port.QueryCollectionsOptions{HeaderOnly: true})
	if err != nil {
		return errors.WithStack(err)
	}
	vmodel.Collections = colls
	return nil
}

func (h *Handler) fillEditFilesystemSourcePageViewModel(r *http.Request, errMsg string) (*component.EditFilesystemSourcePageVModel, error) {
	vmodel := &component.EditFilesystemSourcePageVModel{Error: errMsg}
	ctx := r.Context()

	if err := common.FillViewModel(ctx, vmodel, r,
		h.fillEditFilesystemSourceAppLayout,
		h.fillEditFilesystemSourceData,
		h.fillEditFilesystemSourceCollections,
	); err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillEditFilesystemSourceAppLayout(ctx context.Context, vmodel *component.EditFilesystemSourcePageVModel, r *http.Request) error {
	return fillAdminAppLayout(ctx, &vmodel.AppLayoutVModel, "filesystem-sources")
}

func (h *Handler) fillEditFilesystemSourceData(ctx context.Context, vmodel *component.EditFilesystemSourcePageVModel, r *http.Request) error {
	id := model.FilesystemSourceID(r.PathValue("id"))
	src, err := h.filesystemSourceStore.GetFilesystemSourceByID(ctx, id)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			return common.NewHTTPError(http.StatusNotFound)
		}
		return errors.WithStack(err)
	}
	vmodel.Source = src

	if len(src.BackendConfig()) > 0 {
		var values map[string]any
		if err := json.Unmarshal(src.BackendConfig(), &values); err == nil {
			vmodel.BackendValues = values
		}
	}

	return nil
}

func (h *Handler) fillEditFilesystemSourceCollections(ctx context.Context, vmodel *component.EditFilesystemSourcePageVModel, r *http.Request) error {
	colls, err := h.documentStore.QueryCollections(ctx, port.QueryCollectionsOptions{HeaderOnly: true})
	if err != nil {
		return errors.WithStack(err)
	}
	vmodel.Collections = colls
	return nil
}

// fillAdminAppLayout is a shared helper to populate AppLayoutVModel for admin pages.
func fillAdminAppLayout(ctx context.Context, layout *commonComp.AppLayoutVModel, selectedItem string) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	isAdmin := slices.Contains(user.Roles(), authz.RoleAdmin)
	*layout = commonComp.AppLayoutVModel{
		User:         user,
		IsAdmin:      isAdmin,
		SelectedItem: selectedItem,
		NavigationItems: func(vmodel commonComp.AppLayoutVModel) templ.Component {
			return commonComp.AdminNavigationItems(vmodel)
		},
		FooterItems: func(vmodel commonComp.AppLayoutVModel) templ.Component {
			return commonComp.AdminFooterItems(vmodel)
		},
	}

	return nil
}

// --- Form parsing helpers ---

// parseBackendConfigFromForm builds a JSON backend config from multipart form values.
func parseBackendConfigFromForm(r *http.Request, backendType string) (json.RawMessage, error) {
	m := map[string]any{}

	switch backendType {
	case "local":
		m["path"] = r.FormValue("backend_path")

	case "sftp":
		m["host"] = r.FormValue("backend_host")
		if p := r.FormValue("backend_port"); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				m["port"] = n
			}
		}
		m["username"] = r.FormValue("backend_username")
		if pw := r.FormValue("backend_password"); pw != "" {
			m["password"] = pw
		} else if existing := r.FormValue("backend_password_existing"); existing != "" {
			m["password"] = existing
		}
		if pk, err := parseFileRefFromForm(r, "backend_private_key"); err == nil && pk != nil {
			m["privateKey"] = pk
		}
		if passphrase := r.FormValue("backend_private_key_passphrase"); passphrase != "" {
			m["privateKeyPassphrase"] = passphrase
		}
		if hk, err := parseFileRefFromForm(r, "backend_host_key"); err == nil && hk != nil {
			m["hostKey"] = hk
		}
		m["insecureIgnoreHostKey"] = r.FormValue("backend_insecure_ignore_host_key") == "true"
		if bp := r.FormValue("backend_base_path"); bp != "" {
			m["basePath"] = bp
		}
		if t := r.FormValue("backend_timeout"); t != "" {
			m["timeout"] = t
		}

	case "ftp":
		m["host"] = r.FormValue("backend_host")
		if p := r.FormValue("backend_port"); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				m["port"] = n
			}
		}
		if u := r.FormValue("backend_username"); u != "" {
			m["username"] = u
		}
		if pw := r.FormValue("backend_password"); pw != "" {
			m["password"] = pw
		}
		if bp := r.FormValue("backend_base_path"); bp != "" {
			m["basePath"] = bp
		}
		if t := r.FormValue("backend_timeout"); t != "" {
			m["timeout"] = t
		}

	case "smb":
		m["host"] = r.FormValue("backend_host")
		if p := r.FormValue("backend_port"); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				m["port"] = n
			}
		}
		m["share"] = r.FormValue("backend_share")
		if u := r.FormValue("backend_username"); u != "" {
			m["username"] = u
		}
		if pw := r.FormValue("backend_password"); pw != "" {
			m["password"] = pw
		}
		if d := r.FormValue("backend_domain"); d != "" {
			m["domain"] = d
		}
		if ws := r.FormValue("backend_workstation"); ws != "" {
			m["workstation"] = ws
		}
		if bp := r.FormValue("backend_base_path"); bp != "" {
			m["basePath"] = bp
		}

	case "webdav":
		m["host"] = r.FormValue("backend_host")
		if p := r.FormValue("backend_port"); p != "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 {
				m["port"] = n
			}
		}
		if path := r.FormValue("backend_path"); path != "" {
			m["path"] = path
		}
		m["useTLS"] = r.FormValue("backend_use_tls") == "true"
		if u := r.FormValue("backend_username"); u != "" {
			m["username"] = u
		}
		if pw := r.FormValue("backend_password"); pw != "" {
			m["password"] = pw
		} else if existing := r.FormValue("backend_password_existing"); existing != "" {
			m["password"] = existing
		}
		if t := r.FormValue("backend_timeout"); t != "" {
			m["timeout"] = t
		}

	case "minio":
		m["endpoint"] = r.FormValue("backend_endpoint")
		m["accessKey"] = r.FormValue("backend_access_key")
		if sk := r.FormValue("backend_secret_key"); sk != "" {
			m["secretKey"] = sk
		} else if existing := r.FormValue("backend_secret_key_existing"); existing != "" {
			m["secretKey"] = existing
		}
		if bucket := r.FormValue("backend_bucket"); bucket != "" {
			m["bucket"] = bucket
		}
		if region := r.FormValue("backend_region"); region != "" {
			m["region"] = region
		}
		if bp := r.FormValue("backend_base_path"); bp != "" {
			m["basePath"] = bp
		}
		m["secure"] = r.FormValue("backend_secure") == "true"

	case "git":
		m["url"] = r.FormValue("backend_url")
		if branch := r.FormValue("backend_branch"); branch != "" {
			m["branch"] = branch
		}
		if pi := r.FormValue("backend_pull_interval"); pi != "" {
			m["pullInterval"] = pi
		}

	default:
		return nil, errors.Errorf("type de backend inconnu : '%s'", backendType)
	}

	data, err := json.Marshal(m)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return json.RawMessage(data), nil
}

// parseFileRefFromForm reads a file upload from "<name>_file" or falls back to "<name>_content" (existing base64).
func parseFileRefFromForm(r *http.Request, fieldName string) (map[string]any, error) {
	file, _, err := r.FormFile(fieldName + "_file")
	if err == nil {
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, errors.Wrapf(err, "could not read uploaded file for field '%s'", fieldName)
		}
		if len(data) > 0 {
			return map[string]any{"content": base64.StdEncoding.EncodeToString(data)}, nil
		}
	}

	if content := r.FormValue(fieldName + "_content"); content != "" {
		return map[string]any{"content": content}, nil
	}

	return nil, nil
}

func parseFilesystemSourceOptions(r *http.Request) model.FilesystemSourceOptions {
	opts := model.DefaultFilesystemSourceOptions()

	if dir := r.FormValue("directory"); dir != "" {
		opts.Directory = dir
	}
	if filter := r.FormValue("filter"); filter != "" {
		opts.Filter = filter
	}
	if strategy := r.FormValue("etag_strategy"); strategy != "" {
		opts.ETagStrategy = strategy
	}
	if c := r.FormValue("concurrency"); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n > 0 {
			opts.Concurrency = n
		}
	}
	opts.Recursive = r.FormValue("recursive") == "true"
	opts.DeleteOrphans = r.FormValue("delete_orphans") == "true"
	if st := r.FormValue("source_template"); st != "" {
		opts.SourceTemplate = st
	}

	return opts
}

func parseFilesystemSourceSyncInterval(r *http.Request) *time.Duration {
	raw := r.FormValue("sync_interval")
	if raw == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return nil
	}
	return &d
}
