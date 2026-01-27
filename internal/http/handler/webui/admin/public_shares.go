package admin

import (
	"context"
	"net/http"
	"slices"
	"strconv"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/crypto"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/admin/component"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	commonComp "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/bornholm/go-x/templx/form"
	formx "github.com/bornholm/go-x/templx/form"
	"github.com/bornholm/go-x/templx/form/renderer/bulma"
	"github.com/pkg/errors"
)

func (h *Handler) getPublicSharesPage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillPublicSharesPageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	publicSharesPage := component.PublicSharesPage(*vmodel)
	templ.Handler(publicSharesPage).ServeHTTP(w, r)
}

func (h *Handler) getNewPublicSharePage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillNewPublicSharePageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	newPublicSharePage := component.NewPublicSharePage(*vmodel)
	templ.Handler(newPublicSharePage).ServeHTTP(w, r)
}

func (h *Handler) getEditPublicSharePage(w http.ResponseWriter, r *http.Request) {
	vmodel, err := h.fillEditPublicSharePageViewModel(r)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	editPublicSharePage := component.EditPublicSharePage(*vmodel)
	templ.Handler(editPublicSharePage).ServeHTTP(w, r)
}

func (h *Handler) postEditPublicShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := httpCtx.User(ctx)
	if user == nil {
		common.HandleError(w, r, errors.New("user not found in context"))
		return
	}

	publicShareID := model.PublicShareID(r.PathValue("id"))
	if publicShareID == "" {
		common.HandleError(w, r, errors.New("public share ID is required"))
		return
	}

	// Get existing public share
	existingShare, err := h.publicShareStore.GetPublicShareByID(ctx, publicShareID)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	collections, err := h.documentStore.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	form := h.newPublicShareForm(collections)
	if err := form.Handle(r); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	if !form.IsValid(ctx) {
		vmodel, err := h.fillEditPublicSharePageViewModel(r)
		if err != nil {
			common.HandleError(w, r, errors.WithStack(err))
			return
		}

		vmodel.PublicShareForm = form

		editPublicSharePage := component.EditPublicSharePage(*vmodel)
		templ.Handler(editPublicSharePage).ServeHTTP(w, r)

		return
	}

	// Get form values
	title, _ := form.GetFieldValue("title")
	description, _ := form.GetFieldValue("description")
	collectionIDsStr, _ := form.GetFieldValues("collections")

	// Convert collection IDs
	var selectedCollections []model.Collection
	for _, idStr := range collectionIDsStr {
		collectionID := model.CollectionID(idStr)
		collection, err := h.documentStore.GetCollectionByID(ctx, collectionID)
		if err != nil {
			if errors.Is(err, port.ErrNotFound) {
				continue // Skip invalid collection IDs
			}
			common.HandleError(w, r, errors.WithStack(err))
			return
		}
		selectedCollections = append(selectedCollections, collection)
	}

	// Update public share
	updatedShare := &basePublicShare{
		id:          existingShare.ID(),
		ownerID:     existingShare.OwnerID(),
		token:       existingShare.Token(), // Keep existing token
		title:       title,
		description: description,
		collections: selectedCollections,
	}

	_, err = h.publicShareStore.SavePublicShare(ctx, updatedShare)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	// Redirect to public shares list
	http.Redirect(w, r, "/admin/public-shares", http.StatusSeeOther)
}

func (h *Handler) postPublicShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := httpCtx.User(ctx)
	if user == nil {
		common.HandleError(w, r, errors.New("user not found in context"))
		return
	}

	collections, err := h.documentStore.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	form := h.newPublicShareForm(collections)
	if err := form.Handle(r); err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	if !form.IsValid(ctx) {
		vmodel, err := h.fillNewPublicSharePageViewModel(r)
		if err != nil {
			common.HandleError(w, r, errors.WithStack(err))
			return
		}

		vmodel.PublicShareForm = form

		editPublicSharePage := component.NewPublicSharePage(*vmodel)
		templ.Handler(editPublicSharePage).ServeHTTP(w, r)

		return
	}

	// Generate secure token
	token, err := crypto.GenerateSecureToken()
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	// Get form values
	title, _ := form.GetFieldValue("title")
	description, _ := form.GetFieldValue("description")
	collectionIDsStr, _ := form.GetFieldValues("collections")

	// Convert collection IDs
	var selectedCollections []model.Collection
	for _, idStr := range collectionIDsStr {
		collectionID := model.CollectionID(idStr)
		collection, err := h.documentStore.GetCollectionByID(ctx, collectionID)
		if err != nil {
			if errors.Is(err, port.ErrNotFound) {
				continue // Skip invalid collection IDs
			}
			common.HandleError(w, r, errors.WithStack(err))
			return
		}
		selectedCollections = append(selectedCollections, collection)
	}

	// Create public share
	publicShare := &basePublicShare{
		id:          model.NewPublicShareID(),
		ownerID:     user.ID(),
		token:       token,
		title:       title,
		description: description,
		collections: selectedCollections,
	}

	_, err = h.publicShareStore.SavePublicShare(ctx, publicShare)
	if err != nil {
		common.HandleError(w, r, errors.WithStack(err))
		return
	}

	// Redirect to public shares list
	http.Redirect(w, r, "/admin/public-shares", http.StatusSeeOther)
}

func (h *Handler) fillPublicSharesPageViewModel(r *http.Request) (*component.PublicSharesPageVModel, error) {
	vmodel := &component.PublicSharesPageVModel{}
	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillPublicSharesPageVModelNavbar,
		h.fillPublicSharesPageVModelPublicShares,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillNewPublicSharePageViewModel(r *http.Request) (*component.NewPublicSharePageVModel, error) {
	vmodel := &component.NewPublicSharePageVModel{}
	ctx := r.Context()

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillNewPublicSharePageVModelNavbar,
		h.fillNewPublicSharePageVModelForm,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillPublicSharesPageVModelNavbar(ctx context.Context, vmodel *component.PublicSharesPageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillPublicSharesPageVModelPublicShares(ctx context.Context, vmodel *component.PublicSharesPageVModel, r *http.Request) error {
	// Parse pagination parameters
	page := 0
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p - 1 // Convert to 0-based
		}
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	opts := port.QueryPublicSharesOptions{
		Page:  &page,
		Limit: &limit,
	}

	publicShares, err := h.publicShareStore.QueryPublicShares(ctx, opts)
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.PublicShares = publicShares
	vmodel.CurrentPage = page + 1 // Convert back to 1-based
	vmodel.PageSize = limit

	return nil
}

func (h *Handler) fillNewPublicSharePageVModelNavbar(ctx context.Context, vmodel *component.NewPublicSharePageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillNewPublicSharePageVModelForm(ctx context.Context, vmodel *component.NewPublicSharePageVModel, r *http.Request) error {
	collections, err := h.documentStore.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.PublicShareForm = h.newPublicShareForm(collections)
	return nil
}

func (h *Handler) fillEditPublicSharePageViewModel(r *http.Request) (*component.EditPublicSharePageVModel, error) {
	vmodel := &component.EditPublicSharePageVModel{}
	ctx := r.Context()

	publicShareID := model.PublicShareID(r.PathValue("id"))
	if publicShareID == "" {
		return nil, errors.New("public share ID is required")
	}

	err := common.FillViewModel(
		ctx,
		vmodel, r,
		h.fillEditPublicSharePageVModelNavbar,
		h.fillEditPublicSharePageVModelPublicShare,
		h.fillEditPublicSharePageVModelForm,
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return vmodel, nil
}

func (h *Handler) fillEditPublicSharePageVModelNavbar(ctx context.Context, vmodel *component.EditPublicSharePageVModel, r *http.Request) error {
	user := httpCtx.User(ctx)
	if user == nil {
		return errors.New("could not retrieve user from context")
	}

	vmodel.Navbar = commonComp.NavbarVModel{
		User: user,
	}

	return nil
}

func (h *Handler) fillEditPublicSharePageVModelPublicShare(ctx context.Context, vmodel *component.EditPublicSharePageVModel, r *http.Request) error {
	publicShareID := model.PublicShareID(r.PathValue("id"))

	publicShare, err := h.publicShareStore.GetPublicShareByID(ctx, publicShareID)
	if err != nil {
		return errors.WithStack(err)
	}

	vmodel.PublicShare = publicShare

	return nil
}

func (h *Handler) fillEditPublicSharePageVModelForm(ctx context.Context, vmodel *component.EditPublicSharePageVModel, r *http.Request) error {
	collections, err := h.documentStore.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	form := h.newPublicShareForm(collections)

	// Pre-populate form with existing values
	if vmodel.PublicShare != nil {
		form.SetFieldValues("title", vmodel.PublicShare.Title())
		form.SetFieldValues("description", vmodel.PublicShare.Description())
		form.SetFieldValues("collections", slices.Collect(func(yield func(string) bool) {
			for _, c := range vmodel.PublicShare.Collections() {
				if !yield(string(c.ID())) {
					return
				}
			}
		})...)
	}

	vmodel.PublicShareForm = form
	return nil
}

func (h *Handler) newPublicShareForm(collections []model.PersistedCollection) *form.Form {
	form := formx.New([]form.Field{
		formx.NewField("title",
			formx.WithLabel("Titre"),
			formx.WithRequired(true),
			formx.WithDescription("Ex: Partage documentation technique"),
			formx.WithValidation(formx.RequiredRule{}),
		),
		formx.NewField("description",
			formx.WithLabel("Description"),
			formx.WithDescription("Description du partage"),
			formx.WithType("textarea"),
		),
		formx.NewField("collections",
			formx.WithLabel("Collections"),
			formx.WithDescription("Collections à utiliser"),
			formx.WithType("select"),
			formx.WithAttribute("multiple", true),
			formx.WithRequired(true),
			formx.WithSelectOptions(slices.Collect(func(yield func(formx.SelectOption) bool) {
				for _, c := range collections {
					if !yield(formx.SelectOption{
						Label: c.Label(),
						Value: string(c.ID()),
					}) {
						return
					}
				}
			})...),
		),
	}, form.WithDefaultRenderer(bulma.NewFieldRenderer()))

	return form
}

// basePublicShare implements model.OwnedPublicShare
type basePublicShare struct {
	id          model.PublicShareID
	ownerID     model.UserID
	token       string
	title       string
	description string
	collections []model.Collection
}

func (ps *basePublicShare) ID() model.PublicShareID {
	return ps.id
}

func (ps *basePublicShare) OwnerID() model.UserID {
	return ps.ownerID
}

func (ps *basePublicShare) Token() string {
	return ps.token
}

func (ps *basePublicShare) Title() string {
	return ps.title
}

func (ps *basePublicShare) Description() string {
	return ps.description
}

func (ps *basePublicShare) Collections() []model.Collection {
	return ps.collections
}

var _ model.OwnedPublicShare = &basePublicShare{}
