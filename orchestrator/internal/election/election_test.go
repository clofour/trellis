package election

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

func TestCurrentReturnsOnlyHeldLeader(t *testing.T) {
	id := uuid.New()
	value := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"node_id":%q,"address":"node:8128"}`, id)))
	held := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		session := ""
		if held {
			session = "session-id"
		}
		fmt.Fprintf(w, `[{"Key":"trellis/default/leader","Value":%q,"Session":%q}]`, value, session)
	}))
	defer ts.Close()
	config := api.DefaultConfig()
	config.Address = ts.URL
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	elector := New(client, "default", Leader{}, 15*time.Second)

	leader, err := elector.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if leader == nil || leader.NodeID != id || leader.Address != "node:8128" {
		t.Fatalf("unexpected leader: %#v", leader)
	}
	held = false
	leader, err = elector.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if leader != nil {
		t.Fatalf("expected no leader, got %#v", leader)
	}
}
