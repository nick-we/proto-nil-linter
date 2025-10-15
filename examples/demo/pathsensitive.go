package demo

import (
	"context"
	"errors"
)

// GOOD: Nil guard prevents false positive
func (s *UserService) GetUserWithGuard(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := findUser(req.UserId) // Returns MaybeNil

	if user == nil {
		// Early return when nil
		return nil, errors.New("user not found")
	}

	// Here: user is NeverNil due to guard above
	// Should NOT report error!
	return &UserResponse{
		User: user,
	}, nil
}

// BAD: No nil guard, MaybeNil used directly
func (s *UserService) GetUserNoGuard(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := findUser(req.UserId) // Returns MaybeNil

	// No nil check - should warn or error
	return &UserResponse{
		User: user, // MaybeNil used directly
	}, nil
}

// GOOD: Checks != nil
func (s *UserService) GetUserPositiveCheck(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := findUser(req.UserId)

	if user != nil {
		// user is NeverNil here
		return &UserResponse{
			User: user, // Should be OK
		}, nil
	}

	return nil, errors.New("not found")
}

// GOOD: Multiple guards
func (s *UserService) GetUserMultipleGuards(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := findUser(req.UserId)

	// First check
	if user == nil {
		user = createDefaultUser() // Now NeverNil
	}

	// user is NeverNil at this point
	return &UserResponse{
		User: user, // Should be OK
	}, nil
}
