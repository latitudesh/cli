package commands

import (
	"fmt"
	"path"

	"github.com/dave/jennifer/jen"
	"github.com/latitudesh/lsh/internal/utils"
)

func GenerateResource(cmd Command) {
	titledRoot := utils.TitleStr(cmd.Root)

	f := jen.NewFile(cmd.Root)

	f.ImportAlias("github.com/latitudesh/latitudesh-go", "sdk")

	f.Type().Id(titledRoot).Struct(
		jen.Id("Data").Index().Op("*").Id(utils.Singular(titledRoot)),
	)

	f.Func().Params(
		jen.Id("m").Op("*").Id(titledRoot),
	).Id("GetData").Params().Index().Qual("github.com/latitudesh/lsh/internal/renderer", "ResponseData").Block(
		jen.Var().Id("data").Index().Qual("github.com/latitudesh/lsh/internal/renderer", "ResponseData"),

		jen.Line(),

		jen.For().List(jen.Id("_"), jen.Id("v")).Op(":=").Range().Id("m").Dot("Data").Block(
			jen.Id("data").Op("=").Append(jen.Id("data"), jen.Id("v")),
		),

		jen.Return(jen.Id("data")),
	)

	f.Type().Id(utils.Singular(titledRoot)).Struct(
		jen.Id("Attributes").Qual("github.com/latitudesh/latitudesh-go", utils.Singular(titledRoot)),
	)

	f.Func().Params(
		jen.Id("m").Op("*").Id(utils.Singular(titledRoot)),
	).Id("GetData").Params().Index().Qual("github.com/latitudesh/lsh/internal/renderer", "ResponseData").Block(
		jen.Return(jen.Index().Qual("github.com/latitudesh/lsh/internal/renderer", "ResponseData").Values(jen.Id("m"))),
	)

	f.Func().Params(
		jen.Id("m").Op("*").Id(utils.Singular(titledRoot)),
	).Id("Validate").Params(jen.Id("formats").Qual("github.com/go-openapi/strfmt", "Registry")).Error().Block(
		jen.Return(jen.Nil()),
	)

	f.Func().Params(
		jen.Id("m").Op("*").Id(utils.Singular(titledRoot)),
	).Id("ContextValidate").Params(jen.Id("ctx").Qual("context", "Context"), jen.Id("formats").Qual("github.com/go-openapi/strfmt", "Registry")).Error().Block(
		jen.Return(jen.Nil()),
	)

	f.Func().Params(
		jen.Id("m").Op("*").Id(utils.Singular(titledRoot)),
	).Id("MarshalBinary").Params().Params(jen.Index().Byte(), jen.Error()).Block(
		jen.Return(jen.Index().Byte().Block(), jen.Nil()),
	)

	f.Func().Params(
		jen.Id("m").Op("*").Id(utils.Singular(titledRoot)),
	).Id("UnmarshalBinary").Params(jen.Id("b").Index().Byte()).Error().Block(
		jen.Return(jen.Nil()),
	)

	f.Comment("// TODO: The render table needs to be filled manually")
	//Table generation only generates a table with an ID column, other values should be added manually
	f.Func().Params(
		jen.Id("m").Op("*").Id(utils.Singular(titledRoot)),
	).Id("TableRow").Params().Qual("github.com/latitudesh/lsh/internal/output/table", "Row").Block(
		jen.Id("attr").Op(":=").Id("m").Dot("Attributes"),
		jen.Line(),
		jen.Return(jen.Id("table").Dot("Row").Values(
			jen.Dict{
				jen.Lit("id"): jen.Id("table").Dot("Cell").Values(
					jen.Dict{
						jen.Id("Label"): jen.Lit("ID"),
						jen.Id("Value"): jen.Id("table").Dot("String").Call(jen.Id("attr").Dot("ID")),
					},
				),
			},
		)),
	)

	filepath := path.Join("cmd", cmd.Root, cmd.Root+".go")
	err := f.Save(filepath)
	if err != nil {
		fmt.Println(err)
	}
}
