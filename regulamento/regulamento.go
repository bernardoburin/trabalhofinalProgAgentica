// Package regulamento guarda o texto do regulamento e do adendo, que vai no
// prompt do agente.
package regulamento

import _ "embed"

//go:embed regulamento.md
var Texto string
