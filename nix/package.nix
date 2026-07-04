{
  buildGoModule,
  lib,
  src,
  version,
  vpsadminGoClientSource ? null,
}:

buildGoModule {
  pname = "vpsf-status";
  inherit src version;

  vendorHash =
    if vpsadminGoClientSource != null then
      "sha256-N6vS4SGVeWkBc23/z8zT1Hbz+MC7BvOsjqJkw+bXcgI="
    else
      "sha256-PjJc/wdD/qiGW0A7OgL1VaM2UDvUBepsFxdfexSHOT8=";

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
