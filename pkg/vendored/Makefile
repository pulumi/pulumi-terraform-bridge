.PHONY: build install vendor-tfplugin vendor

build::
	echo "Use 'nix build' or 'make tfplugin' instead"

# This and the following targets run inside nix. Use `nix develop` to debug.
tfplugin5.proto:
	cp "${OPENTOFU}/docs/plugin-protocol/tfplugin5.5.proto" "$@"

tfplugin6.proto:
	cp "${OPENTOFU}/docs/plugin-protocol/tfplugin6.5.proto" "$@"

tfplugin5_pulumi.proto: tfplugin5.proto
	./sub.sh 5 "$<" "$@"

tfplugin6_pulumi.proto: tfplugin6.proto
	./sub.sh 6 "$<" "$@"

tfplugin5_pulumi.pb.go: tfplugin5_pulumi.proto
	protoc --go_out=. "$<"
	mv github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin5/tfplugin5_pulumi.pb.go "$@"

tfplugin6_pulumi.pb.go: tfplugin6_pulumi.proto
	protoc --go_out=. "$<"
	mv github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6/tfplugin6_pulumi.pb.go "$@"

tfplugin5_pulumi_grpc.pb.go: tfplugin5.proto
	protoc --go-grpc_out=. "$<"
	mv github.com/opentofu/opentofu/internal/tfplugin5/tfplugin5_grpc.pb.go "$@"

tfplugin6_pulumi_grpc.pb.go: tfplugin6.proto
	protoc --go-grpc_out=. "$<"
	mv github.com/opentofu/opentofu/internal/tfplugin6/tfplugin6_grpc.pb.go "$@"

install:: tfplugin5_pulumi.pb.go tfplugin6_pulumi.pb.go tfplugin5_pulumi_grpc.pb.go tfplugin6_pulumi_grpc.pb.go
	mkdir -p "${out}"
	cp $^ "${out}/"

vendor:: vendor-tfplugin
	go generate

# This target runs outside of the nix environment.
vendor-tfplugin::
	nix build
	rm -rf tfplugin5
	mkdir -p tfplugin5
	cp result/tfplugin5* ./tfplugin5/
	rm -rf tfplugin6
	mkdir -p tfplugin6
	cp result/tfplugin6* ./tfplugin6/
