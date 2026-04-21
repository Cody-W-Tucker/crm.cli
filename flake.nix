{
  description = "A Nix-flake-based Bun development environment";

  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1"; # unstable Nixpkgs
    bun2nix = {
      url = "github:nix-community/bun2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    { self, ... }@inputs:

    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forEachSupportedSystem =
        f:
        inputs.nixpkgs.lib.genAttrs supportedSystems (
          system:
          f {
            inherit system;
            pkgs = import inputs.nixpkgs { inherit system; };
          }
        );
    in
    {
      devShells = forEachSupportedSystem (
        { pkgs, system }:
        {
          default = pkgs.mkShellNoCC {
            packages = with pkgs; [
              bun
              nodejs
              self.formatter.${system}
              inputs.bun2nix.packages.${system}.default
            ];
          };
        }
      );

      packages = forEachSupportedSystem (
        { pkgs, system }:
        if pkgs.stdenv.hostPlatform.isLinux then
          {
            crm-cli = pkgs.callPackage ./nix/package.nix {
              bun2nix = inputs.bun2nix.packages.${system}.default;
            };
            default = self.packages.${system}.crm-cli;
          }
        else
          { }
      );

      apps = forEachSupportedSystem (
        { system, ... }:
        if builtins.hasAttr "crm-cli" self.packages.${system} then
          {
            crm-cli = {
              type = "app";
              program = "${builtins.getAttr "crm-cli" self.packages.${system}}/bin/crm";
            };
            default = builtins.getAttr "crm-cli" self.apps.${system};
          }
        else
          { }
      );

      homeManagerModules.default = import ./nix/modules/home-manager.nix { inherit self; };

      formatter = forEachSupportedSystem ({ pkgs, ... }: pkgs.nixfmt);
    };
}
