{
  inputs = {
    nixpkgs.url = github:NixOS/nixpkgs/nixos-24.05;
    opentofu_src.url = github:opentofu/opentofu/v1.7.2;
    opentofu_src.flake = false;
  };

  outputs = { self, nixpkgs, opentofu_src }: let

  packages = sys: let
    pkgs = import nixpkgs { system = sys; };
    defaultPkg = pkgs.stdenv.mkDerivation {
      name = "tfplugin-protos";
      src = ./.;
      buildInputs = [
        pkgs.gnused
        pkgs.protobuf3_20
        pkgs.protoc-gen-go
        pkgs.protoc-gen-go-grpc
      ];
      OPENTOFU = "${opentofu_src}";
    };
  in {
    default = defaultPkg;
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
