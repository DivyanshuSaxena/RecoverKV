LDFLAGS += -L/usr/local/lib `pkg-config --libs protobuf grpc++ grpc`\
           -Wl,--no-as-needed -lgrpc++_reflection -Wl,--as-needed\
           -ldl
THFLAGS = -pthread

CXX = g++
CXXFLAGS += `pkg-config --cflags protobuf grpc`
CXXFLAGS += -std=c++11

GRPC_CPP_PLUGIN = grpc_cpp_plugin
GRPC_CPP_PLUGIN_PATH ?= `which $(GRPC_CPP_PLUGIN)`

PROTOS_PATH = ../proto
vpath %.proto $(PROTOS_PATH)

all: lib739kv.so

lib739kv.so: recoverKV.pb.o recoverKV.grpc.pb.o client.o
	rm -f *.so
	$(CXX) $(CXXFLAGS) $^ $(LDFLAGS) -fPIC -shared -o $@

recoverKV.pb.o: recoverKV.pb.cc
	$(CXX) $(CXXFLAGS) $< -c -fPIC -o $@

recoverKV.grpc.pb.o: recoverKV.grpc.pb.cc
	$(CXX) $(CXXFLAGS) $< -c -fPIC -o $@

client.o: client.cc
	$(CXX) $(CXXFLAGS) $< -c -fPIC -o $@

.PRECIOUS: %.grpc.pb.cc
%.grpc.pb.cc: %.proto
	protoc -I $(PROTOS_PATH) --grpc_out=. --plugin=protoc-gen-grpc=$(GRPC_CPP_PLUGIN_PATH) $<

.PRECIOUS: %.pb.cc
%.pb.cc: %.proto
	protoc -I $(PROTOS_PATH) --cpp_out=. $<

test: performance_test correctness_test multiperf_test

performance_test: tests/performance_test.cc
	$(CXX) $(CXXFLAGS) $< -L. -l739kv -Wl,-rpath=. -o $@

multiperf_test: tests/multiperf_test.cc
	$(CXX) $(CXXFLAGS) $< -L. $(THFLAGS) -l739kv -Wl,-rpath=. -o $@

correctness_test: tests/correctness_test.cc
	$(CXX) $(CXXFLAGS) $< -L. -l739kv -Wl,-rpath=. -o $@

clean:
	rm -f *.o *.pb.cc *_test