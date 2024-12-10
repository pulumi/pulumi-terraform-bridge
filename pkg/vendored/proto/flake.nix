{
  inputs = {
    nixpkgs.url = github:NixOS/nixpkgs/nixos-24.05;
    opentofu_src.url = github:opentofu/opentofu/v1.7.3;
    opentofu_src.flake = false;

  };

  outputs = { self, nixpkgs, opentofu_src }: let
    packages = sys: let
      pkgs = import nixpkgs { system = sys; };

      tfplugin6-protos = pkgs.stdenv.mkDerivation {
        coreutils = pkgs.coreutils;
        gnused = pkgs.gnused;
        name = "tfplugin6-protos";
        builder = "${pkgs.bash}/bin/bash";
        args = [ "-c" ''
          export PATH=$coreutils/bin:$gnused/bin
          mkdir -p $out
          orig=github.com/opentofu/opentofu/internal/tfplugin6
          dest=github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6
          cat $src/docs/plugin-protocol/tfplugin6.5.proto |
              sed -r "s#$orig#$dest#g" |
              sed -r "s#package tfplugin6;#package tfplugin6_pulumi;#g" >$out/tfplugin6_pulumi.proto
        ''];
        src = opentofu_src;
        system = sys;
      };

      tfplugin5-protos = pkgs.stdenv.mkDerivation {
        coreutils = pkgs.coreutils;
        gnused = pkgs.gnused;
        name = "tfplugin5-protos";
        builder = "${pkgs.bash}/bin/bash";
        args = [ "-c" ''
          export PATH=$coreutils/bin:$gnused/bin
          mkdir -p $out
          orig=github.com/opentofu/opentofu/internal/tfplugin5
          dest=github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin5
          cat $src/docs/plugin-protocol/tfplugin5.5.proto |
              sed -r "s#$orig#$dest#g" |
              sed -r "s#package tfplugin5;#package tfplugin5_pulumi;#g" >$out/tfplugin5_pulumi.proto
        ''];
        src = opentofu_src;
        system = sys;
      };

      tfplugin6-go = pkgs.stdenv.mkDerivation {
        name = "tfplugin6-go";
        src = ./.;
        builder = "${pkgs.bash}/bin/bash";
        coreutils = pkgs.coreutils;
        protoc = pkgs.protobuf3_20;
        gengo = pkgs.protoc-gen-go;
        gengogrpc = pkgs.protoc-gen-go-grpc;
        args = [ "-c" ''
          set -euo pipefail
          export PATH=$coreutils/bin:$protoc/bin:$gengo/bin:$gengogrpc/bin
          mkdir -p $out
          cp ${tfplugin6-protos}/tfplugin6_pulumi.proto $out/tfplugin6_pulumi.proto
          cd $out
          protoc --go_out=. --go-grpc_out=. --proto_path ${tfplugin6-protos} ${tfplugin6-protos}/tfplugin6_pulumi.proto
          mv $out/github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin6/* $out/
          rm -rf $out/github.com
        ''];
        system = sys;
      };

      tfplugin5-go = pkgs.stdenv.mkDerivation {
        name = "tfplugin5-go";
        src = ./.;
        builder = "${pkgs.bash}/bin/bash";
        coreutils = pkgs.coreutils;
        protoc = pkgs.protobuf3_20;
        gengo = pkgs.protoc-gen-go;
        gengogrpc = pkgs.protoc-gen-go-grpc;
        args = [ "-c" ''
          set -euo pipefail
          export PATH=$coreutils/bin:$protoc/bin:$gengo/bin:$gengogrpc/bin
          mkdir -p $out
          cp ${tfplugin5-protos}/tfplugin5_pulumi.proto $out/tfplugin5_pulumi.proto
          cd $out
          protoc --go_out=. --go-grpc_out=. --proto_path ${tfplugin5-protos} ${tfplugin5-protos}/tfplugin5_pulumi.proto
          mv $out/github.com/pulumi/pulumi-terraform-bridge/v3/pkg/vendored/tfplugin5/* $out/
          rm -rf $out/github.com
        ''];
        system = sys;
      };

    in {
      inherit tfplugin6-go tfplugin5-go;

      default = pkgs.symlinkJoin { name = "protos"; paths = [ tfplugin6-go tfplugin5-go ]; };
    };
  in {
    packages = {
      "x86_64-darwin" = packages "x86_64-darwin";
      "aarch64-darwin" = packages "aarch64-darwin";
      "x86_64-linux" = packages "x86_64-linux";
      "aarch64-linux" = packages "aarch64-linux";
    };
  };
}
