package demo

import "context"

// UserService implements user operations
type UserService struct{}

// GetUser returns a user - BAD: assigns nil to non-optional field
func (s *UserService) GetUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		User: nil, // This should be caught!
	}, nil
}

// GetUserGood returns a user properly
func (s *UserService) GetUserGood(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		User: &User{
			Name:  "John Doe",
			Email: "john@example.com",
			Address: &Address{
				Street: "123 Main St",
				City:   "Boston",
			},
		},
	}, nil
}

// GetUserBadNested has nil in nested message
func (s *UserService) GetUserBadNested(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		User: &User{
			Name:    "Jane",
			Email:   "jane@example.com",
			Address: nil, // This should be caught!
		},
	}, nil
}
