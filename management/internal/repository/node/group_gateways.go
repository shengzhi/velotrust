package node

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	servicecontext "velotrust/management/internal/context"
	iammodel "velotrust/management/internal/model/iam"
	ztna "velotrust/management/internal/model/ztna"
	accesschanges "velotrust/management/internal/repository/ztna"
	"velotrust/shared/repository/database"
)

func (r *Repository) ListGroupGatewayOptions(ctx *servicecontext.Context, groupID int64) ([]ztna.GroupGatewayOption, error) {
	if ctx == nil || groupID <= 0 {
		return nil, errors.New("service context and positive security-group ID are required")
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	var group struct {
		ID int64 `xorm:"'id'"`
	}
	found, err := s.SQL("SELECT id FROM public."+iammodel.TableGroup+" WHERE id=?", groupID).Get(&group)
	if err != nil {
		return nil, fmt.Errorf("validate security group for Gateway options: %w", err)
	}
	if !found {
		return nil, errors.New("security group was not found")
	}
	items := make([]ztna.GroupGatewayOption, 0)
	err = s.SQL(`SELECT gateway.id, gateway.node_name AS name, gateway.node_code,
		gateway.node_status AS status,
		EXISTS (SELECT 1 FROM public.t_ztna_gateway_group_binding binding
			WHERE binding.group_id=? AND binding.gateway_node_id=gateway.id) AS bound
		FROM public.t_ztna_node gateway
		WHERE gateway.node_type='gateway' AND (gateway.node_status='active' OR EXISTS (
			SELECT 1 FROM public.t_ztna_gateway_group_binding binding
			WHERE binding.group_id=? AND binding.gateway_node_id=gateway.id))
		ORDER BY gateway.node_name, gateway.id`, groupID, groupID).Find(&items)
	if err != nil {
		return nil, fmt.Errorf("list active and already-bound Gateways for security group: %w", err)
	}
	return items, nil
}

func (r *Repository) SetGroupGateways(ctx *servicecontext.Context, actorID, groupID, expectedVersion int64, gatewayIDs []int64) error {
	if ctx == nil || actorID <= 0 || groupID <= 0 || expectedVersion <= 0 {
		return errors.New("authenticated operator, security group, and expected version are required")
	}
	requested := append([]int64(nil), gatewayIDs...)
	sort.Slice(requested, func(i, j int) bool { return requested[i] < requested[j] })
	for i, id := range requested {
		if id <= 0 || (i > 0 && requested[i-1] == id) {
			return errors.New("Gateway IDs must be positive and unique")
		}
	}

	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		if err := requirePermission(tx.Session, actorID, "ztna.node.write"); err != nil {
			return err
		}
		var group iammodel.Group
		found, err := tx.Session.SQL("SELECT * FROM public."+iammodel.TableGroup+" WHERE id=? FOR UPDATE", groupID).Get(&group)
		if err != nil {
			return fmt.Errorf("lock security group before Gateway binding update: %w", err)
		}
		if !found {
			return errors.New("security group was not found")
		}
		if group.VersionNo != expectedVersion {
			return errors.New("security group changed; refresh and try again")
		}

		var current []ztna.GatewayGroupBinding
		if err := tx.Session.SQL("SELECT * FROM public."+ztna.TableGatewayGroupBinding+" WHERE group_id=? ORDER BY gateway_node_id FOR UPDATE", groupID).Find(&current); err != nil {
			return fmt.Errorf("lock current security-group Gateway bindings: %w", err)
		}
		currentIDs := make([]int64, len(current))
		currentByGateway := make(map[int64]ztna.GatewayGroupBinding, len(current))
		for i := range current {
			currentIDs[i] = current[i].GatewayNodeID
			currentByGateway[current[i].GatewayNodeID] = current[i]
		}
		sort.Slice(currentIDs, func(i, j int) bool { return currentIDs[i] < currentIDs[j] })
		if equalInt64s(currentIDs, requested) {
			return nil
		}

		if group.Status != "active" {
			for _, id := range requested {
				if _, alreadyBound := currentByGateway[id]; !alreadyBound {
					return errors.New("disabled security groups cannot be bound to additional Gateways")
				}
			}
		}
		if len(requested) > 0 {
			inClause, args := int64InClause(requested)
			validGateways := make([]struct {
				ID int64 `xorm:"'id'"`
			}, 0, len(requested))
			query := `SELECT id FROM public.t_ztna_node WHERE node_type='gateway' AND id IN (` + inClause + `) AND node_status='active'`
			if len(currentIDs) > 0 {
				currentClause, currentArgs := int64InClause(currentIDs)
				query = `SELECT id FROM public.t_ztna_node WHERE node_type='gateway' AND id IN (` + inClause + `) AND (node_status='active' OR id IN (` + currentClause + `))`
				args = append(args, currentArgs...)
			}
			query += " ORDER BY id FOR KEY SHARE"
			if err := tx.Session.SQL(query, args...).Find(&validGateways); err != nil {
				return fmt.Errorf("validate active or already-bound Gateways: %w", err)
			}
			if len(validGateways) != len(requested) {
				return errors.New("one or more selected Gateways are missing, inactive, or revoked")
			}
		}

		requestedSet := make(map[int64]struct{}, len(requested))
		for _, id := range requested {
			requestedSet[id] = struct{}{}
		}
		changedGatewayIDs := make([]int64, 0)
		for _, binding := range current {
			if _, keep := requestedSet[binding.GatewayNodeID]; keep {
				continue
			}
			binding.HardDelete()
			if _, err := tx.Save(&binding); err != nil {
				return fmt.Errorf("remove security-group Gateway binding: %w", err)
			}
			changedGatewayIDs = append(changedGatewayIDs, binding.GatewayNodeID)
		}
		for _, gatewayID := range requested {
			if _, exists := currentByGateway[gatewayID]; exists {
				continue
			}
			binding := ztna.GatewayGroupBinding{GatewayNodeID: gatewayID, GroupID: groupID}
			binding.CreateNew(ctx.UserName())
			if _, err := tx.Save(&binding); err != nil {
				return fmt.Errorf("save security-group Gateway binding: %w", err)
			}
			changedGatewayIDs = append(changedGatewayIDs, gatewayID)
		}

		group.Update(ctx.UserName())
		if _, err := tx.Save(&group); err != nil {
			return fmt.Errorf("record security-group Gateway authorization update: %w", err)
		}
		sort.Slice(changedGatewayIDs, func(i, j int) bool { return changedGatewayIDs[i] < changedGatewayIDs[j] })
		for _, gatewayID := range changedGatewayIDs {
			if _, err := accesschanges.CreateAccessChange(ctx, tx, "node", gatewayID, "authorization_changed"); err != nil {
				return fmt.Errorf("queue Gateway %d configuration refresh after group binding: %w", gatewayID, err)
			}
		}

		var users []struct {
			ID int64 `xorm:"'id'"`
		}
		if err := tx.Session.SQL(`SELECT employee.id AS id
			FROM public.t_iam_user_group membership
			JOIN public.t_iam_user employee ON employee.id=membership.user_id
			WHERE membership.group_id=? AND employee.account_type='employee' AND employee.user_status='active'
			ORDER BY employee.id`, groupID).Find(&users); err != nil {
			return fmt.Errorf("find employees affected by security-group Gateway binding: %w", err)
		}
		for _, user := range users {
			if _, err := accesschanges.CreateAccessChange(ctx, tx, "user", user.ID, "authorization_changed"); err != nil {
				return fmt.Errorf("queue Client configuration refresh for employee %d: %w", user.ID, err)
			}
		}
		return nil
	})
}

func equalInt64s(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func int64InClause(values []int64) (string, []interface{}) {
	placeholders := make([]string, len(values))
	args := make([]interface{}, len(values))
	for i, value := range values {
		placeholders[i] = "?"
		args[i] = value
	}
	return strings.Join(placeholders, ","), args
}
