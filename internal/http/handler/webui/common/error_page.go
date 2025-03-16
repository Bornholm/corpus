package common

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"github.com/pkg/errors"
)

type HTTPError interface {
	error
	StatusCode() int
}

type UserFacingError interface {
	error
	UserMessage() string
}

func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	vmodel := component.ErrorPageVModel{}

	statusCode := http.StatusInternalServerError

	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		statusCode = httpErr.StatusCode()
	}

	w.WriteHeader(statusCode)

	var userFacingErr UserFacingError
	if errors.As(err, &userFacingErr) {
		vmodel.Message = userFacingErr.UserMessage()
	} else {
		vmodel.Message = http.StatusText(statusCode)
	}

	if httpErr == nil && userFacingErr == nil {
		slog.ErrorContext(r.Context(), "unexpected error", slog.Any("error", errors.WithStack(err)))
	}

	errorPage := component.ErrorPage(vmodel)

	templ.Handler(errorPage).ServeHTTP(w, r)
}
