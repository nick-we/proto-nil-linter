package demo

import "context"

// Test switch statement handling
func (s *UserService) GetUserSwitch(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	var user *User

	switch req.UserId {
	case "":
		user = &User{
			Name:  "Default",
			Email: "default@example.com",
			// Missing Address!
		}
	case "admin":
		user = &User{
			Name:    "Admin",
			Email:   "admin@example.com",
			Address: &Address{Street: "Admin St", City: "Admin City"},
		}
	default:
		user = &User{
			Name:  "User",
			Email: "user@example.com",
			// Missing Address!
		}
	}

	return &UserResponse{
		User: user, // Should detect missing Address on some paths
	}, nil
}

// All cases complete
func (s *UserService) GetUserSwitchComplete(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	var user *User

	switch req.UserId {
	case "":
		user = &User{
			Name:    "Default",
			Email:   "default@example.com",
			Address: &Address{Street: "Default St", City: "Default City"},
		}
	default:
		user = &User{
			Name:    "User",
			Email:   "user@example.com",
			Address: &Address{Street: "User St", City: "User City"},
		}
	}

	return &UserResponse{
		User: user, // OK - Address present on all paths
	}, nil
}
