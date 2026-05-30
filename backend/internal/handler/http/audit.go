package http

import (
	"fmt"
	"lumipluse-backend/internal/pkg/utils"
	"time"
)

// auditLog records admin actions with structured fields.
func auditLog(action string, detail string) {
	entry := fmt.Sprintf(`{"time":"%s","type":"audit","action":"%s","detail":"%s"}`,
		time.Now().UTC().Format(time.RFC3339),
		action,
		detail,
	)
	utils.Info("audit: %s", entry)
}
