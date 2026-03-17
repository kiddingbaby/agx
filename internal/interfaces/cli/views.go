package cli

import (
	"sort"
	"time"

	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	domainprovider "github.com/kiddingbaby/agx/internal/domain/provider"
)

type targetView struct {
	Name               string                    `json:"name"`
	Family             domainprovider.Family     `json:"family"`
	Kind               domainprovider.Kind       `json:"kind"`
	Access             domainprovider.AccessMode `json:"access"`
	Auth               domainprovider.AuthMode   `json:"auth"`
	BaseURL            string                    `json:"base_url,omitempty"`
	Model              string                    `json:"model,omitempty"`
	EnvKeys            []string                  `json:"env_keys,omitempty"`
	Env                map[string]string         `json:"env,omitempty"`
	WireAPI            domainprovider.WireAPI    `json:"wire_api,omitempty"`
	RequiresOpenAIAuth *bool                     `json:"requires_openai_auth,omitempty"`
}

func toTargetView(target domainprovider.Target, revealEnv bool) targetView {
	keys := make([]string, 0, len(target.Env))
	for k := range target.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := targetView{
		Name:               target.Name,
		Family:             target.Family,
		Kind:               target.Kind,
		Access:             target.Access,
		Auth:               target.Auth,
		BaseURL:            target.BaseURL,
		Model:              target.Model,
		EnvKeys:            keys,
		WireAPI:            target.WireAPI,
		RequiresOpenAIAuth: target.RequiresOpenAIAuth,
	}
	if revealEnv && len(target.Env) > 0 {
		out.Env = make(map[string]string, len(target.Env))
		for k, v := range target.Env {
			out.Env[k] = v
		}
	}
	return out
}

type bindingView struct {
	Family domainprovider.Family `json:"family"`
	Target string                `json:"target"`
}

func toBindingView(binding domainprovider.Binding) bindingView {
	return bindingView{
		Family: binding.Family,
		Target: binding.Target,
	}
}

type keyView struct {
	ID        string             `json:"id"`
	Provider  domainkey.Provider `json:"provider"`
	Profile   string             `json:"profile"`
	Name      string             `json:"name"`
	BaseURL   string             `json:"base_url,omitempty"`
	Tags      []string           `json:"tags,omitempty"`
	Active    bool               `json:"active"`
	CreatedAt time.Time          `json:"created_at,omitempty"`
	UpdatedAt time.Time          `json:"updated_at,omitempty"`
}

func toKeyView(k domainkey.Key, normalizedProfile string) keyView {
	return keyView{
		ID:        k.ID,
		Provider:  k.Provider,
		Profile:   normalizedProfile,
		Name:      k.Name,
		BaseURL:   k.BaseURL,
		Tags:      k.Tags,
		Active:    k.Active,
		CreatedAt: k.CreatedAt,
		UpdatedAt: k.UpdatedAt,
	}
}

type profileView struct {
	Provider  domainkey.Provider         `json:"provider"`
	Name      string                     `json:"name"`
	Strategy  domainkey.RotationStrategy `json:"strategy"`
	FixedKey  string                     `json:"fixed_key,omitempty"`
	NextIndex int                        `json:"next_index,omitempty"`
	UpdatedAt time.Time                  `json:"updated_at,omitempty"`
}
