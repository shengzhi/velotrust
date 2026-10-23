// Package iam adapts IAM application services to the Management JRPC contract.
package iam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"velotrust/management/internal/middleware"
	model "velotrust/management/internal/model/iam"
	appIAM "velotrust/management/internal/service/iam"
	"velotrust/pkg/jrpc"
)

type Handler struct {
	service       *appIAM.Service
	methodCatalog []MethodCatalogItem
}

func New(service *appIAM.Service) (*Handler, error) {
	if service == nil {
		return nil, errors.New("IAM service is required")
	}
	return &Handler{service: service}, nil
}

var methodPolicies = map[string]middleware.IAMMethodPolicy{
	"Login":                     {Public: true},
	"BeginPasskeyLogin":         {Public: true},
	"FinishPasskeyLogin":        {Public: true},
	"Logout":                    {Authenticated: true},
	"ChangePassword":            {Authenticated: true},
	"GetCurrentUser":            {Authenticated: true},
	"BeginPasskeyRegistration":  {Authenticated: true},
	"FinishPasskeyRegistration": {Authenticated: true},
	"RevokePasskey":             {Authenticated: true},
	"SetupTOTP":                 {Authenticated: true},
	"ConfirmTOTP":               {Authenticated: true},
	"DisableTOTP":               {Authenticated: true},
	"ListUsers":                 {Authenticated: true, Permission: "iam.user.read"},
	"ListGroups":                {Authenticated: true, Permission: "iam.group.read"},
	"SaveGroup":                 {Authenticated: true, Permission: "iam.group.write"},
	"ListGroupMembers":          {Authenticated: true, Permission: "iam.group.read"},
	"SetGroupMembers":           {Authenticated: true, Permission: "iam.group.write"},
	"CreateUser":                {Authenticated: true, Permission: "iam.user.write"},
	"SetUserStatus":             {Authenticated: true, Permission: "iam.user.write"},
	"ResetUserPassword":         {Authenticated: true, Permission: "iam.user.write"},
	"ListRoles":                 {Authenticated: true, Permission: "iam.role.read"},
	"ListRoleMenus":             {Authenticated: true, Permission: "iam.role.read"},
	"SetRoleMenus":              {Authenticated: true, Permission: "iam.role.write"},
	"ListCurrentUserMenus":      {Authenticated: true},
	"ListUserRoles":             {Authenticated: true, Permission: "iam.user.read"},
	"ListRolePermissions":       {Authenticated: true, Permission: "iam.role.read"},
	"CreateRole":                {Authenticated: true, Permission: "iam.role.write"},
	"GrantRole":                 {Authenticated: true, Permission: "iam.role.grant"},
	"RevokeRole":                {Authenticated: true, Permission: "iam.role.grant"},
	"ListPermissionCatalog":     {Authenticated: true, Permission: "iam.role.read"},
	"ListResourceCatalog":       {Authenticated: true, Permission: "iam.role.read"},
	"ListMethodCatalog":         {Authenticated: true, Permission: "iam.role.read"},
	"GrantPermission":           {Authenticated: true, Permission: "iam.role.write"},
	"RevokePermission":          {Authenticated: true, Permission: "iam.role.write"},
	"ListMenus":                 {Authenticated: true, Permission: "iam.role.read"},
	"CreateMenu":                {Authenticated: true, Permission: "iam.role.write"},
	"UpdateMenu":                {Authenticated: true, Permission: "iam.role.write"},
	"DeleteMenu":                {Authenticated: true, Permission: "iam.role.write"},
}

func Register(server *jrpc.Server, service *appIAM.Service) error {
	handler, err := New(service)
	if err != nil {
		return err
	}
	if err := server.RegisterServiceWithOptions(handler, "IAM", jrpc.ServiceOptions{PublicMethods: []string{"Login", "BeginPasskeyLogin", "FinishPasskeyLogin"}}, middleware.RequireIAMMethodPolicy(service, methodPolicies)); err != nil {
		return err
	}
	catalog, err := buildMethodCatalog(server.Methods())
	if err != nil {
		return err
	}
	handler.methodCatalog = catalog
	return nil
}

func buildMethodCatalog(methods []jrpc.MethodDescriptor) ([]MethodCatalogItem, error) {
	catalog := make([]MethodCatalogItem, 0)
	for _, method := range methods {
		if method.Service != "IAM" {
			continue
		}
		policy, ok := methodPolicies[method.Method]
		if !ok {
			return nil, fmt.Errorf("IAM JRPC method %s has no access declaration", method.Method)
		}
		if err := validateMethodPolicy(method.Method, policy); err != nil {
			return nil, err
		}
		expectedAccess := jrpc.AccessAuthenticated
		if policy.Public {
			expectedAccess = jrpc.AccessPublic
		}
		if method.Access != expectedAccess {
			return nil, fmt.Errorf("IAM JRPC method %s transport access does not match its policy", method.Method)
		}
		catalog = append(catalog, MethodCatalogItem{
			MethodCode:     method.Service + "_" + method.Method,
			ServiceName:    method.Service,
			MethodName:     method.Method,
			PolicyType:     methodPolicyType(policy),
			PermissionCode: policy.Permission,
		})
	}
	return catalog, nil
}

func validateMethodPolicy(method string, policy middleware.IAMMethodPolicy) error {
	if policy.Public {
		if policy.Authenticated || policy.Permission != "" {
			return fmt.Errorf("IAM JRPC method %s has conflicting public and protected access declarations", method)
		}
		return nil
	}
	if !policy.Authenticated {
		return fmt.Errorf("IAM JRPC method %s is not explicitly public or authenticated", method)
	}
	return nil
}

func methodPolicyType(policy middleware.IAMMethodPolicy) string {
	if policy.Permission != "" {
		return "permission"
	}
	if policy.Public {
		return "public"
	}
	return "authenticated"
}

type LoginArgs struct {
	Username string `json:"username" validate:"required,max=64"`
	Password string `json:"password" validate:"required,max=1024"`
	TOTPCode string `json:"totp_code" validate:"omitempty,len=6,numeric"`
}

type LoginReply = appIAM.SessionGrant

type ChangePasswordArgs struct {
	CurrentPassword string `json:"current_password" validate:"required,max=1024"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=1024"`
}

func (h *Handler) ChangePassword(ctx *jrpc.Context, args *ChangePasswordArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("password change arguments are required")
	}
	if err := h.service.ChangeOwnPassword(middleware.ServiceContext(ctx), args.CurrentPassword, args.NewPassword); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

func (h *Handler) Login(ctx *jrpc.Context, args *LoginArgs) (*LoginReply, error) {
	if args == nil {
		return nil, errors.New("login arguments are required")
	}
	var sourceIP, userAgent, requestID string
	if ctx != nil {
		sourceIP, requestID = ctx.ClientIP(), ctx.RequestID()
		if ctx.HttpRequest != nil {
			userAgent = ctx.HttpRequest.UserAgent()
		}
	}
	return h.service.LoginWithTOTP(middleware.AuthenticationContext(ctx), args.Username, args.Password, args.TOTPCode, sourceIP, userAgent, requestID)
}

type SetupTOTPArgs struct {
	Password string `json:"password" validate:"required,max=1024"`
}

func (h *Handler) SetupTOTP(ctx *jrpc.Context, args *SetupTOTPArgs) (*appIAM.TOTPSetup, error) {
	if args == nil {
		return nil, errors.New("password reauthentication is required")
	}
	return h.service.SetupTOTP(middleware.ServiceContext(ctx), args.Password)
}

type TOTPCodeArgs struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

func (h *Handler) ConfirmTOTP(ctx *jrpc.Context, args *TOTPCodeArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("TOTP code is required")
	}
	if err := h.service.ConfirmTOTP(middleware.ServiceContext(ctx), args.Code); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type DisableTOTPArgs struct {
	Password string `json:"password" validate:"required,max=1024"`
	Code     string `json:"code" validate:"required,len=6,numeric"`
}

func (h *Handler) DisableTOTP(ctx *jrpc.Context, args *DisableTOTPArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("password and TOTP code are required")
	}
	if err := h.service.DisableTOTP(middleware.ServiceContext(ctx), args.Password, args.Code); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type BeginPasskeyLoginReply struct {
	Ceremony *appIAM.PasskeyLoginStart `json:"ceremony"`
}

func (h *Handler) BeginPasskeyLogin(ctx *jrpc.Context, _ *struct{}) (*BeginPasskeyLoginReply, error) {
	ceremony, err := h.service.BeginPasskeyLogin(middleware.AuthenticationContext(ctx), requestOrigin(ctx))
	if err != nil {
		return nil, err
	}
	return &BeginPasskeyLoginReply{Ceremony: ceremony}, nil
}

type FinishPasskeyLoginArgs struct {
	ChallengeID string          `json:"challenge_id" validate:"required,len=64"`
	Response    json.RawMessage `json:"response" validate:"required"`
}

func (h *Handler) FinishPasskeyLogin(ctx *jrpc.Context, args *FinishPasskeyLoginArgs) (*LoginReply, error) {
	if args == nil {
		return nil, errors.New("Passkey response is required")
	}
	var sourceIP, userAgent, requestID string
	if ctx != nil {
		sourceIP, requestID = ctx.ClientIP(), ctx.RequestID()
		if ctx.HttpRequest != nil {
			userAgent = ctx.HttpRequest.UserAgent()
		}
	}
	return h.service.FinishPasskeyLogin(middleware.AuthenticationContext(ctx), args.ChallengeID, requestOrigin(ctx), args.Response, sourceIP, userAgent, requestID)
}

type BeginPasskeyRegistrationReply struct {
	Ceremony *appIAM.PasskeyRegistrationStart `json:"ceremony"`
}

func (h *Handler) BeginPasskeyRegistration(ctx *jrpc.Context, _ *struct{}) (*BeginPasskeyRegistrationReply, error) {
	ceremony, err := h.service.BeginPasskeyRegistration(middleware.ServiceContext(ctx), requestOrigin(ctx))
	if err != nil {
		return nil, err
	}
	return &BeginPasskeyRegistrationReply{Ceremony: ceremony}, nil
}

type FinishPasskeyRegistrationArgs struct {
	ChallengeID    string          `json:"challenge_id" validate:"required,len=64"`
	CredentialName string          `json:"credential_name" validate:"max=128"`
	Response       json.RawMessage `json:"response" validate:"required"`
}

func (h *Handler) FinishPasskeyRegistration(ctx *jrpc.Context, args *FinishPasskeyRegistrationArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("Passkey response is required")
	}
	if err := h.service.FinishPasskeyRegistration(middleware.ServiceContext(ctx), args.ChallengeID, args.CredentialName, requestOrigin(ctx), args.Response); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type RevokePasskeyArgs struct {
	CredentialID int64 `json:"credential_id" validate:"required,gt=0"`
}

func (h *Handler) RevokePasskey(ctx *jrpc.Context, args *RevokePasskeyArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("credential ID is required")
	}
	if err := h.service.RevokePasskey(middleware.ServiceContext(ctx), args.CredentialID); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type LogoutReply struct {
	Success bool `json:"success"`
}

func (h *Handler) Logout(ctx *jrpc.Context, _ *struct{}) (*LogoutReply, error) {
	if ctx == nil || ctx.Identity == nil {
		return nil, errors.New("IAM session is required")
	}
	if err := h.service.Logout(middleware.ServiceContext(ctx), ctx.Identity.EntityID()); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type CurrentUserReply struct {
	User *appIAM.UserView `json:"user"`
}

func (h *Handler) GetCurrentUser(ctx *jrpc.Context, _ *struct{}) (*CurrentUserReply, error) {
	user, err := h.service.CurrentUser(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &CurrentUserReply{User: user}, nil
}

type ListUsersArgs struct {
	Page        int    `json:"page" validate:"omitempty,gte=1"`
	PageSize    int    `json:"page_size" validate:"omitempty,gte=1,lte=100"`
	AccountType string `json:"account_type" validate:"omitempty,oneof=platform employee"`
}

type ListUsersReply struct {
	Items []appIAM.UserView `json:"items"`
	Total int64             `json:"total"`
}

type ListGroupsReply struct {
	Items []model.Group `json:"items"`
}

func (h *Handler) ListGroups(ctx *jrpc.Context, _ *struct{}) (*ListGroupsReply, error) {
	items, err := h.service.ListGroups(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListGroupsReply{Items: items}, nil
}

type SaveGroupArgs struct {
	ID              int64  `json:"id" validate:"omitempty,gt=0"`
	ExpectedVersion int64  `json:"expected_version" validate:"omitempty,gt=0"`
	GroupName       string `json:"group_name" validate:"required,max=128"`
	GroupCode       string `json:"group_code" validate:"required,max=64"`
	Description     string `json:"description" validate:"omitempty,max=512"`
	Status          string `json:"status" validate:"required,oneof=active disabled"`
	SortOrder       int    `json:"sort_order" validate:"gte=0"`
}

func (h *Handler) SaveGroup(ctx *jrpc.Context, args *SaveGroupArgs) (*model.Group, error) {
	if args == nil {
		return nil, errors.New("IAM group arguments are required")
	}
	return h.service.SaveGroup(middleware.ServiceContext(ctx), appIAM.GroupInput{
		ID: args.ID, VersionNo: args.ExpectedVersion, GroupName: args.GroupName,
		GroupCode: args.GroupCode, Description: args.Description, Status: args.Status,
		SortOrder: args.SortOrder,
	})
}

type ListGroupMembersArgs struct {
	GroupID int64 `json:"group_id" validate:"required,gt=0"`
}

type ListGroupMembersReply struct {
	Items []model.GroupMember `json:"items"`
}

func (h *Handler) ListGroupMembers(ctx *jrpc.Context, args *ListGroupMembersArgs) (*ListGroupMembersReply, error) {
	if args == nil {
		return nil, errors.New("IAM group member query arguments are required")
	}
	items, err := h.service.ListGroupMembers(middleware.ServiceContext(ctx), args.GroupID)
	if err != nil {
		return nil, err
	}
	return &ListGroupMembersReply{Items: items}, nil
}

type SetGroupMembersArgs struct {
	GroupID int64   `json:"group_id" validate:"required,gt=0"`
	UserIDs []int64 `json:"user_ids" validate:"max=5000,dive,gt=0"`
}

func (h *Handler) SetGroupMembers(ctx *jrpc.Context, args *SetGroupMembersArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("IAM group membership arguments are required")
	}
	if err := h.service.SetGroupMembers(middleware.ServiceContext(ctx), args.GroupID, args.UserIDs); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

func (h *Handler) ListUsers(ctx *jrpc.Context, args *ListUsersArgs) (*ListUsersReply, error) {
	if args == nil {
		args = &ListUsersArgs{}
	}
	users, total, err := h.service.ListUsers(middleware.ServiceContext(ctx), args.Page, args.PageSize, args.AccountType)
	if err != nil {
		return nil, err
	}
	return &ListUsersReply{Items: users, Total: total}, nil
}

type CreateUserArgs struct {
	AccountType string `json:"account_type" validate:"required,oneof=platform employee"`
	Username    string `json:"username" validate:"required,max=64"`
	DisplayName string `json:"display_name" validate:"max=128"`
	Email       string `json:"email" validate:"omitempty,email,max=128"`
	Password    string `json:"password" validate:"omitempty,min=8,max=1024"`
}

type CreateUserReply struct {
	User *appIAM.UserView `json:"user"`
}

func (h *Handler) CreateUser(ctx *jrpc.Context, args *CreateUserArgs) (*CreateUserReply, error) {
	if args == nil {
		return nil, errors.New("create user arguments are required")
	}
	user, err := h.service.CreateUser(middleware.ServiceContext(ctx), appIAM.CreateUserInput{
		AccountType: args.AccountType, Username: args.Username, DisplayName: args.DisplayName, Email: args.Email, Password: args.Password,
	})
	if err != nil {
		return nil, err
	}
	return &CreateUserReply{User: user}, nil
}

type SetUserStatusArgs struct {
	UserID int64  `json:"user_id" validate:"required,gt=0"`
	Status string `json:"status" validate:"required,oneof=active disabled"`
}

func (h *Handler) SetUserStatus(ctx *jrpc.Context, args *SetUserStatusArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("user status arguments are required")
	}
	if err := h.service.SetUserStatus(middleware.ServiceContext(ctx), args.UserID, args.Status); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type ResetUserPasswordArgs struct {
	UserID      int64  `json:"user_id" validate:"required,gt=0"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=1024"`
}

func (h *Handler) ResetUserPassword(ctx *jrpc.Context, args *ResetUserPasswordArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("password reset arguments are required")
	}
	if err := h.service.ResetUserPassword(middleware.ServiceContext(ctx), args.UserID, args.NewPassword); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type CreateRoleArgs struct {
	RoleName    string `json:"role_name" validate:"required,max=64"`
	RoleCode    string `json:"role_code" validate:"required,max=64"`
	Description string `json:"description" validate:"max=512"`
}

type CreateRoleReply struct {
	Role *model.Role `json:"role"`
}

func (h *Handler) CreateRole(ctx *jrpc.Context, args *CreateRoleArgs) (*CreateRoleReply, error) {
	if args == nil {
		return nil, errors.New("create role arguments are required")
	}
	role, err := h.service.CreateRole(middleware.ServiceContext(ctx), appIAM.CreateRoleInput{
		RoleName: args.RoleName, RoleCode: args.RoleCode, Description: args.Description,
	})
	if err != nil {
		return nil, err
	}
	return &CreateRoleReply{Role: role}, nil
}

type ListRolesReply struct {
	Items []model.Role `json:"items"`
}

func (h *Handler) ListRoles(ctx *jrpc.Context, _ *struct{}) (*ListRolesReply, error) {
	roles, err := h.service.ListRoles(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListRolesReply{Items: roles}, nil
}

type RoleMenusArgs struct {
	RoleID  int64   `json:"role_id" validate:"required,gt=0"`
	MenuIDs []int64 `json:"menu_ids" validate:"max=1000,dive,gt=0"`
}

type ListRoleMenusArgs struct {
	RoleID int64 `json:"role_id" validate:"required,gt=0"`
}

type RoleMenusReply struct {
	Items []model.Menu `json:"items"`
}

func (h *Handler) ListRoleMenus(ctx *jrpc.Context, args *ListRoleMenusArgs) (*RoleMenusReply, error) {
	if args == nil || args.RoleID <= 0 {
		return nil, errors.New("role ID is required")
	}
	menus, err := h.service.ListRoleMenus(middleware.ServiceContext(ctx), args.RoleID)
	if err != nil {
		return nil, err
	}
	return &RoleMenusReply{Items: menus}, nil
}

func (h *Handler) SetRoleMenus(ctx *jrpc.Context, args *RoleMenusArgs) (*LogoutReply, error) {
	if args == nil || args.RoleID <= 0 {
		return nil, errors.New("role ID is required")
	}
	if err := h.service.SetRoleMenus(middleware.ServiceContext(ctx), args.RoleID, args.MenuIDs); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type CurrentUserMenusReply struct {
	Items []model.Menu `json:"items"`
}

func (h *Handler) ListCurrentUserMenus(ctx *jrpc.Context, _ *struct{}) (*CurrentUserMenusReply, error) {
	menus, err := h.service.ListCurrentUserMenus(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &CurrentUserMenusReply{Items: menus}, nil
}

type ListMenusReply struct {
	Items []model.Menu `json:"items"`
}

func (h *Handler) ListMenus(ctx *jrpc.Context, _ *struct{}) (*ListMenusReply, error) {
	menus, err := h.service.ListMenus(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListMenusReply{Items: menus}, nil
}

type MenuArgs struct {
	ID        int64  `json:"id" validate:"omitempty,gt=0"`
	ParentID  *int64 `json:"parent_id" validate:"omitempty,gt=0"`
	MenuName  string `json:"menu_name" validate:"required,max=128"`
	MenuCode  string `json:"menu_code" validate:"required,max=64"`
	Path      string `json:"path" validate:"max=255"`
	Component string `json:"component" validate:"max=255"`
	Icon      string `json:"icon" validate:"max=128"`
	SortOrder int    `json:"sort_order" validate:"gte=0,lte=1000000"`
	MenuType  string `json:"menu_type" validate:"required,oneof=directory page external button"`
	Visible   bool   `json:"visible"`
	Status    string `json:"status" validate:"required,oneof=active disabled"`
	VersionNo int64  `json:"version_no" validate:"omitempty,gt=0"`
}

func menuInput(args *MenuArgs) appIAM.MenuInput {
	return appIAM.MenuInput{
		ID: args.ID, ParentID: args.ParentID, MenuName: args.MenuName, MenuCode: args.MenuCode,
		Path: args.Path, Component: args.Component, Icon: args.Icon, SortOrder: args.SortOrder,
		MenuType: args.MenuType, Visible: args.Visible, Status: args.Status, VersionNo: args.VersionNo,
	}
}

type MenuReply struct {
	Menu *model.Menu `json:"menu"`
}

func (h *Handler) CreateMenu(ctx *jrpc.Context, args *MenuArgs) (*MenuReply, error) {
	if args == nil {
		return nil, errors.New("create menu arguments are required")
	}
	menu, err := h.service.CreateMenu(middleware.ServiceContext(ctx), menuInput(args))
	if err != nil {
		return nil, err
	}
	return &MenuReply{Menu: menu}, nil
}

func (h *Handler) UpdateMenu(ctx *jrpc.Context, args *MenuArgs) (*MenuReply, error) {
	if args == nil {
		return nil, errors.New("update menu arguments are required")
	}
	menu, err := h.service.UpdateMenu(middleware.ServiceContext(ctx), menuInput(args))
	if err != nil {
		return nil, err
	}
	return &MenuReply{Menu: menu}, nil
}

type DeleteMenuArgs struct {
	MenuID int64 `json:"menu_id" validate:"required,gt=0"`
}

func (h *Handler) DeleteMenu(ctx *jrpc.Context, args *DeleteMenuArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("menu ID is required")
	}
	if err := h.service.DeleteMenu(middleware.ServiceContext(ctx), args.MenuID); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type ListUserRolesArgs struct {
	UserID int64 `json:"user_id" validate:"required,gt=0"`
}

func (h *Handler) ListUserRoles(ctx *jrpc.Context, args *ListUserRolesArgs) (*ListRolesReply, error) {
	if args == nil {
		return nil, errors.New("user ID is required")
	}
	roles, err := h.service.ListUserRoles(middleware.ServiceContext(ctx), args.UserID)
	if err != nil {
		return nil, err
	}
	return &ListRolesReply{Items: roles}, nil
}

type ListPermissionCatalogReply struct {
	Items []model.Permission `json:"items"`
}

type ListResourceCatalogReply struct {
	Items []model.Resource `json:"items"`
}

type MethodCatalogItem struct {
	MethodCode     string `json:"method_code"`
	ServiceName    string `json:"service_name"`
	MethodName     string `json:"method_name"`
	PolicyType     string `json:"policy_type"`
	PermissionCode string `json:"permission_code,omitempty"`
}

type ListMethodCatalogReply struct {
	Items []MethodCatalogItem `json:"items"`
}

func (h *Handler) ListPermissionCatalog(ctx *jrpc.Context, _ *struct{}) (*ListPermissionCatalogReply, error) {
	permissions, err := h.service.ListPermissionCatalog(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListPermissionCatalogReply{Items: permissions}, nil
}

func (h *Handler) ListResourceCatalog(ctx *jrpc.Context, _ *struct{}) (*ListResourceCatalogReply, error) {
	resources, err := h.service.ListResourceCatalog(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListResourceCatalogReply{Items: resources}, nil
}

func (h *Handler) ListMethodCatalog(_ *jrpc.Context, _ *struct{}) (*ListMethodCatalogReply, error) {
	return &ListMethodCatalogReply{Items: append([]MethodCatalogItem(nil), h.methodCatalog...)}, nil
}

type ListRolePermissionsArgs struct {
	RoleID int64 `json:"role_id" validate:"required,gt=0"`
}

func (h *Handler) ListRolePermissions(ctx *jrpc.Context, args *ListRolePermissionsArgs) (*ListPermissionCatalogReply, error) {
	if args == nil {
		return nil, errors.New("role ID is required")
	}
	permissions, err := h.service.ListRolePermissions(middleware.ServiceContext(ctx), args.RoleID)
	if err != nil {
		return nil, err
	}
	return &ListPermissionCatalogReply{Items: permissions}, nil
}

type GrantPermissionArgs struct {
	RoleID       int64 `json:"role_id" validate:"required,gt=0"`
	PermissionID int64 `json:"permission_id" validate:"required,gt=0"`
}

func (h *Handler) GrantPermission(ctx *jrpc.Context, args *GrantPermissionArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("grant permission arguments are required")
	}
	if err := h.service.GrantPermission(middleware.ServiceContext(ctx), args.RoleID, args.PermissionID); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

func (h *Handler) RevokePermission(ctx *jrpc.Context, args *GrantPermissionArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("revoke permission arguments are required")
	}
	if err := h.service.RevokePermission(middleware.ServiceContext(ctx), args.RoleID, args.PermissionID); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

type GrantRoleArgs struct {
	UserID int64 `json:"user_id" validate:"required,gt=0"`
	RoleID int64 `json:"role_id" validate:"required,gt=0"`
}

func (h *Handler) GrantRole(ctx *jrpc.Context, args *GrantRoleArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("grant role arguments are required")
	}
	if err := h.service.GrantRole(middleware.ServiceContext(ctx), args.UserID, args.RoleID); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

func (h *Handler) RevokeRole(ctx *jrpc.Context, args *GrantRoleArgs) (*LogoutReply, error) {
	if args == nil {
		return nil, errors.New("revoke role arguments are required")
	}
	if err := h.service.RevokeRole(middleware.ServiceContext(ctx), args.UserID, args.RoleID); err != nil {
		return nil, err
	}
	return &LogoutReply{Success: true}, nil
}

func requestContext(ctx *jrpc.Context) context.Context {
	if ctx != nil && ctx.HttpRequest != nil {
		return ctx.HttpRequest.Context()
	}
	return context.Background()
}

func requestOrigin(ctx *jrpc.Context) string {
	if ctx == nil || ctx.HttpRequest == nil {
		return ""
	}
	return ctx.HttpRequest.Header.Get("Origin")
}
