package iam

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/lib/pq"
	servicecontext "velotrust/management/internal/context"
	"velotrust/management/internal/model/iam"
	"velotrust/shared/repository/database"
)

func (r *Repository) ListGroups(ctx *servicecontext.Context) ([]iam.Group, error) {
	var groups []iam.Group
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := "SELECT group_row.* FROM " + iam.TableGroup + " group_row ORDER BY group_row.sort_order, group_row.group_name, group_row.id"
	if err := s.SQL(query).Find(&groups); err != nil {
		return nil, fmt.Errorf("list IAM employee groups: %w", err)
	}
	if len(groups) == 0 {
		return groups, nil
	}
	type memberCount struct {
		GroupID int64 `xorm:"'group_id'"`
		Count   int64 `xorm:"'member_count'"`
	}
	var counts []memberCount
	if err := s.SQL("SELECT user_group.group_id, COUNT(employee.id) AS member_count FROM " + iam.TableUserGroup + " user_group JOIN " + iam.TableUser + " employee ON employee.id=user_group.user_id AND employee.account_type='employee' GROUP BY user_group.group_id").Find(&counts); err != nil {
		return nil, fmt.Errorf("count IAM employee group members: %w", err)
	}
	byGroup := make(map[int64]int64, len(counts))
	for _, count := range counts {
		byGroup[count.GroupID] = count.Count
	}
	for index := range groups {
		groups[index].MemberCount = byGroup[groups[index].ID]
	}
	return groups, nil
}

func (r *Repository) SaveGroup(ctx *servicecontext.Context, group *iam.Group, expectedVersion int64, audit *iam.AuditLog) error {
	if group == nil || audit == nil {
		return errors.New("IAM group and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		if group.ID == 0 {
			group.CreateNew(ctx.UserName())
			if _, err := tx.Save(group); err != nil {
				return groupWriteError(err)
			}
		} else {
			var current iam.Group
			found, err := tx.SQL("SELECT * FROM "+iam.TableGroup+" WHERE id=? FOR UPDATE", group.ID).Get(&current)
			if err != nil {
				return fmt.Errorf("lock IAM group for update: %w", err)
			}
			if !found {
				return ErrGroupNotFound
			}
			if expectedVersion <= 0 || current.VersionNo != expectedVersion {
				return ErrGroupVersionConflict
			}
			current.GroupName, current.GroupCode = group.GroupName, group.GroupCode
			current.Description, current.Status, current.SortOrder = group.Description, group.Status, group.SortOrder
			current.Update(ctx.UserName())
			current.Cols("group_name", "group_code", "description", "status", "sort_order")
			if _, err := tx.Save(&current); err != nil {
				if errors.Is(err, database.ErrVersionConflict) {
					return ErrGroupVersionConflict
				}
				return groupWriteError(err)
			}
			*group = current
		}
		audit.TargetType = "group"
		audit.TargetID = stringPtr(fmt.Sprintf("%d", group.ID))
		if err := insertAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write IAM group audit: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListGroupMembers(ctx *servicecontext.Context, groupID int64) ([]iam.GroupMember, error) {
	var exists struct {
		Exists bool `xorm:"'exists'"`
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if _, err := s.SQL("SELECT EXISTS (SELECT 1 FROM "+iam.TableGroup+" WHERE id=?) AS exists", groupID).Get(&exists); err != nil {
		return nil, fmt.Errorf("check IAM group before listing employees: %w", err)
	}
	if !exists.Exists {
		return nil, ErrGroupNotFound
	}
	var members []iam.GroupMember
	query := "SELECT employee.id, employee.username, employee.display_name, employee.email, employee.user_status, EXISTS (SELECT 1 FROM " + iam.TableUserGroup + " user_group WHERE user_group.group_id=? AND user_group.user_id=employee.id) AS is_member FROM " + iam.TableUser + " employee WHERE employee.account_type='employee' ORDER BY employee.username, employee.id"
	if err := s.SQL(query, groupID).Find(&members); err != nil {
		return nil, fmt.Errorf("list employees for IAM group: %w", err)
	}
	return members, nil
}

func (r *Repository) SetGroupMembers(ctx *servicecontext.Context, groupID int64, userIDs []int64, audit *iam.AuditLog) error {
	if groupID <= 0 || audit == nil {
		return errors.New("IAM group and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var group iam.Group
		found, err := tx.SQL("SELECT * FROM "+iam.TableGroup+" WHERE id=? FOR UPDATE", groupID).Get(&group)
		if err != nil {
			return fmt.Errorf("lock IAM group for membership update: %w", err)
		}
		if !found {
			return ErrGroupNotFound
		}
		if len(userIDs) > 0 {
			var employees []iam.User
			if err := tx.Table(iam.TableUser).In("id", userIDs).Where("account_type=?", iam.AccountTypeEmployee).Cols("id").Find(&employees); err != nil {
				return fmt.Errorf("validate employee users for IAM group: %w", err)
			}
			if len(employees) != len(userIDs) {
				return ErrGroupMembersInvalid
			}
		}

		var current []iam.UserGroup
		if err := selectManyForUpdate(tx.Session, iam.TableUserGroup, &current, "group_id=?", groupID); err != nil {
			return fmt.Errorf("lock current IAM group members: %w", err)
		}
		requested := make(map[int64]struct{}, len(userIDs))
		for _, id := range userIDs {
			requested[id] = struct{}{}
		}
		currentIDs := make([]int64, 0, len(current))
		for i := range current {
			currentIDs = append(currentIDs, current[i].UserID)
			if _, keep := requested[current[i].UserID]; keep {
				delete(requested, current[i].UserID)
				continue
			}
			current[i].HardDelete()
			if _, err := tx.Save(&current[i]); err != nil {
				return fmt.Errorf("remove employee from IAM group: %w", err)
			}
		}
		sort.Slice(currentIDs, func(i, j int) bool { return currentIDs[i] < currentIDs[j] })
		for id := range requested {
			membership := &iam.UserGroup{UserID: id, GroupID: groupID}
			membership.CreateNew(ctx.UserName())
			if _, err := tx.Save(membership); err != nil {
				return fmt.Errorf("add employee to IAM group: %w", err)
			}
		}
		if len(requested) == 0 && len(currentIDs) == len(userIDs) {
			same := true
			for i := range userIDs {
				if i > 0 && userIDs[i-1] >= userIDs[i] {
					same = false
					break
				}
				if currentIDs[i] != userIDs[i] {
					same = false
					break
				}
			}
			if same {
				return nil
			}
		}
		group.Update(ctx.UserName())
		group.Cols()
		if _, err := tx.Save(&group); err != nil {
			return fmt.Errorf("update IAM group member version: %w", err)
		}
		audit.TargetType = "group_members"
		audit.TargetID = stringPtr(fmt.Sprintf("%d", groupID))
		audit.ChangesJSON, err = json.Marshal(map[string][]int64{"before_user_ids": currentIDs, "after_user_ids": sortedIDs(userIDs)})
		if err != nil {
			return fmt.Errorf("marshal IAM group membership audit: %w", err)
		}
		if err := insertAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write IAM group membership audit: %w", err)
		}
		return nil
	})
}

func groupWriteError(err error) error {
	var postgresError *pq.Error
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.Constraint == "uq_t_iam_group_code" {
		return ErrGroupCodeExists
	}
	return fmt.Errorf("save IAM group: %w", err)
}

func sortedIDs(values []int64) []int64 {
	result := append([]int64(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
