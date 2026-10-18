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
}
