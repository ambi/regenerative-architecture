package usecases

// 管理者向け Application メタデータ操作 (Create / Update / Delete) と protocol binding
// の接続 / 解除。SCL Application bounded context の admin interface 群 (wi-69)。

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/application/domain"
	"ra-idp-go/internal/application/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
)

var ErrApplicationNotFound = errors.New("application not found")

type ApplicationDeps struct {
	Repo           ports.ApplicationRepository
	AssignmentRepo ports.AssignmentRepository
	Emit           func(spec.DomainEvent)
}

type CreateApplicationInput struct {
	ActorSub  string
	Name      string
	Kind      spec.ApplicationKind
	IconURL   string
	LaunchURL string
	Now       time.Time
}

func CreateApplication(ctx context.Context, deps ApplicationDeps, in CreateApplicationInput) (*spec.Application, error) {
	tenantID := tenancy.TenantID(ctx)
	now := adminNow(in.Now)
	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	app := &spec.Application{
		TenantID:      tenantID,
		ApplicationID: id,
		Name:          strings.TrimSpace(in.Name),
		Kind:          in.Kind,
		Status:        spec.ApplicationActive,
		IconURL:       strings.TrimSpace(in.IconURL),
		LaunchURL:     strings.TrimSpace(in.LaunchURL),
		Bindings:      []spec.ProtocolBinding{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := domain.ValidateApplication(app); err != nil {
		return nil, err
	}
	if err := deps.Repo.Save(ctx, app); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ApplicationCreated{At: now, TenantID: tenantID, ActorSub: in.ActorSub, ApplicationID: id})
	return app, nil
}

type UpdateApplicationInput struct {
	ActorSub      string
	ApplicationID string
	Name          *string
	Status        *spec.ApplicationStatus
	IconURL       *string
	LaunchURL     *string
	Now           time.Time
}

func UpdateApplication(ctx context.Context, deps ApplicationDeps, in UpdateApplicationInput) (*spec.Application, error) {
	tenantID := tenancy.TenantID(ctx)
	app, err := deps.Repo.FindByID(ctx, tenantID, in.ApplicationID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	updated := *app
	updated.Bindings = slices.Clone(app.Bindings)
	changed := []string{}
	if in.Name != nil {
		if name := strings.TrimSpace(*in.Name); name != app.Name {
			updated.Name = name
			changed = append(changed, "name")
		}
	}
	if in.Status != nil && *in.Status != app.Status {
		updated.Status = *in.Status
		changed = append(changed, "status")
	}
	if in.IconURL != nil {
		if icon := strings.TrimSpace(*in.IconURL); icon != app.IconURL {
			updated.IconURL = icon
			changed = append(changed, "icon_url")
		}
	}
	if in.LaunchURL != nil {
		if launch := strings.TrimSpace(*in.LaunchURL); launch != app.LaunchURL {
			updated.LaunchURL = launch
			changed = append(changed, "launch_url")
		}
	}
	if len(changed) == 0 {
		return &updated, nil
	}
	if err := domain.ValidateApplication(&updated); err != nil {
		return nil, err
	}
	updated.UpdatedAt = adminNow(in.Now)
	if err := deps.Repo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ApplicationUpdated{
		At: updated.UpdatedAt, TenantID: tenantID, ActorSub: in.ActorSub, ApplicationID: app.ApplicationID, ChangedFields: changed,
	})
	return &updated, nil
}

func DeleteApplication(ctx context.Context, deps ApplicationDeps, actorSub, applicationID string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	app, err := deps.Repo.FindByID(ctx, tenantID, applicationID)
	if err != nil {
		return err
	}
	if app == nil {
		return ErrApplicationNotFound
	}
	if err := deps.AssignmentRepo.DeleteByApplication(ctx, tenantID, applicationID); err != nil {
		return err
	}
	if err := deps.Repo.Delete(ctx, tenantID, applicationID); err != nil {
		return err
	}
	emit(deps.Emit, &spec.ApplicationDeleted{At: adminNow(now), TenantID: tenantID, ActorSub: actorSub, ApplicationID: applicationID})
	return nil
}

type AttachBindingInput struct {
	ActorSub      string
	ApplicationID string
	Binding       spec.ProtocolBinding
	Now           time.Time
}

func AttachBinding(ctx context.Context, deps ApplicationDeps, in AttachBindingInput) (*spec.Application, error) {
	tenantID := tenancy.TenantID(ctx)
	app, err := deps.Repo.FindByID(ctx, tenantID, in.ApplicationID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	if err := domain.ValidateBinding(in.Binding); err != nil {
		return nil, err
	}
	updated := *app
	updated.Bindings = slices.Clone(app.Bindings)
	// 同種別 binding は置き換える (1 application に 1 種別 1 binding)。
	updated.Bindings = slices.DeleteFunc(updated.Bindings, func(b spec.ProtocolBinding) bool {
		return b.Type == in.Binding.Type
	})
	updated.Bindings = append(updated.Bindings, in.Binding)
	if err := domain.ValidateApplication(&updated); err != nil {
		return nil, err
	}
	updated.UpdatedAt = adminNow(in.Now)
	if err := deps.Repo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.ProtocolBindingAttached{
		At: updated.UpdatedAt, TenantID: tenantID, ActorSub: in.ActorSub, ApplicationID: app.ApplicationID, BindingType: string(in.Binding.Type),
	})
	return &updated, nil
}

func DetachBinding(ctx context.Context, deps ApplicationDeps, actorSub, applicationID string, bindingType spec.ProtocolBindingType, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	app, err := deps.Repo.FindByID(ctx, tenantID, applicationID)
	if err != nil {
		return err
	}
	if app == nil {
		return ErrApplicationNotFound
	}
	updated := *app
	updated.Bindings = slices.DeleteFunc(slices.Clone(app.Bindings), func(b spec.ProtocolBinding) bool {
		return b.Type == bindingType
	})
	updated.UpdatedAt = adminNow(now)
	if err := deps.Repo.Save(ctx, &updated); err != nil {
		return err
	}
	emit(deps.Emit, &spec.ProtocolBindingDetached{
		At: updated.UpdatedAt, TenantID: tenantID, ActorSub: actorSub, ApplicationID: applicationID, BindingType: string(bindingType),
	})
	return nil
}
