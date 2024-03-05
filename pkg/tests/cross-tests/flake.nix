{
  description = "A custom development shell for running cross-tests";

  inputs = {
    # branch: nixos-unstable
    nixpkgs.url = "github:NixOS/nixpkgs/842d9d80cfd4560648c785f8a4e6f3b096790e19";
    pulumi-yaml-src = {
      # branch: t0yv0/with-pulumi-debug-support
      url = "github:pulumi/pulumi-yaml/400ac91e791a1912e2e91811517a73a9dc4df026";
      flake = false;
    };
    pulumi-src = {
      # branch: t0yv0/schema-loader-respects-debug-providers-backup
      url = "github:pulumi/pulumi/ddb43d57b2009e28752d4112dfbbc5db62c0b59b";
      flake = false;
    };
  };

  outputs = { self, nixpkgs, pulumi-yaml-src, pulumi-src }: let

    make-pulumi-yaml = pkgs:
      pkgs.buildGo121Module rec {
        name = "pulumi-yaml";
        src = pulumi-yaml-src;
        subPackages = [ "cmd/pulumi-language-yaml" ];
        vendorHash = "sha256-Gy1UK65s8bJBylT7ueEhLRbBcgvfarR1WB/4D/QqeS4=";
        doCheck = false;
      };

    make-pulumi = pkgs:
      pkgs.buildGo121Module rec {
        name = "pulumi";
        subPackages = [ "cmd/pulumi" ];
        modRoot = "pkg";
        src = pulumi-src;
        vendorHash = "sha256-e9OL/wG6fjxof5HuUskV6oCRBY0lYANo7WcweB/o/vY=";
        doCheck = false;
      };

    make-pulumi-with-yaml = pkgs:
      pkgs.symlinkJoin {
        name = "pulumi-with-yaml";
        paths = [
          (make-pulumi pkgs)
          (make-pulumi-yaml pkgs)
        ];
      };

    make-shell = system: let
      pkgs = import nixpkgs { system = system; };
    in
      pkgs.mkShellNoCC {
        buildInputs = [ (make-pulumi-with-yaml pkgs) ];
      };

  in {
    devShells.x86_64-darwin.default = make-shell "x86_64-darwin";
  };
}
