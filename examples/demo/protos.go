package demo

// Proto-generated message structures (simplified for demo)

type messageState struct{}
type sizeCache int32
type unknownFields []byte

// User represents a user
type User struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Name    string   `protobuf:"bytes,1,opt,name=name,proto3"`
	Email   string   `protobuf:"bytes,2,opt,name=email,proto3"`
	Address *Address `protobuf:"bytes,3,opt,name=address,proto3"`
}

// Address represents an address
type Address struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Street string `protobuf:"bytes,1,opt,name=street,proto3"`
	City   string `protobuf:"bytes,2,opt,name=city,proto3"`
}

// UserResponse is a gRPC response
type UserResponse struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	User *User `protobuf:"bytes,1,opt,name=user,proto3"`
}

// UserRequest is a gRPC request
type UserRequest struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	UserId string `protobuf:"bytes,1,opt,name=user_id,proto3"`
}
