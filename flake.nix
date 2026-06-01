{
  description = "vpsFree.cz status service";

  inputs = {
    vpsadmin.url = "github:vpsfreecz/vpsadmin";
    vpsadminos.follows = "vpsadmin/vpsadminos";
    nixpkgs.follows = "vpsadminos/nixpkgs";
  };

  outputs =
    {
      self,
      nixpkgs,
      vpsadmin,
      vpsadminos,
      ...
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      testSystems = [ "x86_64-linux" ];

      forAllSystems = nixpkgs.lib.genAttrs systems;
      forTestSystems = nixpkgs.lib.genAttrs testSystems;
      version = self.shortRev or self.dirtyShortRev or "dirty";
      hasTestRunner = system: builtins.elem system testSystems;

      suiteArgsFor = system: {
        vpsadminosPath = vpsadminos.outPath;
        vpsadminPath = vpsadmin.outPath;
        vpsfStatusModule = self.nixosModules.vpsf-status;
        vpsfStatusPackage = self.packages.${system}.vpsf-status;
      };
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
        }
        // nixpkgs.lib.optionalAttrs (hasTestRunner system) {
          test-runner = vpsadminos.packages.${system}.test-runner;
        });

      apps = forAllSystems (
        system:
        nixpkgs.lib.optionalAttrs (hasTestRunner system) {
          test-runner = {
            type = "app";
            program = "${vpsadminos.packages.${system}.test-runner}/bin/test-runner";
          };
        }
      );

      tests = forTestSystems (
        system:
        vpsadminos.lib.testFramework.mkTests {
          inherit system;
          pkgsPath = nixpkgs.outPath;
          testsRoot = ./tests;
          suiteArgs = suiteArgsFor system;
        }
      );

      testsMeta = forTestSystems (
        system:
        vpsadminos.lib.testFramework.mkTestsMeta {
          inherit system;
          pkgsPath = nixpkgs.outPath;
          testsRoot = ./tests;
          suiteArgs = suiteArgsFor system;
        }
      );

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
