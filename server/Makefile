all: init protoc build

gen: protoc build

init:
	rm -rf go.mod
	go mod init recoverKV

protoc:
	# Generate the protobuf files using the ../proto directory
	protoc -I=../proto --go_out=. --go-grpc_out=. recoverKV.proto

build:
	# Build main file
	go build

clean:
	rm -r gen
	rm recoverKV *.log
