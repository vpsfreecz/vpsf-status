testFn:
{
  vpsadminosPath,
  vpsadminPath,
  vpsfStatusModule,
  vpsfStatusPackage,
  ...
}@args:
let
  upstream = import (vpsadminosPath + "/tests/make-test.nix") testFn;
  mergedExtraArgs = {
    vpsadminos = vpsadminosPath;
    vpsadmin = vpsadminPath;
    inherit
      vpsfStatusModule
      vpsfStatusPackage
      ;
  }
  // (args.extraArgs or { });
  argsWithExtra = args // {
    extraArgs = mergedExtraArgs;
  };
in
upstream argsWithExtra
