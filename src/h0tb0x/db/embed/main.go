package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"
)

var (
	pkg = flag.String("pkg", "db", "Name of the package to generate.")
)

type codeArgs struct {
	Package    string
	Name       string
	Latest     string
	Migrations []string
}

const CODE = `package {{.Package}}
func init() {
	schemas["{{.Name}}"] = &Schema{
		name: "{{.Name}}",
		latest: {{.Latest}},
		migrations: []string{
{{range .Migrations}}
			{{.}},{{end}}
		},
	}
}
`

func quote(buf []byte) string {
	return "`\n" + string(buf) + "`"
}

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s [options] <directory>\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(0)
	}

	dir, _ := filepath.Abs(filepath.Clean(flag.Args()[0]))

	infos, err := ioutil.ReadDir(filepath.Join(dir, "migrations"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[e] %s\n", err)
		return
	}

	latest, err := ioutil.ReadFile(filepath.Join(dir, "schema.sql"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[e] %s\n", err)
		return
	}

	tmpl := template.Must(template.New("code").Parse(CODE))
	args := &codeArgs{
		Package:    *pkg,
		Name:       filepath.Base(dir),
		Latest:     quote(latest),
		Migrations: []string{},
	}

	for _, fi := range infos {
		if fi.IsDir() {
			continue
		}
		data, err := ioutil.ReadFile(filepath.Join(dir, "migrations", fi.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "[e] %s\n", err)
			return
		}
		args.Migrations = append(args.Migrations, quote(data))
	}

	fout, err := os.Create(dir + ".go")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[e] %s\n", err)
		return
	}
	defer fout.Close()

	err = tmpl.Execute(fout, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[e] %s\n", err)
		return
	}
}
