package demo

import "context"

// BAD: user initialized in conditional but with missing fields
func (s *UserService) GetUserConditionalInit(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	var user *User
	if user == nil {
		user = &User{} // Missing Address field!
	}
	return &UserResponse{
		User: user, // Should detect missing Address in user
	}, nil
}

// BAD: Reassignment with missing fields
func (s *UserService) GetUserReassign(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	var user *User
	if req.UserId == "" {
		user = &User{ // Missing Address!
			Name:  "Default",
			Email: "default@example.com",
		}
	}
	return &UserResponse{
		User: user, // Should detect
	}, nil
}

// GOOD: All fields present in conditional
func (s *UserService) GetUserCompleteCond(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	var user *User
	if user == nil {
		user = &User{
			Name:  "Test",
			Email: "test@example.com",
			Address: &Address{
				Street: "123 Main",
				City:   "Boston",
			},
		}
	}
	return &UserResponse{
		User: user,
	}, nil
}
