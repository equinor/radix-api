package swaggerui

import (
	"embed"
	"io/fs"
	// Required to include swagger dependencies
	// _ "github.com/Masterminds/sprig/v3"
	// _ "github.com/go-openapi/analysis"
	// _ "github.com/go-openapi/errors"
	// _ "github.com/go-openapi/inflect"
	// _ "github.com/go-openapi/loads"
	// _ "github.com/go-openapi/loads/fmts"
	// _ "github.com/go-openapi/runtime"
	// _ "github.com/go-openapi/spec"
	// _ "github.com/go-openapi/strfmt"
	// _ "github.com/go-openapi/validate"
	// _ "github.com/go-swagger/go-swagger"
	// _ "github.com/jessevdk/go-flags"
	// _ "github.com/toqueteos/webbrowser"
	//
	// _ "github.com/go-swagger/go-swagger/cmd/swagger/commands"
	// _ "github.com/go-swagger/go-swagger/codescan"
	// _ "github.com/go-swagger/go-swagger/generator"

	_ "github.com/go-swagger/go-swagger"
)

//go:embed html
var swaggerFS embed.FS

// FS returns a FS with SwaggerUI files in root
func FS() fs.FS {
	rootFS, err := fs.Sub(swaggerFS, "html")
	if err != nil {
		panic(err)
	}
	return rootFS
}
