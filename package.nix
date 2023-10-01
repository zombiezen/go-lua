{ lib
, buildGoModule
, nix-gitignore
}:

let
  vendorHash = null;

  root = ./.;
  patterns = [
    "*.nix"
    "/.github/"
    ".vscode/"
    "result"
    "result-*"
  ];
  src = builtins.path {
    name = "zombiezen-go-lua";
    path = root;
    filter = nix-gitignore.gitignoreFilterPure (_: _: true) patterns root;
  };
in


buildGoModule {
  name = "zombiezen-go-lua";

  inherit src;

  inherit vendorHash;

  subPackages = [ "./cmd/zombiezen-lua" ];
  ldflags = [ "-s" "-w" ];

  checkPhase = ''
    runHook preCheck

    export GOFLAGS=''${GOFLAGS//-trimpath/}

    buildGoDir test ./internal/bufseek
    buildGoDir test ./internal/lua54
    buildGoDir test .
    buildGoDir test ./cmd/zombiezen-lua

    runHook postCheck
  '';

  meta = {
    mainProgram = "zombiezen-lua";
    homepage = "https://pkg.go.dev/zombiezen.com/go/lua";
    maintainers = [ lib.maintainers.zombiezen ];
  };
}
