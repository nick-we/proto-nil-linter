package demo

import "context"

// BAD: User created with missing Address field
func (s *UserService) GetUserEmptyInit(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := &User{} // Address is implicitly nil!
	return &UserResponse{
		User: user, // Should detect that user.Address is nil
	}, nil
}

// BAD: User created with only some fields
func (s *UserService) GetUserPartialFields(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := &User{
		Name:  "John",
		Email: "john@example.com",
		// Address missing - implicitly nil!
	}
	return &UserResponse{
		User: user, // Should detect that user.Address is nil
	}, nil
}

// GOOD: User created with all required fields
func (s *UserService) GetUserComplete(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := &User{
		Name:  "John",
		Email: "john@example.com",
		Address: &Address{
			Street: "123 Main",
			City:   "Boston",
		},
	}
	return &UserResponse{
		User: user, // OK - all fields present
	}, nil
}
