package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"golang.org/x/term"
)

var errInteractiveCanceled = errors.New("interactive input canceled")

type profileMutationArgs struct {
	name          string
	baseURL       *string
	apiKey        *string
	bind          []domainprofile.Agent
	unbind        []domainprofile.Agent
	asJSON        bool
	mutationFlags int
}

type promptSession struct {
	lines        lineReader
	passwordFile *os.File
}

type lineReader interface {
	ReadLine() (string, error)
}

type bufferedLineReader struct {
	reader *bufio.Reader
}

func (r *bufferedLineReader) ReadLine() (string, error) {
	return r.reader.ReadString('\n')
}

type fileLineReader struct {
	file *os.File
}

func (r *fileLineReader) ReadLine() (string, error) {
	var line strings.Builder
	var buf [1]byte

	for {
		n, err := r.file.Read(buf[:])
		if n > 0 {
			line.WriteByte(buf[0])
			if buf[0] == '\n' {
				return line.String(), nil
			}
		}
		if err != nil {
			if err == io.EOF && line.Len() > 0 {
				return line.String(), err
			}
			return "", err
		}
	}
}

func isTerminalReader(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func (r *Root) canPrompt(asJSON bool) bool {
	if asJSON || r.isTTY == nil {
		return false
	}
	return r.isTTY()
}

func newPromptSession(stdin io.Reader) *promptSession {
	if file, ok := stdin.(*os.File); ok && isTerminalReader(file) {
		return &promptSession{
			lines:        &fileLineReader{file: file},
			passwordFile: file,
		}
	}
	return &promptSession{
		lines: &bufferedLineReader{reader: bufio.NewReader(stdin)},
	}
}

func parseProfileMutationArgs(r *Root, args []string, usage string) (profileMutationArgs, bool) {
	var parsed profileMutationArgs

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base-url":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --base-url requires a value")
				return profileMutationArgs{}, false
			}
			value := args[i+1]
			parsed.baseURL = &value
			parsed.mutationFlags++
			i++
		case "--api-key":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --api-key requires a value")
				return profileMutationArgs{}, false
			}
			value := args[i+1]
			parsed.apiKey = &value
			parsed.mutationFlags++
			i++
		case "--bind":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --bind requires a value")
				return profileMutationArgs{}, false
			}
			agents, ok := parseAgentList(r, args[i+1])
			if !ok {
				return profileMutationArgs{}, false
			}
			parsed.bind = append(parsed.bind, agents...)
			parsed.mutationFlags++
			i++
		case "--unbind":
			if i+1 >= len(args) {
				fmt.Fprintln(r.stderr, "Error: --unbind requires a value")
				return profileMutationArgs{}, false
			}
			agents, ok := parseAgentList(r, args[i+1])
			if !ok {
				return profileMutationArgs{}, false
			}
			parsed.unbind = append(parsed.unbind, agents...)
			parsed.mutationFlags++
			i++
		case "-o":
			if i+1 >= len(args) || args[i+1] != "json" {
				fmt.Fprintln(r.stderr, "Error: -o requires value json")
				return profileMutationArgs{}, false
			}
			parsed.asJSON = true
			i++
		default:
			if strings.HasPrefix(args[i], "-") || parsed.name != "" {
				fmt.Fprintln(r.stderr, usage)
				return profileMutationArgs{}, false
			}
			parsed.name = args[i]
		}
	}

	parsed.bind = normalizeParsedAgents(parsed.bind)
	parsed.unbind = normalizeParsedAgents(parsed.unbind)

	return parsed, true
}

func (r *Root) promptForAdd(session *promptSession, parsed profileMutationArgs) (profileMutationArgs, error) {
	var err error

	if strings.TrimSpace(parsed.name) == "" {
		parsed.name, err = r.promptProfileName(session, "Relay name: ", "")
		if err != nil {
			return profileMutationArgs{}, err
		}
	}
	if parsed.baseURL == nil {
		value, err := r.promptValidatedText(session, "Base URL: ", "", domainprofile.ValidateBaseURL)
		if err != nil {
			return profileMutationArgs{}, err
		}
		parsed.baseURL = &value
	}
	if parsed.apiKey == nil {
		value, err := r.promptAPIKey(session, "API key: ", "")
		if err != nil {
			return profileMutationArgs{}, err
		}
		parsed.apiKey = &value
	}

	return parsed, nil
}

func (r *Root) promptForEdit(session *promptSession, parsed profileMutationArgs, current domainprofile.Profile) (profileMutationArgs, error) {
	for {
		choice, done, err := r.promptEditField(session, parsed.mutationFlags > 0)
		if err != nil {
			return profileMutationArgs{}, err
		}
		if done {
			return parsed, nil
		}

		switch choice {
		case "base_url":
			baseURL, err := r.promptValidatedText(session, fmt.Sprintf("Base URL [%s]: ", current.BaseURL), current.BaseURL, domainprofile.ValidateBaseURL)
			if err != nil {
				return profileMutationArgs{}, err
			}
			parsed.baseURL = &baseURL
		case "api_key":
			apiKey, err := r.promptAPIKey(session, "API key [keep current]: ", current.APIKey)
			if err != nil {
				return profileMutationArgs{}, err
			}
			parsed.apiKey = &apiKey
		}
		parsed.mutationFlags++
	}
}

func (r *Root) promptEditField(session *promptSession, hasChanges bool) (string, bool, error) {
	prompt := "Edit [1 url, 2 key, Enter done]: "
	if hasChanges {
		prompt = "Edit more [1 url, 2 key, Enter done]: "
	}

	for {
		fmt.Fprint(r.stdout, prompt)
		line, err := session.lines.ReadLine()
		if err != nil && len(line) == 0 {
			return "", false, errInteractiveCanceled
		}

		value := strings.TrimSpace(strings.ToLower(line))
		if value == "" {
			if !hasChanges {
				fmt.Fprintln(r.stdout, "No changes made.")
			}
			return "", true, nil
		}
		switch value {
		case "1", "url", "base", "base-url", "base_url":
			return "base_url", false, nil
		case "2", "key", "api-key", "api_key":
			return "api_key", false, nil
		default:
			fmt.Fprintln(r.stdout, "Invalid value: use 1/2 or url/key")
		}
	}
}

func (r *Root) printInteractiveEditSummary(current domainprofile.Profile) {
	fmt.Fprintf(r.stdout, "Current relay: %s\n", current.Name)
	fmt.Fprintf(r.stdout, "  base_url=%s\n", current.BaseURL)
	fmt.Fprintln(r.stdout, "  api_key=[hidden]")
}

func (r *Root) promptProfileName(session *promptSession, prompt, current string) (string, error) {
	return r.promptValidatedText(session, prompt, current, domainprofile.ValidateProfileName)
}

func (r *Root) promptValidatedText(session *promptSession, prompt, current string, validate func(string) error) (string, error) {
	for {
		fmt.Fprint(r.stdout, prompt)
		line, err := session.lines.ReadLine()
		if err != nil && len(line) == 0 {
			return "", errInteractiveCanceled
		}

		value := strings.TrimSpace(line)
		if value == "" && current != "" {
			return current, nil
		}
		if err := validate(value); err != nil {
			fmt.Fprintf(r.stdout, "Invalid value: %v\n", err)
			continue
		}
		return value, nil
	}
}

func (r *Root) promptAPIKey(session *promptSession, prompt, current string) (string, error) {
	if session.passwordFile == nil {
		return r.promptValidatedText(session, prompt, current, domainprofile.ValidateAPIKey)
	}

	for {
		fmt.Fprint(r.stdout, prompt)
		secret, err := term.ReadPassword(int(session.passwordFile.Fd()))
		fmt.Fprintln(r.stdout)
		if err != nil {
			return "", errInteractiveCanceled
		}

		value := strings.TrimSpace(string(secret))
		if value == "" && current != "" {
			return current, nil
		}
		if err := domainprofile.ValidateAPIKey(value); err != nil {
			fmt.Fprintf(r.stdout, "Invalid value: %v\n", err)
			continue
		}
		return value, nil
	}
}
