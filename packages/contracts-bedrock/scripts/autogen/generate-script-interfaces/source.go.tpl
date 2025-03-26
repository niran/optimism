// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package {{.Package}}

import (
	"https://github.com/lmittmann/w3"
)

{{range .Abi.Methods}}
	var {{.Name}} = w3.MustNewFunc("{{.Signature}}")
{{end}}

{{range .Abi..Events}}
	var {{.Name}} = w3.MustNewEvent("{{.Signature}}")
{{end}}