// Package ztna provides PostgreSQL persistence for current ZTNA policy state.
package ztna

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
	"velotrust/shared/repository/database"
	"xorm.io/xorm"
)

// Repository persists the current ZTNA policy state.
type Repository struct {
	db *database.Database
}

// New creates a ZTNA repository backed by PostgreSQL.
func New(db *database.Database) (*Repository, error) {
	if db == nil || db.Engine() == nil {
		return nil, errors.New("ZTNA database is required")
	}
	if db.Driver() != "postgres" {
		return nil, fmt.Errorf("ZTNA repository requires PostgreSQL, got %q", db.Driver())
	}
	return &Repository{db: db}, nil
}

// SaveAccessPolicy replaces the policy's current rules and relationship sets
// in one transaction. VersionNo is only the shared optimistic-write marker.
func (r *Repository) SaveAccessPolicy(ctx *servicecontext.Context, policy *model.AccessPolicy, subjects []model.PolicySubjectRef, resourceIDs []int64) error {
	if ctx == nil || strings.TrimSpace(ctx.UserName()) == "" {
		return errors.New("authenticated service context is required")
	}
	if policy == nil {
		return errors.New("access policy is required")
	}
	working := *policy
	if err := validateAccessPolicy(&working); err != nil {
		return err
	}
	if err := validatePolicyRelations(subjects, resourceIDs); err != nil {
		return err
	}

	err := r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := validatePolicyRelationTargets(s, subjects, resourceIDs); err != nil {
			return err
		}

		if working.ID == 0 {
			if working.VersionNo != 0 {
				return errors.New("new access policy must not specify a version")
			}
			working.CreateNew(ctx.UserName())
			if _, err := tx.Save(&working); err != nil {
				return fmt.Errorf("insert current access policy: %w", err)
			}
		} else {
			if working.VersionNo <= 0 {
				return errors.New("access policy version is required for update")
			}
			var current model.AccessPolicy
			found, err := s.SQL("SELECT * FROM "+model.TableAccessPolicy+" WHERE id=? FOR UPDATE", working.ID).Get(&current)
			if err != nil {
				return fmt.Errorf("lock current access policy: %w", err)
			}
			if !found || current.VersionNo != working.VersionNo {
				return errors.New("access policy was not found or changed; refresh and try again")
			}
			current.PolicyName = working.PolicyName
			current.PolicyCode = working.PolicyCode
			current.Description = working.Description
			current.Priority = working.Priority
			current.Status = working.Status
			current.RulesJSON = append(current.RulesJSON[:0], working.RulesJSON...)
			current.CompiledHash = working.CompiledHash
			current.Update(ctx.UserName())
			current.Cols("policy_name", "policy_code", "description", "priority", "status", "rules_json", "compiled_hash")
			if _, err := tx.Save(&current); err != nil {
				return fmt.Errorf("update current access policy: %w", err)
			}
			working = current
		}

		if err := replacePolicySubjects(ctx, tx, working.ID, subjects); err != nil {
			return err
		}
		if err := replacePolicyResources(ctx, tx, working.ID, resourceIDs); err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		*policy = working
	}
	return err
}

// GetAccessPolicy returns the current policy row and its current relationship
// sets. No historical policy revision is loaded or implied.
func (r *Repository) GetAccessPolicy(ctx *servicecontext.Context, id int64) (*model.AccessPolicyData, error) {
	if ctx == nil || id <= 0 {
		return nil, errors.New("service context and positive access policy ID are required")
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	var policy model.AccessPolicy
	found, err := s.ID(id).Get(&policy)
	if err != nil {
		return nil, fmt.Errorf("load current access policy: %w", err)
	}
	if !found {
		return nil, nil
	}
	data := &model.AccessPolicyData{Policy: policy, Subjects: []model.PolicySubjectRef{}, ResourceIDs: []int64{}}
	var subjects []model.PolicySubject
	if err := s.SQL("SELECT * FROM "+model.TablePolicySubject+" WHERE policy_id=? ORDER BY subject_type, subject_id", id).Find(&subjects); err != nil {
		return nil, fmt.Errorf("load current access policy subjects: %w", err)
	}
	for _, subject := range subjects {
		data.Subjects = append(data.Subjects, model.PolicySubjectRef{Type: subject.SubjectType, ID: subject.SubjectID})
	}
	var resources []model.PolicyResource
	if err := s.SQL("SELECT * FROM "+model.TablePolicyResource+" WHERE policy_id=? ORDER BY protected_resource_id", id).Find(&resources); err != nil {
		return nil, fmt.Errorf("load current access policy resources: %w", err)
	}
	for _, resource := range resources {
		data.ResourceIDs = append(data.ResourceIDs, resource.ProtectedResourceID)
	}
	return data, nil
}

// ListAccessPolicies returns policy current rows. A policy's version_no remains
// a persistence concurrency token and does not represent policy history.
func (r *Repository) ListAccessPolicies(ctx *servicecontext.Context) ([]model.AccessPolicy, error) {
	if ctx == nil {
		return nil, errors.New("service context is required")
	}
	var policies []model.AccessPolicy
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.OrderBy("priority ASC, id ASC").Find(&policies); err != nil {
		return nil, fmt.Errorf("list current access policies: %w", err)
	}
	return policies, nil
}

func validateAccessPolicy(policy *model.AccessPolicy) error {
	policy.PolicyName = strings.TrimSpace(policy.PolicyName)
	policy.PolicyCode = strings.TrimSpace(policy.PolicyCode)
	policy.Status = strings.TrimSpace(policy.Status)
	if policy.PolicyName == "" || len(policy.PolicyName) > 128 {
		return errors.New("policy name is required and must not exceed 128 characters")
	}
	if policy.PolicyCode == "" || len(policy.PolicyCode) > 64 {
		return errors.New("policy code is required and must not exceed 64 characters")
	}
	if policy.Description != nil && len(*policy.Description) > 512 {
		return errors.New("policy description must not exceed 512 characters")
	}
	if policy.CompiledHash != nil && len(*policy.CompiledHash) > 128 {
		return errors.New("compiled policy hash must not exceed 128 characters")
	}
	if policy.Priority < 0 {
		return errors.New("policy priority must not be negative")
	}
	switch policy.Status {
	case "draft", "active", "disabled":
	default:
		return errors.New("policy status must be draft, active, or disabled")
	}
	rules := strings.TrimSpace(string(policy.RulesJSON))
	if rules == "" || !json.Valid([]byte(rules)) || (rules[0] != '[' && rules[0] != '{') {
		return errors.New("policy rules must be a valid JSON array or object")
	}
	policy.RulesJSON = []byte(rules)
	return nil
}

func validatePolicyRelations(subjects []model.PolicySubjectRef, resources []int64) error {
	seenSubjects := make(map[string]struct{}, len(subjects))
	for _, subject := range subjects {
		if subject.ID <= 0 {
			return errors.New("policy subject ID must be positive")
		}
		switch subject.Type {
		case "user", "group", "role", "device":
		default:
			return fmt.Errorf("unsupported policy subject type %q", subject.Type)
		}
		key := fmt.Sprintf("%s:%d", subject.Type, subject.ID)
		if _, exists := seenSubjects[key]; exists {
			return fmt.Errorf("duplicate policy subject %s", key)
		}
		seenSubjects[key] = struct{}{}
	}
	seenResources := make(map[int64]struct{}, len(resources))
	for _, id := range resources {
		if id <= 0 {
			return errors.New("protected resource ID must be positive")
		}
		if _, exists := seenResources[id]; exists {
			return fmt.Errorf("duplicate protected resource ID %d", id)
		}
		seenResources[id] = struct{}{}
	}
	return nil
}

func validatePolicyRelationTargets(s *xorm.Session, subjects []model.PolicySubjectRef, resourceIDs []int64) error {
	for _, subject := range subjects {
		var table string
		switch subject.Type {
		case "user":
			table = "t_iam_user"
		case "group":
			table = "t_iam_group"
		case "role":
			table = "t_iam_role"
		case "device":
			table = model.TableDevice
		}
		var result struct {
			ID int64 `xorm:"'id'"`
		}
		found, err := s.SQL("SELECT id FROM "+table+" WHERE id=? FOR KEY SHARE", subject.ID).Get(&result)
		if err != nil {
			return fmt.Errorf("validate policy %s subject %d: %w", subject.Type, subject.ID, err)
		}
		if !found {
			return fmt.Errorf("policy %s subject %d does not exist", subject.Type, subject.ID)
		}
	}
	for _, id := range resourceIDs {
		var result struct {
			ID int64 `xorm:"'id'"`
		}
		found, err := s.SQL("SELECT id FROM "+model.TableProtectedResource+" WHERE id=? FOR KEY SHARE", id).Get(&result)
		if err != nil {
			return fmt.Errorf("validate protected resource %d: %w", id, err)
		}
		if !found {
			return fmt.Errorf("protected resource %d does not exist", id)
		}
	}
	return nil
}

func replacePolicySubjects(ctx *servicecontext.Context, tx *database.DBTransaction, policyID int64, subjects []model.PolicySubjectRef) error {
	var current []model.PolicySubject
	if err := tx.Session.SQL("SELECT * FROM "+model.TablePolicySubject+" WHERE policy_id=? FOR UPDATE", policyID).Find(&current); err != nil {
		return fmt.Errorf("lock current policy subjects: %w", err)
	}
	for i := range current {
		current[i].HardDelete()
		if _, err := tx.Save(&current[i]); err != nil {
			return fmt.Errorf("remove old policy subject: %w", err)
		}
	}
	for _, subject := range subjects {
		row := model.PolicySubject{PolicyID: policyID, SubjectType: subject.Type, SubjectID: subject.ID}
		row.CreateNew(ctx.UserName())
		if _, err := tx.Save(&row); err != nil {
			return fmt.Errorf("save current policy subject: %w", err)
		}
	}
	return nil
}

func replacePolicyResources(ctx *servicecontext.Context, tx *database.DBTransaction, policyID int64, resourceIDs []int64) error {
	var current []model.PolicyResource
	if err := tx.Session.SQL("SELECT * FROM "+model.TablePolicyResource+" WHERE policy_id=? FOR UPDATE", policyID).Find(&current); err != nil {
		return fmt.Errorf("lock current policy resources: %w", err)
	}
	for i := range current {
		current[i].HardDelete()
		if _, err := tx.Save(&current[i]); err != nil {
			return fmt.Errorf("remove old policy resource: %w", err)
		}
	}
	for _, resourceID := range resourceIDs {
		row := model.PolicyResource{PolicyID: policyID, ProtectedResourceID: resourceID}
		row.CreateNew(ctx.UserName())
		if _, err := tx.Save(&row); err != nil {
			return fmt.Errorf("save current policy resource: %w", err)
		}
	}
	return nil
}
