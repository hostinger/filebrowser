package errors

import "errors"

var (
	ErrEmptyKey             = errors.New("empty key")
	ErrExist                = errors.New("the resource already exists")
	ErrNotExist             = errors.New("the resource does not exist")
	ErrEmptyPassword        = errors.New("password is empty")
	ErrEmptyUsername        = errors.New("username is empty")
	ErrEmptyRequest         = errors.New("empty request")
	ErrScopeIsRelative      = errors.New("scope is a relative path")
	ErrInvalidDataType      = errors.New("invalid data type")
	ErrIsDirectory          = errors.New("file is directory")
	ErrInvalidOption        = errors.New("invalid option")
	ErrInvalidAuthMethod    = errors.New("invalid auth method")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrInvalidRequestParams = errors.New("invalid request params")
	ErrSourceIsParent       = errors.New("source is parent")
	ErrRootUserDeletion     = errors.New("user with id 1 can't be deleted")
)

type HTTPError struct {
	Err  error
	Type string
}

func (e *HTTPError) Error() string {
	if e.Err == nil {
		return e.Type
	}
	return e.Err.Error()
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

func NewHTTPError(err error, errType string) error {
	return &HTTPError{
		Err:  err,
		Type: errType,
	}
}
