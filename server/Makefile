all: init protoc build

gen: protoc build

init:
	rm -rf go.mod
	go mod init recoverKV
	mkdir -p log data

protoc:
	# Generate the protobuf files using the ../proto directory
	protoc -I=../proto --go_out=. --go-grpc_out=. recoverKV.proto

build:
	# Build server
	go build -o recoverKV server.go persist.go
	# Build load balancer
	go build -o recoverLB load_balancer.go

server:
	rm recoverKV
	go build -o recoverKV server.go persist.go

lb:
	rm recoverLB
	go build -o recoverLB load_balancer.go

clean:
	rm -r gen
	rm recoverKV recoverLB *.log
