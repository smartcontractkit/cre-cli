{
  description = "cre-cli dev shell";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs"; # or nixpkgs-unstable

  outputs = { self, nixpkgs }:
    let
      system = "aarch64-darwin";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        packages = [
          pkgs.go_1_25
        ];
      };
    };
}
