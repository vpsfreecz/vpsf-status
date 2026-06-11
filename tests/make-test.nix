testFn:
{
  testFramework,
  vpsadminPath,
  vpsfStatusModule,
  vpsfStatusPackage,
  ...
}@args:
let
  upstream = testFramework.makeTest testFn;
  mergedExtraArgs = {
    vpsadminos = testFramework.sourcePath;
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
