package command

import (
	_ "embed"
	"fmt"

	"gitlab.com/mnm/bud/gen"
	"gitlab.com/mnm/bud/go/mod"
	"gitlab.com/mnm/bud/internal/gotemplate"
	"gitlab.com/mnm/bud/internal/parser"
)

//go:embed command.gotext
var template string

var generator = gotemplate.MustParse("command.gotext", template)

type Generator struct {
	Module *mod.Module
	Parser *parser.Parser
}

func (g *Generator) GenerateFile(f gen.F, file *gen.File) error {
	// TODO: consider also building when only commands are present
	if err := gen.SkipUnless(f, "bud/web/web.go"); err != nil {
		return err
	}
	// Load command state
	state, err := Load(g.Module, g.Parser)
	if err != nil {
		fmt.Println(state, err)
		return err
	}
	// Generate our template
	code, err := generator.Generate(state)
	if err != nil {
		return err
	}
	// fmt.Println(string(code))
	file.Write(code)
	return nil
}

// func (g *Generator) generateDI(f gen.F, file *gen.File) error {

// }
