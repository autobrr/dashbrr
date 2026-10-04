package discovery

import (
	"context"
	"errors"
	"fmt"

	"github.com/autobrr/dashbrr/internal/database"
	"github.com/autobrr/dashbrr/internal/models"
)

// SyncPlan holds the changes that make the discovered services agree with
// the annotations.
type SyncPlan struct {
	Create []models.ServiceConfiguration
	Update []models.ServiceConfiguration
	Delete []string // instance IDs
}

// Empty reports whether the plan changes nothing.
func (p SyncPlan) Empty() bool {
	return len(p.Create) == 0 && len(p.Update) == 0 && len(p.Delete) == 0
}

// Plan compares the annotated Services with the discovered services in db.
// It never touches a service that discovery does not own. A failed list
// returns an error and no plan.
func (k *KubernetesDiscovery) Plan(ctx context.Context, db *database.DB) (SyncPlan, error) {
	found, err := k.list(ctx)
	if err != nil {
		return SyncPlan{}, err
	}

	services, err := db.GetAllServices(ctx)
	if err != nil {
		return SyncPlan{}, fmt.Errorf("failed to load services: %w", err)
	}

	existing := make(map[string]models.ServiceConfiguration)
	for _, s := range services {
		if models.IsDiscoveredInstanceID(s.InstanceID) {
			s.ID = 0
			existing[s.InstanceID] = s
		}
	}

	var plan SyncPlan
	seen := make(map[string]bool, len(found))
	for _, f := range found {
		seen[f.InstanceID] = true
		e, ok := existing[f.InstanceID]
		// Plex gets its token from the Plex sign-in in the UI, not from an
		// annotation. Keep the stored token.
		if t, _ := models.ServiceTypeFromInstanceID(f.InstanceID); t == "plex" {
			f.APIKey = e.APIKey
		}
		switch {
		case !ok:
			plan.Create = append(plan.Create, f)
		case e != f:
			plan.Update = append(plan.Update, f)
		}
	}
	for _, s := range services {
		if _, owned := existing[s.InstanceID]; owned && !seen[s.InstanceID] {
			plan.Delete = append(plan.Delete, s.InstanceID)
		}
	}
	return plan, nil
}

// Apply writes the plan to db. It applies every change it can and returns
// the errors of the others.
func (p SyncPlan) Apply(ctx context.Context, db *database.DB) error {
	var errs []error
	for _, s := range p.Create {
		if err := db.CreateService(ctx, &s); err != nil {
			errs = append(errs, fmt.Errorf("create %s: %w", s.InstanceID, err))
		}
	}
	for _, s := range p.Update {
		if err := db.UpdateService(ctx, &s); err != nil {
			errs = append(errs, fmt.Errorf("update %s: %w", s.InstanceID, err))
		}
	}
	for _, id := range p.Delete {
		if err := db.DeleteService(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("delete %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// Sync plans and applies the changes. It returns the plan, so that the caller
// knows what changed.
func (k *KubernetesDiscovery) Sync(ctx context.Context, db *database.DB) (SyncPlan, error) {
	plan, err := k.Plan(ctx, db)
	if err != nil {
		return SyncPlan{}, err
	}
	return plan, plan.Apply(ctx, db)
}
