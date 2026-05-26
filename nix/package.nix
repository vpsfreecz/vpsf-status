{
  buildGoModule,
  lib,
  src,
  version,
}:

buildGoModule {
  pname = "vpsf-status";
  inherit src version;

  vendorHash = "sha256-I94x4vk+7pCCMRETqx5ga++07ygXlGK5STXAKuGrzzM=";

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
