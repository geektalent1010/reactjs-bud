package gotemplate

import (
	"bytes"
	"text/template"
)

// MustParse panics if unable to parse
func MustParse(name, code string) *Template {
	template, err := Parse(name, code)
	if err != nil {
		panic(err)
	}
	return template
}

// Parse parses Go code
func Parse(name, code string) (*Template, error) {
	tpl, err := template.New(name).Parse(code)
	if err != nil {
		return nil, err
	}
	return &Template{tpl}, nil
}

// Template struct
type Template struct {
	tpl *template.Template
}

// Generate the code
func (t *Template) Generate(state interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := t.tpl.Execute(buf, state); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
