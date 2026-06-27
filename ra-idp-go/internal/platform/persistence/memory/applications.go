package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"ra-idp-go/internal/application/ports"
	"ra-idp-go/internal/spec"
)

// =====================================================================
// ApplicationRepository (wi-69, ADR-064)
// =====================================================================

type ApplicationRepository struct {
	mu           sync.RWMutex
	applications map[string]*spec.Application // key: tenantKey(tenant_id, application_id)
}

func NewApplicationRepository() *ApplicationRepository {
	return &ApplicationRepository{applications: map[string]*spec.Application{}}
}

func cloneApplication(app *spec.Application) *spec.Application {
	cloned := *app
	cloned.Bindings = slices.Clone(app.Bindings)
	return &cloned
}

func (r *ApplicationRepository) ListByTenant(_ context.Context, tenantID string) ([]*spec.Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Application, 0)
	for _, app := range r.applications {
		if app.TenantID == tenantID {
			out = append(out, cloneApplication(app))
		}
	}
	slices.SortFunc(out, func(a, b *spec.Application) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *ApplicationRepository) FindByID(_ context.Context, tenantID, applicationID string) (*spec.Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	app := r.applications[tenantKey(tenantID, applicationID)]
	if app == nil {
		return nil, nil
	}
	return cloneApplication(app), nil
}

func bindingKey(binding spec.ProtocolBinding) string {
	switch binding.Type {
	case spec.ProtocolBindingOIDC:
		return binding.ClientID
	case spec.ProtocolBindingWsFed:
		return binding.Wtrealm
	default:
		return ""
	}
}

func (r *ApplicationRepository) FindByBinding(_ context.Context, tenantID string, bindingType spec.ProtocolBindingType, key string) (*spec.Application, error) {
	if key == "" {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, app := range r.applications {
		if app.TenantID != tenantID {
			continue
		}
		for _, binding := range app.Bindings {
			if binding.Type == bindingType && bindingKey(binding) == key {
				return cloneApplication(app), nil
			}
		}
	}
	return nil, nil
}

func (r *ApplicationRepository) Save(_ context.Context, app *spec.Application) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.applications[tenantKey(app.TenantID, app.ApplicationID)] = cloneApplication(app)
	return nil
}

func (r *ApplicationRepository) Delete(_ context.Context, tenantID, applicationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.applications, tenantKey(tenantID, applicationID))
	return nil
}

// =====================================================================
// AssignmentRepository (wi-69)
// =====================================================================

type ApplicationAssignmentRepository struct {
	mu          sync.RWMutex
	assignments map[string]*spec.ApplicationAssignment // key: assignmentKey(...)
}

func NewApplicationAssignmentRepository() *ApplicationAssignmentRepository {
	return &ApplicationAssignmentRepository{assignments: map[string]*spec.ApplicationAssignment{}}
}

func assignmentKey(tenantID, applicationID string, subjectType spec.AssignmentSubjectType, subjectID string) string {
	return strings.Join([]string{tenantID, applicationID, string(subjectType), subjectID}, "\x00")
}

func (r *ApplicationAssignmentRepository) ListByApplication(_ context.Context, tenantID, applicationID string) ([]*spec.ApplicationAssignment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.ApplicationAssignment, 0)
	for _, assignment := range r.assignments {
		if assignment.TenantID == tenantID && assignment.ApplicationID == applicationID {
			cloned := *assignment
			out = append(out, &cloned)
		}
	}
	slices.SortFunc(out, func(a, b *spec.ApplicationAssignment) int {
		if c := strings.Compare(string(a.SubjectType), string(b.SubjectType)); c != 0 {
			return c
		}
		return strings.Compare(a.SubjectID, b.SubjectID)
	})
	return out, nil
}

func (r *ApplicationAssignmentRepository) ListBySubjects(_ context.Context, tenantID string, subjects []ports.SubjectRef) ([]*spec.ApplicationAssignment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.ApplicationAssignment, 0)
	for _, assignment := range r.assignments {
		if assignment.TenantID != tenantID {
			continue
		}
		if slices.ContainsFunc(subjects, func(s ports.SubjectRef) bool {
			return s.Type == assignment.SubjectType && s.ID == assignment.SubjectID
		}) {
			cloned := *assignment
			out = append(out, &cloned)
		}
	}
	return out, nil
}

func (r *ApplicationAssignmentRepository) Save(_ context.Context, assignment *spec.ApplicationAssignment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *assignment
	r.assignments[assignmentKey(assignment.TenantID, assignment.ApplicationID, assignment.SubjectType, assignment.SubjectID)] = &cloned
	return nil
}

func (r *ApplicationAssignmentRepository) Delete(_ context.Context, tenantID, applicationID string, subjectType spec.AssignmentSubjectType, subjectID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.assignments, assignmentKey(tenantID, applicationID, subjectType, subjectID))
	return nil
}

func (r *ApplicationAssignmentRepository) DeleteByApplication(_ context.Context, tenantID, applicationID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, assignment := range r.assignments {
		if assignment.TenantID == tenantID && assignment.ApplicationID == applicationID {
			delete(r.assignments, key)
		}
	}
	return nil
}
