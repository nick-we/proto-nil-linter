package demo

import "context"

// Helper function that always returns nil
func getUserFromDB() *User {
	return nil
}

// Helper function that never returns nil
func createDefaultUser() *User {
	return &User{
		Name:  "Default",
		Email: "default@example.com",
		Address: &Address{
			Street: "Unknown",
			City:   "Unknown",
		},
	}
}

// Helper function that may return nil
func findUser(id string) *User {
	if id == "" {
		return nil
	}
	return &User{
		Name:  "Found",
		Email: "found@example.com",
		Address: &Address{
			Street: "123 Main",
			City:   "Boston",
		},
	}
}

// BAD: Uses function that always returns nil
func (s *UserService) GetUserFromDB(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := getUserFromDB() // Returns AlwaysNil
	return &UserResponse{
		User: user, // Should detect as nil from function call
	}, nil
}

// GOOD: Uses function that never returns nil
func (s *UserService) GetDefaultUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := createDefaultUser() // Returns NeverNil
	return &UserResponse{
		User: user, // Should be OK
	}, nil
}

// MAYBE: Uses function that may return nil
func (s *UserService) FindUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	user := findUser(req.UserId) // Returns MaybeNil
	return &UserResponse{
		User: user, // Could warn about MaybeNil
	}, nil
}
