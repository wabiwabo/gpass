package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchange_Success(t *testing.T) {
	expected := &TokenExchangeResponse{
		AccessToken:     "exchanged-token-abc",
		IssuedTokenType: TokenTypeAccessToken,
		TokenType:       "Bearer",
		ExpiresIn:       3600,
		Scope:           "openid profile",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		if got := r.FormValue("grant_type"); got != GrantTypeTokenExchange {
			t.Errorf("grant_type = %q, want %q", got, GrantTypeTokenExchange)
		}
		if got := r.FormValue("subject_token"); got != "original-token" {
			t.Errorf("subject_token = %q, want %q", got, "original-token")
		}
		if got := r.FormValue("subject_token_type"); got != TokenTypeAccessToken {
			t.Errorf("subject_token_type = %q, want %q", got, TokenTypeAccessToken)
		}

		// Verify basic auth.
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("no basic auth credentials")
		}
		if user != "client-id" || pass != "client-secret" {
			t.Errorf("auth = %q:%q, want client-id:client-secret", user, pass)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	exchanger := NewTokenExchanger(server.URL, "client-id", "client-secret")
	resp, err := exchanger.Exchange(context.Background(), TokenExchangeRequest{
		SubjectToken:     "original-token",
		SubjectTokenType: TokenTypeAccessToken,
		Audience:         "target-service",
		Scope:            "openid profile",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}

	if resp.AccessToken != expected.AccessToken {
		t.Errorf("AccessToken = %q, want %q", resp.AccessToken, expected.AccessToken)
	}
	if resp.IssuedTokenType != expected.IssuedTokenType {
		t.Errorf("IssuedTokenType = %q, want %q", resp.IssuedTokenType, expected.IssuedTokenType)
	}
	if resp.ExpiresIn != expected.ExpiresIn {
		t.Errorf("ExpiresIn = %d, want %d", resp.ExpiresIn, expected.ExpiresIn)
	}
}

func TestExchange_InvalidSubjectToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "subject token is invalid",
		})
	}))
	defer server.Close()

	exchanger := NewTokenExchanger(server.URL, "client-id", "client-secret")
	_, err := exchanger.Exchange(context.Background(), TokenExchangeRequest{
		SubjectToken:     "invalid-token",
		SubjectTokenType: TokenTypeAccessToken,
	})
	if err == nil {
		t.Fatal("Exchange() error = nil, want error for invalid subject token")
	}
}

func TestExchange_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	exchanger := NewTokenExchanger(server.URL, "client-id", "client-secret")
	_, err := exchanger.Exchange(context.Background(), TokenExchangeRequest{
		SubjectToken:     "some-token",
		SubjectTokenType: TokenTypeAccessToken,
	})
	if err == nil {
		t.Fatal("Exchange() error = nil, want error for server error")
	}
}

func TestExchange_CorrectGrantType(t *testing.T) {
	var receivedGrantType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		receivedGrantType = r.FormValue("grant_type")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenExchangeResponse{
			AccessToken:     "token",
			IssuedTokenType: TokenTypeAccessToken,
			TokenType:       "Bearer",
			ExpiresIn:       300,
		})
	}))
	defer server.Close()

	exchanger := NewTokenExchanger(server.URL, "client-id", "client-secret")
	_, err := exchanger.Exchange(context.Background(), TokenExchangeRequest{
		SubjectToken:     "token",
		SubjectTokenType: TokenTypeAccessToken,
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}

	if receivedGrantType != GrantTypeTokenExchange {
		t.Errorf("grant_type = %q, want %q", receivedGrantType, GrantTypeTokenExchange)
	}
}

func TestMockTokenExchanger_ReturnsConfiguredResponse(t *testing.T) {
	mock := NewMockTokenExchanger()
	expected := &TokenExchangeResponse{
		AccessToken:     "mock-exchanged-token",
		IssuedTokenType: TokenTypeAccessToken,
		TokenType:       "Bearer",
		ExpiresIn:       1800,
	}
	mock.AddExchange("user-token", expected)

	resp, err := mock.Exchange(context.Background(), TokenExchangeRequest{
		SubjectToken:     "user-token",
		SubjectTokenType: TokenTypeAccessToken,
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if resp.AccessToken != expected.AccessToken {
		t.Errorf("AccessToken = %q, want %q", resp.AccessToken, expected.AccessToken)
	}
}

func TestMockTokenExchanger_UnknownToken(t *testing.T) {
	mock := NewMockTokenExchanger()

	_, err := mock.Exchange(context.Background(), TokenExchangeRequest{
		SubjectToken:     "unknown-token",
		SubjectTokenType: TokenTypeAccessToken,
	})
	if err == nil {
		t.Fatal("Exchange() error = nil, want error for unknown token")
	}
}
