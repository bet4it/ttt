{
  description = "TTT Editor: Terminal Text Tool";

  inputs = {
    nixpkgs.url = "git+file:///home/user/Codes/nixpkgs?ref=fff-c";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = self.shortRev or self.dirtyShortRev or "dev";
      in
      {
        packages = rec {
          ttt = pkgs.buildGoModule {
            pname = "ttt";
            inherit version;
            src = self;
            vendorHash = "sha256-UxT3zsTPsokAwMiMQ1fE5QTK+95GstaOUz8RlFJhfvQ=";

            tags = [ "fff" ];
            nativeBuildInputs = [ pkgs.pkg-config ];
            buildInputs = [ pkgs.fff-c ];
            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
            ];
            subPackages = [ "cmd/ttt" ];
            meta = with pkgs.lib; {
              description = "Terminal Text Tool — an IDE that lives in your terminal";
              homepage = "https://github.com/eugenioenko/ttt";
              license = licenses.mit;
              mainProgram = "ttt";
            };
          };
          default = ttt;
        };
      }
    );
}
