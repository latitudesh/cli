package commands

import (
	"fmt"
	"path"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/latitudesh/lsh/internal/utils"
)

type CmdGenerator struct {
	Name      string
	Root      string
	Operation string
	Cmd       Command
}

func NewCmdGenerator(cmd Command) *CmdGenerator {
	cmdName := parseCmdName(cmd.Name, cmd.Root)
	titledCmd := utils.TitleStr(cmdName)
	titledRoot := utils.TitleStr(cmd.Root)
	opName := fmt.Sprintf("%s%sOperation", titledCmd, titledRoot)

	return &CmdGenerator{
		Name:      parseCmdName(cmd.Name, cmd.Root),
		Root:      titledRoot,
		Operation: opName,
		Cmd:       cmd,
	}
}

// GenerateCmd generates a command file.
func (cg *CmdGenerator) GenerateCmd() {
	f := jen.NewFile(cg.Cmd.Root)

	cg.genNewOperationCmd(f)
	cg.genOperation(f)
	cg.genRegisterFlags(f)
	cg.genPreRun(f)
	cg.genRun(f)

	filepath := path.Join("cmd", cg.Cmd.Root, cg.Name+".go")
	err := f.Save(filepath)
	if err != nil {
		fmt.Println(err)
	}
}

func (cg *CmdGenerator) genNewOperationCmd(f *jen.File) {
	titledName := utils.TitleStr(cg.Name)

	cmdOptions := jen.Dict{
		jen.Id("Use"):   jen.Lit(cg.Name),
		jen.Id("Short"): jen.Lit(cg.Cmd.Short),
		jen.Id("Long"):  jen.Lit(cg.Cmd.Long),
		jen.Id("RunE"):  jen.Id("op").Dot("run"),
	}

	registerFlags := jen.Statement{}

	if len(cg.Cmd.Parameters) != 0 {
		cmdOptions[jen.Id("PreRun")] = jen.Id("op").Dot("preRun")
		registerFlags = *jen.Id("op").Dot("registerFlags").Call(jen.Id("cmd"))
	}

	f.Func().Id(fmt.Sprintf("New%sCmd", titledName)).Params().Op("*").Qual("github.com/spf13/cobra", "Command").Block(
		jen.Id("op").Op(":=").Id(cg.Operation).Block(),
		jen.Id("cmd").Op(":=").Op("&").Qual("github.com/spf13/cobra", "Command").Values(cmdOptions),
		&registerFlags,
		jen.Return(jen.Id("cmd")),
	)
}

func (cg *CmdGenerator) genOperation(f *jen.File) {
	jenParams := []jen.Code{}

	switch cg.Name {
	case "list":
		if len(cg.Cmd.Parameters) != 0 {
			jenParams = append(jenParams, jen.Id("PathParamFlags").Qual("github.com/latitudesh/lsh/internal/cmdflag", "Flags"))
		}
	case "get", "destroy":
		jenParams = append(jenParams, jen.Id("PathParamFlags").Qual("github.com/latitudesh/lsh/internal/cmdflag", "Flags"))
	case "create":
		jenParams = append(jenParams, jen.Id("BodyAttributesFlags").Qual("github.com/latitudesh/lsh/internal/cmdflag", "Flags"))
	default:
		jenParams = append(jenParams,
			jen.Id("PathParamFlags").Qual("github.com/latitudesh/lsh/internal/cmdflag", "Flags"),
			jen.Id("BodyAttributesFlags").Qual("github.com/latitudesh/lsh/internal/cmdflag", "Flags"),
		)
	}

	f.Type().Id(cg.Operation).Struct(jenParams...)
}

func (cg *CmdGenerator) genRegisterFlags(f *jen.File) {
	if len(cg.Cmd.Parameters) == 0 {
		return
	}

	schema := []jen.Code{}

	for _, param := range cg.Cmd.Parameters {
		schema = append(schema, jen.Op("&").Qual("github.com/latitudesh/lsh/internal/cmdflag", "String").Values(jen.Dict{
			jen.Id("Name"):        jen.Lit(param.Name),
			jen.Id("Label"):       jen.Lit(param.Name),
			jen.Id("Description"): jen.Lit(param.Description),
			jen.Id("Required"):    jen.Lit(param.Required),
		}))
	}

	f.Func().Params(
		jen.Id("op").Op("*").Id(cg.Operation),
	).Id("registerFlags").Params(
		jen.Id("cmd").Op("*").Qual("github.com/spf13/cobra", "Command"),
	).Block(
		jen.Id("op").Dot("PathParamFlags").Op("=").Qual("github.com/latitudesh/lsh/internal/cmdflag", "Flags").Values(jen.Dict{
			jen.Id("FlagSet"): jen.Id("cmd").Dot("Flags").Call(),
		}),
		jen.Id("schema").Op(":=").Op("&").Qual("github.com/latitudesh/lsh/internal/cmdflag", "FlagsSchema").Values(schema...),
		jen.Id("op").Dot("PathParamFlags").Dot("Register").Call(jen.Id("schema")),
	)
}

func (cg *CmdGenerator) genPreRun(f *jen.File) {
	if len(cg.Cmd.Parameters) == 0 {
		return
	}

	f.Func().Params(
		jen.Id("op").Op("*").Id(cg.Operation),
	).Id("preRun").Params(
		jen.Id("cmd").Op("*").Qual("github.com/spf13/cobra", "Command"),
		jen.Id("args").Index().String(),
	).Block(
		jen.Id("op").Dot("PathParamFlags").Dot("PreRun").Call(jen.Id("cmd"), jen.Id("args")),
	)
}

func (cg *CmdGenerator) genRun(f *jen.File) {
	f.Func().Params(
		jen.Id("op").Op("*").Id(cg.Operation),
	).Id("run").Params(
		jen.Id("cmd").Op("*").Qual("github.com/spf13/cobra", "Command"),
		jen.Id("args").Index().String(),
	).Error().Block(

		jen.Id("c").Op(":=").Qual("github.com/latitudesh/lsh/cmd/lsh", "NewClient").Call(),
		jen.Line(),
		jen.If(jen.Qual("github.com/latitudesh/lsh/cmd/lsh", "DryRun")).Block(
			jen.Qual("github.com/latitudesh/lsh/cmd/lsh", "LogDebugf").Call(jen.Lit("dry-run flag specified. Skip sending request.")),
			jen.Return(jen.Nil()),
		),
		jen.Line(),

		cg.genkSdkFunc(),

		jen.If(jen.Id("err").Op("!=").Nil()).Block(
			jen.Qual("github.com/latitudesh/lsh/internal/utils", "PrintError").Call(jen.Id("err")),
			jen.Return(jen.Id("err")),
		),
		jen.Line(),

		cg.genDataSection(),

		jen.Line(),

		cg.genRenderSection(),

		jen.Line(),
		jen.Return(jen.Nil()),
	)

}

func (cg *CmdGenerator) genkSdkFunc() *jen.Statement {
	titledCmd := utils.TitleStr(cg.Name)
	sdkSection := jen.Statement{}

	switch cg.Name {
	case "get", "update":
		sdkSection = jen.Statement{
			jen.Id("attr").Op(":=").Struct(jen.Id("ID").String().Tag(map[string]string{"json": "id"})).Block(),
			jen.Line(),
			jen.Id("op").Dot("PathParamFlags").Dot("AssignValues").Call(jen.Op("&").Id("attr")),
			jen.Line(),
			jen.List(jen.Id(cg.Cmd.Root), jen.Id("_"), jen.Id("err")).Op(":=").Id("c").Dot(cg.Root).Dot(titledCmd).Call(jen.Id("attr").Dot("ID"), jen.Nil()),
		}
	case "destroy":
		sdkSection = jen.Statement{
			jen.Id("attr").Op(":=").Struct(jen.Id("ID").String().Tag(map[string]string{"json": "id"})).Block(),
			jen.Line(),
			jen.Id("op").Dot("PathParamFlags").Dot("AssignValues").Call(jen.Op("&").Id("attr")),
			jen.Line(),
			jen.List(jen.Id("resp"), jen.Id("err")).Op(":=").Id("c").Dot(cg.Root).Dot("Delete").Call(jen.Id("attr").Dot("ID")),
		}
	default:
		if cg.Cmd.Root == "servers" && titledCmd == "List" {
			sdkSection = jen.Statement{jen.List(jen.Id(cg.Cmd.Root), jen.Id("_"), jen.Id("err")).Op(":=").Id("c").Dot(cg.Root).Dot(titledCmd).Call(jen.Lit(""), jen.Nil())}
		} else {
			sdkSection = jen.Statement{jen.List(jen.Id(cg.Cmd.Root), jen.Id("_"), jen.Id("err")).Op(":=").Id("c").Dot(cg.Root).Dot(titledCmd).Call(jen.Nil())}
		}
	}

	return &sdkSection
}

func (cg *CmdGenerator) genDataSection() *jen.Statement {
	dataSection := jen.Statement{}

	switch cg.Name {
	case "list":
		dataSection = jen.Statement{
			jen.Id("lshData").Op(":=").Index().Op("*").Id(utils.Singular(cg.Root)).Block(),
			jen.Line(),
			jen.For().List(jen.Id("_"), jen.Id(utils.Singular(cg.Cmd.Root))).Op(":=").Range().Id(cg.Cmd.Root).Block(
				jen.Id(fmt.Sprintf("lsh%s", utils.Singular(cg.Root))).Op(":=").Id(utils.Singular(cg.Root)).Values(
					jen.Dict{
						jen.Id("Attributes"): jen.Id(utils.Singular(cg.Cmd.Root)),
					},
				),
				jen.Id("lshData").Op("=").Append(jen.Id("lshData"), jen.Op("&").Id(fmt.Sprintf("lsh%s", utils.Singular(cg.Root)))),
			),
			jen.Line(),
			jen.Id(fmt.Sprintf("lsh%s", cg.Root)).Op(":=").Id(cg.Root).Values(
				jen.Dict{
					jen.Id("Data"): jen.Id("lshData"),
				},
			),
		}
	case "destroy":
	default:
		dataSection = jen.Statement{
			jen.Id(fmt.Sprintf("lsh%s", cg.Root)).Op(":=").Id(utils.Singular(cg.Root)).Values(
				jen.Dict{
					jen.Id("Attributes"): jen.Op("*").Id(cg.Cmd.Root),
				},
			),
		}
	}

	return &dataSection
}

func (cg *CmdGenerator) genRenderSection() *jen.Statement {

	renderSection := jen.Statement{}

	switch cg.Name {
	case "destroy":
		renderSection = jen.Statement{
			jen.If(jen.Op("!").Qual("github.com/latitudesh/lsh/cmd/lsh", "Debug")).Block(
				jen.If(jen.Id("resp").Dot("StatusCode").Op("!=").Qual("net/http", "StatusNoContent")).Block(
					jen.Qual("log", "Fatal").Call(jen.Lit("Something went wrong while deleting resource.")),
				),
				jen.Qual("fmt", "Printf").Call(jen.Lit(fmt.Sprintf("\n\n%s deleted successfully!\n\n", utils.Singular(cg.Root)))),
			),
		}
	default:
		renderSection = jen.Statement{
			jen.If(jen.Op("!").Qual("github.com/latitudesh/lsh/cmd/lsh", "Debug")).Block(
				jen.Qual("github.com/latitudesh/lsh/internal/utils", "Render").Call(jen.Id(fmt.Sprintf("lsh%s", cg.Root)).Dot("GetData").Call()),
			),
		}
	}

	return &renderSection
}

func parseCmdName(name, root string) string {
	splitName := strings.Split(name, "-")
	if splitName[0] == "get" && splitName[1] == root {
		splitName[0] = "list"
	}
	return splitName[0]
}
