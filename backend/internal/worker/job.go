package worker

import (
	"time"

	"github.com/yourusername/viscod/internal/models"
)

// Job holds a polling task for a device.
type Job struct {
	Device      *models.Device
	ScheduledAt time.Time
}
