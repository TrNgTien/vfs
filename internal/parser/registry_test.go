package parser

import "testing"

func TestFindExtractor_SkipsProtoGenerated(t *testing.T) {
	tests := []struct {
		name     string
		wantSkip bool
	}{
		// Go
		{"handler.go", false},
		{"handler_test.go", true},
		{"service.pb.go", true},
		{"service_grpc.pb.go", true},

		// Python
		{"service.py", false},
		{"service_pb2.py", true},
		{"service_pb2_grpc.py", true},

		// JS/TS
		{"service.ts", false},
		{"service.js", false},
		{"service_pb.ts", true},
		{"service_pb.js", true},
		{"service_grpc_web_pb.js", true},
		{"service_grpc_web_pb.ts", true},

		// Dart
		{"service.dart", false},
		{"service.pb.dart", true},
		{"service.pbenum.dart", true},
		{"service.pbgrpc.dart", true},
		{"service.pbjson.dart", true},

		// Ruby
		{"service.rb", false},
		{"service_pb.rb", true},

		// C# (already skips generated patterns)
		{"service.cs", false},
		{"service.generated.cs", true},

		// Solidity
		{"contract.sol", false},
		{"contract.t.sol", true},
		{"deploy.s.sol", true},

		// Proto source files should NOT be skipped
		{"service.proto", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := FindExtractor(tt.name)
			if tt.wantSkip && ext != nil {
				t.Errorf("FindExtractor(%q) should return nil (skip generated), got extractor", tt.name)
			}
			if !tt.wantSkip && ext == nil {
				t.Errorf("FindExtractor(%q) should return extractor, got nil", tt.name)
			}
		})
	}
}
