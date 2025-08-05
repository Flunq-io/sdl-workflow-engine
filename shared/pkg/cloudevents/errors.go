package cloudevents

import "errors"

// CloudEvent validation errors
var (
	ErrMissingID          = errors.New("cloudevent: missing required 'id' attribute")
	ErrMissingSource      = errors.New("cloudevent: missing required 'source' attribute")
	ErrMissingType        = errors.New("cloudevent: missing required 'type' attribute")
	ErrMissingSpecVersion = errors.New("cloudevent: missing required 'specversion' attribute")
	ErrInvalidSpecVersion = errors.New("cloudevent: invalid 'specversion' attribute")
	ErrInvalidTime        = errors.New("cloudevent: invalid 'time' attribute")
	ErrInvalidData        = errors.New("cloudevent: invalid 'data' attribute")
)
