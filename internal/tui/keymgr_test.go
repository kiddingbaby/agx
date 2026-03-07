package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	domainkey "github.com/kiddingbaby/agx/internal/domain/key"
	"github.com/kiddingbaby/agx/internal/usecase"
)

type keyRepoStub struct {
	keys []domainkey.Key
}

func (s *keyRepoStub) List() []domainkey.Key {
	out := make([]domainkey.Key, len(s.keys))
	copy(out, s.keys)
	return out
}

func (s *keyRepoStub) Add(provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	return nil, errors.New("not implemented")
}

func (s *keyRepoStub) Update(id string, provider domainkey.Provider, profile, name, apiKey, baseURL string, tags []string) (*domainkey.Key, error) {
	return nil, errors.New("not implemented")
}

func (s *keyRepoStub) Delete(id string) error {
	return errors.New("not implemented")
}

func (s *keyRepoStub) Activate(id string) error {
	return errors.New("not implemented")
}

func (s *keyRepoStub) GetActive(provider domainkey.Provider, profile string) (*domainkey.Key, error) {
	return nil, errors.New("not implemented")
}

func (s *keyRepoStub) HasActive(provider domainkey.Provider, profile string) bool {
	return false
}

func (s *keyRepoStub) Resolve(provider domainkey.Provider, profile, identifier string) (*domainkey.Key, error) {
	return nil, errors.New("not implemented")
}

func (s *keyRepoStub) ListProfiles(provider domainkey.Provider) []domainkey.Profile {
	return nil
}

func (s *keyRepoStub) SetProfileStrategy(provider domainkey.Provider, profile string, strategy domainkey.RotationStrategy, fixedKey string) error {
	return errors.New("not implemented")
}

func newTestKeyManagerModel() KeyManagerModel {
	now := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	repo := &keyRepoStub{keys: []domainkey.Key{
		{ID: "c1", Provider: domainkey.ProviderClaude, Profile: domainkey.DefaultProfile, Name: "claude-main", Tags: []string{"prod"}, Active: true, UpdatedAt: now},
		{ID: "c2", Provider: domainkey.ProviderClaude, Profile: domainkey.DefaultProfile, Name: "claude-backup", Tags: []string{"backup"}, UpdatedAt: now},
		{ID: "o1", Provider: domainkey.ProviderOpenAI, Profile: domainkey.DefaultProfile, Name: "openai-main", Tags: []string{"prod"}, UpdatedAt: now},
		{ID: "g1", Provider: domainkey.ProviderGemini, Profile: domainkey.DefaultProfile, Name: "gemini-main", Tags: []string{"lab"}, UpdatedAt: now},
	}}
	m := NewKeyManagerModel(usecase.NewKeyService(repo))
	m.width = 120
	m.height = 40
	return m
}

func keyRuneMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestBuildKeyRowsOnlyForSelectedProvider(t *testing.T) {
	m := newTestKeyManagerModel()

	if len(m.keyRows) != 2 {
		t.Fatalf("initial keyRows = %d, want 2 claude keys", len(m.keyRows))
	}
	for _, row := range m.keyRows {
		k, ok := m.getKeyByID(row.keyID)
		if !ok {
			t.Fatalf("missing key for row id %q", row.keyID)
		}
		if k.Provider != domainkey.ProviderClaude {
			t.Fatalf("provider = %s, want %s", k.Provider, domainkey.ProviderClaude)
		}
	}

	m.selectProvider(1)
	if len(m.keyRows) != 1 {
		t.Fatalf("openai keyRows = %d, want 1", len(m.keyRows))
	}
	k, ok := m.getKeyByID(m.keyRows[0].keyID)
	if !ok || k.Provider != domainkey.ProviderOpenAI {
		t.Fatalf("selected provider rows are not openai")
	}
}

func TestUpdateListProviderSwitchHotkeys(t *testing.T) {
	m := newTestKeyManagerModel()

	model, _ := m.updateList(tea.KeyMsg{Type: tea.KeyRight})
	right := model.(KeyManagerModel)
	if right.selectedProvider != 1 {
		t.Fatalf("right selectedProvider = %d, want 1", right.selectedProvider)
	}
	if len(right.keyRows) != 1 {
		t.Fatalf("right keyRows = %d, want 1", len(right.keyRows))
	}

	model, _ = right.updateList(keyRuneMsg('3'))
	num := model.(KeyManagerModel)
	if num.selectedProvider != 2 {
		t.Fatalf("number selectedProvider = %d, want 2", num.selectedProvider)
	}

	model, _ = num.updateList(tea.KeyMsg{Type: tea.KeyLeft})
	left := model.(KeyManagerModel)
	if left.selectedProvider != 1 {
		t.Fatalf("left selectedProvider = %d, want 1", left.selectedProvider)
	}
}

func TestUpdateListToggleDetailsShortcut(t *testing.T) {
	m := newTestKeyManagerModel()

	model, _ := m.updateList(keyRuneMsg('i'))
	withDetails := model.(KeyManagerModel)
	if !withDetails.showDetails {
		t.Fatalf("showDetails = false, want true")
	}

	view := withDetails.viewList()
	if !strings.Contains(view, "DETAILS") {
		t.Fatalf("view does not contain details section")
	}
	if !strings.Contains(view, "claude-main") {
		t.Fatalf("view does not contain selected key name")
	}
	if !strings.Contains(view, "API key is hidden for security") {
		t.Fatalf("view missing security note")
	}
}

func TestUpdateListEditShortcutOpensEditForm(t *testing.T) {
	m := newTestKeyManagerModel()

	model, _ := m.updateList(keyRuneMsg('e'))
	edited := model.(KeyManagerModel)
	if edited.view != KeyMgrViewForm {
		t.Fatalf("view = %v, want KeyMgrViewForm", edited.view)
	}
	if edited.formMode != "edit" {
		t.Fatalf("formMode = %q, want edit", edited.formMode)
	}
	if edited.formEditingKey != "c1" {
		t.Fatalf("formEditingKey = %q, want c1", edited.formEditingKey)
	}
	if edited.formName.Value() != "claude-main" {
		t.Fatalf("formName = %q, want claude-main", edited.formName.Value())
	}
}

func TestInitFormStartsInNormalMode(t *testing.T) {
	m := newTestKeyManagerModel()
	m.initForm(0)

	if m.formInsertMode {
		t.Fatalf("formInsertMode = true, want false")
	}
	if m.formFocus != 1 {
		t.Fatalf("formFocus = %d, want 1", m.formFocus)
	}
	if m.formName.Focused() {
		t.Fatalf("formName should not be focused in normal mode")
	}
}

func TestUpdateFormNormalModeJKNavigation(t *testing.T) {
	m := newTestKeyManagerModel()
	m.initForm(0)
	m.view = KeyMgrViewForm

	model, _ := m.updateForm(keyRuneMsg('k'))
	got := model.(KeyManagerModel)
	if got.formFocus != 0 {
		t.Fatalf("after k formFocus = %d, want 0", got.formFocus)
	}

	model, _ = got.updateForm(keyRuneMsg('j'))
	got = model.(KeyManagerModel)
	if got.formFocus != 1 {
		t.Fatalf("after j formFocus = %d, want 1", got.formFocus)
	}

	model, _ = got.updateForm(tea.KeyMsg{Type: tea.KeyDown})
	got = model.(KeyManagerModel)
	if got.formFocus != 2 {
		t.Fatalf("after down formFocus = %d, want 2", got.formFocus)
	}
}

func TestUpdateFormInsertModeByIAndEscSemantics(t *testing.T) {
	m := newTestKeyManagerModel()
	m.initForm(0)
	m.view = KeyMgrViewForm

	model, _ := m.updateForm(keyRuneMsg('x'))
	got := model.(KeyManagerModel)
	if got.formName.Value() != "" {
		t.Fatalf("formName = %q, want empty in normal mode", got.formName.Value())
	}

	model, _ = got.updateForm(keyRuneMsg('i'))
	got = model.(KeyManagerModel)
	if !got.formInsertMode {
		t.Fatalf("formInsertMode = false, want true")
	}
	if !got.formName.Focused() {
		t.Fatalf("formName should be focused in insert mode")
	}

	model, _ = got.updateForm(keyRuneMsg('x'))
	got = model.(KeyManagerModel)
	if got.formName.Value() != "x" {
		t.Fatalf("formName = %q, want x", got.formName.Value())
	}

	model, _ = got.updateForm(tea.KeyMsg{Type: tea.KeyEsc})
	got = model.(KeyManagerModel)
	if got.view != KeyMgrViewForm {
		t.Fatalf("view = %v, want stay in KeyMgrViewForm", got.view)
	}
	if got.formInsertMode {
		t.Fatalf("formInsertMode = true, want false after Esc in insert mode")
	}
	if got.formName.Focused() {
		t.Fatalf("formName should be blurred after Esc in insert mode")
	}

	model, _ = got.updateForm(tea.KeyMsg{Type: tea.KeyEsc})
	got = model.(KeyManagerModel)
	if got.view != KeyMgrViewList {
		t.Fatalf("view = %v, want KeyMgrViewList after Esc in normal mode", got.view)
	}
}

func TestUpdateFormNormalModeXClearsCurrentField(t *testing.T) {
	m := newTestKeyManagerModel()
	m.initForm(0)
	m.view = KeyMgrViewForm
	m.formFocus = 3
	m.formKey.SetValue("sk-pasted-long-key")

	model, _ := m.updateForm(keyRuneMsg('x'))
	got := model.(KeyManagerModel)
	if got.formKey.Value() != "" {
		t.Fatalf("formKey = %q, want empty after x", got.formKey.Value())
	}
	if got.view != KeyMgrViewForm {
		t.Fatalf("view = %v, want stay in form", got.view)
	}
}

func TestUpdateFormNormalModeEnterSavesNotCycleProvider(t *testing.T) {
	m := newTestKeyManagerModel()
	m.initForm(0)
	m.view = KeyMgrViewForm
	m.formFocus = 0

	model, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyEnter})
	got := model.(KeyManagerModel)
	if got.formProviderIdx != 0 {
		t.Fatalf("formProviderIdx = %d, want 0 (no cycle on enter)", got.formProviderIdx)
	}
	if got.errMsg == "" {
		t.Fatalf("errMsg should be set when saving empty form")
	}
}

func TestUpdateFormInsertModeCtrlUClearsField(t *testing.T) {
	m := newTestKeyManagerModel()
	m.initForm(0)
	m.view = KeyMgrViewForm

	model, _ := m.updateForm(keyRuneMsg('i'))
	got := model.(KeyManagerModel)
	got.formName.SetValue("to-be-cleared")

	model, _ = got.updateForm(tea.KeyMsg{Type: tea.KeyCtrlU})
	got = model.(KeyManagerModel)
	if got.formName.Value() != "" {
		t.Fatalf("formName = %q, want empty after ctrl+u", got.formName.Value())
	}
}
