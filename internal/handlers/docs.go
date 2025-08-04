package handlers

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/thek4n/paste.thek4n.ru/internal/config"
)

//go:embed docs/templates
var templatesFS embed.FS

type apiDoc struct {
	Title       string
	Description string
	BaseURL     string
	Version     string
	Sections    []section
}

type section struct {
	Name        string
	Description string
	Endpoints   []endpoint
}

type endpoint struct {
	ID              string
	Method          string // GET, POST, PUT, DELETE, etc.
	Path            string
	Description     string
	Parameters      []parameter
	RequestExample  string
	ResponseExample string
}

type parameter struct {
	Name        string
	Type        string
	In          string // query, path, header, body
	Required    bool
	Description string
	Default     string
}

// DocsHandler render html documentation.
func (app *Application) DocsHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templatesFS, "docs/templates/main.tmpl")
	if err != nil {
		panic(err)
	}

	baseURL := fmt.Sprintf("%s://%s", detectProto(r), r.Host)
	apiDoc := apiDoc{
		Title:       "Paste.thek4n.ru API",
		Description: "This API provides access to all the awesome features of our service.",
		BaseURL:     baseURL,
		Version:     app.Version,
		Sections: []section{
			{
				Name:        "Main",
				Description: "Main operations",
				Endpoints: []endpoint{
					{
						ID:              "create-record",
						Method:          "POST",
						Path:            "/",
						Description:     "Save body",
						ResponseExample: fmt.Sprintf("%s/eoVbybwLnlc49q/", baseURL),
						Parameters: []parameter{
							{
								Name:        "ttl",
								Type:        "time",
								In:          "query",
								Required:    false,
								Description: "TTL - time to live of created key. Examples 3h, 30m, 60s. Authorized apikeys can set persist key by providing ttl parameter as 0",
								Default:     config.DefaultTTL.String(),
							},
							{
								Name:        "disposable",
								Type:        "int",
								In:          "query",
								Required:    false,
								Description: "After number of this getting of this key, key will be removed",
								Default:     "0",
							},
							{
								Name:        "len",
								Type:        "int",
								In:          "query",
								Required:    false,
								Description: fmt.Sprintf("Length of key to generate. max=%d, unpriveleged min=%d, privelegen min=%d", config.MaxKeyLength, config.UnprivelegedMinKeyLength, config.PrivelegedMinKeyLength),
								Default:     fmt.Sprintf("%d", config.DefaultKeyLength),
							},
							{
								Name:        "url",
								Type:        "bool",
								In:          "query",
								Required:    false,
								Description: "Is body url. If true after getting this key you will be redirected.",
								Default:     "false",
							},
							{
								Name:        "key",
								Type:        "string",
								In:          "query",
								Required:    false,
								Description: "You can request custom key",
								Default:     "",
							},
							{
								Name:        "apikey",
								Type:        "string",
								In:          "query",
								Required:    false,
								Description: "Apikey to use privileged features",
								Default:     "",
							},
						},
					},
					{
						ID:              "get-record",
						Method:          "GET",
						Path:            "/{key}",
						Description:     "Get previusly saved body with key. If key was saved as url - you will be redirected.",
						ResponseExample: `body`,
						Parameters: []parameter{
							{
								Name:        "key",
								Type:        "string",
								In:          "path",
								Required:    true,
								Description: "Key to request.",
								Default:     "-",
							},
						},
					},
				},
			},
		},
	}

	if app.HealthcheckEnabled {
		apiDoc.Sections = append(apiDoc.Sections, section{
			Name:        "Healthcheck",
			Description: "Healthcheck operations",
			Endpoints: []endpoint{
				{
					ID:          "healthcheck",
					Method:      "GET",
					Path:        "/health",
					Description: "Healthcheck service",
					ResponseExample: fmt.Sprintf(`{
	"version": "%s",
	"availability": true,
	"msg": "ok"
}`, app.Version),
				},
			},
		})
	}

	err = tmpl.Execute(w, apiDoc)
	if err != nil {
		panic(err)
	}
}
