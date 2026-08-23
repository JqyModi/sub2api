package service

import "context"

type signupGrantSuppressedContextKey struct{}

// WithSignupGrantSuppressed marks a registration request whose promotional
// grant should be withheld. Account creation itself remains allowed.
func WithSignupGrantSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, signupGrantSuppressedContextKey{}, true)
}

func signupGrantSuppressed(ctx context.Context) bool {
	value, _ := ctx.Value(signupGrantSuppressedContextKey{}).(bool)
	return value
}
