package desktop

import (
	"github.com/bornholm/go-x/templx/form"
	formx "github.com/bornholm/go-x/templx/form"
	"github.com/bornholm/go-x/templx/form/renderer/bulma"
)

func newServerForm() *form.Form {
	form := formx.New([]form.Field{
		formx.NewField("label",
			formx.WithLabel("Nom"),
			formx.WithRequired(true),
			formx.WithDescription("Nom du serveur"),
		),
		formx.NewField("url",
			formx.WithLabel("URL"),
			formx.WithRequired(true),
			formx.WithDescription("URL du serveur Corpus"),
		),
		formx.NewField("token",
			formx.WithLabel("Jeton d'authentification"),
			formx.WithRequired(true),
			formx.WithType("password"),
			formx.WithDescription("Jeton d'authentification associé à votre compte Corpus"),
			form.WithAttribute("autocomplete", "off"),
		),
		formx.NewField("preferred",
			formx.WithLabel("Utiliser comme serveur préféré"),
			formx.WithRequired(false),
			formx.WithType("checkbox"),
			formx.WithDescription("Sélectionner automatiquement ce serveur au démarrage de l'application"),
		),
	}, form.WithDefaultRenderer(bulma.NewFieldRenderer()))

	return form
}
