{
  inputs = {
    nixpkgs.url = "nixpkgs";
    flake-utils.url = "flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        gopls = pkgs.gopls.override {
          buildGoModule = pkgs.buildGo121Module;
        };
      in {
        checks.package = self.packages.${system}.default;

        packages.default = pkgs.callPackage ./package.nix {
          buildGoModule = pkgs.buildGo121Module;
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.delve
            pkgs.go-tools
            pkgs.gotools

            gopls
          ];

          inputsFrom = [
            self.packages.${system}.default
          ];

          # For Delve + cgo
          # (see https://nixos.wiki/wiki/Go#Using_cgo_on_NixOS):
          hardeningDisable = [ "fortify" ];
        };
      }
    );
}
