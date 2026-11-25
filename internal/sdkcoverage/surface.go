// Package sdkcoverage reconciles the service groups and methods exposed by the
// pinned latitudesh-go-sdk against the commands the lsh CLI ships, using a
// hand-curated manifest as the record of intent.
//
// The SDK is Speakeasy-generated: every service group is a struct hanging off
// the root client, and new platform capability shows up there first. Parsing is
// purely syntactic (go/ast, no type-checking) so it needs nothing but the module
// source already in the local module cache.
package sdkcoverage

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// rootClientType is the SDK's top-level client struct. Its exported fields
// enumerate every service group the SDK exposes, which makes it the single
// authoritative starting point for the walk.
const rootClientType = "Latitudesh"

// maxGroupDepth bounds the nesting walk. The SDK nests exactly one level today
// (Firewalls.Assignments, Teams.Members, Plans.VM, Projects.SSHKeys); the extra
// level is slack so a future addition still gets picked up.
const maxGroupDepth = 3

// Group is one SDK service group together with the exported methods declared on
// it.
type Group struct {
	// Name is the dotted access path from the root client, e.g. "Firewalls" or
	// "Firewalls.Assignments".
	Name string

	// TypeName is the Go type backing the group. It usually matches the last
	// segment of Name, but not always: the field Projects.SSHKeys is typed
	// LatitudeshProjectsSSHKeys, so methods must be resolved by type, never by
	// field name.
	TypeName string

	// Methods are the exported method names on TypeName, sorted.
	Methods []string

	// Routes maps a method name to the HTTP verb and path it calls, when the
	// method body follows the generated shape (a NewRequestWithContext with a
	// literal verb, and a url.JoinPath or utils.GenerateURL with a literal
	// path). Used to recognise one operation reachable through two routes.
	Routes map[string]Route
}

// Route is the HTTP verb and path template an SDK method calls.
type Route struct {
	Verb string
	Path string
}

// Normalized returns the path with every parameter collapsed to "{}", so
// "/virtual_machines/{virtual_machine_id}/backups" and
// "/virtual_machines/{id}/backups" compare equal.
func (r Route) Normalized() string {
	var b strings.Builder
	depth := 0
	for _, c := range r.Path {
		switch {
		case c == '{':
			depth++
			b.WriteString("{}")
		case c == '}':
			depth--
		case depth == 0:
			b.WriteRune(c)
		}
	}
	return b.String()
}

// Surface is the exported service-group surface of one SDK version, keyed by
// dotted path.
type Surface struct {
	Groups map[string]Group
}

// Names returns the dotted group paths in sorted order.
func (s Surface) Names() []string {
	names := make([]string, 0, len(s.Groups))
	for name := range s.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MethodCount is the number of methods across every group.
func (s Surface) MethodCount() int {
	n := 0
	for _, g := range s.Groups {
		n += len(g.Methods)
	}
	return n
}

// ParseSurface reads every non-test .go file at the root of dir (the SDK's
// service groups all live in one flat package) and walks the root client struct
// to build the group graph.
func ParseSurface(dir string) (Surface, error) {
	idx, err := indexPackage(dir)
	if err != nil {
		return Surface{}, err
	}
	if _, ok := idx.structFields[rootClientType]; !ok {
		return Surface{}, fmt.Errorf("sdkcoverage: root client type %q not found in %s", rootClientType, dir)
	}

	surface := Surface{Groups: make(map[string]Group)}
	idx.collectGroups(rootClientType, "", 0, surface.Groups, map[string]bool{})
	return surface, nil
}

// structField is an exported pointer-to-struct field, the shape the SDK uses for
// every service group.
type structField struct {
	name     string
	typeName string
}

// pkgIndex is the raw syntactic index of one SDK source tree.
type pkgIndex struct {
	// structFields maps a struct type name to its exported pointer-to-struct
	// fields, in declaration order.
	structFields map[string][]structField

	// methods maps a receiver type name to its exported method names.
	methods map[string][]string

	// routes maps "Type.Method" to the route its body calls, when found.
	routes map[string]Route
}

func indexPackage(dir string) (*pkgIndex, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("sdkcoverage: reading %s: %w", dir, err)
	}

	idx := &pkgIndex{
		structFields: make(map[string][]structField),
		methods:      make(map[string][]string),
		routes:       make(map[string]Route),
	}
	fset := token.NewFileSet()

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("sdkcoverage: parsing %s: %w", name, err)
		}
		idx.addFile(file)
	}

	return idx, nil
}

func (idx *pkgIndex) addFile(file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil || d.Name == nil || !d.Name.IsExported() {
				continue
			}
			if recv := receiverTypeName(d.Recv); recv != "" {
				idx.methods[recv] = append(idx.methods[recv], d.Name.Name)
				if route, ok := routeOf(d); ok {
					idx.routes[recv+"."+d.Name.Name] = route
				}
			}
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				idx.structFields[ts.Name.Name] = groupFields(st)
			}
		}
	}
}

// collectGroups walks the exported pointer-to-struct fields of typeName,
// recording each one that resolves to a struct in the same package as a group.
// seen guards against a struct referencing itself or an ancestor.
func (idx *pkgIndex) collectGroups(typeName, prefix string, depth int, out map[string]Group, seen map[string]bool) {
	if depth >= maxGroupDepth || seen[typeName] {
		return
	}
	seen[typeName] = true

	for _, field := range idx.structFields[typeName] {
		if _, isLocalStruct := idx.structFields[field.typeName]; !isLocalStruct {
			continue
		}

		path := field.name
		if prefix != "" {
			path = prefix + "." + field.name
		}

		methods := append([]string(nil), idx.methods[field.typeName]...)
		sort.Strings(methods)
		routes := make(map[string]Route)
		for _, m := range methods {
			if r, ok := idx.routes[field.typeName+"."+m]; ok {
				routes[m] = r
			}
		}
		out[path] = Group{Name: path, TypeName: field.typeName, Methods: methods, Routes: routes}

		idx.collectGroups(field.typeName, path, depth+1, out, seen)
	}
}

// groupFields keeps only exported fields of the form `Name *LocalType`. That
// excludes the root client's `SDKVersion string`, every unexported field, and
// qualified types such as `config.SDKConfiguration`.
func groupFields(st *ast.StructType) []structField {
	if st.Fields == nil {
		return nil
	}

	var fields []structField
	for _, field := range st.Fields.List {
		star, ok := field.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := star.X.(*ast.Ident)
		if !ok {
			continue
		}
		for _, name := range field.Names {
			if name.IsExported() {
				fields = append(fields, structField{name: name.Name, typeName: ident.Name})
			}
		}
	}
	return fields
}

// routeOf finds the HTTP verb and path literal a generated method calls. The
// path is the string literal passed to url.JoinPath(baseURL, "...") or
// utils.GenerateURL(ctx, baseURL, "...", ...); the verb is the literal passed
// to http.NewRequestWithContext(ctx, "VERB", ...). Either missing means the
// body is not the generated shape, and no route is recorded.
func routeOf(fn *ast.FuncDecl) (Route, bool) {
	var route Route
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "JoinPath":
			if len(call.Args) >= 2 {
				route.Path = stringLit(call.Args[1])
			}
		case "GenerateURL":
			if len(call.Args) >= 3 {
				route.Path = stringLit(call.Args[2])
			}
		case "NewRequestWithContext":
			if len(call.Args) >= 2 {
				route.Verb = stringLit(call.Args[1])
			}
		}
		return true
	})
	return route, route.Verb != "" && route.Path != ""
}

func stringLit(e ast.Expr) string {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return s
}

func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	switch t := recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}
