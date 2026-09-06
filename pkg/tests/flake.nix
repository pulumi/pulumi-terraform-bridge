# Build a Nix flake that defines a development environment.
# Use this flake as an input:
#
#    https://github.com/nyobe/pulumi-flake/tree/v3.146.0



# Here's a Nix flake that defines a development environment using the pulumi-flake as an input:


{
  description = "Development environment with Pulumi";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    pulumi-flake = {
      url = "github:nyobe/pulumi-flake/v3.139.0";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, flake-utils, pulumi-flake }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        pulumi = pulumi-flake.packages.${system};
      in
        {
          devShells.default = pkgs.mkShell {
            buildInputs = [
              pulumi.default
              pkgs.go
              pkgs.git
            ];
          };
        });
}
