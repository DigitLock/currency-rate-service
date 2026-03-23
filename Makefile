.PHONY: proto clean-proto

PROTO_DIR = proto
GO_OUT = internal/grpc/pb
MODULE = github.com/DigitLock/currency-rate-service/internal/grpc/pb

proto:
	rm -rf $(GO_OUT)/*.go
	protoc \
		-I $(PROTO_DIR) \
		--go_out=$(GO_OUT) \
		--go_opt=module=$(MODULE) \
		--go-grpc_out=$(GO_OUT) \
		--go-grpc_opt=module=$(MODULE) \
		$(PROTO_DIR)/currency_rate/v1/service.proto

clean-proto:
	rm -rf $(GO_OUT)/*.go