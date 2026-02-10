{
  description = "Go 1.25 dev shell";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs";

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux"; # or aarch64-darwin, x86_64-darwin
      pkgs = import nixpkgs { inherit system; };
    in {
      devShells.${system}.default = pkgs.mkShell {
        packages = [
          pkgs.go_1_25
        ];
      };
    };
}
