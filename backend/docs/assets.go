package docs

import _ "embed"

//go:embed swagger.yaml
var OpenAPI []byte

//go:embed scalar.html
var Scalar []byte
