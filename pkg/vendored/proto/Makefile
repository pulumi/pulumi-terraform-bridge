build::
	nix build
	# cp result/tfplugin5* ../tfplugin5/
	# cp result/tfplugin6* ../tfplugin6/
	chmod a+w ../tfplugin5/*
	chmod a+w ../tfplugin6/*

	cp result/tfplugin5_pulumi.pb.go        ../tfplugin5/tfplugin5_pulumi.pb.go
	cp result/tfplugin5_pulumi.proto	../tfplugin5/tfplugin5_pulumi.proto
	cp result/tfplugin5_pulumi_grpc.pb.go   ../tfplugin5/tfplugin5_pulumi_grpc.pb.go
	cp result/tfplugin6_pulumi.pb.go	../tfplugin6/tfplugin6_pulumi.pb.go
	cp result/tfplugin6_pulumi.proto	../tfplugin6/tfplugin6_pulumi.proto
	cp result/tfplugin6_pulumi_grpc.pb.go	../tfplugin6/tfplugin6_pulumi_grpc.pb.go
	chmod a+w ../tfplugin5/*
	chmod a+w ../tfplugin6/*

.PHONY: build
