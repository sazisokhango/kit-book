// Package servicedue reads the externally-maintained service-due data file
// (05-spec/units/u7-service-due-integration/spec.md) and answers, per item,
// whether it is currently blocked from checkout.
//
// RESOLVE-IN-PLAN (from the spec, §6): the file's real format/location is
// not yet confirmed by Chris (Equipment Officer). This implementation picks
// a concrete, documented default so U3 (checkout) has something real to
// call: a plain text file, one item_id per line, listing every item
// currently due for service. Presence in the file means due; absence means
// not due. Blank lines and lines starting with "#" are ignored as comments.
// This is a placeholder convention, not a finalised one — revisit when
// Chris confirms the real format, before this is used against production
// data.
package servicedue

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"bitbucket.org/psybergate/kitbook/internal/kitbookerrors"
)

// itemIDPattern matches the catalogue's "<TYPE>-<NN>" id convention
// (05-spec/units/u2-catalogue-seed/spec.md), e.g. "ROPE-04". A line that
// doesn't match this shape is treated as a malformed file, not a typo to
// silently ignore — U7's fail-closed requirement extends to the parser.
var itemIDPattern = regexp.MustCompile(`^[A-Z]+-[0-9]+$`)

// Checker answers whether an item is currently service-due.
type Checker struct {
	due map[string]bool
}

// Load reads the service-due file at path. It fails closed: a missing file
// returns ServiceDueFileUnavailableError, and a malformed line returns
// ServiceDueFileFormatError, both per the spec's fail-closed scenarios —
// never silently treated as "nothing is due".
func Load(path string) (*Checker, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &kitbookerrors.ServiceDueFileUnavailableError{}
		}
		return nil, &kitbookerrors.ServiceDueFileUnavailableError{}
	}
	defer f.Close()

	due := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !itemIDPattern.MatchString(line) {
			return nil, &kitbookerrors.ServiceDueFileFormatError{}
		}
		due[line] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read service-due file: %w", err)
	}

	return &Checker{due: due}, nil
}

// IsServiceDue reports whether itemID is listed in the service-due file.
func (c *Checker) IsServiceDue(itemID string) bool {
	return c.due[itemID]
}
