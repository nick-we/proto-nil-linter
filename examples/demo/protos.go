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

	Name     string   `protobuf:"bytes,1,opt,name=name,proto3"`
	Email    string   `protobuf:"bytes,2,opt,name=email,proto3"`
	Nickname *string  `protobuf:"bytes,3,opt,name=nickname,proto3,optional"` // Optional
	Address  *Address `protobuf:"bytes,4,opt,name=address,proto3"`           // Required
	Profile  *Profile `protobuf:"bytes,5,opt,name=profile,proto3,optional"`  // Optional
}

// Profile represents a user profile
type Profile struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Bio       string  `protobuf:"bytes,1,opt,name=bio,proto3"`
	AvatarUrl *string `protobuf:"bytes,2,opt,name=avatar_url,proto3,optional"` // Optional
}

// Location represents geographic coordinates
type Location struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Latitude  float64 `protobuf:"fixed64,1,opt,name=latitude,proto3"`
	Longitude float64 `protobuf:"fixed64,2,opt,name=longitude,proto3"`
}

// Address represents an address
type Address struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	Street    string    `protobuf:"bytes,1,opt,name=street,proto3"`
	City      string    `protobuf:"bytes,2,opt,name=city,proto3"`
	Apartment *string   `protobuf:"bytes,3,opt,name=apartment,proto3,optional"` // Optional
	Location  *Location `protobuf:"bytes,4,opt,name=location,proto3,optional"`  // Optional
}

// UserResponse is a gRPC response
type UserResponse struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	User        *User `protobuf:"bytes,1,opt,name=user,proto3"`                  // Required
	RelatedUser *User `protobuf:"bytes,2,opt,name=related_user,proto3,optional"` // Optional
}

// UserRequest is a gRPC request
type UserRequest struct {
	state         messageState
	sizeCache     sizeCache
	unknownFields unknownFields

	UserId string `protobuf:"bytes,1,opt,name=user_id,proto3"`
}
