package profile

import (
	"github.com/bornholm/go-x/templx/form"
	formx "github.com/bornholm/go-x/templx/form"
	"github.com/bornholm/go-x/templx/form/renderer/bulma"
)

func newTokenForm() *form.Form {
	form := formx.New([]form.Field{
		formx.NewField("label",
			formx.WithLabel("Nom du jeton"),
			formx.WithRequired(true),
			formx.WithPlaceholder("Ex: Application mobile, Script de sauvegarde..."),
			formx.WithValidation(formx.RequiredRule{}),
		),
	}, form.WithDefaultRenderer(bulma.NewFieldRenderer()))

	return form
}
