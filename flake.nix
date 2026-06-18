{
  description = "parallax — TUI for managing systemd-nspawn, Podman, and Docker containers";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs, ... }@inputs:

    let
      goVersion = 26;

      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forEachSupportedSystem =
        f:
        nixpkgs.lib.genAttrs supportedSystems (
          system:
          f {
            pkgs = import nixpkgs {
              inherit system;
              overlays = [ self.overlays.default ];
            };
          }
        );
    in
    {
      overlays.default = final: prev: {
        go = final."go_1_${toString goVersion}";
      };

      packages = forEachSupportedSystem (
        { pkgs }:
        {
          default = pkgs.buildGoModule {
            pname = "parallax";
            version = "0.1.0";
            src = ./.;
            vendorHash = "sha256-NA8JYWsYqb+wbTjbaPA243LO0Ta7a3aM66OlG7X66hA=";
          };
        }
      );

      devShells = forEachSupportedSystem (
        { pkgs }:
        {
          default = pkgs.mkShellNoCC {
            packages = with pkgs; [
              go
              gotools
              golangci-lint
              gopls
              delve
              golangci-lint-langserver
              gcc
            ];
          };
        }
      );
    };
}
