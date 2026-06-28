package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	appports "ra-idp-go/internal/application/ports"
	"ra-idp-go/internal/spec"
)

// ApplicationRepository は ApplicationCatalog の Application aggregate を PostgreSQL に
// 永続化する (wi-69)。protocol binding は JSONB に格納し、参照はテナント境界に閉じる。
type ApplicationRepository struct{ Pool *pgxpool.Pool }

const applicationSelect = `SELECT tenant_id,application_id,name,kind,status,icon_url,launch_url,bindings,created_at,updated_at FROM applications`

func scanApplication(row rowScanner) (*spec.Application, error) {
	var (
		app      spec.Application
		bindings []byte
	)
	err := row.Scan(&app.TenantID, &app.ApplicationID, &app.Name, &app.Kind, &app.Status,
		&app.IconURL, &app.LaunchURL, &bindings, &app.CreatedAt, &app.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	app.Bindings = []spec.ProtocolBinding{}
	if len(bindings) > 0 {
		if err := json.Unmarshal(bindings, &app.Bindings); err != nil {
			return nil, err
		}
	}
	return &app, nil
}

func (r *ApplicationRepository) ListByTenant(ctx context.Context, tenantID string) ([]*spec.Application, error) {
	rows, err := r.Pool.Query(ctx, applicationSelect+" WHERE tenant_id=$1 ORDER BY name", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.Application{}
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, app)
	}
	return out, rows.Err()
}

func (r *ApplicationRepository) FindByID(ctx context.Context, tenantID, applicationID string) (*spec.Application, error) {
	return scanApplication(r.Pool.QueryRow(ctx,
		applicationSelect+" WHERE tenant_id=$1 AND application_id=$2", tenantID, applicationID))
}

func (r *ApplicationRepository) FindByBinding(ctx context.Context, tenantID string, bindingType spec.ProtocolBindingType, key string) (*spec.Application, error) {
	if key == "" {
		return nil, nil
	}
	apps, err := r.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, app := range apps {
		for _, binding := range app.Bindings {
			if binding.Type != bindingType {
				continue
			}
			switch bindingType {
			case spec.ProtocolBindingWsFed:
				if binding.Wtrealm == key {
					return app, nil
				}
			case spec.ProtocolBindingSAML:
				if binding.EntityID == key {
					return app, nil
				}
			default:
				if binding.ClientID == key {
					return app, nil
				}
			}
		}
	}
	return nil, nil
}

func (r *ApplicationRepository) Save(ctx context.Context, app *spec.Application) error {
	bindings := app.Bindings
	if bindings == nil {
		bindings = []spec.ProtocolBinding{}
	}
	encoded, err := json.Marshal(bindings)
	if err != nil {
		return err
	}
	_, err = r.Pool.Exec(ctx, `
INSERT INTO applications (tenant_id,application_id,name,kind,status,icon_url,launch_url,bindings,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (tenant_id,application_id) DO UPDATE SET name=EXCLUDED.name,kind=EXCLUDED.kind,
 status=EXCLUDED.status,icon_url=EXCLUDED.icon_url,launch_url=EXCLUDED.launch_url,
 bindings=EXCLUDED.bindings,updated_at=EXCLUDED.updated_at`,
		app.TenantID, app.ApplicationID, app.Name, app.Kind, app.Status, app.IconURL, app.LaunchURL,
		encoded, app.CreatedAt, app.UpdatedAt)
	return err
}

func (r *ApplicationRepository) Delete(ctx context.Context, tenantID, applicationID string) error {
	_, err := r.Pool.Exec(ctx,
		"DELETE FROM applications WHERE tenant_id=$1 AND application_id=$2", tenantID, applicationID)
	return err
}

// ApplicationAssignmentRepository は Application 割当を PostgreSQL に永続化する (wi-69)。
type ApplicationAssignmentRepository struct{ Pool *pgxpool.Pool }

const assignmentSelect = `SELECT tenant_id,application_id,subject_type,subject_id,visibility,created_at FROM application_assignments`

func scanAssignment(row rowScanner) (*spec.ApplicationAssignment, error) {
	var a spec.ApplicationAssignment
	err := row.Scan(&a.TenantID, &a.ApplicationID, &a.SubjectType, &a.SubjectID, &a.Visibility, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func collectAssignments(rows pgx.Rows) ([]*spec.ApplicationAssignment, error) {
	defer rows.Close()
	out := []*spec.ApplicationAssignment{}
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *ApplicationAssignmentRepository) ListByApplication(ctx context.Context, tenantID, applicationID string) ([]*spec.ApplicationAssignment, error) {
	rows, err := r.Pool.Query(ctx,
		assignmentSelect+" WHERE tenant_id=$1 AND application_id=$2 ORDER BY subject_type,subject_id",
		tenantID, applicationID)
	if err != nil {
		return nil, err
	}
	return collectAssignments(rows)
}

func (r *ApplicationAssignmentRepository) ListBySubjects(ctx context.Context, tenantID string, subjects []appports.SubjectRef) ([]*spec.ApplicationAssignment, error) {
	if len(subjects) == 0 {
		return []*spec.ApplicationAssignment{}, nil
	}
	types := make([]string, len(subjects))
	ids := make([]string, len(subjects))
	for i, s := range subjects {
		types[i] = string(s.Type)
		ids[i] = s.ID
	}
	// (subject_type, subject_id) のペアを UNNEST で突き合わせる。
	rows, err := r.Pool.Query(ctx, assignmentSelect+`
 WHERE tenant_id=$1 AND (subject_type,subject_id) IN (
   SELECT * FROM UNNEST($2::text[], $3::text[])
 )`, tenantID, types, ids)
	if err != nil {
		return nil, err
	}
	return collectAssignments(rows)
}

func (r *ApplicationAssignmentRepository) Save(ctx context.Context, a *spec.ApplicationAssignment) error {
	_, err := r.Pool.Exec(ctx, `
INSERT INTO application_assignments (tenant_id,application_id,subject_type,subject_id,visibility,created_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (tenant_id,application_id,subject_type,subject_id) DO UPDATE SET
 visibility=EXCLUDED.visibility,created_at=EXCLUDED.created_at`,
		a.TenantID, a.ApplicationID, a.SubjectType, a.SubjectID, a.Visibility, a.CreatedAt)
	return err
}

func (r *ApplicationAssignmentRepository) Delete(ctx context.Context, tenantID, applicationID string, subjectType spec.AssignmentSubjectType, subjectID string) error {
	_, err := r.Pool.Exec(ctx, `
DELETE FROM application_assignments
 WHERE tenant_id=$1 AND application_id=$2 AND subject_type=$3 AND subject_id=$4`,
		tenantID, applicationID, subjectType, subjectID)
	return err
}

func (r *ApplicationAssignmentRepository) DeleteByApplication(ctx context.Context, tenantID, applicationID string) error {
	_, err := r.Pool.Exec(ctx,
		"DELETE FROM application_assignments WHERE tenant_id=$1 AND application_id=$2", tenantID, applicationID)
	return err
}

// ApplicationOrderingRepository は利用者ごとのポータル手動並び順を PostgreSQL に永続化する
// (wi-70, ADR-069)。application_ids は順序を保つ text[] で格納し、tenant 境界に閉じる。
type ApplicationOrderingRepository struct{ Pool *pgxpool.Pool }

func (r *ApplicationOrderingRepository) Get(ctx context.Context, tenantID, userSub string) (*spec.ApplicationOrdering, error) {
	var o spec.ApplicationOrdering
	err := r.Pool.QueryRow(ctx,
		`SELECT tenant_id,user_sub,application_ids,updated_at FROM application_orderings
 WHERE tenant_id=$1 AND user_sub=$2`, tenantID, userSub).
		Scan(&o.TenantID, &o.UserSub, &o.ApplicationIDs, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *ApplicationOrderingRepository) Save(ctx context.Context, o *spec.ApplicationOrdering) error {
	ids := o.ApplicationIDs
	if ids == nil {
		ids = []string{}
	}
	_, err := r.Pool.Exec(ctx, `
INSERT INTO application_orderings (tenant_id,user_sub,application_ids,updated_at)
VALUES ($1,$2,$3,$4)
ON CONFLICT (tenant_id,user_sub) DO UPDATE SET
 application_ids=EXCLUDED.application_ids,updated_at=EXCLUDED.updated_at`,
		o.TenantID, o.UserSub, ids, o.UpdatedAt)
	return err
}
