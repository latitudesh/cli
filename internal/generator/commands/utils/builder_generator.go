package commands

import (
	"fmt"
	"path"

	"github.com/dave/jennifer/jen"
	"github.com/latitudesh/lsh/internal/utils"
)

// GenerateCmdBuilder generates the Command builder file.
// The Command builder file is responsible for adding all the subcomands to the main command and them add the main command to the CLI.
func GenerateCmdBuilder(cmds []Command) {
	rootName := cmds[0].Root
	root := fmt.Sprintf("%sCmd", rootName)

	f := jen.NewFile("cmd")
	init := f.Func().Id("init").Params()

	jenCmd := []jen.Code{}
	for _, cmd := range cmds {
		cmdName := parseCmdName(cmd.Name, rootName)
		titledCmd := utils.TitleStr(cmdName)

		jenCmd = append(jenCmd, jen.Id(root).Dot("AddCommand").Call(
			jen.Qual(
				fmt.Sprintf("github.com/latitudesh/lsh/cmd/%s", rootName),
				fmt.Sprintf("New%sCmd()", titledCmd),
			),
		))
	}
	jenCmd = append(jenCmd, jen.Line(), jen.Id("rootCmd").Dot("AddCommand").Call(jen.Id(root)))

	block := jen.Block(jenCmd...)
	init.Add(block)

	f.Var().Id(root).Op("=").Op("&").Qual("github.com/spf13/cobra", "Command").Values(jen.Dict{
		jen.Id("Use"): jen.Lit(rootName),
	})

	fileName := fmt.Sprintf("build_%s.go", rootName)

	err := f.Save(path.Join("cmd", fileName))
	if err != nil {
		fmt.Println(err)
	}
}
