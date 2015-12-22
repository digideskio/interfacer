// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/mvdan/interfacer"

	_ "golang.org/x/tools/go/gcimporter"
	"golang.org/x/tools/go/types"
)

var tmpl = template.Must(template.New("std").Parse(`// Generated by generate/std

package interfacer

var pkgs = map[string]struct{}{
{{range $_, $pkg := .Pkgs}}	"{{$pkg}}": struct{}{},
{{end}}}

var ifaces = map[string]string{
{{range $_, $pt := .Ifaces}}	"{{$pt.Type}}": "{{$pt.Path}}",
{{end}}}

var funcs = map[string]string{
{{range $_, $pt := .Funcs}}	"{{$pt.Type}}": "{{$pt.Path}}",
{{end}}}
`))

var out = flag.String("o", "", "output file")

type byLength []string

func (l byLength) Len() int {
	return len(l)
}
func (l byLength) Less(i, j int) bool {
	if len(l[i]) == len(l[j]) {
		return l[i] < l[j]
	}
	return len(l[i]) < len(l[j])
}
func (l byLength) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type pkgType struct {
	Type string
	Path string
}

func pkgName(fullname string) string {
	sp := strings.Split(fullname, ".")
	if len(sp) == 1 {
		return ""
	}
	return sp[0]
}

func fullName(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func prepare(in map[string]string) []pkgType {
	pkgNames := make(map[string][]string)
	nameTypes := make(map[string]string)
	for typestr, fullname := range in {
		path := pkgName(fullname)
		pkgNames[path] = append(pkgNames[path], fullname)
		nameTypes[fullname] = typestr
	}
	var result []pkgType
	for _, path := range pkgs {
		names := pkgNames[path]
		sort.Sort(interfacer.ByAlph(names))
		for _, fullname := range names {
			result = append(result, pkgType{
				Type: nameTypes[fullname],
				Path: fullname,
			})
		}
	}
	return result
}

func generate(w io.Writer) error {
	imported := make(map[string]*types.Package)
	sort.Sort(byLength(pkgs))
	ifaces := make(map[string]string)
	funcs := make(map[string]string)
	for _, path := range pkgs {
		if strings.Contains(path, "internal") {
			continue
		}
		scope := types.Universe
		all := true
		if path != "" {
			pkg, err := types.DefaultImport(imported, path)
			if err != nil {
				return err
			}
			scope = pkg.Scope()
			all = false
		}
		ifs, funs := interfacer.FromScope(scope, all)
		for iface, name := range ifs {
			if _, e := ifaces[iface]; e {
				continue
			}
			ifaces[iface] = fullName(path, name)
		}
		for fun, name := range funs {
			if _, e := funcs[fun]; e {
				continue
			}
			funcs[fun] = fullName(path, name)
		}
	}
	return tmpl.Execute(w, struct {
		Pkgs          []string
		Ifaces, Funcs []pkgType
	}{
		Pkgs:   pkgs,
		Ifaces: prepare(ifaces),
		Funcs:  prepare(funcs),
	})
}

func main() {
	flag.Parse()
	w := os.Stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			errExit(err)
		}
		defer f.Close()
		w = f
	}
	if err := generate(w); err != nil {
		errExit(err)
	}
}

func errExit(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}
