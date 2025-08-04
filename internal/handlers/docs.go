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

type parameterLocation string

const (
	inQuery  parameterLocation = "query"
	inPath   parameterLocation = "path"
	inHeader parameterLocation = "header"
	inBody   parameterLocation = "body"
)

type httpMethod string

const (
	methodGet    httpMethod = "GET"
	methodPost   httpMethod = "POST"
	methodPut    httpMethod = "PUT"
	methodDelete httpMethod = "DELETE"
	methodPatch  httpMethod = "PATCH"
)

// API Documentation Models.
type (
	apiDoc struct {
		Title       string
		Description string
		BaseURL     string
		Version     string
		Sections    []section
	}

	section struct {
		Name        string
		Description string
		Endpoints   []endpoint
	}

	endpoint struct {
		ID              string
		Method          httpMethod
		Path            string
		Description     string
		Parameters      []parameter
		RequestExample  string
		ResponseExample string
	}

	parameter struct {
		Name        string
		Type        string
		In          parameterLocation
		Required    bool
		Description string
		Default     string
	}
)

// DocsHandler renders HTML documentation for the API.
func (app *Application) DocsHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templatesFS, "docs/templates/main.tmpl")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	doc := app.buildAPIDoc(r)

	if err := tmpl.Execute(w, doc); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// buildAPIDoc constructs the API documentation structure.
func (app *Application) buildAPIDoc(r *http.Request) apiDoc {
	baseURL := fmt.Sprintf("%s://%s", detectProto(r), r.Host)

	return apiDoc{
		Title:       "Paste.thek4n.ru API",
		Description: "This API provides access to all the awesome features of our service.",
		BaseURL:     baseURL,
		Version:     app.Version,
		Sections:    app.buildSections(baseURL),
	}
}

// buildSections constructs all documentation sections.
func (app *Application) buildSections(baseURL string) []section {
	sections := []section{
		app.buildMainSection(baseURL),
	}

	if app.HealthcheckEnabled {
		sections = append(sections, app.buildHealthcheckSection())
	}

	return sections
}

// buildMainSection constructs the main API operations section.
func (app *Application) buildMainSection(baseURL string) section {
	return section{
		Name:        "Main",
		Description: "Main operations",
		Endpoints: []endpoint{
			{
				ID:              "create-record",
				Method:          methodPost,
				Path:            "/",
				Description:     "Save body",
				ResponseExample: fmt.Sprintf("%s/eoVbybwLnlc49q/", baseURL),
				Parameters:      app.getCreateRecordParameters(),
			},
			{
				ID:              "get-record",
				Method:          methodGet,
				Path:            "/{key}",
				Description:     "Get previously saved body with key. If key was saved as url - you will be redirected.",
				ResponseExample: "body",
				Parameters:      app.getKeyPathParameter(),
			},
			{
				ID:              "get-record-clicks",
				Method:          methodGet,
				Path:            "/{key}/clicks",
				Description:     "Get clicks count for key.",
				ResponseExample: "1",
				Parameters:      app.getKeyPathParameter(),
			},
		},
	}
}

// buildHealthcheckSection constructs the healthcheck section.
func (app *Application) buildHealthcheckSection() section {
	return section{
		Name:        "Healthcheck",
		Description: "Healthcheck operations",
		Endpoints: []endpoint{
			{
				ID:          "healthcheck",
				Method:      methodGet,
				Path:        "/health",
				Description: "Healthcheck service",
				ResponseExample: fmt.Sprintf(`{
	"version": "%s",
	"availability": true,
	"msg": "ok"
}`, app.Version),
			},
		},
	}
}

// getCreateRecordParameters returns parameters for the create record endpoint.
func (app *Application) getCreateRecordParameters() []parameter {
	return []parameter{
		{
			Name:        "ttl",
			Type:        "time",
			In:          inQuery,
			Required:    false,
			Description: "TTL - time to live of created key. Examples 3h, 30m, 60s. Authorized apikeys can set persist key by providing ttl parameter as 0",
			Default:     config.DefaultTTL.String(),
		},
		{
			Name:        "disposable",
			Type:        "int",
			In:          inQuery,
			Required:    false,
			Description: "After number of this getting of this key, key will be removed",
			Default:     "0",
		},
		{
			Name:        "len",
			Type:        "int",
			In:          inQuery,
			Required:    false,
			Description: fmt.Sprintf("Length of key to generate. max=%d, unpriveleged min=%d, privelegen min=%d", config.MaxKeyLength, config.UnprivelegedMinKeyLength, config.PrivelegedMinKeyLength),
			Default:     fmt.Sprintf("%d", config.DefaultKeyLength),
		},
		{
			Name:        "url",
			Type:        "bool",
			In:          inQuery,
			Required:    false,
			Description: "Is body url. If true after getting this key you will be redirected.",
			Default:     "false",
		},
		{
			Name:        "key",
			Type:        "string",
			In:          inQuery,
			Required:    false,
			Description: "You can request custom key",
			Default:     "",
		},
		{
			Name:        "apikey",
			Type:        "string",
			In:          inQuery,
			Required:    false,
			Description: "Apikey to use privileged features",
			Default:     "",
		},
		{
			Name:        "body",
			Type:        "string",
			In:          inBody,
			Required:    true,
			Description: "Body to cache.",
			Default:     "",
		},
	}
}

// getKeyPathParameter returns the common key path parameter.
func (app *Application) getKeyPathParameter() []parameter {
	return []parameter{
		{
			Name:        "key",
			Type:        "string",
			In:          inPath,
			Required:    true,
			Description: "Key to request.",
			Default:     "",
		},
	}
}
