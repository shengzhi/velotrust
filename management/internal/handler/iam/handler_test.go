package iam

import (
	"strings"
	"testing"

	repository "velotrust/management/internal/repository/iam"
	appIAM "velotrust/management/internal/service/iam"
	"velotrust/pkg/jrpc"
)

type registrationStore struct{ repository.Store }

func TestAllIAMJRPCMethodsHaveDeclaredAccessPolicies(t *testing.T) {
	service, err := appIAM.New(registrationStore{}, appIAM.Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	server := jrpc.NewServer()
	if err := Register(server, service); err != nil {
		t.Fatalf("register IAM methods and policies: %v", err)
	}
	methods := server.Methods()
	if len(methods) == 0 {
		t.Fatal("no IAM JRPC methods were registered")
	}
	for _, method := range methods {
		if method.Service == "IAM" {
			if _, ok := methodPolicies[method.Method]; !ok {
				t.Errorf("IAM method %s has no access policy", method.Method)
			}
		}
	}

	catalog, err := buildMethodCatalog(methods)
	if err != nil {
		t.Fatalf("build JRPC method catalog: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("JRPC method catalog is empty")
	}
	registeredIAMMethods := 0
	for _, method := range methods {
		if method.Service == "IAM" {
			registeredIAMMethods++
		}
	}
	if len(catalog) != registeredIAMMethods {
		t.Fatalf("catalog has %d entries for %d registered IAM methods", len(catalog), registeredIAMMethods)
	}
	byCode := make(map[string]MethodCatalogItem, len(catalog))
	for _, item := range catalog {
		byCode[item.MethodCode] = item
	}
	if item, ok := byCode["IAM_Login"]; !ok || item.PolicyType != "public" {
		t.Errorf("login method should be listed as public, got %+v, found=%v", item, ok)
	}
	if item, ok := byCode["IAM_ListUsers"]; !ok || item.PolicyType != "permission" || item.PermissionCode != "iam.user.read" {
		t.Errorf("user listing should map to iam.user.read, got %+v, found=%v", item, ok)
	}
	if item, ok := byCode["IAM_SetupTOTP"]; !ok || item.PolicyType != "authenticated" {
		t.Errorf("TOTP setup should require authentication, got %+v, found=%v", item, ok)
	}

	missingPolicy := append(append([]jrpc.MethodDescriptor(nil), methods...), jrpc.MethodDescriptor{Service: "IAM", Method: "Unmapped", Access: jrpc.AccessAuthenticated})
	if _, err := buildMethodCatalog(missingPolicy); err == nil {
		t.Fatal("method catalog accepted a JRPC method without an access declaration")
	}
}
