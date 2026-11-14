package services

// CAT-SECQ1-C (W4), sync endpoints: the address-class approval at create and URL-change time. See handlers/storage_root_class.go for the
// storage-root counterpart; both use netguard.ClassifyEndpoint and database.AutoClassifyApprovedClass.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/models"
)

// Sentinels the handler maps to 409 / 403.
var (
	// ErrPrivateConfirmationRequired: the destination is on a private network (or cannot be resolved now) and the operator has not confirmed it.
	ErrPrivateConfirmationRequired = errors.New("private network confirmation required")
	// ErrStepUpRequired: widening the approved class (public -> private) needs a verified step-up.
	ErrStepUpRequired = errors.New("step-up authentication required")
)

type syncClassPlan struct {
	networked bool
	class     netguard.Class // ClassPublic or ClassPrivate (an unresolvable destination plans as private)
}

// planSyncClass classifies the destination of endpoint. A local endpoint and a bucket URL (s3://, gs://) have no destination to classify.
func planSyncClass(endpoint *models.SyncEndpoint) (syncClassPlan, error) {
	if endpoint.Type != models.SyncTypeWebDAV && endpoint.Type != models.SyncTypeCloudStorage {
		return syncClassPlan{}, nil
	}
	if endpoint.Type == models.SyncTypeCloudStorage && bucketURL(endpoint.URL) {
		return syncClassPlan{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := netguard.ClassifyEndpoint(ctx, "", strings.TrimSpace(endpoint.URL))
	if err == nil {
		return syncClassPlan{networked: true, class: c}, nil
	}
	if errors.Is(err, netguard.ErrBlocked) {
		return syncClassPlan{}, fmt.Errorf("invalid endpoint URL: %w", err)
	}
	return syncClassPlan{networked: true, class: netguard.ClassPrivate}, nil
}

// approveCreated stores the class of a freshly created endpoint (automatic classification, then the operator's confirmation for private).
// On any failure the new row is removed again: an endpoint must not exist with an approval nobody made.
func (s *SyncService) approveCreated(endpoint *models.SyncEndpoint, plan syncClassPlan) error {
	if !plan.networked {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	fail := func(err error) error {
		_ = s.syncRepo.DeleteEndpoint(endpoint.ID)
		return err
	}
	got, aerr := s.syncRepo.AutoClassifyApprovedClass(ctx, endpoint.ID, endpoint.URL)
	if aerr != nil {
		if errors.Is(aerr, netguard.ErrBlocked) {
			return fail(fmt.Errorf("invalid endpoint URL: %w", aerr))
		}
		got = netguard.ClassPrivate // not classifiable now: the stored default (private) stands, for a confirmed endpoint only
	}
	if got == netguard.ClassPrivate {
		if !endpoint.ConfirmPrivateNetwork {
			return fail(ErrPrivateConfirmationRequired)
		}
		if err := s.syncRepo.SetApprovedClass(ctx, endpoint.ID, netguard.ClassPrivate, database.ApprovedSourceOperator); err != nil {
			return fail(err)
		}
	}
	endpoint.ApprovedClass = got.String()
	return nil
}

// checkURLChange enforces the rules of a URL change BEFORE the connection test: a private (or unresolvable) destination needs the
// operator's confirmation, and moving a public approval to private additionally needs a verified step-up.
func (s *SyncService) checkURLChange(endpointID int, plan syncClassPlan, updates *models.UpdateSyncEndpointRequest) error {
	if !plan.networked || plan.class == netguard.ClassPublic {
		return nil
	}
	if !updates.ConfirmPrivateNetwork {
		return ErrPrivateConfirmationRequired
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cur, _, err := s.syncRepo.GetApprovedClass(ctx, endpointID)
	if err != nil {
		return fmt.Errorf("read approved class: %w", err)
	}
	if cur == netguard.ClassPublic && !updates.StepUpVerified {
		return ErrStepUpRequired
	}
	return nil
}

// storeURLChangeClass records the class of the NEW destination after the update was stored. The previous operator decision was about the old URL, so it is
// replaced: public destinations are approved for public (automatic), private ones for private (the operator confirmed it).
func (s *SyncService) storeURLChangeClass(endpoint *models.SyncEndpoint, plan syncClassPlan) error {
	if !plan.networked {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	src := database.ApprovedSourceAuto
	if plan.class == netguard.ClassPrivate {
		src = database.ApprovedSourceOperator
	}
	if err := s.syncRepo.ReplaceApprovedClass(ctx, endpoint.ID, plan.class, src); err != nil {
		return err
	}
	endpoint.ApprovedClass = plan.class.String()
	return nil
}
