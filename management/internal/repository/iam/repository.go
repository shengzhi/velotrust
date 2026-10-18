package iam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"velotrust/management/internal/model/iam"
	"velotrust/shared/repository/database"
	"xorm.io/xorm"
)

const platformAdminRole = "platform_admin"

var bootstrapPermissions = []struct {
	code, name, resource, action string
}{
	{"iam.user.read", "Read users", "iam.user", "read"},
	{"iam.user.write", "Manage users", "iam.user", "write"},
	{"iam.role.read", "Read roles", "iam.role", "read"},
	{"iam.role.write", "Manage roles", "iam.role", "write"},
	{"iam.role.grant", "Grant roles", "iam.role", "grant"},
	{"iam.audit_log.read", "Read audit log", "iam.audit_log", "read"},
}

// Repository is the XORM adapter for the VeloTrust IAM schema.
type Repository struct{ db *database.Database }

func New(db *database.Database) (*Repository, error) {
	if db == nil || db.Engine() == nil {
		return nil, errors.New("IAM database is required")
	}
	if db.Driver() != "postgres" {
		return nil, fmt.Errorf("IAM repository requires PostgreSQL, got %q", db.Driver())
	}
	return &Repository{db: db}, nil
}

func (r *Repository) FindUserByUsername(ctx context.Context, username string) (*iam.User, error) {
	user := new(iam.User)
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.Where("LOWER(username)=LOWER(?)", username).Get(user)
	if err != nil {
		return nil, fmt.Errorf("find IAM user by username: %w", err)
	}
	if !found {
		return nil, nil
	}
	return user, nil
}

func (r *Repository) FindUserByID(ctx context.Context, id int64) (*iam.User, error) {
	user := new(iam.User)
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.ID(id).Get(user)
	if err != nil {
		return nil, fmt.Errorf("find IAM user by ID: %w", err)
	}
	if !found {
		return nil, nil
	}
	return user, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *iam.User, audit *iam.AuditLog) error {
	if user == nil {
		return errors.New("IAM user is required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		if err := insertUser(s, user); err != nil {
			return fmt.Errorf("insert IAM user: %w", err)
		}
		if audit != nil {
			audit.UserID = int64Ptr(user.ID)
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM user audit: %w", err)
			}
		}
		return nil
	})
}

func insertUser(s *xorm.Session, user *iam.User) error {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = user.CreatedAt
	}
	if user.VersionNo == 0 {
		user.VersionNo = 1
	}
	_, err := s.InsertOne(user)
	return err
}

func (r *Repository) CreateLoginSession(ctx context.Context, session *iam.UserSession, log *iam.LoginLog, clientIP, usernameHash, ipHash string) error {
	if session == nil || log == nil {
		return errors.New("session and login event are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		now := time.Now().UTC()
		session.CreatedAt = now
		if session.LastSeenAt == nil {
			session.LastSeenAt = &now
		}
		if _, err := s.InsertOne(session); err != nil {
			return fmt.Errorf("insert IAM session: %w", err)
		}
		result, err := s.Exec("UPDATE "+iam.TableUser+" SET last_login_at=?, last_login_ip=?, updated_at=?, version_no=version_no+1 WHERE id=? AND user_status='active'", now, nullable(clientIP), now, session.UserID)
		if err != nil {
			return fmt.Errorf("update IAM last login: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check IAM last login update: %w", err)
		}
		if affected != 1 {
			return errors.New("IAM user is no longer active")
		}
		if usernameHash != "" || ipHash != "" {
			if usernameHash == "" || ipHash == "" {
				return errors.New("both IAM login throttle keys are required")
			}
			if _, err := s.Exec("DELETE FROM "+iam.TableAuthThrottle+" WHERE bucket_key_hash IN (?, ?)", usernameHash, ipHash); err != nil {
				return fmt.Errorf("clear successful IAM login throttle: %w", err)
			}
		}
		log.UserID = int64Ptr(session.UserID)
		log.SessionID = stringPtr(session.SessionID)
		log.OccurredAt = now
		if _, err := s.InsertOne(log); err != nil {
			return fmt.Errorf("insert IAM login event: %w", err)
		}
		return nil
	})
}

func (r *Repository) LoginBlocked(ctx context.Context, usernameHash, ipHash string) (bool, error) {
	var result struct {
		Blocked bool `xorm:"'blocked'"`
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	_, err := s.SQL(
		"SELECT (EXISTS (SELECT 1 FROM "+iam.TableAuthThrottle+" WHERE dimension='username' AND bucket_key_hash=? AND blocked_until > CURRENT_TIMESTAMP) OR EXISTS (SELECT 1 FROM "+iam.TableAuthThrottle+" WHERE dimension='source_ip' AND bucket_key_hash=? AND blocked_until > CURRENT_TIMESTAMP)) AS blocked",
		usernameHash, ipHash).Get(&result)
	if err != nil {
		return false, fmt.Errorf("check IAM login throttle: %w", err)
	}
	return result.Blocked, nil
}

func (r *Repository) RecordLoginFailure(ctx context.Context, log *iam.LoginLog, usernameHash, ipHash string, maxFailures, windowMinutes, blockMinutes int) error {
	if log == nil {
		return errors.New("login event is required")
	}
	if maxFailures < 1 || windowMinutes < 1 || blockMinutes < 1 {
		return errors.New("invalid IAM throttle settings")
	}
	now := time.Now().UTC()
	window := time.Duration(windowMinutes) * time.Minute
	windowStart := now.Truncate(window)
	blockedUntil := now.Add(time.Duration(blockMinutes) * time.Minute)
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		for _, item := range []struct{ dimension, hash string }{{"username", usernameHash}, {"source_ip", ipHash}} {
			query := "INSERT INTO " + iam.TableAuthThrottle + " (bucket_key_hash, dimension, window_started_at, failure_count, blocked_until, updated_at) VALUES (?, ?, ?, 1, CASE WHEN 1 >= ? THEN ? ELSE NULL END, ?) " +
				"ON CONFLICT (dimension, bucket_key_hash, window_started_at) DO UPDATE SET failure_count=" + iam.TableAuthThrottle + ".failure_count + 1, blocked_until=CASE WHEN " + iam.TableAuthThrottle + ".failure_count + 1 >= ? THEN ? ELSE " + iam.TableAuthThrottle + ".blocked_until END, updated_at=?"
			if _, err := s.Exec(query, item.hash, item.dimension, windowStart, maxFailures, blockedUntil, now, maxFailures, blockedUntil, now); err != nil {
				return fmt.Errorf("update IAM login throttle: %w", err)
			}
		}
		log.OccurredAt = now
		if _, err := s.InsertOne(log); err != nil {
			return fmt.Errorf("insert failed IAM login event: %w", err)
		}
		return nil
	})
}

func (r *Repository) ClearLoginThrottle(ctx context.Context, usernameHash, ipHash string) error {
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	_, err := s.Exec("DELETE FROM "+iam.TableAuthThrottle+" WHERE bucket_key_hash IN (?, ?)", usernameHash, ipHash)
	if err != nil {
		return fmt.Errorf("clear IAM login throttle: %w", err)
	}
	return nil
}

func (r *Repository) ResolveSession(ctx context.Context, tokenHash string) (*iam.UserSession, *iam.User, error) {
	session := new(iam.UserSession)
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.Where("token_hash=? AND status='active'", tokenHash).Get(session)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve IAM session: %w", err)
	}
	if !found {
		return nil, nil, nil
	}
	user, err := r.FindUserByID(ctx, session.UserID)
	if err != nil {
		return nil, nil, err
	}
	return session, user, nil
}

func (r *Repository) TouchSession(ctx context.Context, sessionID, tokenHash, clientIP string) error {
	now := time.Now().UTC()
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	result, err := s.Exec("UPDATE "+iam.TableUserSession+" SET last_seen_at=? WHERE session_id=? AND token_hash=? AND status='active' AND expires_at>?", now, sessionID, tokenHash, now)
	if err != nil {
		return fmt.Errorf("touch IAM session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return fmt.Errorf("IAM session is no longer active")
	}
	return nil
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID, reason string) error {
	now := time.Now().UTC()
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	_, err := s.Exec("UPDATE "+iam.TableUserSession+" SET status='revoked', revoked_at=?, revocation_reason=? WHERE session_id=? AND status='active'", now, reason, sessionID)
	if err != nil {
		return fmt.Errorf("revoke IAM session: %w", err)
	}
	return nil
}

func (r *Repository) RevokeUserSessions(ctx context.Context, userID int64, reason string) error {
	now := time.Now().UTC()
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	_, err := s.Exec("UPDATE "+iam.TableUserSession+" SET status='revoked', revoked_at=?, revocation_reason=? WHERE user_id=? AND status='active'", now, reason, userID)
	if err != nil {
		return fmt.Errorf("revoke IAM user sessions: %w", err)
	}
	return nil
}

func (r *Repository) HasPermission(ctx context.Context, userID int64, code string) (bool, error) {
	var result struct {
		Allowed bool `xorm:"'allowed'"`
	}
	query := "SELECT EXISTS (SELECT 1 FROM " + iam.TableUserRole + " ur JOIN " + iam.TableUser + " u ON u.id=ur.user_id JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableRolePermission + " rp ON rp.role_id=r.id JOIN " + iam.TablePermission + " p ON p.id=rp.permission_id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE ur.user_id=? AND u.user_status='active' AND r.status='active' AND p.permission_code=? AND p.status='active' AND res.status='active') AS allowed"
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	_, err := s.SQL(query, userID, code).Get(&result)
	if err != nil {
		return false, fmt.Errorf("check IAM permission: %w", err)
	}
	return result.Allowed, nil
}

func (r *Repository) ListPermissions(ctx context.Context, userID int64) ([]string, error) {
	var rows []struct {
		Code string `xorm:"'permission_code'"`
	}
	query := "SELECT DISTINCT p.permission_code FROM " + iam.TableUserRole + " ur JOIN " + iam.TableUser + " u ON u.id=ur.user_id JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableRolePermission + " rp ON rp.role_id=r.id JOIN " + iam.TablePermission + " p ON p.id=rp.permission_id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE ur.user_id=? AND u.user_status='active' AND r.status='active' AND p.status='active' AND res.status='active' ORDER BY p.permission_code"
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.SQL(query, userID).Find(&rows); err != nil {
		return nil, fmt.Errorf("list IAM permissions: %w", err)
	}
	result := make([]string, len(rows))
	for i := range rows {
		result[i] = rows[i].Code
	}
	return result, nil
}

func (r *Repository) ListUsers(ctx context.Context, page, pageSize int) ([]iam.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	total, err := s.Table(iam.TableUser).Count(new(iam.User))
	if err != nil {
		return nil, 0, fmt.Errorf("count IAM users: %w", err)
	}
	var users []iam.User
	err = s.Table(iam.TableUser).OrderBy("id DESC").Limit(pageSize, (page-1)*pageSize).Find(&users)
	if err != nil {
		return nil, 0, fmt.Errorf("list IAM users: %w", err)
	}
	return users, total, nil
}

func (r *Repository) CreateRole(ctx context.Context, role *iam.Role, audit *iam.AuditLog) error {
	if role == nil {
		return errors.New("IAM role is required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		now := time.Now().UTC()
		role.CreatedAt, role.UpdatedAt = now, now
		role.VersionNo = 1
		if _, err := s.InsertOne(role); err != nil {
			return fmt.Errorf("insert IAM role: %w", err)
		}
		if audit != nil {
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM role audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ListRoles(ctx context.Context) ([]iam.Role, error) {
	var roles []iam.Role
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.Where("status='active'").OrderBy("role_name ASC").Find(&roles); err != nil {
		return nil, fmt.Errorf("list IAM roles: %w", err)
	}
	return roles, nil
}

func (r *Repository) ListUserRoles(ctx context.Context, userID int64) ([]iam.Role, error) {
	var roles []iam.Role
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := "SELECT r.* FROM " + iam.TableRole + " r JOIN " + iam.TableUserRole + " ur ON ur.role_id=r.id WHERE ur.user_id=? AND r.status='active' ORDER BY r.role_name ASC"
	if err := s.SQL(query, userID).Find(&roles); err != nil {
		return nil, fmt.Errorf("list IAM roles for user: %w", err)
	}
	return roles, nil
}

func (r *Repository) ListRolePermissions(ctx context.Context, roleID int64) ([]iam.Permission, error) {
	var permissions []iam.Permission
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := "SELECT p.* FROM " + iam.TablePermission + " p JOIN " + iam.TableRolePermission + " rp ON rp.permission_id=p.id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE rp.role_id=? AND p.status='active' AND res.status='active' ORDER BY p.permission_code ASC"
	if err := s.SQL(query, roleID).Find(&permissions); err != nil {
		return nil, fmt.Errorf("list IAM permissions for role: %w", err)
	}
	return permissions, nil
}

func (r *Repository) ListRoleMenus(ctx context.Context, roleID int64) ([]iam.Menu, error) {
	if roleID <= 0 {
		return nil, errors.New("positive IAM role ID is required")
	}
	var role iam.Role
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.ID(roleID).Get(&role)
	if err != nil {
		return nil, fmt.Errorf("find IAM role for menu listing: %w", err)
	}
	if !found {
		return nil, ErrRoleNotFound
	}
	var menus []iam.Menu
	query := "SELECT m.* FROM " + iam.TableMenu + " m JOIN " + iam.TableRoleMenu + " rm ON rm.menu_id=m.id WHERE rm.role_id=? ORDER BY m.parent_id NULLS FIRST, m.sort_order ASC, m.id ASC"
	if err := s.SQL(query, roleID).Find(&menus); err != nil {
		return nil, fmt.Errorf("list menus for IAM role: %w", err)
	}
	return menus, nil
}

func (r *Repository) ListUserMenus(ctx context.Context, userID int64) ([]iam.Menu, error) {
	if userID <= 0 {
		return nil, errors.New("positive IAM user ID is required")
	}
	var menus []iam.Menu
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := "SELECT DISTINCT m.* FROM " + iam.TableMenu + " m WHERE m.status='active' AND (" +
		"EXISTS (SELECT 1 FROM " + iam.TableRoleMenu + " rm JOIN " + iam.TableUserRole + " ur ON ur.role_id=rm.role_id JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableUser + " u ON u.id=ur.user_id WHERE rm.menu_id=m.id AND ur.user_id=? AND r.status='active' AND u.user_status='active') OR " +
		"EXISTS (SELECT 1 FROM " + iam.TableMenu + " child JOIN " + iam.TableRoleMenu + " rm ON rm.menu_id=child.id JOIN " + iam.TableUserRole + " ur ON ur.role_id=rm.role_id JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableUser + " u ON u.id=ur.user_id WHERE child.parent_id=m.id AND child.status='active' AND ur.user_id=? AND r.status='active' AND u.user_status='active')) " +
		"ORDER BY m.parent_id NULLS FIRST, m.sort_order ASC, m.id ASC"
	if err := s.SQL(query, userID, userID).Find(&menus); err != nil {
		return nil, fmt.Errorf("list menus for IAM user: %w", err)
	}
	return menus, nil
}

func (r *Repository) SetRoleMenus(ctx context.Context, actorID, roleID int64, menuIDs []int64, audit *iam.AuditLog) error {
	if actorID <= 0 || roleID <= 0 {
		return errors.New("active actor and IAM role are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		var role iam.Role
		found, err := s.SQL("SELECT * FROM "+iam.TableRole+" WHERE id=? FOR UPDATE", roleID).Get(&role)
		if err != nil {
			return fmt.Errorf("find IAM role for menu update: %w", err)
		}
		if !found || role.Status != "active" {
			return ErrRoleNotFound
		}
		if role.IsSystem {
			return ErrRoleMenuSystemRole
		}
		var allowed struct {
			Count int64 `xorm:"'count'"`
		}
		permissionQuery := "SELECT COUNT(*) AS count FROM " + iam.TableUser + " u WHERE u.id=? AND u.user_status='active' AND EXISTS (SELECT 1 FROM " + iam.TableUserRole + " ur JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableRolePermission + " rp ON rp.role_id=r.id JOIN " + iam.TablePermission + " p ON p.id=rp.permission_id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE ur.user_id=u.id AND r.status='active' AND p.status='active' AND res.status='active' AND p.permission_code='iam.role.write')"
		if _, err := s.SQL(permissionQuery, actorID).Get(&allowed); err != nil {
			return fmt.Errorf("check IAM role menu write permission: %w", err)
		}
		if allowed.Count != 1 {
			return errors.New("active IAM role write permission is required")
		}

		if len(menuIDs) > 0 {
			var assignedMenus []iam.Menu
			if err := s.Table(iam.TableMenu).In("id", menuIDs).Find(&assignedMenus); err != nil {
				return fmt.Errorf("validate IAM role menu assignment: %w", err)
			}
			if len(assignedMenus) != len(menuIDs) {
				return ErrRoleMenuInvalid
			}
		}

		var current []struct {
			MenuID int64 `xorm:"'menu_id'"`
		}
		if err := s.Table(iam.TableRoleMenu).Where("role_id=?", roleID).Cols("menu_id").Find(&current); err != nil {
			return fmt.Errorf("load current IAM role menus: %w", err)
		}
		currentIDs := make([]int64, len(current))
		for i := range current {
			currentIDs[i] = current[i].MenuID
		}
		sort.Slice(currentIDs, func(i, j int) bool { return currentIDs[i] < currentIDs[j] })
		sort.Slice(menuIDs, func(i, j int) bool { return menuIDs[i] < menuIDs[j] })
		if equalInt64s(currentIDs, menuIDs) {
			return nil
		}
		if _, err := s.Exec("DELETE FROM "+iam.TableRoleMenu+" WHERE role_id=?", roleID); err != nil {
			return fmt.Errorf("remove previous IAM role menus: %w", err)
		}
		for _, menuID := range menuIDs {
			if _, err := s.Exec("INSERT INTO "+iam.TableRoleMenu+" (role_id, menu_id, created_at, created_by) VALUES (?, ?, CURRENT_TIMESTAMP, ?)", roleID, menuID, auditActor(audit)); err != nil {
				return fmt.Errorf("assign menu to IAM role: %w", err)
			}
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", roleID))
			audit.ChangesJSON, err = json.Marshal(map[string][]int64{"before_menu_ids": currentIDs, "after_menu_ids": menuIDs})
			if err != nil {
				return fmt.Errorf("marshal IAM role menu audit: %w", err)
			}
			if _, err := s.Exec("UPDATE "+iam.TableRole+" SET updated_at=CURRENT_TIMESTAMP, updated_by=?, version_no=version_no+1 WHERE id=?", auditActor(audit), roleID); err != nil {
				return fmt.Errorf("update IAM role menu version: %w", err)
			}
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM role menu audit: %w", err)
			}
		}
		return nil
	})
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *Repository) ListMenus(ctx context.Context) ([]iam.Menu, error) {
	var menus []iam.Menu
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.OrderBy("parent_id NULLS FIRST, sort_order ASC, id ASC").Find(&menus); err != nil {
		return nil, fmt.Errorf("list IAM menus: %w", err)
	}
	return menus, nil
}

func (r *Repository) CreateMenu(ctx context.Context, menu *iam.Menu, audit *iam.AuditLog) error {
	if menu == nil {
		return errors.New("IAM menu is required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		if err := validateMenuParent(s, menu); err != nil {
			return err
		}
		now := time.Now().UTC()
		menu.CreatedAt, menu.UpdatedAt = now, now
		menu.VersionNo = 1
		if _, err := s.InsertOne(menu); err != nil {
			if errors.Is(menuWriteError(err), ErrMenuCodeExists) {
				return ErrMenuCodeExists
			}
			return fmt.Errorf("insert IAM menu: %w", err)
		}
		if _, err := s.Exec("INSERT INTO "+iam.TableRoleMenu+" (role_id, menu_id, created_at, created_by) SELECT id, ?, CURRENT_TIMESTAMP, 'system' FROM "+iam.TableRole+" WHERE role_code=? AND status='active' ON CONFLICT (role_id, menu_id) DO NOTHING", menu.ID, platformAdminRole); err != nil {
			return fmt.Errorf("assign new menu to platform administrator: %w", err)
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", menu.ID))
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM menu creation audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) UpdateMenu(ctx context.Context, menu *iam.Menu, expectedVersion int64, audit *iam.AuditLog) error {
	if menu == nil || menu.ID <= 0 || expectedVersion <= 0 {
		return errors.New("IAM menu and expected version are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		current := new(iam.Menu)
		found, err := s.ID(menu.ID).Get(current)
		if err != nil {
			return fmt.Errorf("find IAM menu for update: %w", err)
		}
		if !found {
			return ErrMenuNotFound
		}
		if current.VersionNo != expectedVersion {
			return ErrMenuVersionConflict
		}
		if current.MenuType != menu.MenuType {
			var children int64
			children, err = s.Table(iam.TableMenu).Where("parent_id=?", menu.ID).Count(new(iam.Menu))
			if err != nil {
				return fmt.Errorf("check IAM menu children before type change: %w", err)
			}
			if children > 0 {
				return ErrMenuHasChildren
			}
		}
		if err := validateMenuParent(s, menu); err != nil {
			return err
		}
		if menu.ParentID != nil && *menu.ParentID == menu.ID {
			return ErrMenuParentInvalid
		}
		now := time.Now().UTC()
		values := map[string]interface{}{
			"parent_id": menu.ParentID, "menu_name": menu.MenuName, "menu_code": menu.MenuCode,
			"path": menu.Path, "component": menu.Component, "icon": menu.Icon,
			"sort_order": menu.SortOrder, "menu_type": menu.MenuType, "visible": menu.Visible,
			"status": menu.Status, "updated_at": now, "updated_by": menu.UpdatedBy,
			"version_no": expectedVersion + 1,
		}
		rows, err := s.Table(iam.TableMenu).Where("id=? AND version_no=?", menu.ID, expectedVersion).Update(values)
		if err != nil {
			if errors.Is(menuWriteError(err), ErrMenuCodeExists) {
				return ErrMenuCodeExists
			}
			return fmt.Errorf("update IAM menu: %w", err)
		}
		if rows != 1 {
			return ErrMenuVersionConflict
		}
		menu.VersionNo = expectedVersion + 1
		menu.UpdatedAt = now
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", menu.ID))
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM menu update audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) DeleteMenu(ctx context.Context, menuID int64, audit *iam.AuditLog) error {
	if menuID <= 0 {
		return errors.New("positive IAM menu ID is required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		menu := new(iam.Menu)
		found, err := s.ID(menuID).Get(menu)
		if err != nil {
			return fmt.Errorf("find IAM menu for deletion: %w", err)
		}
		if !found {
			return ErrMenuNotFound
		}
		var counts struct {
			Children int64 `xorm:"'children'"`
			Roles    int64 `xorm:"'roles'"`
		}
		query := "SELECT (SELECT COUNT(*) FROM " + iam.TableMenu + " WHERE parent_id=?) AS children, (SELECT COUNT(*) FROM " + iam.TableRoleMenu + " WHERE menu_id=?) AS roles"
		if _, err := s.SQL(query, menuID, menuID).Get(&counts); err != nil {
			return fmt.Errorf("check IAM menu references: %w", err)
		}
		if counts.Children > 0 {
			return ErrMenuHasChildren
		}
		if counts.Roles > 0 {
			return ErrMenuAssignedToRole
		}
		if _, err := s.ID(menuID).Delete(new(iam.Menu)); err != nil {
			return fmt.Errorf("delete IAM menu: %w", err)
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", menuID))
			changes, err := json.Marshal(map[string]string{"menu_code": menu.MenuCode, "menu_name": menu.MenuName})
			if err != nil {
				return fmt.Errorf("encode IAM menu deletion audit: %w", err)
			}
			audit.ChangesJSON = changes
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM menu deletion audit: %w", err)
			}
		}
		return nil
	})
}

func validateMenuParent(s *xorm.Session, menu *iam.Menu) error {
	if menu.MenuType == "directory" {
		if menu.ParentID != nil {
			return ErrMenuParentInvalid
		}
		return nil
	}
	if menu.ParentID == nil || *menu.ParentID <= 0 {
		return ErrMenuParentInvalid
	}
	parentType := "directory"
	if menu.MenuType == "button" {
		parentType = "page"
	}
	parent := new(iam.Menu)
	found, err := s.Where("id=? AND menu_type=? AND status='active'", *menu.ParentID, parentType).Get(parent)
	if err != nil {
		return fmt.Errorf("validate IAM menu parent: %w", err)
	}
	if !found {
		return ErrMenuParentInvalid
	}
	return nil
}

func menuWriteError(err error) error {
	var postgresError *pq.Error
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.Constraint == "uq_t_iam_menu_code" {
		return ErrMenuCodeExists
	}
	return err
}

func (r *Repository) GrantRole(ctx context.Context, actorID, userID, roleID int64, audit *iam.AuditLog) error {
	if actorID <= 0 || userID <= 0 || roleID <= 0 {
		return errors.New("active actor, user, and role are required")
	}
	if actorID == userID {
		return errors.New("an operator cannot grant a role to themselves")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		var count struct {
			Value int64 `xorm:"'count'"`
		}
		query := "SELECT COUNT(*) AS count FROM " + iam.TableUser + " u CROSS JOIN " + iam.TableRole + " r CROSS JOIN " + iam.TableUser + " actor WHERE u.id=? AND u.user_status='active' AND r.id=? AND r.status='active' AND actor.id=? AND actor.user_status='active'"
		if _, err := s.SQL(query, userID, roleID, actorID).Get(&count); err != nil {
			return err
		}
		if count.Value != 1 {
			return errors.New("active user and role are required")
		}
		var unauthorized struct {
			Count int64 `xorm:"'count'"`
		}
		permissionQuery := "SELECT COUNT(*) AS count FROM " + iam.TableRolePermission + " target_rp JOIN " + iam.TablePermission + " target_p ON target_p.id=target_rp.permission_id JOIN " + iam.TableResource + " target_res ON target_res.id=target_p.resource_id WHERE target_rp.role_id=? AND target_p.status='active' AND target_res.status='active' AND NOT EXISTS (SELECT 1 FROM " + iam.TableUserRole + " actor_ur JOIN " + iam.TableRole + " actor_r ON actor_r.id=actor_ur.role_id JOIN " + iam.TableRolePermission + " actor_rp ON actor_rp.role_id=actor_r.id JOIN " + iam.TablePermission + " actor_p ON actor_p.id=actor_rp.permission_id JOIN " + iam.TableResource + " actor_res ON actor_res.id=actor_p.resource_id WHERE actor_ur.user_id=? AND actor_r.status='active' AND actor_p.status='active' AND actor_res.status='active' AND actor_p.permission_code=target_p.permission_code)"
		if _, err := s.SQL(permissionQuery, roleID, actorID).Get(&unauthorized); err != nil {
			return fmt.Errorf("verify IAM role grant permissions: %w", err)
		}
		if unauthorized.Count != 0 {
			return errors.New("an operator cannot grant permissions they do not have")
		}
		_, err := s.Exec("INSERT INTO "+iam.TableUserRole+" (user_id, role_id, created_at, created_by) VALUES (?, ?, CURRENT_TIMESTAMP, ?) ON CONFLICT (user_id, role_id) DO NOTHING", userID, roleID, auditActor(audit))
		if err != nil {
			return fmt.Errorf("grant IAM role: %w", err)
		}
		if audit != nil {
			audit.UserID = int64Ptr(userID)
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM role grant audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) RevokeRole(ctx context.Context, actorID, userID, roleID int64, audit *iam.AuditLog) error {
	if actorID <= 0 || userID <= 0 || roleID <= 0 || actorID == userID {
		return errors.New("active operator, target user, and role are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		if _, err := s.Exec("LOCK TABLE " + iam.TableUserRole + " IN EXCLUSIVE MODE"); err != nil {
			return fmt.Errorf("lock IAM role assignments: %w", err)
		}
		var actor struct {
			Count int64 `xorm:"'count'"`
		}
		query := "SELECT COUNT(*) AS count FROM " + iam.TableUser + " u WHERE u.id=? AND u.user_status='active' AND EXISTS (SELECT 1 FROM " + iam.TableUserRole + " ur JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableRolePermission + " rp ON rp.role_id=r.id JOIN " + iam.TablePermission + " p ON p.id=rp.permission_id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE ur.user_id=u.id AND r.status='active' AND p.status='active' AND res.status='active' AND p.permission_code='iam.role.grant')"
		if _, err := s.SQL(query, actorID).Get(&actor); err != nil || actor.Count != 1 {
			if err != nil {
				return err
			}
			return errors.New("active role grant permission is required")
		}
		var role struct {
			RoleCode string `xorm:"'role_code'"`
			IsSystem bool   `xorm:"'is_system'"`
		}
		found, err := s.Table(iam.TableRole).Where("id=?", roleID).Get(&role)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("IAM role not found")
		}
		if role.RoleCode == platformAdminRole {
			var admins struct {
				Count int64 `xorm:"'count'"`
			}
			countQuery := "SELECT COUNT(DISTINCT u.id) AS count FROM " + iam.TableUser + " u JOIN " + iam.TableUserRole + " ur ON ur.user_id=u.id JOIN " + iam.TableRole + " r ON r.id=ur.role_id WHERE u.user_status='active' AND r.role_code=? AND r.status='active'"
			if _, err := s.SQL(countQuery, platformAdminRole).Get(&admins); err != nil {
				return err
			}
			var target struct {
				Count int64 `xorm:"'count'"`
			}
			if _, err := s.SQL("SELECT COUNT(*) AS count FROM "+iam.TableUserRole+" WHERE user_id=? AND role_id=?", userID, roleID).Get(&target); err != nil {
				return err
			}
			if target.Count > 0 && admins.Count <= 1 {
				return errors.New("cannot revoke the last active platform administrator role")
			}
		}
		result, err := s.Exec("DELETE FROM "+iam.TableUserRole+" WHERE user_id=? AND role_id=?", userID, roleID)
		if err != nil {
			return fmt.Errorf("revoke IAM role: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed > 0 && audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d:%d", userID, roleID))
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM role revoke audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ListPermissionCatalog(ctx context.Context) ([]iam.Permission, error) {
	var permissions []iam.Permission
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := "SELECT p.* FROM " + iam.TablePermission + " p JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE p.status='active' AND res.status='active' ORDER BY res.resource_code, p.action_code"
	if err := s.SQL(query).Find(&permissions); err != nil {
		return nil, fmt.Errorf("list active IAM permission catalog: %w", err)
	}
	return permissions, nil
}

func (r *Repository) GrantPermission(ctx context.Context, actorID, roleID, permissionID int64, audit *iam.AuditLog) error {
	if actorID <= 0 || roleID <= 0 || permissionID <= 0 {
		return errors.New("active actor, role, and permission are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		var count struct {
			Value int64 `xorm:"'count'"`
		}
		query := "SELECT COUNT(*) AS count FROM " + iam.TableRole + " r CROSS JOIN " + iam.TablePermission + " p JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE r.id=? AND r.status='active' AND p.id=? AND p.status='active' AND res.status='active' AND EXISTS (SELECT 1 FROM " + iam.TableUser + " actor_u JOIN " + iam.TableUserRole + " actor_ur ON actor_ur.user_id=actor_u.id JOIN " + iam.TableRole + " actor_r ON actor_r.id=actor_ur.role_id JOIN " + iam.TableRolePermission + " actor_rp ON actor_rp.role_id=actor_r.id JOIN " + iam.TablePermission + " actor_p ON actor_p.id=actor_rp.permission_id JOIN " + iam.TableResource + " actor_res ON actor_res.id=actor_p.resource_id WHERE actor_u.id=? AND actor_u.user_status='active' AND actor_r.status='active' AND actor_p.status='active' AND actor_res.status='active' AND actor_p.permission_code=p.permission_code)"
		if _, err := s.SQL(query, roleID, permissionID, actorID).Get(&count); err != nil {
			return err
		}
		if count.Value != 1 {
			return errors.New("active role and permission are required")
		}
		_, err := s.Exec("INSERT INTO "+iam.TableRolePermission+" (role_id, permission_id, created_at, created_by) VALUES (?, ?, CURRENT_TIMESTAMP, ?) ON CONFLICT (role_id, permission_id) DO NOTHING", roleID, permissionID, auditActor(audit))
		if err != nil {
			return fmt.Errorf("grant IAM permission: %w", err)
		}
		if audit != nil {
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM permission grant audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) RevokePermission(ctx context.Context, actorID, roleID, permissionID int64, audit *iam.AuditLog) error {
	if actorID <= 0 || roleID <= 0 || permissionID <= 0 {
		return errors.New("active operator, role, and permission are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		var allowed struct {
			Count int64 `xorm:"'count'"`
		}
		query := "SELECT COUNT(*) AS count FROM " + iam.TableUser + " u WHERE u.id=? AND u.user_status='active' AND EXISTS (SELECT 1 FROM " + iam.TableUserRole + " ur JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableRolePermission + " rp ON rp.role_id=r.id JOIN " + iam.TablePermission + " p ON p.id=rp.permission_id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE ur.user_id=u.id AND r.status='active' AND p.status='active' AND res.status='active' AND p.permission_code='iam.role.write')"
		if _, err := s.SQL(query, actorID).Get(&allowed); err != nil || allowed.Count != 1 {
			if err != nil {
				return err
			}
			return errors.New("active role write permission is required")
		}
		var role struct {
			IsSystem bool `xorm:"'is_system'"`
		}
		found, err := s.Table(iam.TableRole).Where("id=?", roleID).Get(&role)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("IAM role not found")
		}
		if role.IsSystem {
			return errors.New("permissions on system roles cannot be revoked")
		}
		result, err := s.Exec("DELETE FROM "+iam.TableRolePermission+" WHERE role_id=? AND permission_id=?", roleID, permissionID)
		if err != nil {
			return fmt.Errorf("revoke IAM permission: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed > 0 && audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d:%d", roleID, permissionID))
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM permission revoke audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) SetUserStatus(ctx context.Context, actorID, userID int64, status, updatedBy string, audit *iam.AuditLog) error {
	if actorID <= 0 || userID <= 0 || actorID == userID || (status != "active" && status != "disabled") {
		return errors.New("valid operator, target user, and status are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		if _, err := s.Exec("LOCK TABLE " + iam.TableUserRole + " IN EXCLUSIVE MODE"); err != nil {
			return fmt.Errorf("lock IAM role assignments: %w", err)
		}
		var actor struct {
			Count int64 `xorm:"'count'"`
		}
		query := "SELECT COUNT(*) AS count FROM " + iam.TableUser + " u WHERE u.id=? AND u.user_status='active' AND EXISTS (SELECT 1 FROM " + iam.TableUserRole + " ur JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableRolePermission + " rp ON rp.role_id=r.id JOIN " + iam.TablePermission + " p ON p.id=rp.permission_id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE ur.user_id=u.id AND r.status='active' AND p.status='active' AND res.status='active' AND p.permission_code='iam.user.write')"
		if _, err := s.SQL(query, actorID).Get(&actor); err != nil || actor.Count != 1 {
			if err != nil {
				return err
			}
			return errors.New("active user write permission is required")
		}
		var target struct {
			Status string `xorm:"'user_status'"`
		}
		found, err := s.Table(iam.TableUser).Where("id=?", userID).Get(&target)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("IAM user not found")
		}
		if target.Status == status {
			return nil
		}
		now := time.Now().UTC()
		if _, err := s.Exec("UPDATE "+iam.TableUser+" SET user_status=?, updated_at=?, updated_by=?, version_no=version_no+1 WHERE id=?", status, now, updatedBy, userID); err != nil {
			return fmt.Errorf("update IAM user status: %w", err)
		}
		if status == "disabled" {
			var admins struct {
				Count int64 `xorm:"'count'"`
			}
			countQuery := "SELECT COUNT(DISTINCT u.id) AS count FROM " + iam.TableUser + " u JOIN " + iam.TableUserRole + " ur ON ur.user_id=u.id JOIN " + iam.TableRole + " r ON r.id=ur.role_id WHERE u.user_status='active' AND r.role_code=? AND r.status='active'"
			if _, err := s.SQL(countQuery, platformAdminRole).Get(&admins); err != nil {
				return err
			}
			if admins.Count == 0 {
				return errors.New("cannot disable the last active platform administrator")
			}
			if _, err := s.Exec("UPDATE "+iam.TableUserSession+" SET status='revoked', revoked_at=?, revocation_reason='user_disabled' WHERE user_id=? AND status='active'", now, userID); err != nil {
				return fmt.Errorf("revoke disabled user's sessions: %w", err)
			}
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", userID))
			audit.ChangesJSON = []byte(fmt.Sprintf(`{"user_status":{"from":%q,"to":%q}}`, target.Status, status))
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM user status audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ResetUserPassword(ctx context.Context, actorID, userID int64, salt, hash string, version int16, updatedBy string, audit *iam.AuditLog) error {
	if actorID <= 0 || userID <= 0 || actorID == userID || salt == "" || hash == "" || version <= 0 {
		return errors.New("valid operator, target user, and password hash are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		var actor struct {
			Count int64 `xorm:"'count'"`
		}
		query := "SELECT COUNT(*) AS count FROM " + iam.TableUser + " u WHERE u.id=? AND u.user_status='active' AND EXISTS (SELECT 1 FROM " + iam.TableUserRole + " ur JOIN " + iam.TableRole + " r ON r.id=ur.role_id JOIN " + iam.TableRolePermission + " rp ON rp.role_id=r.id JOIN " + iam.TablePermission + " p ON p.id=rp.permission_id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE ur.user_id=u.id AND r.status='active' AND p.status='active' AND res.status='active' AND p.permission_code='iam.user.write')"
		if _, err := s.SQL(query, actorID).Get(&actor); err != nil || actor.Count != 1 {
			if err != nil {
				return err
			}
			return errors.New("active user write permission is required")
		}
		var target struct {
			ID int64 `xorm:"'id'"`
		}
		found, err := s.Table(iam.TableUser).Where("id=?", userID).Get(&target)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("IAM user not found")
		}
		now := time.Now().UTC()
		if _, err := s.Exec("UPDATE "+iam.TableUser+" SET password_salt=?, password_hash=?, password_hash_version=?, auth_source=CASE WHEN auth_source='external' THEN 'mixed' ELSE auth_source END, must_change_password=TRUE, updated_at=?, updated_by=?, version_no=version_no+1 WHERE id=?", salt, hash, version, now, updatedBy, userID); err != nil {
			return fmt.Errorf("reset IAM user password: %w", err)
		}
		if _, err := s.Exec("UPDATE "+iam.TableUserSession+" SET status='revoked', revoked_at=?, revocation_reason='password_reset' WHERE user_id=? AND status='active'", now, userID); err != nil {
			return fmt.Errorf("revoke sessions after IAM password reset: %w", err)
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", userID))
			audit.ChangesJSON = []byte(`{"password":"reset","must_change_password":true}`)
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM password reset audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ChangeOwnPassword(ctx context.Context, userID int64, expectedHash, salt, hash string, version int16, currentSessionID string, audit *iam.AuditLog) error {
	if userID <= 0 || expectedHash == "" || salt == "" || hash == "" || version <= 0 || strings.TrimSpace(currentSessionID) == "" {
		return errors.New("active user, current session, and password hashes are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		now := time.Now().UTC()
		result, err := s.Exec("UPDATE "+iam.TableUser+" SET password_salt=?, password_hash=?, password_hash_version=?, must_change_password=FALSE, updated_at=?, updated_by=username, version_no=version_no+1 WHERE id=? AND user_status='active' AND password_hash=?", salt, hash, version, now, userID, expectedHash)
		if err != nil {
			return fmt.Errorf("change IAM user password: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return errors.New("IAM password changed concurrently or user is inactive")
		}
		if _, err := s.Exec("UPDATE "+iam.TableUserSession+" SET status='revoked', revoked_at=?, revocation_reason='password_changed' WHERE user_id=? AND status='active' AND session_id<>?", now, userID, currentSessionID); err != nil {
			return fmt.Errorf("revoke other sessions after IAM password change: %w", err)
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", userID))
			audit.ChangesJSON = []byte(`{"password":"changed","must_change_password":false}`)
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM password change audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) FindTOTPAuthenticator(ctx context.Context, userID int64) (*iam.UserAuthenticator, error) {
	row := new(iam.UserAuthenticator)
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.Where("user_id=? AND authenticator_type='totp'", userID).Get(row)
	if err != nil {
		return nil, fmt.Errorf("load IAM TOTP authenticator: %w", err)
	}
	if !found {
		return nil, nil
	}
	return row, nil
}

func (r *Repository) SavePendingTOTP(ctx context.Context, row *iam.UserAuthenticator, audit *iam.AuditLog) error {
	if row == nil || row.UserID <= 0 || row.SecretCiphertext == "" || row.EncryptionKeyVersion == "" {
		return errors.New("valid pending TOTP authenticator is required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		var user struct {
			Count int64 `xorm:"'count'"`
		}
		if _, err := s.SQL("SELECT COUNT(*) AS count FROM "+iam.TableUser+" WHERE id=? AND user_status='active'", row.UserID).Get(&user); err != nil {
			return err
		}
		if user.Count != 1 {
			return errors.New("active IAM user is required")
		}
		query := "INSERT INTO " + iam.TableUserAuthenticator + " (user_id, authenticator_type, secret_ciphertext, encryption_key_version, status, verified_at, last_used_at, last_used_step, created_at, updated_at) VALUES (?, 'totp', ?, ?, 'pending', NULL, NULL, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) ON CONFLICT (user_id, authenticator_type) DO UPDATE SET secret_ciphertext=EXCLUDED.secret_ciphertext, encryption_key_version=EXCLUDED.encryption_key_version, status='pending', verified_at=NULL, last_used_at=NULL, last_used_step=0, updated_at=CURRENT_TIMESTAMP WHERE " + iam.TableUserAuthenticator + ".status IN ('pending','disabled','revoked')"
		result, err := s.Exec(query, row.UserID, row.SecretCiphertext, row.EncryptionKeyVersion)
		if err != nil {
			return fmt.Errorf("save pending IAM TOTP authenticator: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return errors.New("an active TOTP authenticator already exists")
		}
		if audit != nil {
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM TOTP setup audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ActivateTOTP(ctx context.Context, userID, step int64, audit *iam.AuditLog) error {
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		now := time.Now().UTC()
		result, err := s.Exec("UPDATE "+iam.TableUserAuthenticator+" SET status='active', verified_at=?, last_used_at=?, last_used_step=?, updated_at=? WHERE user_id=? AND authenticator_type='totp' AND status='pending' AND EXISTS (SELECT 1 FROM "+iam.TableUser+" WHERE id=? AND user_status='active')", now, now, step, now, userID, userID)
		if err != nil {
			return fmt.Errorf("activate IAM TOTP authenticator: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return errors.New("pending TOTP setup was not found")
		}
		if audit != nil {
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM TOTP activation audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ConsumeTOTP(ctx context.Context, userID, step int64) error {
	result, err := r.db.Engine().Context(ctx).Exec("UPDATE "+iam.TableUserAuthenticator+" SET last_used_step=?, last_used_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE user_id=? AND authenticator_type='totp' AND status='active' AND last_used_step < ?", step, userID, step)
	if err != nil {
		return fmt.Errorf("consume IAM TOTP code: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("TOTP code is invalid or already used")
	}
	return nil
}

func (r *Repository) DisableTOTP(ctx context.Context, userID, step int64, audit *iam.AuditLog) error {
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		result, err := s.Exec("UPDATE "+iam.TableUserAuthenticator+" SET status='disabled', last_used_step=?, last_used_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE user_id=? AND authenticator_type='totp' AND status='active' AND last_used_step < ? AND EXISTS (SELECT 1 FROM "+iam.TableUser+" WHERE id=? AND user_status='active')", step, userID, step, userID)
		if err != nil {
			return fmt.Errorf("disable IAM TOTP authenticator: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return errors.New("active TOTP authenticator was not found")
		}
		if audit != nil {
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write IAM TOTP disable audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) WriteAudit(ctx context.Context, audit *iam.AuditLog) error {
	if audit == nil {
		return errors.New("IAM audit record is required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error { return insertAudit(s, audit) })
}

func (r *Repository) BootstrapAdmin(ctx context.Context, user *iam.User, audit *iam.AuditLog) error {
	if user == nil || audit == nil {
		return errors.New("bootstrap user and audit record are required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		if _, err := s.Exec("LOCK TABLE " + iam.TableUserRole + " IN EXCLUSIVE MODE"); err != nil {
			return fmt.Errorf("lock IAM bootstrap state: %w", err)
		}
		var admins struct {
			Count int64 `xorm:"'count'"`
		}
		query := "SELECT COUNT(*) AS count FROM " + iam.TableUserRole + " ur JOIN " + iam.TableUser + " u ON u.id=ur.user_id JOIN " + iam.TableRole + " r ON r.id=ur.role_id WHERE u.user_status='active' AND r.role_code=? AND r.status='active'"
		if _, err := s.SQL(query, platformAdminRole).Get(&admins); err != nil {
			return err
		}
		if admins.Count > 0 {
			return errors.New("VeloTrust bootstrap is already complete")
		}
		now := time.Now().UTC()
		var role iam.Role
		found, err := s.Where("role_code=?", platformAdminRole).Get(&role)
		if err != nil {
			return err
		}
		if !found {
			role = iam.Role{RoleName: "Platform Administrator", RoleCode: platformAdminRole, IsSystem: true, Status: "active", CreatedAt: now, UpdatedAt: now, VersionNo: 1, CreatedBy: "bootstrap", UpdatedBy: "bootstrap"}
			if _, err := s.InsertOne(&role); err != nil {
				return fmt.Errorf("create platform administrator role: %w", err)
			}
		}
		if _, err := s.Exec("INSERT INTO "+iam.TableRoleMenu+" (role_id, menu_id, created_at, created_by) SELECT ?, id, ?, 'bootstrap' FROM "+iam.TableMenu+" WHERE status='active' ON CONFLICT (role_id, menu_id) DO NOTHING", role.ID, now); err != nil {
			return fmt.Errorf("assign active menus to platform administrator role: %w", err)
		}
		user.UserStatus, user.AuthSource = "active", "local"
		user.CreatedBy, user.UpdatedBy = "bootstrap", "bootstrap"
		if err := insertUser(s, user); err != nil {
			return fmt.Errorf("create bootstrap user: %w", err)
		}
		if _, err := s.Exec("INSERT INTO "+iam.TableUserRole+" (user_id, role_id, created_at, created_by) VALUES (?, ?, ?, ?)", user.ID, role.ID, now, "bootstrap"); err != nil {
			return fmt.Errorf("assign platform administrator role: %w", err)
		}
		for _, seed := range bootstrapPermissions {
			resource := &iam.Resource{ResourceName: seed.resource, ResourceCode: seed.resource, ResourceType: "management", Status: "active", CreatedAt: now, UpdatedAt: now, CreatedBy: "bootstrap", UpdatedBy: "bootstrap", VersionNo: 1}
			found, err := s.Where("resource_code=?", seed.resource).Get(resource)
			if err != nil {
				return err
			}
			if !found {
				if _, err := s.InsertOne(resource); err != nil {
					return fmt.Errorf("create IAM resource seed: %w", err)
				}
			}
			permission := &iam.Permission{PermissionName: seed.name, PermissionCode: seed.code, ResourceID: resource.ID, ActionCode: seed.action, Status: "active", CreatedAt: now, UpdatedAt: now, CreatedBy: "bootstrap", UpdatedBy: "bootstrap", VersionNo: 1}
			found, err = s.Where("permission_code=?", seed.code).Get(permission)
			if err != nil {
				return err
			}
			if !found {
				if _, err := s.InsertOne(permission); err != nil {
					return fmt.Errorf("create IAM permission seed: %w", err)
				}
			}
			if _, err := s.Exec("INSERT INTO "+iam.TableRolePermission+" (role_id, permission_id, created_at, created_by) VALUES (?, ?, ?, ?) ON CONFLICT (role_id, permission_id) DO NOTHING", role.ID, permission.ID, now, "bootstrap"); err != nil {
				return fmt.Errorf("assign IAM permission seed: %w", err)
			}
		}
		audit.UserID = int64Ptr(user.ID)
		if err := insertAudit(s, audit); err != nil {
			return fmt.Errorf("write bootstrap audit: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListWebAuthnCredentials(ctx context.Context, userID int64, rpID string) ([]iam.WebAuthnCredential, error) {
	var credentials []iam.WebAuthnCredential
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	err := s.Where("user_id=? AND rp_id=? AND status='active'", userID, rpID).OrderBy("id ASC").Find(&credentials)
	if err != nil {
		return nil, fmt.Errorf("list active WebAuthn credentials: %w", err)
	}
	return credentials, nil
}

func (r *Repository) FindUserByCredential(ctx context.Context, rpID string, credentialID []byte) (*iam.User, []iam.WebAuthnCredential, error) {
	var credential iam.WebAuthnCredential
	s := r.db.Engine().NewSession().Context(ctx)
	found, err := s.Where("rp_id=? AND credential_id=? AND status='active'", rpID, credentialID).Get(&credential)
	if err != nil {
		_ = s.Close()
		return nil, nil, fmt.Errorf("find active WebAuthn credential: %w", err)
	}
	if !found {
		_ = s.Close()
		return nil, nil, nil
	}
	_ = s.Close()
	user, err := r.FindUserByID(ctx, credential.UserID)
	if err != nil || user == nil {
		return user, nil, err
	}
	credentials, err := r.ListWebAuthnCredentials(ctx, user.ID, rpID)
	return user, credentials, err
}

func (r *Repository) CreateAuthChallenge(ctx context.Context, challenge *iam.AuthChallenge) error {
	if challenge == nil || challenge.ChallengeID == "" || challenge.ChallengeHash == "" || challenge.ChallengeType == "" {
		return errors.New("complete WebAuthn challenge is required")
	}
	if challenge.Status == "" {
		challenge.Status = "pending"
	}
	if challenge.MaxAttempts == 0 {
		challenge.MaxAttempts = 1
	}
	if challenge.CreatedAt.IsZero() {
		challenge.CreatedAt = time.Now().UTC()
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if _, err := s.InsertOne(challenge); err != nil {
		return fmt.Errorf("create WebAuthn challenge: %w", err)
	}
	return nil
}

func (r *Repository) ConsumeAuthChallenge(ctx context.Context, challengeID, challengeHash string) (*iam.AuthChallenge, error) {
	challenge := new(iam.AuthChallenge)
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.Where("challenge_id=? AND challenge_hash=? AND status='pending' AND expires_at>CURRENT_TIMESTAMP AND attempt_count<max_attempts", challengeID, challengeHash).Get(challenge)
	if err != nil {
		return nil, fmt.Errorf("load WebAuthn challenge: %w", err)
	}
	if !found {
		return nil, nil
	}
	now := time.Now().UTC()
	result, err := s.Exec("UPDATE "+iam.TableAuthChallenge+" SET status='consumed', consumed_at=?, attempt_count=attempt_count+1 WHERE challenge_id=? AND challenge_hash=? AND status='pending' AND expires_at>? AND attempt_count<max_attempts", now, challengeID, challengeHash, now)
	if err != nil {
		return nil, fmt.Errorf("consume WebAuthn challenge: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows != 1 {
		return nil, nil
	}
	challenge.Status, challenge.AttemptCount, challenge.ConsumedAt = "consumed", challenge.AttemptCount+1, &now
	return challenge, nil
}

func (r *Repository) CreateWebAuthnCredential(ctx context.Context, credential *iam.WebAuthnCredential, audit *iam.AuditLog) error {
	if credential == nil || len(credential.CredentialID) == 0 || len(credential.PublicKeyCOSE) == 0 {
		return errors.New("complete WebAuthn credential is required")
	}
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		if credential.Status == "" {
			credential.Status = "active"
		}
		if credential.CreatedAt.IsZero() {
			credential.CreatedAt = time.Now().UTC()
		}
		if credential.Transports == "" {
			credential.Transports = "[]"
		}
		if _, err := s.InsertOne(credential); err != nil {
			return fmt.Errorf("save WebAuthn credential: %w", err)
		}
		if audit != nil {
			audit.UserID = int64Ptr(credential.UserID)
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write WebAuthn registration audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) UpdateWebAuthnCredentialUse(ctx context.Context, credential *iam.WebAuthnCredential) error {
	if credential == nil || credential.ID <= 0 {
		return errors.New("WebAuthn credential ID is required")
	}
	now := time.Now().UTC()
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	result, err := s.Exec("UPDATE "+iam.TableWebAuthnCredential+" SET signature_count=?, backup_state=?, last_used_at=? WHERE id=? AND status='active' AND (signature_count=0 OR ?>signature_count)", credential.SignatureCount, credential.BackupState, now, credential.ID, credential.SignatureCount)
	if err != nil {
		return fmt.Errorf("update WebAuthn credential usage: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return errors.New("WebAuthn credential counter did not advance or credential was revoked")
	}
	return nil
}

func (r *Repository) RevokeWebAuthnCredential(ctx context.Context, userID, credentialID int64, audit *iam.AuditLog) error {
	return r.db.WithTransaction(ctx, func(s *xorm.Session) error {
		now := time.Now().UTC()
		result, err := s.Exec("UPDATE "+iam.TableWebAuthnCredential+" SET status='revoked', revoked_at=?, revocation_reason='user_request' WHERE user_id=? AND id=? AND status='active'", now, userID, credentialID)
		if err != nil {
			return fmt.Errorf("revoke WebAuthn credential: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil || rows != 1 {
			return errors.New("active WebAuthn credential not found")
		}
		if audit != nil {
			audit.UserID = int64Ptr(userID)
			if err := insertAudit(s, audit); err != nil {
				return fmt.Errorf("write WebAuthn revocation audit: %w", err)
			}
		}
		return nil
	})
}

func insertAudit(s *xorm.Session, audit *iam.AuditLog) error {
	if audit.EventID == "" || audit.Action == "" || audit.TargetType == "" || audit.Result == "" {
		return errors.New("IAM audit event is incomplete")
	}
	// XORM writes a nil []byte as an empty string, which PostgreSQL cannot cast
	// to JSONB. Audit records without change details use an empty JSON object.
	if len(audit.ChangesJSON) == 0 {
		audit.ChangesJSON = []byte(`{}`)
	}
	if audit.OccurredAt.IsZero() {
		audit.OccurredAt = time.Now().UTC()
	}
	if _, err := s.InsertOne(audit); err != nil {
		return err
	}
	return nil
}

func auditActor(audit *iam.AuditLog) string {
	if audit == nil || audit.UserName == "" {
		return "system"
	}
	return audit.UserName
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func int64Ptr(value int64) *int64    { return &value }
func stringPtr(value string) *string { return &value }

var _ Store = (*Repository)(nil)
