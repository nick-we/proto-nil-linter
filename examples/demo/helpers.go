package demo

import "context"

// Helper function that creates invalid response
func buildInvalidResponse() *UserResponse {
	return &UserResponse{
		User: nil, // Should be detected even though in helper
	}
}

// Helper function with missing fields
func buildPartialUser() *User {
	return &User{
		Name:  "Partial",
		Email: "partial@example.com",
		// Missing Address!
	}
}

// Helper that returns nil directly
func getNilUser() *User {
	return nil
}

// gRPC handler calling helper with nil
func (s *UserService) GetUserViaHelperNil(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return buildInvalidResponse(), nil // Should detect nil in helper
}

// gRPC handler calling helper with partial init
func (s *UserService) GetUserViaHelperPartial(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := buildPartialUser() // Missing Address
	return &UserResponse{
		User: user, // Should detect missing Address from helper
	}, nil
}

// gRPC handler calling nil-returning helper
func (s *UserService) GetUserViaHelperNilReturn(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := getNilUser() // Returns AlwaysNil
	return &UserResponse{
		User: user, // Should detect nil from helper
	}, nil
}

// gRPC handler - good case
func (s *UserService) GetUserViaHelperGood(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := createDefaultUser() // Returns complete user
	return &UserResponse{
		User: user, // OK
	}, nil
}
