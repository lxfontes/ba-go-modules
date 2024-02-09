// Package bindgen generates Go source code from a fully-resolved WIT package.
// It generates one or more Go packages, with functions, types, constants, and variables,
// along with the associated code to lift and lower Go types into Canonical ABI representation.
package bindgen

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ydnar/wasm-tools-go/internal/codec"
	"github.com/ydnar/wasm-tools-go/internal/go/gen"
	"github.com/ydnar/wasm-tools-go/wit"
)

const (
	HeaderPattern = `// Code generated by %s. DO NOT EDIT.`
	GoSuffix      = ".wit.go"
	cmPackage     = "github.com/ydnar/wasm-tools-go/cm"
)

// Go generates one or more Go packages from [wit.Resolve] res.
// It returns any error that occurs during code generation.
func Go(res *wit.Resolve, opts ...Option) ([]*gen.Package, error) {
	g, err := newGenerator(res, opts...)
	if err != nil {
		return nil, err
	}
	return g.generate()
}

type generator struct {
	opts options
	res  *wit.Resolve

	// versioned is set to true if there are multiple versions of a WIT package in res,
	// which affects the generated Go package paths.
	versioned bool

	// packages are Go packages indexed on Go package paths.
	packages map[string]*gen.Package

	// witPackages map WIT identifier paths to Go packages.
	witPackages map[string]*gen.Package

	// worldPackages map [wit.World] to Go packages.
	worldPackages map[*wit.World]*gen.Package

	// interfacePackages map [wit.Interface] to Go packages.
	interfacePackages map[*wit.Interface]*gen.Package

	// typeDecls map [wit.TypeDef] to their equivalent Go declaration.
	typeDecls map[*wit.TypeDef]gen.Decl

	// funcDecls map [wit.Function] to their equivalent Go declaration.
	funcDecls map[*wit.Function]gen.Decl

	// defined represent whether a type or function has been defined.
	defined map[gen.Decl]bool
}

func newGenerator(res *wit.Resolve, opts ...Option) (*generator, error) {
	g := &generator{
		packages:          make(map[string]*gen.Package),
		witPackages:       make(map[string]*gen.Package),
		worldPackages:     make(map[*wit.World]*gen.Package),
		interfacePackages: make(map[*wit.Interface]*gen.Package),
		typeDecls:         make(map[*wit.TypeDef]gen.Decl),
		funcDecls:         make(map[*wit.Function]gen.Decl),
		defined:           make(map[gen.Decl]bool),
	}
	err := g.opts.apply(opts...)
	if err != nil {
		return nil, err
	}
	if g.opts.generator == "" {
		_, file, _, _ := runtime.Caller(0)
		_, g.opts.generator = filepath.Split(filepath.Dir(filepath.Dir(file)))
	}
	if g.opts.packageName == "" {
		g.opts.packageName = res.Packages[0].Name.Namespace
	}
	if g.opts.cmPackage == "" {
		g.opts.cmPackage = cmPackage
	}
	g.res = res
	return g, nil
}

func (g *generator) generate() ([]*gen.Package, error) {
	g.detectVersionedPackages()

	err := g.declareTypeDefs()
	if err != nil {
		return nil, err
	}

	// err := g.defineInterfaces()
	// if err != nil {
	// 	return nil, err
	// }

	err = g.defineWorlds()
	if err != nil {
		return nil, err
	}

	var packages []*gen.Package
	for _, path := range codec.SortedKeys(g.packages) {
		packages = append(packages, g.packages[path])
	}
	return packages, nil
}

func (g *generator) detectVersionedPackages() {
	packages := make(map[string]string)
	for _, pkg := range g.res.Packages {
		id := pkg.Name
		id.Version = nil
		path := id.String()
		if packages[path] != "" && packages[path] != pkg.Name.String() {
			g.versioned = true
		} else {
			packages[path] = pkg.Name.String()
		}
	}
	if g.versioned {
		fmt.Fprintf(os.Stderr, "Multiple versions of package(s) detected\n")
	}
}

// declareTypeDefs declares all type definitions in res.
func (g *generator) declareTypeDefs() error {
	for _, t := range g.res.TypeDefs {
		err := g.declareTypeDef(t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) declareTypeDef(t *wit.TypeDef) error {
	if t.Name == nil {
		return nil
	}
	name := *t.Name

	var ownerID wit.Ident
	switch owner := t.Owner.(type) {
	case *wit.World:
		ownerID = owner.Package.Name
		ownerID.Extension = owner.Name
	case *wit.Interface:
		ownerID = owner.Package.Name
		ownerID.Extension = *owner.Name // FIXME: this might panic
	}

	pkg := g.packageForIdent(ownerID)
	file := pkg.File(ownerID.Extension + GoSuffix)
	decl := file.Decl(GoName(name))
	g.typeDecls[t] = decl

	fmt.Fprintf(os.Stderr, "Type:\t%s.%s\n\t%s.%s\n", ownerID.String(), name, decl.Package.Path, decl.Name)

	return nil
}

func (g *generator) defineInterfaces() error {
	var interfaces []*wit.Interface
	for _, i := range g.res.Interfaces {
		if i.Name != nil {
			interfaces = append(interfaces, i)
		}
	}
	fmt.Fprintf(os.Stderr, "Generating Go for %d named interface(s)\n", len(interfaces))
	for _, i := range interfaces {
		g.defineInterface(i, *i.Name)
	}
	return nil
}

// By default, each WIT interface and world maps to a single Go package.
// Options might override the Go package, including combining multiple
// WIT interfaces and/or worlds into a single Go package.
func (g *generator) defineWorlds() error {
	fmt.Fprintf(os.Stderr, "Generating Go for %d world(s)\n", len(g.res.Worlds))
	for _, w := range g.res.Worlds {
		g.defineWorld(w)
	}
	return nil
}

func (g *generator) defineWorld(w *wit.World) error {
	if g.worldPackages[w] != nil {
		return nil
	}
	id := w.Package.Name
	id.Extension = w.Name
	pkg := g.packageForIdent(id)
	g.worldPackages[w] = pkg

	file := pkg.File(id.Extension + GoSuffix)
	var b strings.Builder
	fmt.Fprintf(&b, "Package %s represents the %s \"%s\".\n", pkg.Name, w.WITKind(), id.String())
	if w.Docs.Contents != "" {
		b.WriteString("\n")
		b.WriteString(w.Docs.Contents)
	}
	file.PackageDocs = b.String()

	// fmt.Printf("// World: %s\n\n", id.String())
	for _, name := range codec.SortedKeys(w.Imports) {
		var err error
		switch v := w.Imports[name].(type) {
		case *wit.Interface:
			err = g.defineInterface(v, name)
		case *wit.TypeDef:
			err = g.defineTypeDef(v, name)
		case *wit.Function:
			err = g.defineImportedFunction(v, id)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) defineInterface(i *wit.Interface, name string) error {
	if g.interfacePackages[i] != nil {
		return nil
	}
	if i.Name != nil {
		name = *i.Name
	}
	id := i.Package.Name
	id.Extension = name
	pkg := g.packageForIdent(id)
	g.interfacePackages[i] = pkg

	file := pkg.File(id.Extension + GoSuffix)
	var b strings.Builder
	fmt.Fprintf(&b, "Package %s represents the %s \"%s\".\n", pkg.Name, i.WITKind(), id.String())
	if i.Docs.Contents != "" {
		b.WriteString("\n")
		b.WriteString(i.Docs.Contents)
	}
	file.PackageDocs = b.String()

	for _, name := range codec.SortedKeys(i.TypeDefs) {
		g.defineTypeDef(i.TypeDefs[name], name)
	}

	for _, name := range codec.SortedKeys(i.Functions) {
		g.defineImportedFunction(i.Functions[name], id)
	}

	return nil
}

func (g *generator) defineTypeDef(t *wit.TypeDef, name string) error {
	if t.Name != nil {
		name = *t.Name
	}

	decl := g.typeDecls[t]
	if decl == (gen.Decl{}) {
		return fmt.Errorf("typedef %s not declared", name)
	}
	if g.defined[decl] {
		return nil
	}
	// TODO: should we emit data for aliases?
	if t.Root() != t {
		return nil
	}

	var ownerID wit.Ident
	switch owner := t.Owner.(type) {
	case *wit.World:
		ownerID = owner.Package.Name
		ownerID.Extension = owner.Name
	case *wit.Interface:
		ownerID = owner.Package.Name
		ownerID.Extension = *owner.Name // FIXME: this might panic
	}

	pkg := decl.Package
	file := pkg.File(ownerID.Extension + GoSuffix)

	fmt.Fprintf(file, "// %s represents the %s \"%s#%s\".\n", decl.Name, t.WITKind(), ownerID.String(), name)
	fmt.Fprintf(file, "//\n")
	fmt.Fprint(file, gen.FormatDocComments(t.WIT(nil, "")))
	fmt.Fprintf(file, "//\n")
	if t.Docs.Contents != "" {
		fmt.Fprintf(file, "//\n%s", gen.FormatDocComments(t.Docs.Contents))
	}
	fmt.Fprintf(file, "type %s ", decl.Name)
	g.printTypeDef(file, t)
	fmt.Fprint(file, "\n\n")

	return nil
}

func (g *generator) printTypeDef(file *gen.File, t *wit.TypeDef) error {
	switch kind := t.Kind.(type) {
	// [Record], [Resource], [Handle], [Flags], [Tuple], [Variant], [Enum],
	// [Option], [Result], [List], [Future], [Stream], or [Type].
	case *wit.Resource:
		return g.printResource(file, kind)
	case *wit.OwnedHandle:
		fmt.Fprintf(file, "any /* TODO: *wit.OwnedHandle */")
	case *wit.BorrowedHandle:
		fmt.Fprintf(file, "any /* TODO: *wit.BorrowedHandle */")
	case *wit.Flags:
		fmt.Fprintf(file, "any /* TODO: *wit.Flags */")
	case *wit.Record:
		fmt.Fprintf(file, "any /* TODO: *wit.Record */")
	case *wit.Tuple:
		fmt.Fprintf(file, "any /* TODO: *wit.Tuple */")
	case *wit.Variant:
		fmt.Fprintf(file, "any /* TODO: *wit.Variant */")
	case *wit.Enum:
		fmt.Fprintf(file, "any /* TODO: *wit.Enum */")
	case *wit.Option:
		fmt.Fprintf(file, "any /* TODO: *wit.Option */")
	case *wit.Result:
		fmt.Fprintf(file, "any /* TODO: *wit.Result */")
	case *wit.List:
		fmt.Fprintf(file, "any /* TODO: *wit.List */")
	case *wit.Future:
		fmt.Fprintf(file, "any /* TODO: *wit.Future */")
	case *wit.Stream:
		fmt.Fprintf(file, "any /* TODO: *wit.Stream */")
	// wit.Type is last because wit.TypeDef implements wit.Type
	// TODO: be more specific?
	case wit.Type:
		_ = kind
		fmt.Fprintf(file, "any /* TODO: wit.Type */")
	}
	return nil
}

func (g *generator) printResource(file *gen.File, r *wit.Resource) error {
	cm := file.Import(g.opts.cmPackage)
	fmt.Fprintf(file, "%s.Resource\n\n", cm)
	fmt.Fprintf(file, "// TODO: resource methods")
	return nil
}

func (g *generator) printOption(file *gen.File, r *wit.Option) error {
	cm := file.Import(g.opts.cmPackage)
	fmt.Fprintf(file, "%s.Option[any /* TODO */]\n\n", cm)
	return nil
}

func (g *generator) defineImportedFunction(f *wit.Function, ownerID wit.Ident) error {
	if !f.IsFreestanding() {
		return nil
	}
	if _, ok := g.funcDecls[f]; ok {
		return nil
	}

	pkg := g.packageForIdent(ownerID)
	file := pkg.File(ownerID.Extension + GoSuffix)

	funcDecl := file.Decl(GoName(f.Name))
	g.funcDecls[f] = funcDecl
	snakeDecl := file.Decl(SnakeName(f.Name))

	fmt.Fprintf(file, "// %s represents the %s \"%s#%s\".\n", funcDecl.Name, f.WITKind(), ownerID.String(), f.Name)
	if f.Docs.Contents != "" {
		fmt.Fprintf(file, "//\n%s", gen.FormatDocComments(f.Docs.Contents))
	}
	fmt.Fprintf(file, "func %s()\n\n", funcDecl.Name)
	fmt.Fprintf(file, "//go:wasmimport %s %s\n", ownerID.String(), f.Name)
	fmt.Fprintf(file, "func %s()\n\n", snakeDecl.Name)

	return nil
}

func (g *generator) packageForIdent(id wit.Ident) *gen.Package {
	// Find existing
	pkg := g.witPackages[id.String()]
	if pkg != nil {
		return pkg
	}

	// Create a new package
	path := id.Namespace + "/" + id.Package + "/" + GoPackageName(id.Extension)
	if g.opts.packageRoot != "" && g.opts.packageRoot != "std" {
		path = g.opts.packageRoot + "/" + path
	}
	name := id.Extension
	if g.versioned && id.Version != nil {
		path += "/v" + id.Version.String()
	}

	name = GoPackageName(name)
	// Ensure local name doesn’t conflict with Go keywords or predeclared identifiers
	if gen.Unique(name, gen.IsReserved) != name {
		// Try with package prefix, like error -> ioerror
		name = id.Package + name
		if gen.Unique(name, gen.IsReserved) != name {
			// Try with namespace prefix, like ioerror -> wasiioerror
			name = gen.Unique(id.Namespace+name, gen.IsReserved)
		}
	}

	pkg = gen.NewPackage(path + "#" + name)
	g.packages[pkg.Path] = pkg
	g.witPackages[id.String()] = pkg

	return pkg
}
