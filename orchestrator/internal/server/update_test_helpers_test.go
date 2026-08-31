package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/catalog"
	"github.com/clofour/trellis/internal/client"
)

func newNopStateController() *StateController {
	return NewStateController(memoryStore{}, "test")
}

type testAgent struct {
	server *httptest.Server
	host   string
	port   int
}

func newTestAgent() *testAgent {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.OperationResponse{Code: "ok"})
	}))
	host, portStr, _ := net.SplitHostPort(ts.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return &testAgent{server: ts, host: "http://" + host, port: port}
}

func newTestAgentClient() *client.AgentClient {
	return client.NewAgentClient("test-token", nil)
}

func newNopCatalog() *catalog.ServiceCatalog {
	return catalog.New()
}
