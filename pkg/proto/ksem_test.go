package proto

import "testing"

// Loading the package already parses the embedded descriptor; a hand-edited ksem.pb.go panics here.
func TestDescriptor(t *testing.T) {
	if File_ksem_proto == nil || File_ksem_proto.Messages().Len() == 0 {
		t.Fatal("ksem.proto descriptor not loaded")
	}
}
