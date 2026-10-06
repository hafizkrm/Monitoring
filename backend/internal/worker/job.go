package worker

import (
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// Job holds a polling task for a device.
type Job struct {
	Device      *models.Device
	ScheduledAt time.Time
}
