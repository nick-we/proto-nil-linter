package demo

import "context"

// Date represents a proto date message (simplified)
type Date struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Year  int32 `protobuf:"varint,1,opt,name=year,proto3"`
	Month int32 `protobuf:"varint,2,opt,name=month,proto3"`
	Day   int32 `protobuf:"varint,3,opt,name=day,proto3"`
}

// Movement represents a movement record
type Movement struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Id       string `protobuf:"bytes,1,opt,name=id,proto3"`
	KepRunId *Date  `protobuf:"bytes,2,opt,name=kep_run_id,proto3"` // Non-optional message field
	Status   string `protobuf:"bytes,3,opt,name=status,proto3"`
}

// ListMovementsResponse contains list of movements
type ListMovementsResponse struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Movements []*Movement `protobuf:"bytes,1,rep,name=movements,proto3"`
}

// ListMovementsRequest is the request
type ListMovementsRequest struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Limit int32 `protobuf:"varint,1,opt,name=limit,proto3"`
}

// BAD: Mutates slice element field to nil
func (s *UserService) ListMovementsBadMutation(ctx context.Context, req *ListMovementsRequest) (*ListMovementsResponse, error) {
	movements := []*Movement{
		{
			Id: "M1",
			KepRunId: &Date{
				Year:  2024,
				Month: 10,
				Day:   15,
			},
			Status: "active",
		},
	}

	movements[0].KepRunId = nil // Setting non-optional message field to nil!

	return &ListMovementsResponse{
		Movements: movements,
	}, nil
}

// GOOD: No nil mutation
func (s *UserService) ListMovementsGood(ctx context.Context, req *ListMovementsRequest) (*ListMovementsResponse, error) {
	movements := []*Movement{
		{
			Id: "M1",
			KepRunId: &Date{
				Year:  2024,
				Month: 10,
				Day:   15,
			},
			Status: "active",
		},
	}

	return &ListMovementsResponse{
		Movements: movements,
	}, nil
}
