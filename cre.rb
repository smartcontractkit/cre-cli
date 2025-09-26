require "open3"
class Cre < Formula
  desc "Command-line Interface for Chainlink Runtime Environment"
  homepage "https://github.com/smartcontractkit/cre-cli"
  url "https://github.com/smartcontractkit/cre-cli/archive/refs/tags/v0.6.0-alpha.0.tar.gz"
  sha256 "6d42b6953950ce4c8deeebe782844ebc817dc6e77def2def5c5a075ba0446dc8"
  license "MIT"

  depends_on "go" => :build

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

    # 2. Assert that the standard output stream was empty
    assert_empty stdout_str, "Command produced unexpected stdout: #{stdout_str}"

    # 3. Assert that the standard error contains the correct version string
    assert_match "cre v#{version}", stderr_str
  end
end
