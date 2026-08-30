package server

import (
	"encoding/json"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/spec"
)

// allocationRecord is the durable wire representation. The legacy fields are
// deliberately confined here: old records can still be read and current
// records retain the compatibility status/name projection without making those
// fields part of the live Allocation model.
type allocationRecord struct {
	Namespace     string
	JobName       string
	TaskGroupName string
	ID            string `json:"allocation_id,omitempty"`
	Name          string `json:"name,omitempty"`
	Generation    uint64 `json:"generation"`
	JobRevision   int    `json:"job_revision"`
	Tasks         []spec.TaskSpec
	Task          *spec.TaskSpec `json:",omitempty"`
	Status        string         `json:"status,omitempty"`
	Phase         lifecycle.Phase  `json:"phase,omitempty"`
	Health        lifecycle.Health `json:"health,omitempty"`
	lifecycle.Diagnostic
	Node     *Node
	Revision int               `json:"revision,omitempty"`
	Ports    []api.PortMapping `json:"ports,omitempty"`
}

func encodeAllocationRecord(a *Allocation) ([]byte, error) {
	a.normalize(time.Now().UTC())
	record := allocationRecord{
		Namespace:     a.Namespace,
		JobName:       a.JobName,
		TaskGroupName: a.TaskGroupName,
		ID:            a.ID,
		Name:          a.ID,
		Generation:    a.Generation,
		JobRevision:   a.JobRevision,
		Tasks:         a.Tasks,
		Status:        a.compatibilityStatus(),
		Phase:         a.Phase,
		Health:        a.Health,
		Diagnostic:    a.Diagnostic,
		Node:          a.Node,
		Revision:      a.JobRevision,
		Ports:         a.Ports,
	}
	return json.Marshal(record)
}

func decodeAllocationRecord(raw []byte, now time.Time) (*Allocation, error) {
	var record allocationRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, err
	}

	id := record.ID
	if id == "" {
		id = record.Name
	}
	jobRevision := record.JobRevision
	if jobRevision == 0 {
		jobRevision = record.Revision
	}
	tasks := record.Tasks
	if len(tasks) == 0 && record.Task != nil {
		tasks = []spec.TaskSpec{*record.Task}
	}
	phase, health := record.Phase, record.Health
	if !phase.Valid() {
		phase, health = lifecycle.Legacy(record.Status)
	}
	if !health.Valid() {
		health = lifecycle.HealthUnknown
	}

	allocation := &Allocation{
		Namespace:     record.Namespace,
		JobName:       record.JobName,
		TaskGroupName: record.TaskGroupName,
		ID:            id,
		Generation:    record.Generation,
		JobRevision:   jobRevision,
		Tasks:         tasks,
		Phase:         phase,
		Health:        health,
		Diagnostic:    record.Diagnostic,
		Node:          record.Node,
		Ports:         record.Ports,
	}
	allocation.normalize(now)
	return allocation, nil
}
