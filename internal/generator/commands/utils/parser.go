package commands

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/latitudesh/lsh/internal/utils"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

func ParseSpec(commands []string, spec []byte) []Command {
	document, err := libopenapi.NewDocument(spec)
	if err != nil {
		log.Fatal(err)
	}

	// Build document
	model, errors := document.BuildV3Model()
	if len(errors) > 0 {
		for i := range errors {
			fmt.Printf("error: %e\n", errors[i])
		}
		log.Fatalf("cannot create v3 model from "+
			"document: %d errors reported",
			len(errors))
	}

	cmdToGenerate := []Command{}

	for _, cmd := range commands {
		// Get paths for command
		// ie: /tags and /tags/{tag_id} belong to the same CLI command
		relatedPaths := []string{}
		allPaths := model.Index.GetAllPaths()
		relatedPaths = append(relatedPaths, "/"+cmd)

		rx := regexp.MustCompile(fmt.Sprintf("%s/{.*}$", cmd))
		for k := range allPaths {
			if rx.MatchString(k) {
				relatedPaths = append(relatedPaths, k)
			}
		}

		// Context for ordered map iteration
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Get info about root command
		for _, subC := range relatedPaths {
			path := model.Model.Paths.PathItems.GetOrZero(subC)
			if path == nil {
				log.Fatalf("%s path not found in OpenApi specification. Aborting execution!", subC)
			}

			c := orderedmap.Iterate(ctx, path.GetOperations())
			for pr := range c {
				cmd := Command{
					Name:       pr.Value().OperationId,
					Short:      pr.Value().Summary,
					Long:       pr.Value().Description,
					Method:     pr.Key(),
					Root:       cmd,
					Parameters: parseParameters(pr.Value()),
					Body:       parseBody(pr.Value()),
				}
				cmdToGenerate = append(cmdToGenerate, cmd)
			}
		}
	}
	return cmdToGenerate
}

func parseBody(params *v3.Operation) []CmdBody {
	result := []CmdBody{}
	firstContent := params.RequestBody
	if firstContent != nil {
		if firstContent.Content.First().Value().Schema.Schema().Properties.First().Value().Schema().Properties != nil {
			rend, _ := firstContent.Content.First().Value().Schema.Schema().Properties.First().Value().Schema().Properties.Get("attributes")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			c := orderedmap.Iterate(ctx, rend.Schema().Properties)
			for pr := range c {
				nullable := true
				if pr.Value().Schema().Nullable != nil {
					nullable = *pr.Value().Schema().Nullable
				}

				body := CmdBody{
					Name:        pr.Key(),
					Description: pr.Value().Schema().Description,
					Nullable:    nullable,
				}
				result = append(result, body)
			}
		}
	}
	return result
}

func parseParameters(params *v3.Operation) []CmdParameter {
	result := []CmdParameter{}
	resourceName := utils.Singular(strings.Split(params.OperationId, "-")[1])
	rx := regexp.MustCompile(`\[(.*)\]`)

	for _, param := range params.Parameters {

		paramName := param.Name

		if strings.Split(paramName, "_")[0] == resourceName {
			paramName = "id"
		}

		matchName := rx.FindStringSubmatch(paramName)
		if len(matchName) > 1 {
			paramName = matchName[1]
		}

		paramName = strings.Replace(paramName, "][", "_", -1)

		param := CmdParameter{
			Name:        paramName,
			Description: param.Description,
			Required:    *param.Required,
		}
		result = append(result, param)
	}

	return result
}
