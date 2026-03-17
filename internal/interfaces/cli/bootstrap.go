package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/usecase"
)

func (r *Root) bootstrapImportKeys(reader *bufio.Reader, provider domainkey.Provider, profile string) ([]domainkey.Key, error) {
	fmt.Fprintf(r.stderr, "\nAdd API keys for %s (profile=%s).\n", provider, domainkey.NormalizeProfileName(profile))
	fmt.Fprintln(r.stderr, "Paste one key per line, then press Enter on an empty line to finish.")
	fmt.Fprintln(r.stderr, "Formats:")
	fmt.Fprintln(r.stderr, "  <key>")
	fmt.Fprintln(r.stderr, "  <name> <key>")
	fmt.Fprintln(r.stderr, "  <name>=<key>")
	fmt.Fprintln(r.stderr, "  env:VAR or file:/path are also supported as key values.")

	imported := []domainkey.Key{}
	index := 0
	for {
		fmt.Fprint(r.stderr, "> ")
		line, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		name, value, err := parseNameValueLine(line)
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, errors.New("key value cannot be empty")
		}
		if strings.TrimSpace(name) == "" {
			index++
			name = fmt.Sprintf("%s-%02d", provider, index)
		}

		unique, err := r.uniqueKeyName(provider, profile, name)
		if err != nil {
			return nil, err
		}
		secret, err := resolveBootstrapSecret(value)
		if err != nil {
			return nil, err
		}
		k, err := r.keySvc.Add(provider, profile, unique, secret, "", nil)
		if err != nil {
			return nil, err
		}
		imported = append(imported, *k)
	}
	return imported, nil
}

type bootstrapImportScope struct {
	Provider domainkey.Provider
	Profile  string
}

func (r *Root) bootstrapImportKeysShared(reader *bufio.Reader, scopes []bootstrapImportScope, header string) ([]domainkey.Key, error) {
	if len(scopes) == 0 {
		return nil, errors.New("no import scope provided")
	}

	header = strings.TrimSpace(header)
	if header == "" {
		header = "Add API keys:"
	}
	fmt.Fprintf(r.stderr, "\n%s\n", header)
	fmt.Fprintln(r.stderr, "Paste one key per line, then press Enter on an empty line to finish.")
	fmt.Fprintln(r.stderr, "Formats:")
	fmt.Fprintln(r.stderr, "  <key>")
	fmt.Fprintln(r.stderr, "  <name> <key>")
	fmt.Fprintln(r.stderr, "  <name>=<key>")
	fmt.Fprintln(r.stderr, "  env:VAR or file:/path are also supported as key values.")

	type material struct {
		Name   string
		Secret string
	}

	materials := []material{}
	index := 0
	for {
		fmt.Fprint(r.stderr, "> ")
		line, err := readLine(reader)
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		name, value, err := parseNameValueLine(line)
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, errors.New("key value cannot be empty")
		}
		if strings.TrimSpace(name) == "" {
			index++
			name = fmt.Sprintf("key-%02d", index)
		}
		secret, err := resolveBootstrapSecret(value)
		if err != nil {
			return nil, err
		}
		materials = append(materials, material{Name: name, Secret: secret})
	}

	created := []domainkey.Key{}
	createdIDs := []string{}
	rollback := func() {
		for i := len(createdIDs) - 1; i >= 0; i-- {
			_ = r.keySvc.Delete(createdIDs[i])
		}
	}

	for _, scope := range scopes {
		provider := scope.Provider
		profile := domainkey.NormalizeProfileName(scope.Profile)
		for _, m := range materials {
			unique, err := r.uniqueKeyName(provider, profile, m.Name)
			if err != nil {
				rollback()
				return nil, err
			}
			k, err := r.keySvc.Add(provider, profile, unique, m.Secret, "", nil)
			if err != nil {
				rollback()
				return nil, err
			}
			created = append(created, *k)
			createdIDs = append(createdIDs, k.ID)
		}
	}

	return created, nil
}

func (r *Root) uniqueKeyName(provider domainkey.Provider, profile, desired string) (string, error) {
	name := strings.TrimSpace(desired)
	if name == "" {
		return "", errors.New("key name cannot be empty")
	}
	if strings.IndexFunc(name, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }) >= 0 {
		return "", fmt.Errorf("key name cannot contain whitespace: %q", desired)
	}

	candidate := name
	for i := 2; i < 1000; i++ {
		_, err := r.keySvc.FindByIdentifierInScope(provider, profile, candidate)
		if err != nil {
			if usecase.IsKeyNotFoundError(err) {
				return candidate, nil
			}
			return "", err
		}
		candidate = fmt.Sprintf("%s-%d", name, i)
	}
	return "", fmt.Errorf("failed to find a unique key name for %q", name)
}

func parseNameValueLine(line string) (name string, value string, err error) {
	if left, right, ok := strings.Cut(line, "="); ok {
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		if left != "" && right != "" && strings.IndexFunc(left, func(r rune) bool { return r == ' ' || r == '\t' }) < 0 {
			return left, right, nil
		}
	}
	fields := strings.Fields(line)
	if len(fields) == 1 {
		return "", fields[0], nil
	}
	if len(fields) >= 2 {
		return fields[0], fields[1], nil
	}
	return "", "", errors.New("empty line")
}

func resolveBootstrapSecret(value string) (string, error) {
	v := strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(v, "env:"):
		name := strings.TrimSpace(strings.TrimPrefix(v, "env:"))
		if name == "" {
			return "", errors.New("env var name is empty")
		}
		got := strings.TrimSpace(os.Getenv(name))
		if got == "" {
			return "", fmt.Errorf("env %s is empty", name)
		}
		return got, nil
	case strings.HasPrefix(v, "file:"):
		path := strings.TrimSpace(strings.TrimPrefix(v, "file:"))
		if path == "" {
			return "", errors.New("file path is empty")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		got := strings.TrimSpace(string(data))
		if got == "" {
			return "", fmt.Errorf("key file is empty: %s", path)
		}
		return got, nil
	default:
		return v, nil
	}
}

func promptString(reader *bufio.Reader, w io.Writer, prompt string) (string, error) {
	fmt.Fprint(w, prompt)
	line, err := readLine(reader)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func promptStringRequired(reader *bufio.Reader, w io.Writer, prompt string) (string, error) {
	for {
		fmt.Fprint(w, prompt)
		line, err := readLine(reader)
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err == nil {
		return strings.TrimRight(line, "\r\n"), nil
	}
	if errors.Is(err, io.EOF) {
		if strings.TrimSpace(line) != "" {
			return strings.TrimRight(line, "\r\n"), nil
		}
		return "", err
	}
	return "", err
}
