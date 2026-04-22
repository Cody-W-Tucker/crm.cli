{ self }:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs."crm-cli";
  tomlFormat = pkgs.formats.toml { };
  mountPath = cfg.settings.mount.default_path or null;
in
{
  options.programs."crm-cli" = {
    enable = lib.mkEnableOption "crm-cli";

    autoMount = lib.mkEnableOption "mounting crm-cli at login";

    package = lib.mkOption {
      type = lib.types.nullOr lib.types.package;
      default =
        if builtins.hasAttr "crm-cli" self.packages.${pkgs.stdenv.hostPlatform.system} then
          builtins.getAttr "crm-cli" self.packages.${pkgs.stdenv.hostPlatform.system}
        else
          null;
      defaultText = lib.literalExpression "self.packages.${pkgs.stdenv.hostPlatform.system}.crm-cli";
      description = "The crm-cli package to install.";
    };

    settings = lib.mkOption {
      type = tomlFormat.type;
      default = { };
      example = {
        database.path = "~/.crm/crm.db";
        defaults.format = "json";
        mount.default_path = "~/crm";
        mount.readonly = false;
        mount.max_recent_activity = 20;
        mount.search_limit = 50;
        phone.default_country = "US";
        phone.display = "international";
        pipeline.stages = [
          "lead"
          "qualified"
          "proposal"
          "closed-won"
          "closed-lost"
        ];
        pipeline.won_stage = "closed-won";
        pipeline.lost_stage = "closed-lost";
        hooks.pre-contact-add = "echo adding contact";
      };
      description = ''
        crm-cli configuration written to `~/.crm/config.toml`.
        Use this to set the default mount location and any other supported crm.toml options.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = cfg.package != null;
        message = "programs.crm-cli.package is null for ${pkgs.stdenv.hostPlatform.system}; provide a package explicitly.";
      }
      {
        assertion = !(cfg.autoMount && mountPath == null);
        message = ''programs."crm-cli".autoMount requires settings.mount.default_path to be set.'';
      }
      {
        assertion = !(cfg.autoMount && lib.hasPrefix "~/" mountPath);
        message = ''programs."crm-cli".autoMount requires settings.mount.default_path to be an absolute path; `~` is not expanded by crm-cli.'';
      }
    ];

    home.packages = [ cfg.package ];
    home.file.".crm/config.toml".source = tomlFormat.generate "crm-cli-config.toml" cfg.settings;

    systemd.user.services.crm-mount = lib.mkIf cfg.autoMount {
      Unit = {
        Description = "Mount crm-cli filesystem";
      };

      Service = {
        Type = "oneshot";
        RemainAfterExit = true;
        ExecStart = "${lib.getExe cfg.package} mount";
        ExecStop = "${lib.getExe cfg.package} unmount";
      };

      Install = {
        WantedBy = [ "default.target" ];
      };
    };
  };
}
