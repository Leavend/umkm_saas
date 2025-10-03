package handlers

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openAPISpec []byte

const redocHTML = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>UMKM SaaS API Docs</title>
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style>
      body {
        margin: 0;
        padding: 0;
      }
      redoc {
        display: block;
        height: 100vh;
      }
    </style>
  </head>
  <body>
    <redoc spec-url="/v1/openapi.json"></redoc>
    <script src="https://cdn.jsdelivr.net/npm/redoc@2.2.0/bundles/redoc.standalone.js"></script>
  </body>
</html>`

func (a *App) OpenAPIJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPISpec)
}

func (a *App) OpenAPIDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(redocHTML))
}
