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
      "sha256-1+51I1kIJMVq5gBcrh7oPvFJiUgdcbUnjqb0y40SoRo="
    else
      "sha256-D8wImkdUvapGiudMcbvtGwkPOjbQHGKEGPGY7A4yFa4=";

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
