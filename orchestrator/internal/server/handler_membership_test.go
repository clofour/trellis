package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

type recordingClusterJoiner struct {
	removedID string
	err       error
}

func (*recordingClusterJoiner) AddVoter(string, string) error { return nil }

func (j *recordingClusterJoiner) RemoveServer(id string) error {
	j.removedID = id
	return j.err
}

func TestHandleRaftMemberRemove(t *testing.T) {
	joiner := &recordingClusterJoiner{}
	control := &Server{joiner: joiner}
	e := echo.New()
	NewHandler(control).Register(e)

	req := httptest.NewRequest(http.MethodDelete, "/v1/raft/members/node-2.example:8128", nil)
	req = req.WithContext(context.WithValue(req.Context(), AdminContextKey, true))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if joiner.removedID != "node-2.example:8128" {
		t.Fatalf("removed ID = %q, want %q", joiner.removedID, "node-2.example:8128")
	}
}

func TestHandleRaftMemberRemoveFailure(t *testing.T) {
	joiner := &recordingClusterJoiner{err: errors.New("not the leader")}
	control := &Server{joiner: joiner}
	e := echo.New()
	NewHandler(control).Register(e)

	req := httptest.NewRequest(http.MethodDelete, "/v1/raft/members/node-2", nil)
	req = req.WithContext(context.WithValue(req.Context(), AdminContextKey, true))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestHandleRaftMemberRemoveRequiresClusterAuthorization(t *testing.T) {
	joiner := &recordingClusterJoiner{}
	control := &Server{joiner: joiner}
	e := echo.New()
	NewHandler(control).Register(e)

	req := httptest.NewRequest(http.MethodDelete, "/v1/raft/members/node-2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if joiner.removedID != "" {
		t.Fatalf("unexpected removal of %q", joiner.removedID)
	}
}
