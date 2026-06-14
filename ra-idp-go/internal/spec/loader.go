package spec

// SCL ドキュメント全体のローダー。TS の src/spec-bindings/scl.ts に対応する。

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/goccy/go-yaml"
)

type SCL struct {
	System         string                  `yaml:"system"`
	SpecVersion    string                  `yaml:"spec_version"`
	Components     map[string]Component    `yaml:"components"`
	Standards      map[string]Standard     `yaml:"standards"`
	Vocabulary     map[string]Vocabulary   `yaml:"vocabulary"`
	Models         map[string]Model        `yaml:"models"`
	Interfaces     map[string]Interface    `yaml:"interfaces"`
	States         map[string]StateMachine `yaml:"states"`
	Invariants     map[string]Invariant    `yaml:"invariants"`
	Scenarios      map[string]Scenario     `yaml:"scenarios"`
	Permissions    map[string]Permission   `yaml:"permissions"`
	Objectives     map[string]Objective    `yaml:"objectives"`
	Assurance      map[string]Assurance    `yaml:"assurance"`
	UserExperience UserExperience          `yaml:"user_experience"`
	Annotations    map[string]any          `yaml:"annotations"`
}

type Component struct {
	Description     string                `yaml:"description"`
	OwnsModels      []string              `yaml:"owns_models"`
	OwnsStates      []string              `yaml:"owns_states"`
	OwnsEvents      []string              `yaml:"owns_events"`
	OwnsInterfaces  []string              `yaml:"owns_interfaces"`
	OwnsInvariants  []string              `yaml:"owns_invariants"`
	OwnsPermissions []string              `yaml:"owns_permissions"`
	OwnsObjectives  []string              `yaml:"owns_objectives"`
	DependsOn       []ComponentDependency `yaml:"depends_on"`
	Annotations     map[string]any        `yaml:"annotations"`
}

type ComponentDependency struct {
	Component string `yaml:"component"`
	Reason    string `yaml:"reason"`
}

type Standard struct {
	Title        string                `yaml:"title"`
	Version      string                `yaml:"version"`
	URL          string                `yaml:"url"`
	Roles        []string              `yaml:"roles"`
	Scope        string                `yaml:"scope"`
	Requirements []StandardRequirement `yaml:"requirements"`
}

type StandardRequirement struct {
	ID        string              `yaml:"id"`
	Section   string              `yaml:"section"`
	Strength  string              `yaml:"strength"`
	Adoption  string              `yaml:"adoption"`
	Statement string              `yaml:"statement"`
	Reason    string              `yaml:"reason"`
	RelatesTo map[string][]string `yaml:"relates_to"`
}

type UserExperience struct {
	Accessibility map[string]string               `yaml:"accessibility"`
	Locales       []string                        `yaml:"locales"`
	Screens       map[string]UserExperienceScreen `yaml:"screens"`
	Transitions   []UserExperienceTransition      `yaml:"transitions"`
	Requirements  []UserExperienceRequirement     `yaml:"requirements"`
}

type UserExperienceScreen struct {
	Route      string   `yaml:"route"`
	Purpose    string   `yaml:"purpose"`
	Interfaces []string `yaml:"interfaces"`
	States     []string `yaml:"states"`
}

type UserExperienceTransition struct {
	From      string `yaml:"from"`
	To        string `yaml:"to"`
	Trigger   string `yaml:"trigger"`
	Interface string `yaml:"interface"`
	External  bool   `yaml:"external"`
}

type UserExperienceRequirement struct {
	ID         string   `yaml:"id"`
	Category   string   `yaml:"category"`
	Adoption   string   `yaml:"adoption"`
	Statement  string   `yaml:"statement"`
	Reason     string   `yaml:"reason"`
	Screens    []string `yaml:"screens"`
	Interfaces []string `yaml:"interfaces"`
	Standards  []string `yaml:"standards"`
	Scenarios  []string `yaml:"scenarios"`
	Invariants []string `yaml:"invariants"`
}

type Vocabulary struct {
	Description      string             `yaml:"description"`
	Definition       string             `yaml:"definition"`
	Aliases          []string           `yaml:"aliases"`
	Context          string             `yaml:"context"`
	NotToConfuseWith []NotToConfuseWith `yaml:"not_to_confuse_with"`
	Annotations      map[string]any     `yaml:"annotations"`
}

type NotToConfuseWith struct {
	Term   string `yaml:"term"`
	Reason string `yaml:"reason"`
}

type Model struct {
	Kind        string              `yaml:"kind"`
	Description string              `yaml:"description"`
	Identity    any                 `yaml:"identity"`
	Fields      map[string]FieldDef `yaml:"fields"`
	Values      []string            `yaml:"values"`
	Payload     map[string]FieldDef `yaml:"payload"`
	Annotations map[string]any      `yaml:"annotations"`
}

type FieldDef struct {
	Type        string         `yaml:"type"`
	Optional    bool           `yaml:"optional"`
	Default     any            `yaml:"default"`
	Constraints []any          `yaml:"constraints"`
	Description string         `yaml:"description"`
	Annotations map[string]any `yaml:"annotations"`
}

type Interface struct {
	Description string              `yaml:"description"`
	Steps       []string            `yaml:"steps"`
	Input       map[string]FieldDef `yaml:"input"`
	Output      map[string]FieldDef `yaml:"output"`
	Errors      []string            `yaml:"errors"`
	Emits       []string            `yaml:"emits"`
	Idempotent  bool                `yaml:"idempotent"`
	ReadOnly    bool                `yaml:"read_only"`
	Bindings    []Binding           `yaml:"bindings"`
	Annotations map[string]any      `yaml:"annotations"`
}

// Binding は generic な map で受け、kind に応じて型変換するスタイル。
// Go の sum type 表現は冗長なため、kind ベース field アクセスを許容する。
type Binding map[string]any

func (b Binding) Kind() string {
	if s, ok := b["kind"].(string); ok {
		return s
	}
	return ""
}

func (b Binding) String(k string) string {
	if s, ok := b[k].(string); ok {
		return s
	}
	return ""
}

type StateMachine struct {
	Description string         `yaml:"description"`
	Target      string         `yaml:"target"`
	Initial     string         `yaml:"initial"`
	Terminal    []string       `yaml:"terminal"`
	Transitions []Transition   `yaml:"transitions"`
	Annotations map[string]any `yaml:"annotations"`
}

type Transition struct {
	From   string   `yaml:"from"`
	Event  string   `yaml:"event"`
	To     string   `yaml:"to"`
	Guard  any      `yaml:"guard"`
	Effect []string `yaml:"effect"`
}

type Invariant struct {
	Description string         `yaml:"description"`
	Target      string         `yaml:"target"`
	Assuming    any            `yaml:"assuming"`
	Always      any            `yaml:"always"`
	Eventually  any            `yaml:"eventually"`
	Never       any            `yaml:"never"`
	Within      string         `yaml:"within"`
	Severity    string         `yaml:"severity"`
	Annotations map[string]any `yaml:"annotations"`
}

type Scenario struct {
	Description string           `yaml:"description"`
	Steps       []string         `yaml:"steps"`
	Where       []map[string]any `yaml:"where"`
	Tags        []string         `yaml:"tags"`
	Annotations map[string]any   `yaml:"annotations"`
}

type Permission struct {
	Description string         `yaml:"description"`
	Actor       string         `yaml:"actor"`
	Action      string         `yaml:"action"`
	Resource    string         `yaml:"resource"`
	AllowWhen   any            `yaml:"allow_when"`
	DenyWhen    any            `yaml:"deny_when"`
	Annotations map[string]any `yaml:"annotations"`
}

type Objective struct {
	Kind        string         `yaml:"kind"`
	Description string         `yaml:"description"`
	Reference   string         `yaml:"reference"`
	Metric      string         `yaml:"metric"`
	Target      string         `yaml:"target"`
	Window      string         `yaml:"window"`
	Policy      string         `yaml:"policy"`
	Retention   string         `yaml:"retention"`
	TTL         string         `yaml:"ttl"`
	SingleUse   bool           `yaml:"single_use"`
	Value       any            `yaml:"value"`
	Interface   string         `yaml:"interface"`
	Note        string         `yaml:"note"`
	Annotations map[string]any `yaml:"annotations"`
}

type Assurance struct {
	Claim       string                       `yaml:"claim"`
	Risk        string                       `yaml:"risk"`
	RiskLevel   string                       `yaml:"risk_level"`
	DerivedFrom map[string][]string          `yaml:"derived_from"`
	Acceptance  AssuranceAcceptance          `yaml:"acceptance"`
	Evidence    map[string]AssuranceEvidence `yaml:"evidence"`
	Approval    AssuranceApproval            `yaml:"approval"`
	Annotations map[string]any               `yaml:"annotations"`
}

type AssuranceAcceptance struct {
	Evidence  string                `yaml:"evidence"`
	Criterion string                `yaml:"criterion"`
	All       []AssuranceAcceptance `yaml:"all"`
	Any       []AssuranceAcceptance `yaml:"any"`
	Not       *AssuranceAcceptance  `yaml:"not"`
}

type AssuranceEvidence struct {
	Kind         string              `yaml:"kind"`
	Producer     string              `yaml:"producer"`
	Evaluation   string              `yaml:"evaluation"`
	Environments []string            `yaml:"environments"`
	Recheck      string              `yaml:"recheck"`
	Covers       map[string][]string `yaml:"covers"`
	Procedure    string              `yaml:"procedure"`
	Oracle       string              `yaml:"oracle"`
}

type AssuranceApproval struct {
	When           []string `yaml:"when"`
	Role           string   `yaml:"role"`
	DecisionRecord bool     `yaml:"decision_record"`
}

// =====================================================================
// ローダー
// =====================================================================

var loaded *SCL

func LoadSCL() (*SCL, error) {
	if loaded != nil {
		return loaded, nil
	}
	path := os.Getenv("SCL_PATH")
	if path == "" {
		_, here, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("loader: cannot determine caller path")
		}
		root := filepath.Join(filepath.Dir(here), "..", "..")
		path = filepath.Join(root, "spec", "scl.yaml")
	}
	raw, err := os.ReadFile(path) //nolint:gosec // SCL_PATH is an explicit operator-controlled configuration path.
	if err != nil {
		return nil, fmt.Errorf("loader: read %s: %w", path, err)
	}
	s, err := DecodeSCL(raw)
	if err != nil {
		return nil, fmt.Errorf("loader: unmarshal scl.yaml: %w", err)
	}
	loaded = s
	return loaded, nil
}

func DecodeSCL(raw []byte) (*SCL, error) {
	var s SCL
	if err := yaml.NewDecoder(bytes.NewReader(raw), yaml.Strict()).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// MustLoadSCL は LoadSCL の panic 版（main 配線で使う）。
func MustLoadSCL() *SCL {
	s, err := LoadSCL()
	if err != nil {
		panic(err)
	}
	return s
}

// =====================================================================
// 派生ビュー
// =====================================================================

var wireAliasPattern = regexp.MustCompile(`^[a-z][a-z0-9_:.-]*$`)

// ToWire は PascalCase 名をワイヤ形式 (snake_case 等) に変換する。
// vocabulary[].aliases から WIRE_ALIAS_PATTERN に最初に一致するものを返す。
func (s *SCL) ToWire(name string) string {
	entry, ok := s.Vocabulary[name]
	if !ok {
		return name
	}
	for _, a := range entry.Aliases {
		if wireAliasPattern.MatchString(a) {
			return a
		}
	}
	return name
}

func (s *SCL) ToWireAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = s.ToWire(n)
	}
	return out
}

func (s *SCL) EnumValues(modelName string) ([]string, error) {
	m, ok := s.Models[modelName]
	if !ok {
		return nil, fmt.Errorf("model %s not found", modelName)
	}
	if m.Kind != "enum" {
		return nil, fmt.Errorf("%s is not an enum", modelName)
	}
	return m.Values, nil
}

func (s *SCL) EnumWireValues(modelName string) ([]string, error) {
	v, err := s.EnumValues(modelName)
	if err != nil {
		return nil, err
	}
	return s.ToWireAll(v), nil
}

func (s *SCL) StatesOf(machineName string) ([]string, error) {
	sm, ok := s.States[machineName]
	if !ok {
		return nil, fmt.Errorf("state machine %s not found", machineName)
	}
	seen := map[string]struct{}{sm.Initial: {}}
	out := []string{sm.Initial}
	for _, t := range sm.Terminal {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	for _, tr := range sm.Transitions {
		for _, n := range []string{tr.From, tr.To} {
			if _, ok := seen[n]; !ok {
				seen[n] = struct{}{}
				out = append(out, n)
			}
		}
	}
	return out, nil
}

func (s *SCL) EventsOf(machineName string) ([]string, error) {
	sm, ok := s.States[machineName]
	if !ok {
		return nil, fmt.Errorf("state machine %s not found", machineName)
	}
	seen := map[string]struct{}{}
	out := []string{}
	for _, tr := range sm.Transitions {
		if _, ok := seen[tr.Event]; !ok {
			seen[tr.Event] = struct{}{}
			out = append(out, tr.Event)
		}
	}
	return out, nil
}

func (s *SCL) VocabularyCodes() map[string]struct{} {
	out := map[string]struct{}{}
	for name := range s.Vocabulary {
		out[s.ToWire(name)] = struct{}{}
	}
	return out
}

func (s *SCL) HTTPBinding(iface Interface) (Binding, bool) {
	for _, b := range iface.Bindings {
		if b.Kind() == "http" {
			return b, true
		}
	}
	return nil, false
}

const (
	AuthCodeFlowName   = "AuthorizationCodeFlow"
	DeviceCodeFlowName = "DeviceCodeFlow"
)
