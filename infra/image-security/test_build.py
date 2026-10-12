"""Exercise public Web build configuration without building or loading an image."""
import base64
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


BUILD_SCRIPT = Path(__file__).with_name("build.sh").resolve()
PUBLIC_KEY = "NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY"


class PublicWebBuildConfig(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        # Synthetic public configuration only; never consult a local .env.
        self.key = "pk_live_" + base64.b64encode(b"clerk.example.com$").decode()

    def run_build(self, component, key=None):
        env = {"PATH": os.environ["PATH"], "HOME": str(self.directory)}
        if key is not None:
            env[PUBLIC_KEY] = key
        # Shell functions isolate expensive/external operations while running
        # the real wrapper. A Docker call stops before archive verification.
        shell = '''
df() { printf 'Filesystem 1024-blocks Used Available Capacity Mounted\\nfixture 99999999 0 99999999 0%% /\\n'; }
id() { printf '1001\\n'; }
git() { case "$1" in rev-parse) printf '%040d\\n' 1 ;; status) : ;; esac; }
docker() { printf '%s\\0' "$@" > docker-args; return 93; }
build_script=$1
shift
. "$build_script"
'''
        result = subprocess.run(["sh", "-c", shell, "test-build", str(BUILD_SCRIPT), component],
                                cwd=self.directory, env=env, text=True, capture_output=True)
        args_path = self.directory / "docker-args"
        args = args_path.read_bytes().decode().rstrip("\0").split("\0") if args_path.exists() else []
        return result, args

    def test_missing_or_invalid_web_public_config_fails_before_docker_or_output_writes(self):
        for key in (None, "", "pk_test_" + self.key[8:], "sk_live_secret", "pk_live_",
                    "pk_live_invalid!", "pk_live_" + base64.b64encode(b"not-a-clerk-host").decode(),
                    "pk_live_" + base64.b64encode(b"clerk.example.com$\nother.example.com$").decode(),
                    self.key + "\n"):
            with self.subTest(key_kind="missing" if key is None else "invalid"):
                result, args = self.run_build("web", key)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertIn(PUBLIC_KEY, result.stderr)
                self.assertIn("pk_live_", result.stderr)
                self.assertEqual(args, [], "Invalid config must not invoke Docker")
                self.assertFalse((self.directory / ".image-security").exists())

    def test_web_passes_only_public_config_to_final_production_build(self):
        result, args = self.run_build("web", self.key)
        self.assertEqual(result.returncode, 93, result.stderr)
        build_args = [args[index + 1] for index, argument in enumerate(args) if argument == "--build-arg"]
        self.assertEqual(build_args, ["APP_COMMIT_SHA=" + "0" * 39 + "1", f"{PUBLIC_KEY}={self.key}"])
        self.assertEqual(args[:2], ["buildx", "build"])
        self.assertEqual(args[args.index("--platform") + 1], "linux/amd64")
        self.assertEqual(args[args.index("--file") + 1], "web/Dockerfile")
        self.assertEqual(args[-1], "web")
        self.assertNotIn("--target", args)
        self.assertNotIn("--push", args)
        self.assertNotIn("--secret", args)

    def test_non_web_image_does_not_require_or_forward_web_config(self):
        for component in ("api", "worker", "audio-concat", "maintenance"):
            with self.subTest(component=component):
                result, args = self.run_build(component, "sk_live_invalid")
                self.assertEqual(result.returncode, 93, result.stderr)
                self.assertFalse(any(PUBLIC_KEY in argument for argument in args))


if __name__ == "__main__":
    unittest.main()
