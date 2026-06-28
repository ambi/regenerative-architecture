package spec

// SCL ドキュメント全体のローダー。TS の src/spec-bindings/scl.ts に対応する。

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"

	"github.com/goccy/go-yaml"
)

// SCL は context-first SCL 2.0 の集約ビュー。トップレベル scl.yaml は context_map のみを持ち、
// 各 context ファイル (contexts/*.yaml) の glossary / models / interfaces などをロード時に合成する。
// 合成されるセクションは `yaml:"-"` とし、トップレベルの strict デコードでは対象にしない。
type SCL struct {
	System         string                     `yaml:"system"`
	SpecVersion    string                     `yaml:"spec_version"`
	ContextMap     map[string]ContextMapEntry `yaml:"context_map"`
	Standards      map[string]Standard        `yaml:"-"`
	Vocabulary     map[string]Vocabulary      `yaml:"-"`
	Models         map[string]Model           `yaml:"-"`
	Interfaces     map[string]Interface       `yaml:"-"`
	States         map[string]StateMachine    `yaml:"-"`
	Invariants     map[string]Invariant       `yaml:"-"`
	Scenarios      map[string]Scenario        `yaml:"-"`
	Permissions    map[string]Permission      `yaml:"-"`
	Objectives     map[string]Objective       `yaml:"-"`
	UserExperience UserExperience             `yaml:"-"`
	Annotations    map[string]any             `yaml:"annotations"`
}

// ContextMapEntry は context_map の 1 つの bounded context エントリ。
// ownership は対応する context ファイルが定義する模型・interface などで暗黙に決まる。
type ContextMapEntry struct {
	Description string                       `yaml:"description"`
	Path        string                       `yaml:"path"`
	Publishes   []string                     `yaml:"publishes"`
	DependsOn   map[string]ContextDependency `yaml:"depends_on"`
	Annotations map[string]any               `yaml:"annotations"`
}

// ContextDependency は depends_on の 1 つの依存関係 (via / uses / reason)。
type ContextDependency struct {
	Via    string   `yaml:"via"`
	Uses   []string `yaml:"uses"`
	Reason string   `yaml:"reason"`
}

// contextDocument は 1 つの context ファイル (contexts/*.yaml) のデコード対象。
// 各セクションは集約 SCL に合成される。
type contextDocument struct {
	System         string                  `yaml:"system"`
	SpecVersion    string                  `yaml:"spec_version"`
	Context        string                  `yaml:"context"`
	Standards      map[string]Standard     `yaml:"standards"`
	Glossary       map[string]Vocabulary   `yaml:"glossary"`
	Models         map[string]Model        `yaml:"models"`
	States         map[string]StateMachine `yaml:"states"`
	Interfaces     map[string]Interface    `yaml:"interfaces"`
	Invariants     map[string]Invariant    `yaml:"invariants"`
	Permissions    map[string]Permission   `yaml:"permissions"`
	Objectives     map[string]Objective    `yaml:"objectives"`
	Scenarios      map[string]Scenario     `yaml:"scenarios"`
	UserExperience UserExperience          `yaml:"user_experience"`
	Annotations    map[string]any          `yaml:"annotations"`
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
	// Inline Schema
	Fields map[string]FieldDef `yaml:"fields"`
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
	Goal              string              `yaml:"goal"`
	PrimaryActor      string              `yaml:"primary_actor"`
	Scope             string              `yaml:"scope"`
	Level             string              `yaml:"level"`
	Preconditions     []string            `yaml:"preconditions"`
	SuccessGuarantees []string            `yaml:"success_guarantees"`
	MainSuccess       []string            `yaml:"main_success"`
	Extensions        []ScenarioExtension `yaml:"extensions"`
	Steps             []string            `yaml:"steps"`
	Where             []map[string]any    `yaml:"where"`
	Tags              []string            `yaml:"tags"`
	Description       string              `yaml:"description"`
	Annotations       map[string]any      `yaml:"annotations"`
}

type ScenarioExtension struct {
	At        string   `yaml:"at"`
	Condition string   `yaml:"condition"`
	Steps     []string `yaml:"steps"`
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
		root := filepath.Join(filepath.Dir(here), "..", "..", "..")
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
	if err := s.loadContexts(filepath.Dir(path)); err != nil {
		return nil, err
	}
	loaded = s
	return loaded, nil
}

// DecodeSCL はトップレベル scl.yaml (context_map) を strict にデコードする。
// glossary / models / interfaces などの合成セクションは loadContexts が context ファイルから取り込む。
func DecodeSCL(raw []byte) (*SCL, error) {
	var s SCL
	if err := yaml.NewDecoder(bytes.NewReader(raw), yaml.Strict()).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// loadContexts は context_map の各 path を読み込み、合成セクションへマージする。
// 同一キーが複数 context で定義されれば衝突として拒否する (暗黙の単一所有を保証する)。
func (s *SCL) loadContexts(dir string) error {
	s.Standards = map[string]Standard{}
	s.Vocabulary = map[string]Vocabulary{}
	s.Models = map[string]Model{}
	s.Interfaces = map[string]Interface{}
	s.States = map[string]StateMachine{}
	s.Invariants = map[string]Invariant{}
	s.Scenarios = map[string]Scenario{}
	s.Permissions = map[string]Permission{}
	s.Objectives = map[string]Objective{}
	s.UserExperience.Accessibility = map[string]string{}
	s.UserExperience.Screens = map[string]UserExperienceScreen{}

	// 決定論的なマージ順 (衝突メッセージの安定化) のため context 名を並べる。
	names := make([]string, 0, len(s.ContextMap))
	for name := range s.ContextMap {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		entry := s.ContextMap[name]
		if entry.Path == "" {
			continue
		}
		path := filepath.Join(dir, filepath.FromSlash(entry.Path))
		raw, err := os.ReadFile(path) //nolint:gosec // path は context_map が宣言する spec ファイル。
		if err != nil {
			return fmt.Errorf("loader: read context %s: %w", name, err)
		}
		var doc contextDocument
		if err := yaml.NewDecoder(bytes.NewReader(raw), yaml.Strict()).Decode(&doc); err != nil {
			return fmt.Errorf("loader: unmarshal context %s (%s): %w", name, entry.Path, err)
		}
		if err := s.mergeContext(name, doc); err != nil {
			return err
		}
	}
	return nil
}

// mergeContext は 1 つの context ファイルの各セクションを集約 SCL へ取り込む。
func (s *SCL) mergeContext(ctxName string, doc contextDocument) error {
	if err := mergeMap(s.Standards, doc.Standards, ctxName, "standard"); err != nil {
		return err
	}
	for name, entry := range doc.Glossary {
		if entry.Context == "" {
			entry.Context = ctxName
		}
		if _, ok := s.Vocabulary[name]; ok {
			return fmt.Errorf("loader: glossary %s defined by multiple contexts (last %s)", name, ctxName)
		}
		s.Vocabulary[name] = entry
	}
	if err := mergeMap(s.Models, doc.Models, ctxName, "model"); err != nil {
		return err
	}
	if err := mergeMap(s.Interfaces, doc.Interfaces, ctxName, "interface"); err != nil {
		return err
	}
	if err := mergeMap(s.States, doc.States, ctxName, "state"); err != nil {
		return err
	}
	if err := mergeMap(s.Invariants, doc.Invariants, ctxName, "invariant"); err != nil {
		return err
	}
	if err := mergeMap(s.Scenarios, doc.Scenarios, ctxName, "scenario"); err != nil {
		return err
	}
	if err := mergeMap(s.Permissions, doc.Permissions, ctxName, "permission"); err != nil {
		return err
	}
	if err := mergeMap(s.Objectives, doc.Objectives, ctxName, "objective"); err != nil {
		return err
	}
	s.mergeUserExperience(doc.UserExperience)
	return nil
}

// mergeUserExperience は context 横断のユーザー体験を集約する。System context が
// accessibility / locales / 横断 screen を持ち、各 context が固有 screen / transition / requirement を足す。
func (s *SCL) mergeUserExperience(ux UserExperience) {
	maps.Copy(s.UserExperience.Accessibility, ux.Accessibility)
	for _, locale := range ux.Locales {
		if !slices.Contains(s.UserExperience.Locales, locale) {
			s.UserExperience.Locales = append(s.UserExperience.Locales, locale)
		}
	}
	maps.Copy(s.UserExperience.Screens, ux.Screens)
	s.UserExperience.Transitions = append(s.UserExperience.Transitions, ux.Transitions...)
	s.UserExperience.Requirements = append(s.UserExperience.Requirements, ux.Requirements...)
}

// mergeMap は src の各エントリを dst へ移し、キー衝突を拒否する。
func mergeMap[V any](dst, src map[string]V, ctxName, kind string) error {
	for name, value := range src {
		if _, ok := dst[name]; ok {
			return fmt.Errorf("loader: %s %s defined by multiple contexts (last %s)", kind, name, ctxName)
		}
		dst[name] = value
	}
	return nil
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
