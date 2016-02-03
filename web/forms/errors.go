package forms

import "fmt"
import "strings"

// Represents a specific error with a form submission.
type ErrorInfo struct {
	// Field to which this error pertains. May be nil.
	FieldInfo *FieldInfo

	// Error message.
	Message string
}

func (ei *ErrorInfo) Error() string {
	if ei.FieldInfo == nil {
		return ei.Message
	}

	n := ei.FieldInfo.Label
	if n == "" {
		n = ei.FieldInfo.Name
	}
	return fmt.Sprintf("%s: %s", n, ei.Message)
}

// Represents a target for generated form errors.
type ErrorSink interface {
	error
	Add(errorInfo ErrorInfo)
	Num() int
}

// Represents zero or more errors with a form submission.
//
// Implements ErrorSink.
type Errors []*ErrorInfo

func (e *Errors) Add(errorInfo ErrorInfo) {
	*e = append(*e, &errorInfo)
}

// Number of errors.
func (e Errors) Num() int {
	return len(e)
}

// Returns a list of the errors as a string. Returns the empty string if there
// are no errors.
func (e Errors) Error() string {
	var s []string
	for _, v := range e {
		s = append(s, v.Error())
	}
	return strings.Join(s, "; ")
}
