// DigiPanel Session, Throttle, and Account Management Tests
// These cover the security properties the router depends on: only issued
// session tokens resolve, logout revokes immediately, repeated failures are
// throttled, weak passwords are rejected, and plans cannot be deleted while in
// use.

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"digipanel/core/rbac"
	"digipanel/modules/hosting"
)

func hostingCreateRequest(username, domain, pkg string) hosting.CreateAccountRequest {
	return hosting.CreateAccountRequest{
		TenantID: "tenant-default",
		Username: username,
		Domain:   domain,
		Package:  pkg,
	}
}

func TestSessionRevokedOnLogout(t *testing.T) {
	state, handler := setupIntegratedTestServer()
	token := adminToken(t, handler)

	body := bytes.NewReader([]byte(`{}`))
	logout := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", body)
	logout.Header.Set("Authorization", "Bearer "+token)
	logout.Header.Set("Content-Type", "application/json")
	logoutRecorder := httptest.NewRecorder()
	handler.ServeHTTP(logoutRecorder, logout)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("logout failed: %d %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	me := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	me.Header.Set("Authorization", "Bearer "+token)
	meRecorder := httptest.NewRecorder()
	handler.ServeHTTP(meRecorder, me)
	if meRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token must be rejected, got %d", meRecorder.Code)
	}
	if state.Sessions.Count() != 0 {
		t.Fatalf("expected no live sessions after logout, got %d", state.Sessions.Count())
	}
}

func TestSessionExpiry(t *testing.T) {
	store := NewSessionStore()
	base := time.Now()
	store.now = func() time.Time { return base }

	session, err := store.Create("admin", rbac.RoleAdmin, "127.0.0.1")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := store.Resolve(session.Token); err != nil {
		t.Fatalf("fresh session must resolve: %v", err)
	}

	// Move past the idle window without touching the session.
	store.now = func() time.Time { return base.Add(sessionIdleTTL + time.Minute) }
	if _, err := store.Resolve(session.Token); err == nil {
		t.Fatal("idle session must expire")
	}
}

func TestSessionTokenIsRandomPerLogin(t *testing.T) {
	_, handler := setupIntegratedTestServer()
	first := adminToken(t, handler)
	second := adminToken(t, handler)
	if first == second {
		t.Fatal("each login must issue a distinct token")
	}
	if len(first) <= len("token-role-admin") {
		t.Fatalf("session token is not opaque: %q", first)
	}
}

func TestLoginThrottleBlocksRepeatedFailures(t *testing.T) {
	state, handler := setupIntegratedTestServer()

	attempt := func(password string) int {
		payload, _ := json.Marshal(map[string]string{"username": "admin", "password": password})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder.Code
	}

	for index := 0; index < loginFailureLimit; index++ {
		if code := attempt("DefinitelyWrong2026!"); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d expected 401, got %d", index, code)
		}
	}

	if code := attempt("DefinitelyWrong2026!"); code != http.StatusTooManyRequests {
		t.Fatalf("expected throttling after %d failures, got %d", loginFailureLimit, code)
	}
	// A blocked key cannot be bypassed by presenting the correct password.
	if code := attempt(testAdminPassword); code != http.StatusTooManyRequests {
		t.Fatalf("expected throttle to hold even for correct credentials, got %d", code)
	}
	if state.LoginThrottle == nil {
		t.Fatal("login throttle was not initialised")
	}
}

func TestWeakPasswordRejected(t *testing.T) {
	state, _ := setupIntegratedTestServer()
	for _, weak := range []string{"short", "onlyletters", "1234567890"} {
		if err := state.createUser("weak_user", weak, rbac.RoleUser, ""); err == nil {
			t.Fatalf("weak password %q was accepted", weak)
		}
	}
}

func TestUserManagementLifecycle(t *testing.T) {
	state, handler := setupIntegratedTestServer()
	admin := adminToken(t, handler)

	create := func(body map[string]string) int {
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+admin)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder.Code
	}

	if code := create(map[string]string{"username": "ops_reseller", "password": "OpsReseller2026!", "role": "reseller"}); code != http.StatusCreated {
		t.Fatalf("expected 201 for user creation, got %d", code)
	}
	if code := create(map[string]string{"username": "ops_reseller", "password": "OpsReseller2026!", "role": "reseller"}); code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate user, got %d", code)
	}
	if code := create(map[string]string{"username": "bad_role", "password": "BadRole2026!", "role": "superuser"}); code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid role, got %d", code)
	}

	// The new account can authenticate and holds the reseller role.
	resellerToken := login(t, handler, "ops_reseller", "OpsReseller2026!")
	me := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	me.Header.Set("Authorization", "Bearer "+resellerToken)
	meRecorder := httptest.NewRecorder()
	handler.ServeHTTP(meRecorder, me)
	var profile struct {
		Username string   `json:"username"`
		Role     string   `json:"role"`
		Perms    []string `json:"permissions"`
	}
	_ = json.NewDecoder(meRecorder.Body).Decode(&profile)
	if profile.Role != "reseller" || profile.Username != "ops_reseller" || len(profile.Perms) == 0 {
		t.Fatalf("unexpected profile for created user: %+v", profile)
	}

	// Deleting the account removes it and revokes its session.
	del := httptest.NewRequest(http.MethodDelete, "/api/v1/users?username=ops_reseller", nil)
	del.Header.Set("Authorization", "Bearer "+admin)
	delRecorder := httptest.NewRecorder()
	handler.ServeHTTP(delRecorder, del)
	if delRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting user, got %d %s", delRecorder.Code, delRecorder.Body.String())
	}
	if _, exists := state.Users["ops_reseller"]; exists {
		t.Fatal("deleted user is still present")
	}
	meAfter := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meAfter.Header.Set("Authorization", "Bearer "+resellerToken)
	meAfterRecorder := httptest.NewRecorder()
	handler.ServeHTTP(meAfterRecorder, meAfter)
	if meAfterRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("session of a deleted user must be revoked, got %d", meAfterRecorder.Code)
	}
}

func TestUserRoleCannotCreateUsers(t *testing.T) {
	state, handler := setupIntegratedTestServer()
	userToken := createUserWithRole(t, state, handler, "plain_client", "PlainClient2026!", rbac.RoleUser)

	payload, _ := json.Marshal(map[string]string{"username": "escalated", "password": "Escalated2026!", "role": "admin"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("user role must not create accounts, got %d", recorder.Code)
	}
}

func TestAccountCreationProvisionsClientLogin(t *testing.T) {
	state, handler := setupIntegratedTestServer()
	admin := adminToken(t, handler)

	createAccount := func(body map[string]string) *httptest.ResponseRecorder {
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/hosting/accounts", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+admin)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	withPassword := createAccount(map[string]string{
		"username":      "login_client",
		"domain":        "login-client.test",
		"package":       "starter",
		"panelPassword": "ClientLogin2026!",
	})
	if withPassword.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating account with client login, got %d: %s", withPassword.Code, withPassword.Body.String())
	}
	var created struct {
		Account struct {
			Username string `json:"username"`
		} `json:"account"`
		PanelUsername   string `json:"panelUsername"`
		PanelLoginReady bool   `json:"panelLoginReady"`
		InitialPassword string `json:"initialPassword"`
	}
	if err := json.Unmarshal(withPassword.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode creation response: %v", err)
	}
	if !created.PanelLoginReady || created.PanelUsername != "login_client" {
		t.Fatalf("panel login was not provisioned: %+v", created)
	}
	if created.InitialPassword != "" {
		t.Fatalf("caller supplied password must not be echoed back, got %q", created.InitialPassword)
	}

	clientToken := login(t, handler, "login_client", "ClientLogin2026!")
	profile, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	profile.Header.Set("Authorization", "Bearer "+clientToken)
	profileRecorder := httptest.NewRecorder()
	handler.ServeHTTP(profileRecorder, profile)
	var me struct {
		Role string `json:"role"`
	}
	_ = json.NewDecoder(profileRecorder.Body).Decode(&me)
	if me.Role != "user" {
		t.Fatalf("client login must hold the user role, got %q", me.Role)
	}

	// The generated path issues a one-time password that actually authenticates.
	generated := createAccount(map[string]string{
		"username": "generated_client",
		"domain":   "generated-client.test",
		"package":  "starter",
	})
	if generated.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating account with generated password, got %d: %s", generated.Code, generated.Body.String())
	}
	var generatedBody struct {
		InitialPassword string `json:"initialPassword"`
	}
	_ = json.Unmarshal(generated.Body.Bytes(), &generatedBody)
	if len(generatedBody.InitialPassword) < 10 {
		t.Fatalf("expected a generated initial password, got %q", generatedBody.InitialPassword)
	}
	login(t, handler, "generated_client", generatedBody.InitialPassword)

	// The username is taken, so a second account with the same name is refused
	// before provisioning rather than leaving the hosting tree half created.
	duplicate := createAccount(map[string]string{
		"username": "login_client",
		"domain":   "duplicate-client.test",
		"package":  "starter",
	})
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a duplicate panel username, got %d", duplicate.Code)
	}
	if _, err := state.HostingManager.GetAccount("login_client"); err != nil {
		t.Fatalf("original account disappeared: %v", err)
	}
}

func TestClientRoleIsConfinedToOwnAccount(t *testing.T) {
	state, handler := setupIntegratedTestServer()

	if _, err := state.HostingManager.CreateAccount(hostingCreateRequest("scope_client", "scope-client.test", "starter")); err != nil {
		t.Fatalf("create client account: %v", err)
	}
	if _, err := state.HostingManager.CreateAccount(hostingCreateRequest("other_client", "other-client.test", "starter")); err != nil {
		t.Fatalf("create second account: %v", err)
	}
	clientToken := createUserWithRole(t, state, handler, "scope_client", "ScopeClient2026!", rbac.RoleUser)

	// The account listing is filtered to the signed-in client.
	list := httptest.NewRequest(http.MethodGet, "/api/v1/hosting/accounts", nil)
	list.Header.Set("Authorization", "Bearer "+clientToken)
	listRecorder := httptest.NewRecorder()
	handler.ServeHTTP(listRecorder, list)
	var listed []struct {
		Username string `json:"username"`
	}
	_ = json.NewDecoder(listRecorder.Body).Decode(&listed)
	if len(listed) != 1 || listed[0].Username != "scope_client" {
		t.Fatalf("client saw an unexpected account list: %+v", listed)
	}

	// Another account's files, logs, and usage are refused.
	for _, path := range []string{
		"/api/v1/files/list?username=other_client&path=public_html",
		"/api/v1/hosting/accounts/logs?username=other_client&type=access",
		"/api/v1/hosting/accounts/usage?username=other_client",
		"/api/v1/hosting/accounts/detail?username=other_client",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+clientToken)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("cross-account access to %s must be forbidden, got %d", path, recorder.Code)
		}
	}

	// The client's own files remain readable.
	own := httptest.NewRequest(http.MethodGet, "/api/v1/files/list?username=scope_client&path=public_html", nil)
	own.Header.Set("Authorization", "Bearer "+clientToken)
	ownRecorder := httptest.NewRecorder()
	handler.ServeHTTP(ownRecorder, own)
	if ownRecorder.Code != http.StatusOK {
		t.Fatalf("client must read its own account, got %d: %s", ownRecorder.Code, ownRecorder.Body.String())
	}
}

func TestPackageLifecycleAndInUseProtection(t *testing.T) {
	state, handler := setupIntegratedTestServer()
	admin := adminToken(t, handler)

	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var reader *bytes.Reader
		if body == nil {
			reader = bytes.NewReader(nil)
		} else {
			encoded, _ := json.Marshal(body)
			reader = bytes.NewReader(encoded)
		}
		req := httptest.NewRequest(method, path, reader)
		req.Header.Set("Authorization", "Bearer "+admin)
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	plan := map[string]any{
		"name":                "business",
		"diskLimitBytes":      53687091200,
		"bandwidthLimitBytes": 536870912000,
		"inodesLimit":         500000,
		"maxDomains":          25,
		"maxDatabases":        50,
		"maxEmailAccounts":    100,
		"defaultPhpVersion":   "8.3",
		"memoryLimitMb":       2048,
		"pidsLimit":           200,
	}

	if code := do(http.MethodPost, "/api/v1/hosting/packages", plan).Code; code != http.StatusCreated {
		t.Fatalf("expected 201 creating plan, got %d", code)
	}
	if code := do(http.MethodPost, "/api/v1/hosting/packages", plan).Code; code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate plan, got %d", code)
	}

	plan["memoryLimitMb"] = 4096
	if code := do(http.MethodPut, "/api/v1/hosting/packages?name=business", plan).Code; code != http.StatusOK {
		t.Fatalf("expected 200 updating plan, got %d", code)
	}
	found := false
	for _, pkg := range state.HostingManager.ListPackages() {
		if pkg.Name == "business" && pkg.MemoryLimitMb == 4096 {
			found = true
		}
	}
	if !found {
		t.Fatal("plan update did not reach the hosting manager")
	}

	account, err := state.HostingManager.CreateAccount(hostingCreateRequest("plan_client", "plan-client.test", "business"))
	if err != nil {
		t.Fatalf("create account on new plan: %v", err)
	}
	if account.PackageID != "business" {
		t.Fatalf("account was provisioned on %q instead of the requested plan", account.PackageID)
	}

	if code := do(http.MethodDelete, "/api/v1/hosting/packages?name=business", nil).Code; code != http.StatusConflict {
		t.Fatalf("expected 409 deleting a plan in use, got %d", code)
	}

	if code := do(http.MethodPost, "/api/v1/hosting/accounts/package", map[string]string{"username": "plan_client", "package": "starter"}).Code; code != http.StatusOK {
		t.Fatalf("expected 200 changing package, got %d", code)
	}
	moved, _ := state.HostingManager.GetAccount("plan_client")
	if moved.PackageID != "starter" {
		t.Fatalf("package change did not apply, account still on %q", moved.PackageID)
	}

	if code := do(http.MethodDelete, "/api/v1/hosting/packages?name=business", nil).Code; code != http.StatusOK {
		t.Fatalf("expected 200 deleting an unused plan, got %d", code)
	}
}
