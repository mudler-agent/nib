package chat

import (
	"strings"

	"github.com/mudler/cogito"
	"github.com/mudler/nib/codeindex"
)

type indexArgs struct {
	Path string `json:"path" jsonschema:"path to the source file (relative to the workspace or absolute)"`
}

type indexTool struct {
	resolvePath func(string) string
}

func (t *indexTool) Run(args map[string]any) (string, any, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "index error: 'path' is required", nil, nil
	}
	out, err := codeindex.Index(t.resolvePath(path))
	if err != nil {
		return "index failed: " + err.Error(), nil, nil
	}
	return out, nil, nil
}

func indexToolDefinition(resolvePath func(string) string) cogito.ToolDefinitionInterface {
	return cogito.NewToolDefinition[map[string]any](&indexTool{resolvePath: resolvePath}, indexArgs{},
		"index",
		"Return a compact outline of a source file: imports, type definitions, function signatures, "+
			"and structure, each with its line range in []. It costs a fraction of reading the full file.\n\n"+
			"Use it on a large source file to find which lines to read with offset/limit. "+
			"For a file of ordinary size, read it instead.\n"+
			"Supported files: "+strings.Join(codeindex.SupportedExtensions(), " ")+". "+
			"Other file types return an error; read those instead.",
	)
}
