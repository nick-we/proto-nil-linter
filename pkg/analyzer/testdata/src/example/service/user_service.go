package service

import (
	"context"

	pb "example/proto"
)

// UserService implements the user service
type UserService struct{}

// Good: Properly initialized response
func (s *UserService) GetUserGood(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	return &pb.GetUserResponse{
		User: &pb.User{
			Name:  "John Doe",
			Email: "john@example.com",
			Address: &pb.Address{
				Street:  "123 Main St",
				City:    "New York",
				Country: "USA",
			},
			Age: 30,
		},
	}, nil
}

// Bad: Nil assignment to non-optional field (direct)
func (s *UserService) GetUserBadDirect(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	resp := &pb.GetUserResponse{}
	resp.User = nil // want "nil assignment to non-optional proto field"
	return resp, nil
}

// Bad: Nil in composite literal
func (s *UserService) GetUserBadComposite(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	return &pb.GetUserResponse{
		User: nil, // want "nil assignment to non-optional proto field"
	}, nil
}

// Bad: Nil from variable
func (s *UserService) GetUserBadVariable(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	var user *pb.User = nil
	return &pb.GetUserResponse{
		User: user, // want "nil assignment to non-optional proto field"
	}, nil
}

// Bad: Nil in nested message
func (s *UserService) GetUserBadNested(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	return &pb.GetUserResponse{
		User: &pb.User{
			Name:    "John Doe",
			Email:   "john@example.com",
			Address: nil, // want "nil assignment to non-optional proto field.*nested message"
			Age:     30,
		},
	}, nil
}

// Good: Properly initialized list
func (s *UserService) ListUsersGood(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	return &pb.ListUsersResponse{
		Users: []*pb.User{
			{
				Name:  "Alice",
				Email: "alice@example.com",
				Address: &pb.Address{
					Street:  "456 Oak Ave",
					City:    "Boston",
					Country: "USA",
				},
				Age: 25,
			},
			{
				Name:  "Bob",
				Email: "bob@example.com",
				Address: &pb.Address{
					Street:  "789 Pine Rd",
					City:    "Seattle",
					Country: "USA",
				},
				Age: 35,
			},
		},
		Total: 2,
	}, nil
}

// Bad: Nil item in repeated field
func (s *UserService) ListUsersBadNilItem(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	return &pb.ListUsersResponse{
		Users: []*pb.User{
			{
				Name:  "Alice",
				Email: "alice@example.com",
				Address: &pb.Address{
					Street:  "456 Oak Ave",
					City:    "Boston",
					Country: "USA",
				},
				Age: 25,
			},
			nil, // want "nil item at index 1 in repeated field"
			{
				Name:  "Bob",
				Email: "bob@example.com",
				Address: &pb.Address{
					Street:  "789 Pine Rd",
					City:    "Seattle",
					Country: "USA",
				},
				Age: 35,
			},
		},
		Total: 3,
	}, nil
}

// Bad: Nil nested field in repeated message
func (s *UserService) ListUsersBadNestedNil(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	return &pb.ListUsersResponse{
		Users: []*pb.User{
			{
				Name:    "Alice",
				Email:   "alice@example.com",
				Address: nil, // want "nil assignment to non-optional proto field.*nested message"
				Age:     25,
			},
			{
				Name:  "Bob",
				Email: "bob@example.com",
				Address: &pb.Address{
					Street:  "789 Pine Rd",
					City:    "Seattle",
					Country: "USA",
				},
				Age: 35,
			},
		},
		Total: 2,
	}, nil
}

// Bad: Multiple issues in one response
func (s *UserService) ListUsersBadMultiple(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	users := []*pb.User{
		{
			Name:    "Alice",
			Email:   "alice@example.com",
			Address: nil, // want "nil assignment to non-optional proto field.*nested message"
			Age:     25,
		},
		nil, // want "nil item at index 1 in repeated field"
	}
	return &pb.ListUsersResponse{
		Users: users,
		Total: 2,
	}, nil
}

// Good: Conditional logic but always valid
func (s *UserService) GetUserConditionalGood(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user := &pb.User{
		Name:  "Default User",
		Email: "default@example.com",
		Address: &pb.Address{
			Street:  "Unknown",
			City:    "Unknown",
			Country: "Unknown",
		},
		Age: 0,
	}

	if req.UserId != "" {
		user.Name = "Found User"
		user.Email = "found@example.com"
	}

	return &pb.GetUserResponse{
		User: user,
	}, nil
}

// Bad: Conditional nil assignment
func (s *UserService) GetUserConditionalBad(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	resp := &pb.GetUserResponse{
		User: &pb.User{
			Name:  "Temp",
			Email: "temp@example.com",
			Address: &pb.Address{
				Street:  "Temp St",
				City:    "Temp City",
				Country: "Temp",
			},
		},
	}

	if req.UserId == "" {
		resp.User = nil // want "nil assignment to non-optional proto field"
	}

	return resp, nil
}
