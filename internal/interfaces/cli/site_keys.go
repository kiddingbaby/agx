package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

func (r *Root) resolveScopeForSite(site string) (target *domainprovider.Target, provider domainkey.Provider, profile string, err error) {
	if r.providerSvc == nil {
		return nil, "", "", errors.New("provider config service is unavailable")
	}
	target, _, err = r.resolveSiteTarget(site)
	if err != nil {
		return nil, "", "", err
	}
	provider, profile, err = r.providerSvc.KeyScopeForTarget(*target)
	if err != nil {
		return nil, "", "", err
	}
	return target, provider, profile, nil
}

func resolveKeyMaterial(keyRaw, envRaw, fileRaw string) (string, error) {
	keyRaw = strings.TrimSpace(keyRaw)
	envRaw = strings.TrimSpace(envRaw)
	fileRaw = strings.TrimSpace(fileRaw)

	if keyRaw != "" {
		return resolveBootstrapSecret(keyRaw)
	}
	if envRaw != "" {
		v := strings.TrimSpace(os.Getenv(envRaw))
		if v == "" {
			return "", fmt.Errorf("env %s is empty", envRaw)
		}
		return v, nil
	}
	if fileRaw != "" {
		data, err := os.ReadFile(fileRaw)
		if err != nil {
			return "", err
		}
		v := strings.TrimSpace(string(data))
		if v == "" {
			return "", fmt.Errorf("key file is empty: %s", fileRaw)
		}
		return v, nil
	}
	return "", errors.New("missing key material")
}

func (r *Root) importKeysForScopeFromStdinOrInteractive(provider domainkey.Provider, profile string, tags []string) ([]domainkey.Key, error) {
	if !stdinIsCharDevice() {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		lines := splitLines(string(data))
		return r.importKeysFromLines(provider, profile, tags, lines)
	}

	if !stderrIsCharDevice() {
		return nil, errors.New("interactive paste requires a TTY (stderr)")
	}
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)
	fmt.Fprintf(r.stderr, "Paste keys for %s/%s (one per line), then press Enter on an empty line to finish.\n", provider, domainkey.NormalizeProfileName(profile))
	fmt.Fprintln(r.stderr, "Formats:")
	fmt.Fprintln(r.stderr, "  <key>")
	fmt.Fprintln(r.stderr, "  <name> <key>")
	fmt.Fprintln(r.stderr, "  <name>=<key>")
	fmt.Fprintln(r.stderr, "  env:VAR / file:/path are supported as key values.")

	lines := []string{}
	for {
		fmt.Fprint(r.stderr, "> ")
		line, err := readLine(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		lines = append(lines, line)
	}
	return r.importKeysFromLines(provider, profile, tags, lines)
}

func (r *Root) importKeysFromLines(provider domainkey.Provider, profile string, tags []string, lines []string) ([]domainkey.Key, error) {
	imported := []domainkey.Key{}
	counter := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		hintProviderRaw, rest := splitProviderHint(line)
		if strings.TrimSpace(hintProviderRaw) != "" && strings.TrimSpace(hintProviderRaw) != string(provider) {
			return nil, fmt.Errorf("provider mismatch in line %q (expected %s)", line, provider)
		}

		name, value, err := parseNameValueLine(rest)
		if err != nil {
			return nil, fmt.Errorf("parse line %q: %w", line, err)
		}
		secret, err := resolveBootstrapSecret(value)
		if err != nil {
			return nil, fmt.Errorf("resolve secret for line %q: %w", line, err)
		}
		secret = strings.TrimSpace(secret)
		if secret == "" {
			return nil, fmt.Errorf("empty key value for line %q", line)
		}

		keyName := strings.TrimSpace(name)
		if keyName == "" {
			counter++
			keyName = fmt.Sprintf("%s-%02d", provider, counter)
		}

		unique, err := r.uniqueKeyName(provider, profile, keyName)
		if err != nil {
			return nil, err
		}

		k, err := r.keySvc.Add(provider, profile, unique, secret, "", tags)
		if err != nil {
			return nil, err
		}
		imported = append(imported, *k)
	}
	return imported, nil
}
