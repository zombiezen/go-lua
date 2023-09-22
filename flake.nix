{
  inputs = {
    nixpkgs.url = "nixpkgs";
    flake-utils.url = "flake-utils";
  };

  outputs = { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        gopls = pkgs.gopls.override {
          buildGoModule = pkgs.buildGo121Module;
        };
      in {
        checks.goTest =
          let
            inherit (pkgs) nix-gitignore;

            root = ./.;
            patterns = [
              "*.nix"
              "/.github/"
              ".vscode/"
              "result"
              "result-*"
            ];
            src = builtins.path {
              name = "go-lua-source";
              path = root;
              filter = nix-gitignore.gitignoreFilterPure (_: _: true) patterns root;
            };

            args = {
              inherit src;
              buildInputs = [ pkgs.go_1_21 ];
            };
          in pkgs.runCommandCC "go-lua-test" args ''
            cd "$src"
            export GOCACHE="$TMPDIR/gocache"
            export GOMODCACHE="$TMPDIR/gomod"
            go test -v -mod=readonly -race ./...
            touch $out
          '';

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.delve
            pkgs.go-tools
            pkgs.go_1_21
            pkgs.gotools

            gopls
          ];

          # For Delve + cgo
          # (see https://nixos.wiki/wiki/Go#Using_cgo_on_NixOS):
          hardeningDisable = [ "fortify" ];
        };
      }
    );
}
