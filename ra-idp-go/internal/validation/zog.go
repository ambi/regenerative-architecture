// Package validation contains shared validation adapters.
package validation

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	z "github.com/Oudwins/zog"
)

// Error converts Zog issues to the error contract used by domain and use-case code.
func Error(issues z.ZogIssueList) error {
	if len(issues) == 0 {
		return nil
	}

	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		message := issue.Message
		if message == "" && issue.Err != nil {
			message = issue.Err.Error()
		}
		if message == "" {
			message = issue.Code
		}
		if path := issue.PathString(); path != "" {
			message = fmt.Sprintf("%s: %s", path, message)
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return errors.New("validation failed")
	}
	sort.Strings(messages)
	return errors.New(strings.Join(messages, "; "))
}
