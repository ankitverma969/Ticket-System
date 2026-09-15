package docs

import (
	_ "embed"

	"github.com/swaggo/swag"
)

//go:embed swagger.json
var swaggerJSON string

type s struct{}

func (s *s) ReadDoc() string {
	return swaggerJSON
}

func init() {
	swag.Register(swag.Name, &s{})
}
