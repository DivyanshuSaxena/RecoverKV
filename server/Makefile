# Initiate module
go mod init recoverKV

# Generate the protobuf files using the ../proto directory
protoc -I=../proto --go_out=. --go-grpc_out=. recoverKV.proto

# Build main file
go build