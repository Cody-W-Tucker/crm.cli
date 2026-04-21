{
  lib,
  stdenv,
  bun,
  bun2nix,
  fuse3,
  gcc,
  makeWrapper,
  nodejs,
  pkg-config,
  util-linux,
}:

let
  packageJson = lib.importJSON ../package.json;
in
stdenv.mkDerivation {
  pname = "crm-cli";
  inherit (packageJson) version;

  src = lib.cleanSource ../.;

  bunDeps = bun2nix.fetchBunDeps {
    bunNix = ./bun.nix;
  };

  nativeBuildInputs = [
    bun
    bun2nix.hook
    gcc
    makeWrapper
    nodejs
    pkg-config
  ];

  buildInputs = [ fuse3 ];

  bunInstallFlags = [ "--linker=hoisted" ];
  dontRunLifecycleScripts = true;
  dontUseBunBuild = true;
  dontUseBunCheck = true;

  buildPhase = ''
    runHook preBuild
    bun run build
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    mkdir -p "$out/bin" "$out/libexec/crm-cli"
    gcc -o crm-fuse src/fuse-helper.c $(pkg-config --cflags --libs fuse3)

    cp -r dist "$out/libexec/crm-cli/"
    cp -r node_modules "$out/libexec/crm-cli/"
    cp package.json "$out/libexec/crm-cli/"
    install -Dm755 crm-fuse "$out/libexec/crm-cli/crm-fuse"

    makeWrapper ${nodejs}/bin/node "$out/bin/crm" \
      --add-flags "$out/libexec/crm-cli/dist/cli.js" \
      --set CRM_FUSE_HELPER "$out/libexec/crm-cli/crm-fuse" \
      --set CRM_FUSERMOUNT "${fuse3}/bin/fusermount3" \
      --prefix PATH : "${
        lib.makeBinPath [
          fuse3
          util-linux
        ]
      }"

    runHook postInstall
  '';

  meta = {
    description = packageJson.description;
    homepage = "https://github.com/Cody-W-Tucker/crm.cli";
    license = lib.licenses.mit;
    mainProgram = "crm";
    platforms = lib.platforms.linux;
  };
}
