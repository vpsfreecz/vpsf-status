{
  description = "vpsFree.cz status development shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
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
