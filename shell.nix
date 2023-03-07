{ pkgs ? import <nixpkgs> {} }:

let gotk4-nix = pkgs.fetchFromGitHub {
		owner = "diamondburned";
		repo  = "gotk4-nix";
		rev   = "b5bb50b31ffd7202decfdb8e84a35cbe88d42c64";
		hash  = "sha256:18wxf24shsra5y5hdbxqcwaf3abhvx1xla8k0vnnkrwz0r9n4iqq";
	};

in import "${gotk4-nix}/shell.nix" {
	base = {
		pname = "gotkit";
		version = "dev";
	};

	buildInputs = pkgs: with pkgs; [
		# staticcheck takes forever to build gotk4 twice. I'm good.
		(writeShellScriptBin "staticcheck" "")
	];
}
