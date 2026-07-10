class Cre < Formula
  desc "Chainlink Runtime Environment CLI"
  homepage "https://chain.link/chainlink-runtime-environment"
  version "1.24.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/smartcontractkit/cre-cli/releases/download/v1.24.0/cre_darwin_arm64.zip"
      sha256 "f4fb8837a6100f9c60cdf78af8fa7f3add681b2fa1938d47cd2786f195ee8756"
    end
    on_intel do
      url "https://github.com/smartcontractkit/cre-cli/releases/download/v1.24.0/cre_darwin_amd64.zip"
      sha256 "98f88553f98830cf275cfa935f83b080839a833691ebe6946f8e7592e88b5c38"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/smartcontractkit/cre-cli/releases/download/v1.24.0/cre_linux_arm64.tar.gz"
      sha256 "302cf8356618c7f02191a2ed1c04544d31dea5014f1f16736e7ca66dd9fb17ff"
    end
    on_intel do
      url "https://github.com/smartcontractkit/cre-cli/releases/download/v1.24.0/cre_linux_amd64.tar.gz"
      sha256 "68e93dcda31ee02cee956575f5455e90748e5dba2971ee7868e33e82f8be6027"
    end
  end

  def install
    arch = Hardware::CPU.arm? ? "arm64" : "amd64"
    platform = OS.mac? ? "darwin" : "linux"
    bin.install Dir["cre_v*_#{platform}_#{arch}"].first => "cre"
  end

  def caveats
    <<~EOS
      Go 1.25.3 or later and Bun 1.0.0 or later are recommended for developing
      and running TypeScript CRE workflows.
    EOS
  end

  test do
    assert_match "CRE CLI version v#{version}", shell_output("#{bin}/cre version")
  end
end
