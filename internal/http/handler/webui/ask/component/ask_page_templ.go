// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.819
package component

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import (
	"encoding/base64"
	"github.com/bornholm/corpus/internal/build"
	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	common "github.com/bornholm/corpus/internal/http/handler/webui/common/component"
	"slices"
	"strconv"
	"time"
)

type AskPageVModel struct {
	Query                   string
	TotalDocuments          int64
	Results                 []*port.IndexSearchResult
	Collections             []model.Collection
	SelectedCollectionNames []string

	Response        string
	Duration        time.Duration
	UploadFileModal *UploadFileModalVModel
}

func AskPage(vmodel AskPageVModel) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Var2 := templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
			templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
			templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
			if !templ_7745c5c3_IsBuffer {
				defer func() {
					templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
					if templ_7745c5c3_Err == nil {
						templ_7745c5c3_Err = templ_7745c5c3_BufErr
					}
				}()
			}
			ctx = templ.InitializeContext(ctx)
			if vmodel.UploadFileModal != nil {
				templ_7745c5c3_Err = UploadFileModal(*vmodel.UploadFileModal).Render(ctx, templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, " <div class=\"container\"><section class=\"section\"><div class=\"level\"><div class=\"level-left\"><h1 class=\"title is-size-1 level-item\"><a href=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var3 templ.SafeURL = common.BaseURL(ctx)
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(string(templ_7745c5c3_Var3)))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "\"><img style=\"height:2em;vertical-align:middle\" src=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var4 string
			templ_7745c5c3_Var4, templ_7745c5c3_Err = templ.JoinStringErrs(string(common.BaseURL(ctx, common.WithPath("/assets/logo.svg"))))
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 35, Col: 193}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var4))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "\"><span class=\"ml-2 has-text-grey-dark\" style=\"vertical-align:middle\">Corpus</span></a></h1></div><div class=\"level-right\"><a class=\"button is-outlined is-link\" href=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var5 templ.SafeURL = common.CurrentURL(ctx, common.WithValues("action", "upload"))
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(string(templ_7745c5c3_Var5)))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 4, "\"><span>Indexer un fichier</span><span class=\"icon\"><i class=\"fa fa-plus\"></i></span></a></div></div><form method=\"post\" hx-on:submit=\"htmx.addClass(htmx.find(&#39;#submit-button&#39;), &#39;is-loading&#39;)\"><div class=\"columns\"><div class=\"column is-9\"><div class=\"field is-large\"><label class=\"label\" for=\"query\">Posez votre question <span class=\"has-text-weight-normal has-text-grey\">(")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var6 string
			templ_7745c5c3_Var6, templ_7745c5c3_Err = templ.JoinStringErrs(strconv.FormatInt(vmodel.TotalDocuments, 10))
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 45, Col: 160}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var6))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 5, " documents indexés)</span></label><div class=\"control\"><textarea id=\"query\" name=\"q\" class=\"textarea is-large\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var7 string
			templ_7745c5c3_Var7, templ_7745c5c3_Err = templ.JoinStringErrs(vmodel.Query)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 47, Col: 79}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var7))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 6, "</textarea></div></div></div><div class=\"column\"><label class=\"label\">Collections</label><div class=\"tags are-large\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			for _, c := range vmodel.Collections {
				selected := slices.Contains(vmodel.SelectedCollectionNames, c.Name())
				url := common.CurrentURL(ctx, common.WithValues("collection", c.Name()))
				if selected {
					url = common.CurrentURL(ctx, common.WithoutValues("collection", c.Name()))
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 7, " ")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var8 = []any{"tag", "is-light", templ.KV("is-link", selected)}
				templ_7745c5c3_Err = templ.RenderCSSItems(ctx, templ_7745c5c3_Buffer, templ_7745c5c3_Var8...)
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 8, "<a href=\"")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var9 templ.SafeURL = url
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(string(templ_7745c5c3_Var9)))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 9, "\" title=\"")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var10 string
				templ_7745c5c3_Var10, templ_7745c5c3_Err = templ.JoinStringErrs(c.Description())
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 60, Col: 48}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var10))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 10, "\" class=\"")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var11 string
				templ_7745c5c3_Var11, templ_7745c5c3_Err = templ.JoinStringErrs(templ.CSSClasses(templ_7745c5c3_Var8).String())
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 1, Col: 0}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var11))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 11, "\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var12 string
				templ_7745c5c3_Var12, templ_7745c5c3_Err = templ.JoinStringErrs(c.Name())
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 60, Col: 120}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var12))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 12, "</a>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 13, "</div><p class=\"help\">Sélectionnez les collections de documents que vous souhaitez incorporer à votre recherche. Par défaut, toutes sont utilisées.</p></div></div><button id=\"submit-button\" type=\"submit\" class=\"button is-link is-fullwidth is-large\"><span>Interroger</span> <span class=\"icon\"><i class=\"far fa-comment\"></i></span></button></form>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			if vmodel.Query != "" {
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 14, "<hr class=\"mt-5\"><div class=\"mt-5\"><div class=\"level\"><div class=\"level-left\"><h2 class=\"title is-size-2 level-item\"><span>Réponse</span> <span class=\"has-text-weight-normal has-text-grey subtitle ml-3\">(générée en ")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				var templ_7745c5c3_Var13 string
				templ_7745c5c3_Var13, templ_7745c5c3_Err = templ.JoinStringErrs(vmodel.Duration.Round(time.Second).String())
				if templ_7745c5c3_Err != nil {
					return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 78, Col: 134}
				}
				_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var13))
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 15, ")</span></h2></div><div class=\"level-right\"><div class=\"buttons level-item\">")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				if vmodel.Response != "" {
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 16, "<button class=\"button\" hx-on:click=\"copyResponseToClipboard(this)\" data-encoded-response=\"")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					var templ_7745c5c3_Var14 string
					templ_7745c5c3_Var14, templ_7745c5c3_Err = templ.JoinStringErrs(base64.RawStdEncoding.EncodeToString([]byte(vmodel.Response)))
					if templ_7745c5c3_Err != nil {
						return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 87, Col: 96}
					}
					_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var14))
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 17, "\"><span>Copier</span> <span class=\"icon\"><i class=\"fas fa-copy\"></i></span></button><script type=\"text/javascript\">\n\t\t\t\t\t\t\t\t\t\t\tfunction copyResponseToClipboard(el) {\n\t\t\t\t\t\t\t\t\t\t\t\tconst encodedResponse = el.dataset.encodedResponse;\n\t\t\t\t\t\t\t\t\t\t\t\tconst text = new TextDecoder().decode(Uint8Array.from(atob(encodedResponse), m => m.charCodeAt(0)))\n\t\t\t\t\t\t\t\t\t\t\t\tif (navigator.clipboard) {\n\t\t\t\t\t\t\t\t\t\t\t\t\tnavigator.clipboard.writeText(text);\n\t\t\t\t\t\t\t\t\t\t\t\t} else {\n\t\t\t\t\t\t\t\t\t\t\t\t\tunsecuredCopyToClipboard(text);\n\t\t\t\t\t\t\t\t\t\t\t\t}\n\t\t\t\t\t\t\t\t\t\t\t\tel.innerHTML = '<span>Copié !</span><span class=\"icon\"><i class=\"fas fa-check\"></i></span>'\n\t\t\t\t\t\t\t\t\t\t\t}\n\n\t\t\t\t\t\t\t\t\t\t\tfunction unsecuredCopyToClipboard(text) {\n\t\t\t\t\t\t\t\t\t\t\t\tconst textArea = document.createElement(\"textarea\");\n\t\t\t\t\t\t\t\t\t\t\t\ttextArea.value = text;\n\t\t\t\t\t\t\t\t\t\t\t\tdocument.body.appendChild(textArea);\n\t\t\t\t\t\t\t\t\t\t\t\ttextArea.focus({ preventScroll: true });\n\t\t\t\t\t\t\t\t\t\t\t\ttextArea.select();\n\t\t\t\t\t\t\t\t\t\t\t\ttry {\n\t\t\t\t\t\t\t\t\t\t\t\t\tdocument.execCommand('copy');\n\t\t\t\t\t\t\t\t\t\t\t\t} catch (err) {\n\t\t\t\t\t\t\t\t\t\t\t\t\tconsole.error('Unable to copy to clipboard', err);\n\t\t\t\t\t\t\t\t\t\t\t\t}\n\t\t\t\t\t\t\t\t\t\t\t\tdocument.body.removeChild(textArea);\n\t\t\t\t\t\t\t\t\t\t\t}\n\t\t\t\t\t\t\t\t\t\t</script>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 18, "</div></div></div>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
				if len(vmodel.Results) == 0 {
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 19, "<div class=\"message is-warning is-medium\"><div class=\"message-body\">Aucun résultat correspondant à votre question n'a été trouvé dans la base documentaire.</div></div>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
				} else {
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 20, "<div class=\"content is-size-4\">")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = common.Markdown(vmodel.Response).Render(ctx, templ_7745c5c3_Buffer)
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 21, "</div><h3 class=\"title is-size-3\">Sources</h3><div class=\"content\"><ol>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
					for _, r := range vmodel.Results {
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 22, "<li><a href=\"")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						var templ_7745c5c3_Var15 templ.SafeURL = templ.SafeURL(r.Source.String())
						_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(string(templ_7745c5c3_Var15)))
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 23, "\">")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						var templ_7745c5c3_Var16 string
						templ_7745c5c3_Var16, templ_7745c5c3_Err = templ.JoinStringErrs(r.Source.String())
						if templ_7745c5c3_Err != nil {
							return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 136, Col: 78}
						}
						_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var16))
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
						templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 24, "</a></li>")
						if templ_7745c5c3_Err != nil {
							return templ_7745c5c3_Err
						}
					}
					templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 25, "</ol></div>")
					if templ_7745c5c3_Err != nil {
						return templ_7745c5c3_Err
					}
				}
				templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 26, "</div>")
				if templ_7745c5c3_Err != nil {
					return templ_7745c5c3_Err
				}
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 27, "</section><footer class=\"footer\"><div class=\"content has-text-centered\"><p><b>Corpus</b> (version <a href=\"https://github.com/Bornholm/corpus\" target=\"_blank\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var17 string
			templ_7745c5c3_Var17, templ_7745c5c3_Err = templ.JoinStringErrs(build.ShortVersion)
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `internal/http/handler/webui/ask/component/ask_page.templ`, Line: 147, Col: 110}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var17))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 28, "</a>) |  <a target=\"_blank\" href=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var18 templ.SafeURL = common.BaseURL(ctx, common.WithPath("/docs/index.html"))
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(string(templ_7745c5c3_Var18)))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 29, "\">Documentation API</a></p></div></footer></div>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			return nil
		})
		templ_7745c5c3_Err = common.Page(common.WithTitle("Interrogez vos documents !")).Render(templ.WithChildren(ctx, templ_7745c5c3_Var2), templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
