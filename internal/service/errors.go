package service

// Error represents a custom error with code and message
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface
func (e Error) Error() string {
	return e.Message
}

// NewError creates a new error
func NewError(code, message string) Error {
	return Error{Code: code, Message: message}
}
