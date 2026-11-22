{
  description = "tempus — one dev shell for local work and CI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        # gremlins is not in nixpkgs. v0.5.1 is the version the mutation
        # thresholds in .mutation-thresholds were measured with.
        gremlins = pkgs.buildGoModule rec {
          pname = "gremlins";
          version = "0.5.1";
          src = pkgs.fetchFromGitHub {
            owner = "go-gremlins";
            repo = "gremlins";
            # v0.5.1 is no longer tagged on GitHub (only v0.5.0 and v0.6.0); this is
            # the commit the Go module proxy records for it (@v/v0.5.1.info).
            rev = "2e26f2326cb9a069b89cf149d90832125c74d8a8";
            hash = "sha256-5TeDwT3x7+Hs+UOT9M/LAcSFgYDjcBzbKR7pOmVdsdA=";
          };
          vendorHash = "sha256-M65FyWMT1Akeuv9fgQi2MvZo9itVxAtOg9u6Ibo2uKg=";
          subPackages = [ "cmd/gremlins" ];
          doCheck = false;
        };
      in
      {
        # make verify locally and every CI job except the release run in this
        # shell, so both use the same tools from flake.lock. Bump them with
        # `nix flake update` as a reviewed change: a new golangci-lint, oasdiff
        # or zizmor can report what the old one did not.
        devShells.default = pkgs.mkShellNoCC {
          name = "tempus";

          packages = with pkgs; [
            go_1_26
            golangci-lint
            govulncheck
            gotools # goimports
            gremlins
            oasdiff
            gcov2lcov
            zizmor
            # CodeCharta's ccsh is an npm package running on the JVM; make
            # codecharta fetches the pinned version with npx.
            nodejs_22
            jdk21_headless
            python3 # scripts/codecharta-ratchet.py
            gnumake
            git
            jq
            yq-go # scripts/check-workflow-jobs.sh
          ];

          # The go binary from nixpkgs only bootstraps. The Go that builds,
          # tests and gets scanned by govulncheck is go.mod's toolchain
          # directive, the one the Dockerfile pins. Not plain GOTOOLCHAIN=auto:
          # that starts from the bundled Go and only ever switches UP, so once
          # nixpkgs ships a newer patch release than go.mod names, the scan
          # would see a newer stdlib than the release binary. <toolchain>+auto
          # starts from the declared toolchain and still switches up when a
          # module requires a newer Go (go get raises the directive then).
          # Read when the shell starts; direnv reloads on go.mod changes.
          shellHook = ''
            if [ -f go.mod ]; then
              GOTOOLCHAIN=$(awk '$1 == "toolchain" { print $2 }' go.mod)
              [ -n "$GOTOOLCHAIN" ] || GOTOOLCHAIN="go$(awk '$1 == "go" { print $2 }' go.mod)"
              export GOTOOLCHAIN="$GOTOOLCHAIN+auto"
            fi
          '';
          # The release binary is static (Dockerfile, .goreleaser.yml), and
          # mkShellNoCC brings no C compiler.
          CGO_ENABLED = "0";
        };
      });
}
