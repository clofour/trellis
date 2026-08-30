package server

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/catalog"
	secretstore "github.com/clofour/trellis/internal/secrets"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	server *Server
}

type contextKey string

const NamespaceContextKey contextKey = "trellis-namespace"
const AdminContextKey contextKey = "trellis-admin"

func requestNamespace(c *echo.Context) string {
	if ns, ok := c.Request().Context().Value(NamespaceContextKey).(string); ok && ns != "" {
		return ns
	}
	return c.Request().Header.Get("X-Trellis-Namespace")
}

func NewHandler(server *Server) *Handler {
	return &Handler{
		server: server,
	}
}

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/metrics", h.handleMetrics)
	v1 := e.Group("/v1")
	v1.GET("/nodes", h.handleListNodes)
	v1.POST("/nodes", h.handleRegisterNode)
	v1.POST("/nodes/:id/heartbeat", h.handleHeartbeat)
	v1.POST("/nodes/:id/drain", h.handleDrainNode)
	v1.DELETE("/nodes/:id", h.handleRemoveNode)
	v1.GET("/jobs", h.handleListJobs)
	v1.POST("/jobs", h.handleRegisterJob)
	v1.GET("/jobs/:name", h.handleGetJob)
	v1.DELETE("/jobs/:name", h.handleDeleteJob)
	v1.GET("/allocations", h.handleListAllocations)
	v1.GET("/allocations/:id/events", h.handleAllocationEvents)
	v1.GET("/allocations/:id/logs", h.handleAllocationLogs)
	v1.GET("/internal/discovery", h.handleListDiscovery)
	v1.POST("/raft/join", h.handleRaftJoin)
	v1.PUT("/namespaces/:namespace/secrets/:name", h.handleSetSecret)
	v1.GET("/namespaces/:namespace/secrets", h.handleListSecrets)
	v1.GET("/namespaces/:namespace/secrets/:name", h.handleGetSecret)
	v1.DELETE("/namespaces/:namespace/secrets/:name", h.handleDeleteSecret)
}

func secretNamespace(c *echo.Context) (string, error) {
	c.Response().Header().Set("Cache-Control", "no-store")
	if admin, _ := c.Request().Context().Value(AdminContextKey).(bool); !admin {
		return "", echo.NewHTTPError(http.StatusForbidden, "secret management requires cluster authorization")
	}
	ns := c.Param("namespace")
	if !spec.ValidIdentifier(ns) {
		return "", echo.NewHTTPError(http.StatusBadRequest, "invalid namespace")
	}
	if requested := requestNamespace(c); requested != "" && requested != ns {
		return "", echo.NewHTTPError(http.StatusForbidden, "namespace does not match token scope")
	}
	return ns, nil
}

func (h *Handler) handleSetSecret(c *echo.Context) error {
	ns, err := secretNamespace(c)
	if err != nil {
		return err
	}
	if !spec.ValidIdentifier(c.Param("name")) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid secret name")
	}
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 96<<10)
	var request api.SecretWriteRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	value, err := base64.StdEncoding.DecodeString(request.ValueBase64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "value_base64 is invalid")
	}
	defer clear(value)
	meta, err := h.server.SetSecret(c.Request().Context(), ns, c.Param("name"), value, request.ExpectedVersion)
	if errors.Is(err, secretstore.ErrVersionConflict) {
		return echo.NewHTTPError(http.StatusConflict, "secret version conflict")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "unable to store secret")
	}
	return c.JSON(http.StatusOK, meta)
}

func (h *Handler) handleListSecrets(c *echo.Context) error {
	ns, err := secretNamespace(c)
	if err != nil {
		return err
	}
	items, err := h.server.ListSecrets(c.Request().Context(), ns)
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "unable to list secrets")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) handleGetSecret(c *echo.Context) error {
	ns, err := secretNamespace(c)
	if err != nil {
		return err
	}
	meta, err := h.server.GetSecretMetadata(c.Request().Context(), ns, c.Param("name"))
	if errors.Is(err, secretstore.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "secret not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "unable to read secret metadata")
	}
	return c.JSON(http.StatusOK, meta)
}

func (h *Handler) handleDeleteSecret(c *echo.Context) error {
	ns, err := secretNamespace(c)
	if err != nil {
		return err
	}
	err = h.server.DeleteSecret(c.Request().Context(), ns, c.Param("name"))
	if errors.Is(err, secretstore.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "secret not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "unable to delete secret")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) handleAllocationEvents(c *echo.Context) error {
	events, ok := h.server.AllocationEvents(requestNamespace(c), c.Param("id"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "allocation not found")
	}
	if events == nil {
		events = api.AllocationEventListResponse{}
	}
	return c.JSON(http.StatusOK, events)
}

func (h *Handler) handleAllocationLogs(c *echo.Context) error {
	tail, err := strconv.Atoi(c.QueryParam("tail"))
	if c.QueryParam("tail") == "" {
		tail, err = 100, nil
	}
	if err != nil || tail < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "tail must be a non-negative integer")
	}
	logs, err := h.server.AllocationLogsForNamespace(c.Request().Context(), requestNamespace(c), c.Param("id"), c.QueryParam("follow") == "true", tail)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "allocation not found")
	}
	defer logs.Close()
	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	_, err = io.Copy(c.Response(), logs)
	return err
}

func (h *Handler) handleDrainNode(c *echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid node ID")
	}
	if err := h.server.DrainNode(c.Request().Context(), id); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "node not found")
	}
	return c.NoContent(http.StatusAccepted)
}

func (h *Handler) handleRemoveNode(c *echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid node ID")
	}
	if err := h.server.RemoveNode(c.Request().Context(), id); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "node not found")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) handleListNodes(c *echo.Context) error {
	nodes := h.server.ListNodes()

	result := make(api.NodeListResponse, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, *h.convertNode(&node))
	}

	return c.JSON(200, result)
}

func (h *Handler) handleRegisterNode(c *echo.Context) error {
	ctx := c.Request().Context()

	var request api.NodeRegistrationRequest
	err := c.Bind(&request)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	err = h.server.RegisterNode(ctx, &NodeRegistration{
		ID:                 request.ID,
		Host:               request.Host,
		Port:               request.Port,
		CPU:                request.CPU,
		Memory:             request.Memory,
		OS:                 request.OS,
		Arch:               request.Arch,
		Labels:             request.Labels,
		Volumes:            request.Volumes,
		WireGuardPublicKey: request.WireGuardPublicKey,
		WireGuardEndpoint:  request.WireGuardEndpoint,
		RaftID:             request.RaftID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to register node")
	}

	return c.JSON(http.StatusCreated, api.NodeRegistrationResponse{ID: request.ID})
}

func (h *Handler) handleHeartbeat(c *echo.Context) error {
	ctx := c.Request().Context()

	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	var request api.HeartbeatRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	err = h.server.Heartbeat(ctx, uuid, request.Allocations, request.Volumes)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to process heartbeat")
	}

	return c.JSON(http.StatusOK, h.server.HeartbeatResponse(uuid))
}

func (h *Handler) handleListJobs(c *echo.Context) error {
	jobs := h.server.ListJobs(requestNamespace(c))
	return c.JSON(200, jobs)
}

func (h *Handler) handleGetJob(c *echo.Context) error {
	status, ok := h.server.GetJob(requestNamespace(c), c.Param("name"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}
	return c.JSON(http.StatusOK, status)
}

func (h *Handler) handleDeleteJob(c *echo.Context) error {
	if err := h.server.DeleteJob(c.Request().Context(), requestNamespace(c), c.Param("name")); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) handleRegisterJob(c *echo.Context) error {
	ctx := c.Request().Context()

	var request api.JobRegistrationRequest
	err := c.Bind(&request)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	err = h.server.RegisterJob(ctx, requestNamespace(c), &request.Spec)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "job is invalid or could not be saved")
	}

	return c.NoContent(http.StatusAccepted)
}

func (h *Handler) handleListAllocations(c *echo.Context) error {
	var filter *AllocationListFilter
	job := c.QueryParam("job")
	label := c.QueryParam("label")
	if job != "" || label != "" {
		filter = &AllocationListFilter{Job: job, Label: label}
	}
	allocations := h.server.ListAllocations(requestNamespace(c), filter)
	if allocations == nil {
		allocations = api.AllocationListResponse{}
	}
	return c.JSON(http.StatusOK, allocations)
}

func (h *Handler) handleListDiscovery(c *echo.Context) error {
	// Discovery records are node-internal scheduler data. Namespace-scoped API
	// tokens must not be able to turn them back into a user-facing resource.
	if requestNamespace(c) != "" {
		return echo.NewHTTPError(http.StatusForbidden, "internal discovery is cluster-scoped")
	}

	var filter *catalog.ListFilter
	job := c.QueryParam("job")
	label := c.QueryParam("label")
	if job != "" || label != "" {
		filter = &catalog.ListFilter{Job: job, Label: label}
	}
	entries := h.server.ListServices("", filter)
	if entries == nil {
		entries = api.ServiceListResponse{}
	}
	return c.JSON(http.StatusOK, entries)
}

func (h *Handler) handleRaftJoin(c *echo.Context) error {
	var req api.RaftJoinRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.ID == "" || req.RaftAddress == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "id and raft_address are required")
	}
	if h.server.joiner == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "cluster join not available")
	}
	if err := h.server.joiner.AddVoter(req.ID, req.RaftAddress); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	caCert, caKey, err := h.server.ClusterCA()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "cluster CA unavailable")
	}
	return c.JSON(http.StatusOK, api.RaftJoinResponse{CACert: caCert, CAKey: caKey})
}

func (h *Handler) handleMetrics(c *echo.Context) error {
	promhttp.Handler().ServeHTTP(c.Response(), c.Request())
	return nil
}

func (h *Handler) convertNode(node *Node) *api.NodeResponse {
	return &api.NodeResponse{
		ID:            node.ID,
		Host:          node.Host,
		Port:          node.Port,
		Status:        api.NodeStatusResponse(node.Status),
		LastHeartbeat: node.LastHeartbeat,
		CPU:           node.CPU,
		Memory:        node.Memory,
		Labels:        node.Labels,
		Volumes:       node.Volumes,
	}
}
