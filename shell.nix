{ pkgs ? import <nixpkgs> {} }:

let
	gotk4-nix = pkgs.fetchFromGitHub {
		owner  = "diamondburned";
		repo   = "gotk4-nix";
		rev    = "4eab6a0";
		sha256 = "sha256-WsJ2Cf1hvKT3BUYYVxQ5rNMYi6z7NWccbSsw39lgqO8=";
	};
in

import "${gotk4-nix}/shell.nix" {
	base = {
		pname = "gotkit";
		version = "dev";
	};

	buildInputs = pkgs: with pkgs; [
		libadwaita

		# staticcheck takes forever to build gotk4 twice. I'm good.
		(writeShellScriptBin "staticcheck" "")
	];
}
