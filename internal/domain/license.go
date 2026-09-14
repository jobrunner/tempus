package domain

import (
	"fmt"
	"strings"
)

// Validate reports whether the licence block is complete.
//
// All three fields are required. This is the rule the README and the port docs
// have always stated; until it was enforced here, a provider returning an empty
// block simply produced an unattributed feature that nobody noticed. Attribution
// is the one obligation tempus carries on behalf of its upstream sources, so a
// missing block is a contract violation, not a cosmetic gap.
//
// The error names the fields that are missing: an operator seeing it needs to
// know which provider to fix and what to add, and "invalid license" sends them
// hunting.
func (l License) Validate() error {
	var missing []string
	if strings.TrimSpace(l.Name) == "" {
		missing = append(missing, "name")
	}
	if strings.TrimSpace(l.URL) == "" {
		missing = append(missing, "url")
	}
	if strings.TrimSpace(l.Attribution) == "" {
		missing = append(missing, "attribution")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("incomplete license: missing %s", strings.Join(missing, ", "))
}
