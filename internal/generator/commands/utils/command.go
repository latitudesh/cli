package commands

type Command struct {
	Name       string
	Short      string
	Long       string
	Method     string
	Root       string
	Parameters []CmdParameter
	Body       []CmdBody
}

type CmdParameter struct {
	Name        string
	Description string
	Required    bool
}

type CmdBody struct {
	Name        string
	Description string
	Nullable    bool
}
