package iam

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	servicecontext "velotrust/management/internal/context"
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
	{"gateway.registration.read", "View gateway registration credentials", "gateway.registration", "read"},
	{"gateway.registration.write", "Manage gateway registration credentials", "gateway.registration", "write"},
	{"ipam.network.read", "View IPAM networks", "ipam.network", "read"},
	{"ipam.network.write", "Manage IPAM networks", "ipam.network", "write"},
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

func (r *Repository) FindUserByUsername(ctx *servicecontext.Context, username string) (*iam.User, error) {
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

func (r *Repository) FindUserByID(ctx *servicecontext.Context, id int64) (*iam.User, error) {
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

func (r *Repository) CreateUser(ctx *servicecontext.Context, user *iam.User, audit *iam.AuditLog) error {
	if user == nil {
		return errors.New("IAM user is required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		if err := insertUser(ctx, tx, user); err != nil {
			return fmt.Errorf("insert IAM user: %w", err)
		}
		if audit != nil {
			audit.UserID = int64Ptr(user.ID)
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM user audit: %w", err)
			}
		}
		return nil
	})
}

func insertUser(ctx *servicecontext.Context, tx *database.DBTransaction, user *iam.User) error {
	if user.State() != database.ModelState_New {
		user.CreateNew(ctx.UserName())
	}
	_, err := tx.Save(user)
	return err
}

func (r *Repository) CreateLoginSession(ctx *servicecontext.Context, session *iam.UserSession, log *iam.LoginLog, clientIP, usernameHash, ipHash string) error {
	if session == nil || log == nil {
		return errors.New("session and login event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		now := time.Now().UTC()
		session.CreateNew(ctx.UserName())
		if session.LastSeenAt == nil {
			session.LastSeenAt = &now
		}
		if _, err := tx.Save(session); err != nil {
			return fmt.Errorf("insert IAM session: %w", err)
		}
		var user iam.User
		found, err := s.SQL("SELECT * FROM "+iam.TableUser+" WHERE id=? AND user_status='active' FOR UPDATE", session.UserID).Get(&user)
		if err != nil {
			return fmt.Errorf("update IAM last login: %w", err)
		}
		if !found {
			return errors.New("IAM user is no longer active")
		}
		user.LastLoginAt = &now
		user.LastLoginIP = nullableStringPtr(clientIP)
		user.Update(ctx.UserName())
		user.Cols("last_login_at", "last_login_ip")
		if _, err := tx.Save(&user); err != nil {
			return fmt.Errorf("save IAM last login: %w", err)
		}
		if usernameHash != "" || ipHash != "" {
			if usernameHash == "" || ipHash == "" {
				return errors.New("both IAM login throttle keys are required")
			}
			var throttles []iam.AuthThrottle
			if err := s.In("bucket_key_hash", usernameHash, ipHash).Find(&throttles); err != nil {
				return fmt.Errorf("load successful IAM login throttle rows: %w", err)
			}
			for i := range throttles {
				throttles[i].HardDelete()
				if _, err := tx.Save(&throttles[i]); err != nil {
					return fmt.Errorf("clear successful IAM login throttle: %w", err)
				}
			}
		}
		log.UserID = int64Ptr(session.UserID)
		log.SessionID = stringPtr(session.SessionID)
		log.OccurredAt = now
		log.CreateNew(ctx.UserName())
		if _, err := tx.Save(log); err != nil {
			return fmt.Errorf("insert IAM login event: %w", err)
		}
		return nil
	})
}

func (r *Repository) LoginBlocked(ctx *servicecontext.Context, usernameHash, ipHash string) (bool, error) {
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

func (r *Repository) RecordLoginFailure(ctx *servicecontext.Context, log *iam.LoginLog, usernameHash, ipHash string, maxFailures, windowMinutes, blockMinutes int) error {
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
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		for _, item := range []struct{ dimension, hash string }{{"username", usernameHash}, {"source_ip", ipHash}} {
			if item.hash == "" {
				return errors.New("login throttle key is required")
			}
			if _, err := s.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", item.dimension+":"+item.hash+":"+windowStart.Format(time.RFC3339Nano)); err != nil {
				return fmt.Errorf("lock IAM login throttle key: %w", err)
			}
			var throttle iam.AuthThrottle
			found, err := s.Where("dimension=? AND bucket_key_hash=? AND window_started_at=?", item.dimension, item.hash, windowStart).ForUpdate().Get(&throttle)
			if err != nil {
				return fmt.Errorf("load IAM login throttle: %w", err)
			}
			if found {
				throttle.FailureCount++
				if throttle.FailureCount >= maxFailures {
					throttle.BlockedUntil = &blockedUntil
				}
				throttle.Update(ctx.UserName())
				throttle.Cols("failure_count", "blocked_until")
			} else {
				throttle = iam.AuthThrottle{BucketKeyHash: item.hash, Dimension: item.dimension, WindowStartedAt: windowStart, FailureCount: 1}
				if throttle.FailureCount >= maxFailures {
					throttle.BlockedUntil = &blockedUntil
				}
				throttle.CreateNew(ctx.UserName())
			}
			if _, err := tx.Save(&throttle); err != nil {
				return fmt.Errorf("save IAM login throttle: %w", err)
			}
		}
		log.OccurredAt = now
		log.CreateNew(ctx.UserName())
		if _, err := tx.Save(log); err != nil {
			return fmt.Errorf("insert failed IAM login event: %w", err)
		}
		return nil
	})
}

func (r *Repository) ClearLoginThrottle(ctx *servicecontext.Context, usernameHash, ipHash string) error {
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var throttles []iam.AuthThrottle
		if err := tx.In("bucket_key_hash", usernameHash, ipHash).Find(&throttles); err != nil {
			return fmt.Errorf("load IAM login throttle rows: %w", err)
		}
		for i := range throttles {
			throttles[i].HardDelete()
			if _, err := tx.Save(&throttles[i]); err != nil {
				return fmt.Errorf("clear IAM login throttle: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ResolveSession(ctx *servicecontext.Context, tokenHash string) (*iam.UserSession, *iam.User, error) {
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

func (r *Repository) TouchSession(ctx *servicecontext.Context, sessionID, tokenHash, clientIP string) error {
	now := time.Now().UTC()
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var session iam.UserSession
		found, err := tx.Where("session_id=? AND token_hash=? AND status='active' AND expires_at>?", sessionID, tokenHash, now).ForUpdate().Get(&session)
		if err != nil {
			return fmt.Errorf("load IAM session for touch: %w", err)
		}
		if !found {
			return errors.New("IAM session is no longer active")
		}
		session.LastSeenAt = &now
		session.Update(ctx.UserName())
		session.Cols("last_seen_at")
		session.AddSaveCondition("status=?", "active")
		session.AddSaveCondition("expires_at>?", now)
		if _, err := tx.Save(&session); err != nil {
			return fmt.Errorf("touch IAM session: %w", err)
		}
		return nil
	})
}

func (r *Repository) RevokeSession(ctx *servicecontext.Context, sessionID, reason string) error {
	now := time.Now().UTC()
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var session iam.UserSession
		found, err := tx.Where("session_id=? AND status='active'", sessionID).ForUpdate().Get(&session)
		if err != nil {
			return fmt.Errorf("load IAM session for revocation: %w", err)
		}
		if !found {
			return nil
		}
		session.Status, session.RevokedAt, session.RevocationReason = "revoked", &now, &reason
		session.Update(ctx.UserName())
		session.Cols("status", "revoked_at", "revocation_reason")
		session.AddSaveCondition("status=?", "active")
		if _, err := tx.Save(&session); err != nil {
			return fmt.Errorf("revoke IAM session: %w", err)
		}
		return nil
	})
}

func (r *Repository) RevokeUserSessions(ctx *servicecontext.Context, userID int64, reason string) error {
	now := time.Now().UTC()
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var sessions []iam.UserSession
		if err := tx.Where("user_id=? AND status='active'", userID).ForUpdate().Find(&sessions); err != nil {
			return fmt.Errorf("load IAM user sessions for revocation: %w", err)
		}
		for i := range sessions {
			sessions[i].Status, sessions[i].RevokedAt, sessions[i].RevocationReason = "revoked", &now, &reason
			sessions[i].Update(ctx.UserName())
			sessions[i].Cols("status", "revoked_at", "revocation_reason")
			sessions[i].AddSaveCondition("status=?", "active")
			if _, err := tx.Save(&sessions[i]); err != nil {
				return fmt.Errorf("revoke IAM user session: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) HasPermission(ctx *servicecontext.Context, userID int64, code string) (bool, error) {
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

func (r *Repository) ListPermissions(ctx *servicecontext.Context, userID int64) ([]string, error) {
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

func (r *Repository) ListUsers(ctx *servicecontext.Context, page, pageSize int) ([]iam.User, int64, error) {
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

func (r *Repository) CreateRole(ctx *servicecontext.Context, role *iam.Role, audit *iam.AuditLog) error {
	if role == nil {
		return errors.New("IAM role is required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		role.CreateNew(ctx.UserName())
		if _, err := tx.Save(role); err != nil {
			return fmt.Errorf("insert IAM role: %w", err)
		}
		if audit != nil {
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM role audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ListRoles(ctx *servicecontext.Context) ([]iam.Role, error) {
	var roles []iam.Role
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.Where("status='active'").OrderBy("role_name ASC").Find(&roles); err != nil {
		return nil, fmt.Errorf("list IAM roles: %w", err)
	}
	return roles, nil
}

func (r *Repository) ListUserRoles(ctx *servicecontext.Context, userID int64) ([]iam.Role, error) {
	var roles []iam.Role
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := "SELECT r.* FROM " + iam.TableRole + " r JOIN " + iam.TableUserRole + " ur ON ur.role_id=r.id WHERE ur.user_id=? AND r.status='active' ORDER BY r.role_name ASC"
	if err := s.SQL(query, userID).Find(&roles); err != nil {
		return nil, fmt.Errorf("list IAM roles for user: %w", err)
	}
	return roles, nil
}

func (r *Repository) ListRolePermissions(ctx *servicecontext.Context, roleID int64) ([]iam.Permission, error) {
	var permissions []iam.Permission
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := "SELECT p.* FROM " + iam.TablePermission + " p JOIN " + iam.TableRolePermission + " rp ON rp.permission_id=p.id JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE rp.role_id=? AND p.status='active' AND res.status='active' ORDER BY p.permission_code ASC"
	if err := s.SQL(query, roleID).Find(&permissions); err != nil {
		return nil, fmt.Errorf("list IAM permissions for role: %w", err)
	}
	if err := enrichPermissionResources(s, permissions); err != nil {
		return nil, fmt.Errorf("load resource details for role permissions: %w", err)
	}
	return permissions, nil
}

func (r *Repository) ListRoleMenus(ctx *servicecontext.Context, roleID int64) ([]iam.Menu, error) {
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

func (r *Repository) ListUserMenus(ctx *servicecontext.Context, userID int64) ([]iam.Menu, error) {
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

func (r *Repository) SetRoleMenus(ctx *servicecontext.Context, actorID, roleID int64, menuIDs []int64, audit *iam.AuditLog) error {
	if actorID <= 0 || roleID <= 0 {
		return errors.New("active actor and IAM role are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		var previous []iam.RoleMenu
		if err := s.Where("role_id=?", roleID).ForUpdate().Find(&previous); err != nil {
			return fmt.Errorf("load previous IAM role menus: %w", err)
		}
		for i := range previous {
			previous[i].HardDelete()
			if _, err := tx.Save(&previous[i]); err != nil {
				return fmt.Errorf("remove previous IAM role menu: %w", err)
			}
		}
		for _, menuID := range menuIDs {
			link := &iam.RoleMenu{RoleID: roleID, MenuID: menuID}
			link.CreateNew(ctx.UserName())
			if _, err := tx.Save(link); err != nil {
				return fmt.Errorf("assign menu to IAM role: %w", err)
			}
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", roleID))
			audit.ChangesJSON, err = json.Marshal(map[string][]int64{"before_menu_ids": currentIDs, "after_menu_ids": menuIDs})
			if err != nil {
				return fmt.Errorf("marshal IAM role menu audit: %w", err)
			}
			role.Update(ctx.UserName())
			if _, err := tx.Save(&role); err != nil {
				return fmt.Errorf("update IAM role menu version: %w", err)
			}
			if err := insertAudit(ctx, tx, audit); err != nil {
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

func (r *Repository) ListMenus(ctx *servicecontext.Context) ([]iam.Menu, error) {
	var menus []iam.Menu
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.OrderBy("parent_id NULLS FIRST, sort_order ASC, id ASC").Find(&menus); err != nil {
		return nil, fmt.Errorf("list IAM menus: %w", err)
	}
	return menus, nil
}

func (r *Repository) CreateMenu(ctx *servicecontext.Context, menu *iam.Menu, audit *iam.AuditLog) error {
	if menu == nil {
		return errors.New("IAM menu is required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := validateMenuParent(s, menu); err != nil {
			return err
		}
		menu.CreateNew(ctx.UserName())
		if _, err := tx.Save(menu); err != nil {
			if errors.Is(menuWriteError(err), ErrMenuCodeExists) {
				return ErrMenuCodeExists
			}
			return fmt.Errorf("insert IAM menu: %w", err)
		}
		var admins []iam.Role
		if err := s.Where("role_code=? AND status='active'", platformAdminRole).Find(&admins); err != nil {
			return fmt.Errorf("find platform administrator role for new menu: %w", err)
		}
		for _, admin := range admins {
			var existing iam.RoleMenu
			found, err := s.Where("role_id=? AND menu_id=?", admin.ID, menu.ID).Get(&existing)
			if err != nil {
				return fmt.Errorf("check platform administrator menu assignment: %w", err)
			}
			if found {
				continue
			}
			link := &iam.RoleMenu{RoleID: admin.ID, MenuID: menu.ID}
			link.CreateNew(ctx.UserName())
			if _, err := tx.Save(link); err != nil {
				return fmt.Errorf("assign new menu to platform administrator: %w", err)
			}
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", menu.ID))
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM menu creation audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) UpdateMenu(ctx *servicecontext.Context, menu *iam.Menu, expectedVersion int64, audit *iam.AuditLog) error {
	if menu == nil || menu.ID <= 0 || expectedVersion <= 0 {
		return errors.New("IAM menu and expected version are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		current.ParentID, current.MenuName, current.MenuCode = menu.ParentID, menu.MenuName, menu.MenuCode
		current.Path, current.Component, current.Icon = menu.Path, menu.Component, menu.Icon
		current.SortOrder, current.MenuType, current.Visible, current.Status = menu.SortOrder, menu.MenuType, menu.Visible, menu.Status
		current.Update(ctx.UserName())
		current.Cols("parent_id", "menu_name", "menu_code", "path", "component", "icon", "sort_order", "menu_type", "visible", "status")
		if _, err := tx.Save(current); err != nil {
			if errors.Is(err, database.ErrVersionConflict) {
				return ErrMenuVersionConflict
			}
			if errors.Is(menuWriteError(err), ErrMenuCodeExists) {
				return ErrMenuCodeExists
			}
			return fmt.Errorf("update IAM menu: %w", err)
		}
		*menu = *current
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", menu.ID))
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM menu update audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) DeleteMenu(ctx *servicecontext.Context, menuID int64, audit *iam.AuditLog) error {
	if menuID <= 0 {
		return errors.New("positive IAM menu ID is required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		menu.HardDelete()
		if _, err := tx.Save(menu); err != nil {
			return fmt.Errorf("delete IAM menu: %w", err)
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", menuID))
			changes, err := json.Marshal(map[string]string{"menu_code": menu.MenuCode, "menu_name": menu.MenuName})
			if err != nil {
				return fmt.Errorf("encode IAM menu deletion audit: %w", err)
			}
			audit.ChangesJSON = changes
			if err := insertAudit(ctx, tx, audit); err != nil {
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

func (r *Repository) GrantRole(ctx *servicecontext.Context, actorID, userID, roleID int64, audit *iam.AuditLog) error {
	if actorID <= 0 || userID <= 0 || roleID <= 0 {
		return errors.New("active actor, user, and role are required")
	}
	if actorID == userID {
		return errors.New("an operator cannot grant a role to themselves")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		var assignment iam.UserRole
		found, err := s.Where("user_id=? AND role_id=?", userID, roleID).Get(&assignment)
		if err != nil {
			return fmt.Errorf("check IAM role assignment: %w", err)
		}
		if !found {
			assignment = iam.UserRole{UserID: userID, RoleID: roleID}
			assignment.CreateNew(ctx.UserName())
			if _, err := tx.Save(&assignment); err != nil {
				return fmt.Errorf("grant IAM role: %w", err)
			}
		}
		if audit != nil {
			audit.UserID = int64Ptr(userID)
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM role grant audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) RevokeRole(ctx *servicecontext.Context, actorID, userID, roleID int64, audit *iam.AuditLog) error {
	if actorID <= 0 || userID <= 0 || roleID <= 0 || actorID == userID {
		return errors.New("active operator, target user, and role are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		var assignment iam.UserRole
		found, err = s.Where("user_id=? AND role_id=?", userID, roleID).ForUpdate().Get(&assignment)
		if err != nil {
			return fmt.Errorf("load IAM role assignment for revocation: %w", err)
		}
		if found {
			assignment.HardDelete()
			if _, err := tx.Save(&assignment); err != nil {
				return fmt.Errorf("revoke IAM role: %w", err)
			}
		}
		if found && audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d:%d", userID, roleID))
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM role revoke audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ListPermissionCatalog(ctx *servicecontext.Context) ([]iam.Permission, error) {
	var permissions []iam.Permission
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := "SELECT p.* FROM " + iam.TablePermission + " p JOIN " + iam.TableResource + " res ON res.id=p.resource_id WHERE p.status='active' AND res.status='active' ORDER BY res.resource_code, p.action_code"
	if err := s.SQL(query).Find(&permissions); err != nil {
		return nil, fmt.Errorf("list active IAM permission catalog: %w", err)
	}
	if err := enrichPermissionResources(s, permissions); err != nil {
		return nil, fmt.Errorf("load resource details for permission catalog: %w", err)
	}
	return permissions, nil
}

func (r *Repository) ListResourceCatalog(ctx *servicecontext.Context) ([]iam.Resource, error) {
	var resources []iam.Resource
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.Table(iam.TableResource).OrderBy("resource_code ASC").Find(&resources); err != nil {
		return nil, fmt.Errorf("list IAM resource catalog: %w", err)
	}
	return resources, nil
}

func enrichPermissionResources(s *xorm.Session, permissions []iam.Permission) error {
	if len(permissions) == 0 {
		return nil
	}
	var resources []iam.Resource
	if err := s.Table(iam.TableResource).Cols("id", "resource_name", "resource_code", "resource_type").Find(&resources); err != nil {
		return err
	}
	byID := make(map[int64]iam.Resource, len(resources))
	for _, resource := range resources {
		byID[resource.ID] = resource
	}
	for index := range permissions {
		if resource, ok := byID[permissions[index].ResourceID]; ok {
			permissions[index].ResourceName = resource.ResourceName
			permissions[index].ResourceCode = resource.ResourceCode
			permissions[index].ResourceType = resource.ResourceType
		}
	}
	return nil
}

func (r *Repository) GrantPermission(ctx *servicecontext.Context, actorID, roleID, permissionID int64, audit *iam.AuditLog) error {
	if actorID <= 0 || roleID <= 0 || permissionID <= 0 {
		return errors.New("active actor, role, and permission are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		var assignment iam.RolePermission
		found, err := s.Where("role_id=? AND permission_id=?", roleID, permissionID).Get(&assignment)
		if err != nil {
			return fmt.Errorf("check IAM permission assignment: %w", err)
		}
		if !found {
			assignment = iam.RolePermission{RoleID: roleID, PermissionID: permissionID}
			assignment.CreateNew(ctx.UserName())
			if _, err := tx.Save(&assignment); err != nil {
				return fmt.Errorf("grant IAM permission: %w", err)
			}
		}
		if audit != nil {
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM permission grant audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) RevokePermission(ctx *servicecontext.Context, actorID, roleID, permissionID int64, audit *iam.AuditLog) error {
	if actorID <= 0 || roleID <= 0 || permissionID <= 0 {
		return errors.New("active operator, role, and permission are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		var assignment iam.RolePermission
		found, err = s.Where("role_id=? AND permission_id=?", roleID, permissionID).ForUpdate().Get(&assignment)
		if err != nil {
			return fmt.Errorf("load IAM permission assignment for revocation: %w", err)
		}
		if found {
			assignment.HardDelete()
			if _, err := tx.Save(&assignment); err != nil {
				return fmt.Errorf("revoke IAM permission: %w", err)
			}
		}
		if found && audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d:%d", roleID, permissionID))
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM permission revoke audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) SetUserStatus(ctx *servicecontext.Context, actorID, userID int64, status string, audit *iam.AuditLog) error {
	if actorID <= 0 || userID <= 0 || actorID == userID || (status != "active" && status != "disabled") {
		return errors.New("valid operator, target user, and status are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		var target iam.User
		found, err := s.Where("id=?", userID).ForUpdate().Get(&target)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("IAM user not found")
		}
		if target.UserStatus == status {
			return nil
		}
		target.UserStatus = status
		target.Update(ctx.UserName())
		target.Cols("user_status")
		if _, err := tx.Save(&target); err != nil {
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
			if err := revokeUserSessions(ctx, tx, s, userID, "user_disabled"); err != nil {
				return fmt.Errorf("revoke disabled user's sessions: %w", err)
			}
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", userID))
			audit.ChangesJSON = []byte(fmt.Sprintf(`{"user_status":{"from":%q,"to":%q}}`, target.UserStatus, status))
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM user status audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ResetUserPassword(ctx *servicecontext.Context, actorID, userID int64, salt, hash string, version int16, audit *iam.AuditLog) error {
	if actorID <= 0 || userID <= 0 || actorID == userID || salt == "" || hash == "" || version <= 0 {
		return errors.New("valid operator, target user, and password hash are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		var target iam.User
		found, err := s.Where("id=?", userID).ForUpdate().Get(&target)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("IAM user not found")
		}
		target.PasswordSalt, target.PasswordHash, target.PasswordHashVersion = &salt, &hash, &version
		if target.AuthSource == "external" {
			target.AuthSource = "mixed"
		}
		target.MustChangePassword = true
		target.Update(ctx.UserName())
		target.Cols("password_salt", "password_hash", "password_hash_version", "auth_source", "must_change_password")
		if _, err := tx.Save(&target); err != nil {
			return fmt.Errorf("reset IAM user password: %w", err)
		}
		if err := revokeUserSessions(ctx, tx, s, userID, "password_reset"); err != nil {
			return fmt.Errorf("revoke sessions after IAM password reset: %w", err)
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", userID))
			audit.ChangesJSON = []byte(`{"password":"reset","must_change_password":true}`)
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM password reset audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ChangeOwnPassword(ctx *servicecontext.Context, userID int64, expectedHash, salt, hash string, version int16, currentSessionID string, audit *iam.AuditLog) error {
	if userID <= 0 || expectedHash == "" || salt == "" || hash == "" || version <= 0 || strings.TrimSpace(currentSessionID) == "" {
		return errors.New("active user, current session, and password hashes are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		var user iam.User
		found, err := s.SQL("SELECT * FROM "+iam.TableUser+" WHERE id=? AND user_status='active' AND password_hash=? FOR UPDATE", userID, expectedHash).Get(&user)
		if err != nil {
			return fmt.Errorf("change IAM user password: %w", err)
		}
		if !found {
			return errors.New("IAM password changed concurrently or user is inactive")
		}
		user.PasswordSalt, user.PasswordHash, user.PasswordHashVersion = &salt, &hash, &version
		user.MustChangePassword = false
		user.Update(ctx.UserName())
		user.Cols("password_salt", "password_hash", "password_hash_version", "must_change_password")
		user.AddSaveCondition("user_status=?", "active")
		user.AddSaveCondition("password_hash=?", expectedHash)
		if _, err := tx.Save(&user); err != nil {
			return fmt.Errorf("save IAM password change: %w", err)
		}
		if err := revokeUserSessions(ctx, tx, s, userID, "password_changed", currentSessionID); err != nil {
			return fmt.Errorf("revoke other sessions after IAM password change: %w", err)
		}
		if audit != nil {
			audit.TargetID = stringPtr(fmt.Sprintf("%d", userID))
			audit.ChangesJSON = []byte(`{"password":"changed","must_change_password":false}`)
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM password change audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) FindTOTPAuthenticator(ctx *servicecontext.Context, userID int64) (*iam.UserAuthenticator, error) {
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

func (r *Repository) SavePendingTOTP(ctx *servicecontext.Context, row *iam.UserAuthenticator, audit *iam.AuditLog) error {
	if row == nil || row.UserID <= 0 || row.SecretCiphertext == "" || row.EncryptionKeyVersion == "" {
		return errors.New("valid pending TOTP authenticator is required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		var user struct {
			Count int64 `xorm:"'count'"`
		}
		if _, err := s.SQL("SELECT COUNT(*) AS count FROM "+iam.TableUser+" WHERE id=? AND user_status='active'", row.UserID).Get(&user); err != nil {
			return err
		}
		if user.Count != 1 {
			return errors.New("active IAM user is required")
		}
		var current iam.UserAuthenticator
		found, err := s.Where("user_id=? AND authenticator_type='totp'", row.UserID).ForUpdate().Get(&current)
		if err != nil {
			return fmt.Errorf("load existing IAM TOTP authenticator: %w", err)
		}
		if found {
			if current.Status != "pending" && current.Status != "disabled" && current.Status != "revoked" {
				return errors.New("an active TOTP authenticator already exists")
			}
			current.SecretCiphertext = row.SecretCiphertext
			current.EncryptionKeyVersion = row.EncryptionKeyVersion
			current.Status = "pending"
			current.VerifiedAt = nil
			current.LastUsedAt = nil
			current.LastUsedStep = 0
			current.Update(ctx.UserName())
			current.Cols("secret_ciphertext", "encryption_key_version", "status", "verified_at", "last_used_at", "last_used_step")
			current.AddSaveCondition("status IN ('pending','disabled','revoked')")
			if _, err := tx.Save(&current); err != nil {
				return fmt.Errorf("save pending IAM TOTP authenticator: %w", err)
			}
			*row = current
		} else {
			row.AuthenticatorType = "totp"
			row.Status = "pending"
			row.CreateNew(ctx.UserName())
			if _, err := tx.Save(row); err != nil {
				return fmt.Errorf("create pending IAM TOTP authenticator: %w", err)
			}
		}
		if audit != nil {
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM TOTP setup audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ActivateTOTP(ctx *servicecontext.Context, userID, step int64, audit *iam.AuditLog) error {
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		now := time.Now().UTC()
		var user struct {
			Count int64 `xorm:"'count'"`
		}
		if _, err := s.SQL("SELECT COUNT(*) AS count FROM "+iam.TableUser+" WHERE id=? AND user_status='active'", userID).Get(&user); err != nil {
			return err
		}
		if user.Count != 1 {
			return errors.New("active IAM user is required")
		}
		var authenticator iam.UserAuthenticator
		found, err := s.Where("user_id=? AND authenticator_type='totp' AND status='pending'", userID).ForUpdate().Get(&authenticator)
		if err != nil {
			return fmt.Errorf("load pending IAM TOTP authenticator: %w", err)
		}
		if !found {
			return errors.New("pending TOTP setup was not found")
		}
		authenticator.Status, authenticator.VerifiedAt = "active", &now
		authenticator.LastUsedAt, authenticator.LastUsedStep = &now, step
		authenticator.Update(ctx.UserName())
		authenticator.Cols("status", "verified_at", "last_used_at", "last_used_step")
		authenticator.AddSaveCondition("status=?", "pending")
		if _, err := tx.Save(&authenticator); err != nil {
			return fmt.Errorf("activate IAM TOTP authenticator: %w", err)
		}
		if audit != nil {
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM TOTP activation audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ConsumeTOTP(ctx *servicecontext.Context, userID, step int64) error {
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		now := time.Now().UTC()
		var authenticator iam.UserAuthenticator
		found, err := tx.Where("user_id=? AND authenticator_type='totp' AND status='active' AND last_used_step<?", userID, step).ForUpdate().Get(&authenticator)
		if err != nil {
			return fmt.Errorf("load IAM TOTP authenticator for code consumption: %w", err)
		}
		if !found {
			return errors.New("TOTP code is invalid or already used")
		}
		authenticator.LastUsedStep, authenticator.LastUsedAt = step, &now
		authenticator.Update(ctx.UserName())
		authenticator.Cols("last_used_step", "last_used_at")
		authenticator.AddSaveCondition("status=?", "active")
		authenticator.AddSaveCondition("last_used_step<?", step)
		if _, err := tx.Save(&authenticator); err != nil {
			return fmt.Errorf("consume IAM TOTP code: %w", err)
		}
		return nil
	})
}

func (r *Repository) DisableTOTP(ctx *servicecontext.Context, userID, step int64, audit *iam.AuditLog) error {
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		var user struct {
			Count int64 `xorm:"'count'"`
		}
		if _, err := s.SQL("SELECT COUNT(*) AS count FROM "+iam.TableUser+" WHERE id=? AND user_status='active'", userID).Get(&user); err != nil {
			return err
		}
		if user.Count != 1 {
			return errors.New("active IAM user is required")
		}
		var authenticator iam.UserAuthenticator
		found, err := s.Where("user_id=? AND authenticator_type='totp' AND status='active' AND last_used_step<?", userID, step).ForUpdate().Get(&authenticator)
		if err != nil {
			return fmt.Errorf("disable IAM TOTP authenticator: %w", err)
		}
		if !found {
			return errors.New("active TOTP authenticator was not found")
		}
		now := time.Now().UTC()
		authenticator.Status, authenticator.LastUsedStep, authenticator.LastUsedAt = "disabled", step, &now
		authenticator.Update(ctx.UserName())
		authenticator.Cols("status", "last_used_step", "last_used_at")
		authenticator.AddSaveCondition("status=?", "active")
		authenticator.AddSaveCondition("last_used_step<?", step)
		if _, err := tx.Save(&authenticator); err != nil {
			return fmt.Errorf("disable IAM TOTP authenticator: %w", err)
		}
		if audit != nil {
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write IAM TOTP disable audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) WriteAudit(ctx *servicecontext.Context, audit *iam.AuditLog) error {
	if audit == nil {
		return errors.New("IAM audit record is required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		return insertAudit(ctx, tx, audit)
	})
}

func (r *Repository) BootstrapAdmin(ctx *servicecontext.Context, user *iam.User, audit *iam.AuditLog) error {
	if user == nil || audit == nil {
		return errors.New("bootstrap user and audit record are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
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
		var role iam.Role
		found, err := s.Where("role_code=?", platformAdminRole).Get(&role)
		if err != nil {
			return err
		}
		if !found {
			role = iam.Role{RoleName: "Platform Administrator", RoleCode: platformAdminRole, IsSystem: true, Status: "active"}
			role.CreateNew(ctx.UserName())
			if _, err := tx.Save(&role); err != nil {
				return fmt.Errorf("create platform administrator role: %w", err)
			}
		}
		var menus []iam.Menu
		if err := s.Where("status='active'").Find(&menus); err != nil {
			return fmt.Errorf("load active menus for platform administrator: %w", err)
		}
		for _, menu := range menus {
			var assignment iam.RoleMenu
			found, err := s.Where("role_id=? AND menu_id=?", role.ID, menu.ID).Get(&assignment)
			if err != nil {
				return fmt.Errorf("check administrator menu assignment: %w", err)
			}
			if found {
				continue
			}
			assignment = iam.RoleMenu{RoleID: role.ID, MenuID: menu.ID}
			assignment.CreateNew(ctx.UserName())
			if _, err := tx.Save(&assignment); err != nil {
				return fmt.Errorf("assign active menu to platform administrator role: %w", err)
			}
		}
		user.UserStatus, user.AuthSource = "active", "local"
		if err := insertUser(ctx, tx, user); err != nil {
			return fmt.Errorf("create bootstrap user: %w", err)
		}
		assignment := &iam.UserRole{UserID: user.ID, RoleID: role.ID}
		assignment.CreateNew(ctx.UserName())
		if _, err := tx.Save(assignment); err != nil {
			return fmt.Errorf("assign platform administrator role: %w", err)
		}
		for _, seed := range bootstrapPermissions {
			resource := &iam.Resource{ResourceName: seed.resource, ResourceCode: seed.resource, ResourceType: "management", Status: "active"}
			found, err := s.Where("resource_code=?", seed.resource).Get(resource)
			if err != nil {
				return err
			}
			if !found {
				resource.CreateNew(ctx.UserName())
				if _, err := tx.Save(resource); err != nil {
					return fmt.Errorf("create IAM resource seed: %w", err)
				}
			}
			permission := &iam.Permission{PermissionName: seed.name, PermissionCode: seed.code, ResourceID: resource.ID, ActionCode: seed.action, Status: "active"}
			found, err = s.Where("permission_code=?", seed.code).Get(permission)
			if err != nil {
				return err
			}
			if !found {
				permission.CreateNew(ctx.UserName())
				if _, err := tx.Save(permission); err != nil {
					return fmt.Errorf("create IAM permission seed: %w", err)
				}
			}
			var assignment iam.RolePermission
			found, err = s.Where("role_id=? AND permission_id=?", role.ID, permission.ID).Get(&assignment)
			if err != nil {
				return fmt.Errorf("check IAM permission seed assignment: %w", err)
			}
			if !found {
				assignment = iam.RolePermission{RoleID: role.ID, PermissionID: permission.ID}
				assignment.CreateNew(ctx.UserName())
				if _, err := tx.Save(&assignment); err != nil {
					return fmt.Errorf("assign IAM permission seed: %w", err)
				}
			}
		}
		audit.UserID = int64Ptr(user.ID)
		if err := insertAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write bootstrap audit: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListWebAuthnCredentials(ctx *servicecontext.Context, userID int64, rpID string) ([]iam.WebAuthnCredential, error) {
	var credentials []iam.WebAuthnCredential
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	err := s.Where("user_id=? AND rp_id=? AND status='active'", userID, rpID).OrderBy("id ASC").Find(&credentials)
	if err != nil {
		return nil, fmt.Errorf("list active WebAuthn credentials: %w", err)
	}
	return credentials, nil
}

func (r *Repository) FindUserByCredential(ctx *servicecontext.Context, rpID string, credentialID []byte) (*iam.User, []iam.WebAuthnCredential, error) {
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

func (r *Repository) CreateAuthChallenge(ctx *servicecontext.Context, challenge *iam.AuthChallenge) error {
	if challenge == nil || challenge.ChallengeID == "" || challenge.ChallengeHash == "" || challenge.ChallengeType == "" {
		return errors.New("complete WebAuthn challenge is required")
	}
	if challenge.Status == "" {
		challenge.Status = "pending"
	}
	if challenge.MaxAttempts == 0 {
		challenge.MaxAttempts = 1
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		challenge.CreateNew(ctx.UserName())
		if _, err := tx.Save(challenge); err != nil {
			return fmt.Errorf("save WebAuthn challenge: %w", err)
		}
		return nil
	})
}

func (r *Repository) ConsumeAuthChallenge(ctx *servicecontext.Context, challengeID, challengeHash string) (*iam.AuthChallenge, error) {
	var challenge *iam.AuthChallenge
	err := r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		now := time.Now().UTC()
		challenge = new(iam.AuthChallenge)
		found, err := tx.Where("challenge_id=? AND challenge_hash=? AND status='pending' AND expires_at>? AND attempt_count<max_attempts", challengeID, challengeHash, now).ForUpdate().Get(challenge)
		if err != nil {
			return fmt.Errorf("load WebAuthn challenge: %w", err)
		}
		if !found {
			challenge = nil
			return nil
		}
		challenge.Status, challenge.AttemptCount, challenge.ConsumedAt = "consumed", challenge.AttemptCount+1, &now
		challenge.Update(ctx.UserName())
		challenge.Cols("status", "attempt_count", "consumed_at")
		challenge.AddSaveCondition("status=?", "pending")
		challenge.AddSaveCondition("expires_at>?", now)
		challenge.AddSaveCondition("attempt_count<max_attempts")
		if _, err := tx.Save(challenge); err != nil {
			return fmt.Errorf("consume WebAuthn challenge: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return challenge, nil
}

func (r *Repository) CreateWebAuthnCredential(ctx *servicecontext.Context, credential *iam.WebAuthnCredential, audit *iam.AuditLog) error {
	if credential == nil || len(credential.CredentialID) == 0 || len(credential.PublicKeyCOSE) == 0 {
		return errors.New("complete WebAuthn credential is required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		if credential.Status == "" {
			credential.Status = "active"
		}
		if credential.Transports == "" {
			credential.Transports = "[]"
		}
		credential.CreateNew(ctx.UserName())
		if _, err := tx.Save(credential); err != nil {
			return fmt.Errorf("save WebAuthn credential: %w", err)
		}
		if audit != nil {
			audit.UserID = int64Ptr(credential.UserID)
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write WebAuthn registration audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) UpdateWebAuthnCredentialUse(ctx *servicecontext.Context, credential *iam.WebAuthnCredential) error {
	if credential == nil || credential.ID <= 0 {
		return errors.New("WebAuthn credential ID is required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		now := time.Now().UTC()
		var current iam.WebAuthnCredential
		found, err := tx.Where("id=? AND status='active' AND (signature_count=0 OR signature_count<?)", credential.ID, credential.SignatureCount).ForUpdate().Get(&current)
		if err != nil {
			return fmt.Errorf("load WebAuthn credential usage: %w", err)
		}
		if !found {
			return errors.New("WebAuthn credential counter did not advance or credential was revoked")
		}
		current.SignatureCount, current.BackupState, current.LastUsedAt = credential.SignatureCount, credential.BackupState, &now
		current.Update(ctx.UserName())
		current.Cols("signature_count", "backup_state", "last_used_at")
		current.AddSaveCondition("status=?", "active")
		current.AddSaveCondition("signature_count=0 OR signature_count<?", credential.SignatureCount)
		if _, err := tx.Save(&current); err != nil {
			return fmt.Errorf("update WebAuthn credential usage: %w", err)
		}
		return nil
	})
}

func (r *Repository) RevokeWebAuthnCredential(ctx *servicecontext.Context, userID, credentialID int64, audit *iam.AuditLog) error {
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		now := time.Now().UTC()
		var credential iam.WebAuthnCredential
		found, err := s.Where("user_id=? AND id=? AND status='active'", userID, credentialID).ForUpdate().Get(&credential)
		if err != nil {
			return fmt.Errorf("revoke WebAuthn credential: %w", err)
		}
		if !found {
			return errors.New("active WebAuthn credential not found")
		}
		reason := "user_request"
		credential.Status, credential.RevokedAt, credential.RevocationReason = "revoked", &now, &reason
		credential.Update(ctx.UserName())
		credential.Cols("status", "revoked_at", "revocation_reason")
		credential.AddSaveCondition("status=?", "active")
		if _, err := tx.Save(&credential); err != nil {
			return fmt.Errorf("revoke WebAuthn credential: %w", err)
		}
		if audit != nil {
			audit.UserID = int64Ptr(userID)
			if err := insertAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write WebAuthn revocation audit: %w", err)
			}
		}
		return nil
	})
}

func insertAudit(ctx *servicecontext.Context, tx *database.DBTransaction, audit *iam.AuditLog) error {
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
	if audit.State() != database.ModelState_New {
		audit.CreateNew(ctx.UserName())
	}
	if _, err := tx.Save(audit); err != nil {
		return err
	}
	return nil
}

func revokeUserSessions(ctx *servicecontext.Context, tx *database.DBTransaction, s *xorm.Session, userID int64, reason string, exceptSessionID ...string) error {
	query := "user_id=? AND status='active'"
	args := []any{userID}
	if len(exceptSessionID) > 0 && exceptSessionID[0] != "" {
		query += " AND session_id<>?"
		args = append(args, exceptSessionID[0])
	}
	var sessions []iam.UserSession
	if err := s.Where(query, args...).ForUpdate().Find(&sessions); err != nil {
		return fmt.Errorf("load active IAM sessions for revocation: %w", err)
	}
	now := time.Now().UTC()
	for i := range sessions {
		sessions[i].Status = "revoked"
		sessions[i].RevokedAt = &now
		sessions[i].RevocationReason = &reason
		sessions[i].Update(ctx.UserName())
		sessions[i].Cols("status", "revoked_at", "revocation_reason")
		sessions[i].AddSaveCondition("status=?", "active")
		if _, err := tx.Save(&sessions[i]); err != nil {
			return fmt.Errorf("save revoked IAM session: %w", err)
		}
	}
	return nil
}

func nullableStringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func int64Ptr(value int64) *int64    { return &value }
func stringPtr(value string) *string { return &value }

var _ Store = (*Repository)(nil)
