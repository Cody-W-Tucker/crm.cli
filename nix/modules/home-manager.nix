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
in
{
  options.programs."crm-cli" = {
    enable = lib.mkEnableOption "crm-cli";

    package = lib.mkOption {
      type = lib.types.nullOr lib.types.package;
      default =
        if builtins.hasAttr "crm-cli" self.packages.${pkgs.system} then
          builtins.getAttr "crm-cli" self.packages.${pkgs.system}
        else
          null;
      defaultText = lib.literalExpression "self.packages.${pkgs.system}.crm-cli";
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
        message = "programs.crm-cli.package is null for ${pkgs.system}; provide a package explicitly.";
      }
    ];

    home.packages = [ cfg.package ];
    home.file.".crm/config.toml".source = tomlFormat.generate "crm-cli-config.toml" cfg.settings;
  };
}
