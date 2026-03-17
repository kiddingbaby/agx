package cli

func isHelpToken(arg string) bool {
	return arg == "-h" || arg == "--help"
}

func hasHelpToken(args []string) bool {
	for _, arg := range args {
		if isHelpToken(arg) {
			return true
		}
	}
	return false
}
