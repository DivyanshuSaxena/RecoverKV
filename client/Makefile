LDFLAGS = -L/usr/local/lib `pkg-config --libs protobuf grpc++`\
           -Wl,--no-as-needed -lgrpc++_reflection -lprotobuf -lpthread -Wl,--as-needed\
           -ldl

CXX = g++
CPPFLAGS += `pkg-config --cflags protobuf grpc`
CXXFLAGS += -std=c++11

GRPC_CPP_PLUGIN = grpc_cpp_plugin
GRPC_CPP_PLUGIN_PATH ?= `which $(GRPC_CPP_PLUGIN)`

all: client lib739kv.so

client: recoverKV.pb.o recoverKV.grpc.pb.o client.o
	$(CXX) $^ $(LDFLAGS) -o $@

%.grpc.pb.cc: %.proto
	protoc --grpc_out=. --plugin=protoc-gen-grpc=$(GRPC_CPP_PLUGIN_PATH) $<

%.pb.cc: %.proto
	protoc --cpp_out=. $<

%.o: %.cc
	g++ $(CXXFLAGS) $(CPPFLAGS) $< -c -fPIC -o $@ 

lib739kv.so: recoverKV.grpc.pb.o recoverKV.pb.o client.o
	rm -f *.so
	g++ $(CXXFLAGS) $^ $(LDFLAGS) -fPIC -shared -o $@

clean:
	rm -f *.o *.pb.cc *.pb.h client
