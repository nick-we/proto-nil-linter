package demo

import "context"

// Test with actual proto definitions that have optional and required fields

// GOOD: Optional fields can be nil
func (s *UserService) GetUserOptionalFieldsNil(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	nickname := "Johnny"
	return &UserResponse{
		User: &User{
			Name:     "John",
			Email:    "john@example.com",
			Nickname: &nickname, // Optional - can set it
			Address: &Address{ // Required - must provide
				Street: "123 Main",
				City:   "Boston",
				// Apartment is optional - can omit
				// Location is optional - can omit
			},
			// Profile is optional - can omit
		},
		// RelatedUser is optional - can omit
	}, nil
}

// GOOD: Optional message field set to nil explicitly
func (s *UserService) GetUserWithNilOptional(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		User: &User{
			Name:     "John",
			Email:    "john@example.com",
			Nickname: nil, // ✅ OK: nickname is optional
			Address: &Address{
				Street:    "123 Main",
				City:      "Boston",
				Apartment: nil, // ✅ OK: apartment is optional
				Location:  nil, // ✅ OK: location is optional sub-message
			},
			Profile: nil, // ✅ OK: profile is optional message
		},
		RelatedUser: nil, // ✅ OK: related_user is optional message
	}, nil
}

// BAD: Required field Address is nil
func (s *UserService) GetUserMissingRequired(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		User: &User{
			Name:     "John",
			Email:    "john@example.com",
			Nickname: nil, // ✅ OK: optional
			Address:  nil, // ❌ ERROR: address is REQUIRED
			Profile:  nil, // ✅ OK: optional
		},
		RelatedUser: nil, // ✅ OK: optional
	}, nil
}

// BAD: Required top-level field User is nil
func (s *UserService) GetUserMissingUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	return &UserResponse{
		User:        nil, // ❌ ERROR: user is REQUIRED
		RelatedUser: nil, // ✅ OK: related_user is optional
	}, nil
}
