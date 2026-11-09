package loans

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AppendAudit writes one line to the audit log of the deployment.
func AppendAudit(event string) error {
	dir := os.Getenv("LENDING_AUDIT_DIR")
	if dir == "" {
		dir = "/var/log/lending"
	}
	f, err := os.OpenFile(filepath.Join(dir, "audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), event)
	return err
}
