package http

import (
	"fmt"
	"log"
	"time"
)

// auditLog records admin actions with structured fields.
func auditLog(action string, detail string) {
	entry := fmt.Sprintf(`{"time":"%s","type":"audit","action":"%s","detail":"%s"}`,
		time.Now().UTC().Format(time.RFC3339),
		action,
		detail,
	)
	log.Println(entry)
}
