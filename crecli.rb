require "open3"
class Crecli < Formula
  desc "Command-line Interface for Chainlink Runtime Environment"
  homepage "https://chain.link/chainlink-runtime-environment"
  url "https://github.com/smartcontractkit/cre-cli/archive/refs/tags/v1.0.1.tar.gz"
  sha256 "912ef93ed73ae278960982355e4e738770332aedebfc48939afee4c34e5521ba"
  license "MIT"
  head "https://github.com/smartcontractkit/cre-cli.git", branch: "main"

  depends_on "go"
  depends_on "oven-sh/bun/bun" # TODO: `brew audit` flags this since it's from a tap, need to fix

  def install
    # This build command replicates the logic from the project's Makefile.
    # It injects the version number into the binary using ldflags.
    # - `version` is a helper that pulls the version from the `url` tag (e.g., "0.1.1").
    # - `bin` is the directory where executables should be installed (`/usr/local/bin`).
    # - `#{bin}/cre` is the destination path for the compiled binary.
    system "go", "build", "-v", "-ldflags", "-X github.com/smartcontractkit/cre-cli/cmd/version.Version=v#{version}", "-o", bin/"cre", "."
  end

  test do
    # This test block runs after installation to verify it works.
    # It executes `cre version` and checks if the output contains the correct version string.
    # Execute the command and capture stdout, stderr, and the exit status
    stdout_str, stderr_str, status = Open3.capture3("#{bin}/cre version")
    # 1. Assert that the command was successful (exit code 0)
    assert status.success?, "Command failed with exit code #{status.exitstatus}"

    # 2. Assert that the standard error stream was empty
    assert_empty stderr_str, "Command produced unexpected stdout: #{stderr_str}"

    # 3. Assert that the standard error contains the correct version string
    assert_match "cre v#{version}", stdout_str
  end
end
