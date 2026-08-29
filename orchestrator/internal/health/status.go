package health

type HealthStatus string

const (
	StatusInitializing HealthStatus = "initializing"
	StatusHealthy      HealthStatus = "healthy"
	StatusUnhealthy    HealthStatus = "unhealthy"
)

type TaskHealth struct {
	Status          HealthStatus
	ConsecutivePass int
	ConsecutiveFail int
}

func NewTaskHealth() *TaskHealth {
	return &TaskHealth{
		Status:          StatusInitializing,
		ConsecutivePass: 0,
		ConsecutiveFail: 0,
	}
}

func (t *TaskHealth) RecordResult(pass bool) (bool, HealthStatus) {
	if pass {
		t.ConsecutivePass++
		t.ConsecutiveFail = 0
	} else {
		t.ConsecutiveFail++
		t.ConsecutivePass = 0
	}

	switch t.Status {
	case StatusInitializing, StatusUnhealthy:
		if t.ConsecutivePass >= checkThreshold {
			t.Status = StatusHealthy
			return true, t.Status
		}

	case StatusHealthy:
		if t.ConsecutiveFail >= checkThreshold {
			t.Status = StatusUnhealthy
			return true, t.Status
		}
	}

	return false, t.Status
}
