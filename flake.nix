{
  description = "vpsFree.cz status service";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;
      version = self.shortRev or self.dirtyShortRev or "dirty";
    in
    {
      overlays.default = final: _prev: {
        vpsf-status = final.callPackage ./nix/package.nix {
          src = self;
          inherit version;
        };
      };

      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ self.overlays.default ];
          };
        in
        {
          vpsf-status = pkgs.vpsf-status;
          default = pkgs.vpsf-status;
        });

      nixosModules = {
        vpsf-status =
          { pkgs, lib, ... }:
          {
            imports = [ ./nix/module.nix ];

            services.vpsf-status.package =
              lib.mkDefault self.packages.${pkgs.stdenv.hostPlatform.system}.vpsf-status;
          };

        default = self.nixosModules.vpsf-status;
      };

      devShells = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.mkShell {
            nativeBuildInputs = with pkgs; [
              gnumake
              go
              lefthook
              stdenv.cc
            ];

            shellHook = ''
              if [ -n "$PS1" ]; then
                dev_shell_prompt="(vpsf-status) "
                case "$PS1" in
                  "$dev_shell_prompt"*|\\n"$dev_shell_prompt"*) ;;
                  \\n*) PS1="\\n$dev_shell_prompt''${PS1#\\n}" ;;
                  *) PS1="$dev_shell_prompt$PS1" ;;
                esac
                export PS1
                unset dev_shell_prompt
              fi
            '';
          };
        });
    };
}
