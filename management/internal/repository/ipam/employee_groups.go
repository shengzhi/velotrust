package ipam

import (
	"fmt"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ipam"
)

func (r *Repository) ListEmployeeGroups(ctx *servicecontext.Context) ([]model.PoolGroupOption, error) {
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	var groups []model.PoolGroupOption
	err := s.SQL(`SELECT group_row.id, group_row.group_name AS name, group_row.group_code AS code,
		COUNT(DISTINCT user_row.id) FILTER (WHERE user_row.account_type='employee' AND user_row.user_status='active') AS user_count
		FROM public.t_iam_group group_row
		LEFT JOIN public.t_iam_user_group user_group ON user_group.group_id=group_row.id
		LEFT JOIN public.t_iam_user user_row ON user_row.id=user_group.user_id
		WHERE group_row.status='active'
		GROUP BY group_row.id ORDER BY group_row.sort_order, group_row.group_name, group_row.id`).Find(&groups)
	if err != nil {
		return nil, fmt.Errorf("list active employee groups for IP address pools: %w", err)
	}
	return groups, nil
}
