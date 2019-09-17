package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

var (
	pathCustomSchemasPut  = fmt.Sprintf("/admin/directory/v1/customer/%s/schemas/%s", gSuiteCustomerId, gSuiteCustomSchemaKey)
	pathCustomSchemasPost = fmt.Sprintf("/admin/directory/v1/customer/%s/schemas", gSuiteCustomerId)
	pathMembers           = "/admin/directory/v1/groups/foobarbaz/members"
	pathUsers             = "/admin/directory/v1/users"

	responseBodyMembersGet = `{"members":[
	{"id":"012345678901234567890","email":"foo0-valid@bar.baz"},
	{"id":"112345678901234567890","email":"foo1-missing-pubkey@bar.baz"},
	{"id":"212345678901234567890","email":"foo2-missing-schema@bar.baz"},
	{"id":"312345678901234567890","email":"foo3-malformed-schema@bar.baz"}
]}`

	responseBodyUsersGet = `{"users":[
	{"id":"012345678901234567890","primaryEmail":"foo0-valid@bar.baz","customSchemas":{"wireguard":{"allowedIPs":[{"type":"work","value":"1.1.1.1/32"}],"publicKey":"NkEtSA6GosX40iZFNe9+byAkXweYKvQe3utnFYkQ+00="}}},
	{"id":"112345678901234567890","primaryEmail":"foo1-missing-pubkey@bar.baz","customSchemas":{"wireguard":{"allowedIPs":[{"type":"work","value":"1.1.1.1/32"}]}}},
	{"id":"212345678901234567890","primaryEmail":"foo2-missing-schema@bar.baz"},
	{"id":"312345678901234567890","primaryEmail":"foo3-malformed-schema@bar.baz","customSchemas":{"wireguard":{"publicKey": 0, "allowedIPs": 0}}},
	{"id":"412345678901234567890","primaryEmail":"foo4-not-a-member@bar.baz","customSchemas":{"wireguard":{"allowedIPs":[{"type":"work","value":"1.1.1.2/32"}],"publicKey":"fLEd048HsN8gtVNjQcoNPUXy2mYISEzSMcOR7YZr+Co="}}}
]}`

	responseBodyUserGet = `{"id":"012345678901234567890","primaryEmail":"foo0-valid@bar.baz","customSchemas":{"wireguard":{"allowedIPs":[{"type":"work","value":"1.1.1.1/32"}],"publicKey":"NkEtSA6GosX40iZFNe9+byAkXweYKvQe3utnFYkQ+00="}}}`
)

type fakeRoundTripFunc func(req *http.Request) *http.Response

func (f fakeRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func newFakeClient(fn fakeRoundTripFunc) *http.Client {
	return &http.Client{Transport: fakeRoundTripFunc(fn)}
}

func newFakeHTTPResponse(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: ioutil.NopCloser(bytes.NewBufferString(body))}
}

func TestEnsureGSuiteCustomSchema_Update(t *testing.T) {
	c := newFakeClient(fakeRoundTripFunc(func(req *http.Request) *http.Response {
		if req.Method == http.MethodPut && req.URL.Path == pathCustomSchemasPut {
			return newFakeHTTPResponse(200, `{}`)
		}
		return newFakeHTTPResponse(400, `{}`)
	}))
	svc, err := admin.NewService(context.Background(), option.WithHTTPClient(c))
	if err != nil {
		t.Errorf("TestEnsureGSuiteCustomSchema_Update: %v", err)
	}
	if err = ensureGSuiteCustomSchema(svc); err != nil {
		t.Errorf("ensureGSuiteCustomSchema: %v", err)
	}
}

func TestEnsureGSuiteCustomSchema_Insert(t *testing.T) {
	body, err := gSuiteCustomSchema.MarshalJSON()
	if err != nil {
		t.Fatalf("TestEnsureGSuiteCustomSchema_Insert: cannot marshal custom schema: %v", err)
	}
	c := newFakeClient(fakeRoundTripFunc(func(req *http.Request) *http.Response {
		if req.Method == http.MethodPut && req.URL.Path == pathCustomSchemasPut {
			return newFakeHTTPResponse(404, `{}`)
		}
		if req.Method == http.MethodPost && req.URL.Path == pathCustomSchemasPost {
			return newFakeHTTPResponse(200, string(body))
		}
		return newFakeHTTPResponse(200, `{}`)
	}))
	svc, err := admin.NewService(context.Background(), option.WithHTTPClient(c))
	if err != nil {
		t.Errorf("TestEnsureGSuiteCustomSchema_Insert: %v", err)
	}
	if err = ensureGSuiteCustomSchema(svc); err != nil {
		t.Errorf("ensureGSuiteCustomSchema: %v", err)
	}
}

func TestGetPeerConfigFromGsuiteGroup(t *testing.T) {
	c := newFakeClient(fakeRoundTripFunc(func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet && req.URL.Path == pathMembers {
			return newFakeHTTPResponse(200, responseBodyMembersGet)
		}
		if req.Method == http.MethodGet && req.URL.Path == pathUsers {
			return newFakeHTTPResponse(200, responseBodyUsersGet)
		}
		return newFakeHTTPResponse(400, `{}`)
	}))
	svc, err := admin.NewService(context.Background(), option.WithHTTPClient(c))
	if err != nil {
		t.Errorf("TestGetPeerConfigFromGsuiteGroup: %v", err)
	}
	peers, err := getPeerConfigFromGsuiteGroup(context.Background(), svc, "foobarbaz")
	if err != nil {
		t.Errorf("getPeerConfigFromGsuiteGroup: %v", err)
	}
	ep, _ := newPeerConfig(validPublicKey, "", "", validAllowedIPs)
	expected := []wgtypes.PeerConfig{*ep}
	if diff := cmp.Diff(expected, peers); diff != "" {
		t.Errorf("getPeerConfigFromGsuiteGroup: did not get expected result:\n%s", diff)
	}
}

func TestGetPeerConfigFromGsuiteUser(t *testing.T) {
	c := newFakeClient(fakeRoundTripFunc(func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet && req.URL.Path == path.Join(pathUsers, "foobarbaz") {
			return newFakeHTTPResponse(200, responseBodyUserGet)
		}
		return newFakeHTTPResponse(400, `{}`)
	}))
	svc, err := admin.NewService(context.Background(), option.WithHTTPClient(c))
	if err != nil {
		t.Errorf("TestGetPeerConfigFromGsuiteUser: %v", err)
	}
	peer, err := getPeerConfigFromGsuiteUser(svc, "foobarbaz")
	if err != nil {
		t.Errorf("getPeerConfigFromGsuiteUser: %v", err)
	}
	ep, _ := newPeerConfig(validPublicKey, "", "", validAllowedIPs)
	if diff := cmp.Diff(ep, peer); diff != "" {
		t.Errorf("getPeerConfigFromGsuiteUser: did not get expected result:\n%s", diff)
	}
}

func TestUpdatePeerConfigForGsuiteUser(t *testing.T) {
	c := newFakeClient(fakeRoundTripFunc(func(req *http.Request) *http.Response {
		if req.Method == http.MethodPut && req.URL.Path == path.Join(pathUsers, "foobarbaz") {
			defer func() {
				io.Copy(ioutil.Discard, req.Body)
				req.Body.Close()
			}()
			u := &admin.User{}
			json.NewDecoder(req.Body).Decode(u)
			resp, _ := u.MarshalJSON()
			return newFakeHTTPResponse(200, string(resp))
		}
		return newFakeHTTPResponse(400, `{}`)
	}))
	svc, err := admin.NewService(context.Background(), option.WithHTTPClient(c))
	if err != nil {
		t.Errorf("TestUpdatePeerConfigInGsuite: %v", err)
	}
	expected, _ := newPeerConfig(validPublicKey, "", "", validAllowedIPs)
	peer, err := updatePeerConfigForGsuiteUser(svc, "foobarbaz", expected)
	if err != nil {
		t.Errorf("updatePeerConfigInGsuite: %v", err)
	}
	if diff := cmp.Diff(expected, peer); diff != "" {
		t.Errorf("updatePeerConfigInGsuite: did not get expected result:\n%s", diff)
	}
}

func TestFindNextAvailablePeerAddress(t *testing.T) {
	c := newFakeClient(fakeRoundTripFunc(func(req *http.Request) *http.Response {
		if req.Method == http.MethodGet && req.URL.Path == pathUsers {
			return newFakeHTTPResponse(200, responseBodyUsersGet)
		}
		return newFakeHTTPResponse(400, `{}`)
	}))
	svc, err := admin.NewService(context.Background(), option.WithHTTPClient(c))
	if err != nil {
		t.Errorf("TestFindNextAvailablePeerAddress: %v", err)
	}
	_, n, _ := net.ParseCIDR("1.1.1.0/24")
	address, err := findNextAvailablePeerAddress(context.Background(), svc, n)
	if err != nil {
		t.Errorf("findNextAvailablePeerAddress: %v", err)
	}
	if address.String() != "1.1.1.3/32" {
		t.Errorf("findNextAvailablePeerAddress: did not get expected result: %s", address)
	}
}
