package demo

import "context"

// BAD: User field is implicitly nil (not initialized)
func (s *UserService) GetUserImplicitNil(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		// User field not specified - implicitly nil!
	}, nil
}

// BAD: Partial initialization - Address still nil
func (s *UserService) GetUserPartialInit(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		User: &User{
			Name:  "John",
			Email: "john@example.com",
			// Address not specified - implicitly nil!
		},
	}, nil
}

// GOOD: All required fields initialized
func (s *UserService) GetUserFullInit(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		User: &User{
			Name:  "John",
			Email: "john@example.com",
			Address: &Address{
				Street: "123 Main",
				City:   "Boston",
			},
		},
	}, nil
}
