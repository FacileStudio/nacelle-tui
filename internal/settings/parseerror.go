package settings

// ParseError is an invalid settings file: Load refused it. The interactive
// client catches it to offer a default-settings boot instead of dying.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string { return "parsing " + e.Path + ": " + e.Err.Error() }
func (e *ParseError) Unwrap() error { return e.Err }
