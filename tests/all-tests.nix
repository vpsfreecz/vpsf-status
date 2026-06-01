{
  pkgs ? <nixpkgs>,
  system ? builtins.currentSystem,
  configuration ? null,
  testConfig ? { },
  suiteArgs ? { },
}:
let
  vpsadminosPath = suiteArgs.vpsadminosPath or (throw "suiteArgs.vpsadminosPath is required");
  vpsadminPath = suiteArgs.vpsadminPath or (throw "suiteArgs.vpsadminPath is required");
  vpsfStatusModule = suiteArgs.vpsfStatusModule or (throw "suiteArgs.vpsfStatusModule is required");
  vpsfStatusPackage = suiteArgs.vpsfStatusPackage or (throw "suiteArgs.vpsfStatusPackage is required");
  suiteArgs' = suiteArgs // {
    inherit
      vpsadminosPath
      vpsadminPath
      vpsfStatusModule
      vpsfStatusPackage
      ;
  };

  nixpkgs = import pkgs { inherit system; };
  lib = nixpkgs.lib;
  testLib = import (vpsadminosPath + "/test-runner/nix/lib.nix") {
    inherit
      pkgs
      system
      lib
      configuration
      testConfig
      ;
    suiteArgs = suiteArgs';
    suitePath = ./suite;
  };
in
testLib.makeTests [
  "status-page"
]
