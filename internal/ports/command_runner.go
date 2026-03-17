package ports

// CommandResult captures the outcome of running an external command.
type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
}

// CommandRunner runs external commands.
type CommandRunner interface {
	Run(name string, args []string, env map[string]string) CommandResult
}
