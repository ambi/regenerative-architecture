package spec

import (
	"fmt"
	"strings"
)

func (s *SCL) ValidateCoherence() error {
	if s.System == "" {
		return fmt.Errorf("system is required")
	}
	if s.SpecVersion == "" {
		return fmt.Errorf("spec_version is required")
	}
	if err := s.validateBoundedContexts(); err != nil {
		return err
	}
	if err := s.validateModels(); err != nil {
		return err
	}
	if err := s.validateStates(); err != nil {
		return err
	}
	if err := s.validateStandardReferences(); err != nil {
		return err
	}
	if err := s.validateUserExperienceReferences(); err != nil {
		return err
	}
	if err := s.validateAssuranceReferences(); err != nil {
		return err
	}
	return nil
}

func (s *SCL) validateModels() error {
	validKinds := map[string]bool{
		"entity": true, "value_object": true, "enum": true, "event": true, "error": true,
	}
	for name, model := range s.Models {
		if !validKinds[model.Kind] {
			return fmt.Errorf("model %s: invalid kind %s", name, model.Kind)
		}
		for fieldName, field := range model.Fields {
			if !s.validFieldType(field.Type) {
				return fmt.Errorf("model %s field %s: invalid type %s", name, fieldName, field.Type)
			}
		}
		for fieldName, field := range model.Payload {
			if !s.validFieldType(field.Type) {
				return fmt.Errorf("model %s payload %s: invalid type %s", name, fieldName, field.Type)
			}
		}
	}
	for name, iface := range s.Interfaces {
		for fieldName, field := range iface.Input {
			if !s.validFieldType(field.Type) {
				return fmt.Errorf("interface %s input %s: invalid type %s", name, fieldName, field.Type)
			}
		}
		for fieldName, field := range iface.Output {
			if !s.validFieldType(field.Type) {
				return fmt.Errorf("interface %s output %s: invalid type %s", name, fieldName, field.Type)
			}
		}
	}
	return nil
}

func (s *SCL) validFieldType(fieldType string) bool {
	if strings.HasSuffix(fieldType, "[]") {
		return s.validFieldType(strings.TrimSuffix(fieldType, "[]"))
	}
	if strings.HasPrefix(fieldType, "Set<") && strings.HasSuffix(fieldType, ">") {
		return s.validFieldType(fieldType[4 : len(fieldType)-1])
	}
	if strings.HasPrefix(fieldType, "Map<") && strings.HasSuffix(fieldType, ">") {
		parts := strings.SplitN(fieldType[4:len(fieldType)-1], ",", 2)
		return len(parts) == 2 &&
			s.validFieldType(strings.TrimSpace(parts[0])) &&
			s.validFieldType(strings.TrimSpace(parts[1]))
	}
	builtins := map[string]bool{
		"String": true, "Integer": true, "Float": true, "Boolean": true, "UUID": true,
		"Date": true, "Timestamp": true, "Duration": true, "JSON": true, "Bytes": true,
	}
	if builtins[fieldType] {
		return true
	}
	_, ok := s.Models[fieldType]
	return ok
}

func (s *SCL) validateBoundedContexts() error {
	owners := map[string]string{}
	for name, boundedContext := range s.BoundedContexts {
		if boundedContext.Description == "" {
			return fmt.Errorf("bounded context %s: description is required", name)
		}
		owned := []struct {
			section string
			names   []string
			values  map[string]struct{}
		}{
			{"models", boundedContext.OwnsModels, keysOf(s.Models)},
			{"states", boundedContext.OwnsStates, keysOf(s.States)},
			{"events", boundedContext.OwnsEvents, eventModelKeys(s.Models)},
			{"interfaces", boundedContext.OwnsInterfaces, keysOf(s.Interfaces)},
			{"invariants", boundedContext.OwnsInvariants, keysOf(s.Invariants)},
			{"permissions", boundedContext.OwnsPermissions, keysOf(s.Permissions)},
			{"objectives", boundedContext.OwnsObjectives, keysOf(s.Objectives)},
		}
		for _, group := range owned {
			for _, item := range group.names {
				if _, ok := group.values[item]; !ok {
					return fmt.Errorf("bounded context %s: owns_%s references unknown %s", name, group.section, item)
				}
				key := group.section + ":" + item
				if previous, ok := owners[key]; ok {
					return fmt.Errorf("%s %s is owned by both %s and %s", group.section, item, previous, name)
				}
				owners[key] = name
			}
		}
		for _, dependency := range boundedContext.DependsOn {
			if _, ok := s.BoundedContexts[dependency.BoundedContext]; !ok {
				return fmt.Errorf("bounded context %s: depends_on references unknown bounded context %s", name, dependency.BoundedContext)
			}
			if dependency.Reason == "" {
				return fmt.Errorf("bounded context %s: dependency on %s requires reason", name, dependency.BoundedContext)
			}
		}
	}
	return validateBoundedContextCycles(s.BoundedContexts)
}

func validateBoundedContextCycles(boundedContexts map[string]BoundedContext) error {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := map[string]int{}
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case visiting:
			return fmt.Errorf("bounded context dependency cycle includes %s", name)
		case visited:
			return nil
		}
		state[name] = visiting
		for _, dependency := range boundedContexts[name].DependsOn {
			if err := visit(dependency.BoundedContext); err != nil {
				return err
			}
		}
		state[name] = visited
		return nil
	}
	for name := range boundedContexts {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func (s *SCL) validateStates() error {
	for name, machine := range s.States {
		if _, ok := s.Models[machine.Target]; !ok {
			return fmt.Errorf("state %s: unknown target model %s", name, machine.Target)
		}
		states := map[string]struct{}{machine.Initial: {}}
		for _, terminal := range machine.Terminal {
			states[terminal] = struct{}{}
		}
		transitions := map[string]struct{}{}
		for _, transition := range machine.Transitions {
			states[transition.From] = struct{}{}
			states[transition.To] = struct{}{}
			key := transition.From + "\x00" + transition.Event
			if _, ok := transitions[key]; ok {
				return fmt.Errorf("state %s: duplicate transition from %s on %s", name, transition.From, transition.Event)
			}
			transitions[key] = struct{}{}
			if _, ok := s.Vocabulary[transition.Event]; !ok {
				return fmt.Errorf("state %s: event %s is missing from vocabulary", name, transition.Event)
			}
		}
		for state := range states {
			if state == "" {
				return fmt.Errorf("state %s: state names must not be empty", name)
			}
			if _, ok := s.Vocabulary[state]; !ok {
				return fmt.Errorf("state %s: value %s is missing from vocabulary", name, state)
			}
		}
		for _, terminal := range machine.Terminal {
			for _, transition := range machine.Transitions {
				if transition.From == terminal {
					return fmt.Errorf("state %s: terminal state %s has outgoing transition", name, terminal)
				}
			}
		}
	}
	return nil
}

func (s *SCL) validateStandardReferences() error {
	for standardName, standard := range s.Standards {
		for _, requirement := range standard.Requirements {
			if err := s.validateReferences("standard "+standardName+" requirement "+requirement.ID, requirement.RelatesTo); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SCL) validateUserExperienceReferences() error {
	validCategories := map[string]bool{
		"security": true, "accessibility": true, "privacy": true, "localization": true, "usability": true,
	}
	for name, screen := range s.UserExperience.Screens {
		for _, interfaceName := range screen.Interfaces {
			if _, ok := s.Interfaces[interfaceName]; !ok {
				return fmt.Errorf("user_experience screen %s: unknown interface %s", name, interfaceName)
			}
		}
	}
	for _, transition := range s.UserExperience.Transitions {
		if transition.From != "" {
			if _, ok := s.UserExperience.Screens[transition.From]; !ok {
				return fmt.Errorf("user_experience transition: unknown from screen %s", transition.From)
			}
		}
		if transition.To != "" {
			if _, ok := s.UserExperience.Screens[transition.To]; !ok {
				return fmt.Errorf("user_experience transition: unknown to screen %s", transition.To)
			}
		}
		if transition.Interface != "" {
			if _, ok := s.Interfaces[transition.Interface]; !ok {
				return fmt.Errorf("user_experience transition: unknown interface %s", transition.Interface)
			}
		}
	}
	for _, requirement := range s.UserExperience.Requirements {
		if !validCategories[requirement.Category] {
			return fmt.Errorf("user_experience requirement %s: invalid category %s", requirement.ID, requirement.Category)
		}
		references := map[string][]string{
			"interfaces": requirement.Interfaces,
			"standards":  requirement.Standards,
			"scenarios":  requirement.Scenarios,
			"invariants": requirement.Invariants,
		}
		if err := s.validateReferences("user_experience requirement "+requirement.ID, references); err != nil {
			return err
		}
		for _, screen := range requirement.Screens {
			if _, ok := s.UserExperience.Screens[screen]; !ok {
				return fmt.Errorf("user_experience requirement %s: unknown screen %s", requirement.ID, screen)
			}
		}
	}
	return nil
}

func (s *SCL) validateAssuranceReferences() error {
	for name, obligation := range s.Assurance {
		if err := s.validateReferences("assurance "+name+" derived_from", obligation.DerivedFrom); err != nil {
			return err
		}
		if err := validateAcceptance(name, obligation.Acceptance, obligation.Evidence); err != nil {
			return err
		}
		for evidenceName, evidence := range obligation.Evidence {
			if err := s.validateReferences("assurance "+name+" evidence "+evidenceName, evidence.Covers); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAcceptance(name string, acceptance AssuranceAcceptance, evidence map[string]AssuranceEvidence) error {
	if acceptance.Evidence != "" {
		if _, ok := evidence[acceptance.Evidence]; !ok {
			return fmt.Errorf("assurance %s: acceptance references unknown evidence %s", name, acceptance.Evidence)
		}
	}
	for _, child := range append(acceptance.All, acceptance.Any...) {
		if err := validateAcceptance(name, child, evidence); err != nil {
			return err
		}
	}
	if acceptance.Not != nil {
		return validateAcceptance(name, *acceptance.Not, evidence)
	}
	return nil
}

func (s *SCL) validateReferences(owner string, references map[string][]string) error {
	sections := map[string]map[string]struct{}{
		"standards":   keysOf(s.Standards),
		"vocabulary":  keysOf(s.Vocabulary),
		"models":      keysOf(s.Models),
		"interfaces":  keysOf(s.Interfaces),
		"states":      keysOf(s.States),
		"invariants":  keysOf(s.Invariants),
		"scenarios":   keysOf(s.Scenarios),
		"permissions": keysOf(s.Permissions),
		"objectives":  keysOf(s.Objectives),
		"assurance":   keysOf(s.Assurance),
	}
	for section, names := range references {
		values, ok := sections[section]
		if !ok {
			return fmt.Errorf("%s: unknown reference section %s", owner, section)
		}
		for _, name := range names {
			if _, ok := values[name]; !ok {
				return fmt.Errorf("%s: unknown %s reference %s", owner, section, name)
			}
		}
	}
	return nil
}

func keysOf[V any](values map[string]V) map[string]struct{} {
	keys := make(map[string]struct{}, len(values))
	for key := range values {
		keys[key] = struct{}{}
	}
	return keys
}

func eventModelKeys(models map[string]Model) map[string]struct{} {
	keys := map[string]struct{}{}
	for name, model := range models {
		if model.Kind == "event" {
			keys[name] = struct{}{}
		}
	}
	return keys
}
