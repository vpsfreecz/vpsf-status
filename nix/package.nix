{
  buildGoModule,
  lib,
  perl,
  src,
  version,
  vpsadminGoClientSource ? null,
}:

buildGoModule {
  pname = "vpsf-status";
  inherit src version;

  nativeCheckInputs = [ perl ];

  vendorHash =
    if vpsadminGoClientSource != null then
      "sha256-N6vS4SGVeWkBc23/z8zT1Hbz+MC7BvOsjqJkw+bXcgI="
    else
      "sha256-f6Xzt4QI30t4sjo544+KRx6CYcXk3T1AxqyOJbv2cC4=";

  postPatch = lib.optionalString (vpsadminGoClientSource != null) ''
    cp -a ${vpsadminGoClientSource} ../vpsadmin-go-client
    chmod -R u+w ../vpsadmin-go-client
    go mod edit -replace github.com/vpsfreecz/vpsadmin-go-client=../vpsadmin-go-client
  '';

  postInstall = ''
    mkdir -p $out/share/vpsf-status
    cp -r public templates $out/share/vpsf-status/
  '';

  meta = {
    description = "Status page for vpsFree.cz infrastructure";
    homepage = "https://github.com/vpsfreecz/vpsf-status";
    license = lib.licenses.mit;
    mainProgram = "vpsf-status";
  };
}
