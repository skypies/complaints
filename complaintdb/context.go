package complaintdb

// Provide support for callers (i.e. creators of the Context) to indicate
// things to us ... in the first instance, that the context should run with admin privs

import (
	"golang.org/x/net/context"
)

// To prevent other libs colliding with us in the context.Value keyspace, use these private keys
type contextKey int
const(
	contextPropertiesKey contextKey = iota
)

type ContextProperties struct {
	IsAdmin bool
	ProjectId string
}

func GetContextProperties(ctx context.Context) (ContextProperties, bool) {
	opt, ok := ctx.Value(contextPropertiesKey).(ContextProperties)
	return opt, ok
}

func SetContextProperties(ctx context.Context, props ContextProperties) context.Context {
	return context.WithValue(ctx, contextPropertiesKey, props)
}
