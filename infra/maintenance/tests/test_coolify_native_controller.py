"""PUBLIC isolated native entrypoint/source-closure contracts."""
import hashlib
import json
import os
from pathlib import Path
import py_compile
import shutil
import subprocess
import sys
import unittest


SOURCES = ('host_agent.py', 'coolify_native_controller.py', 'coolify_native_engine.py',
    'maintenance/__init__.py', 'maintenance/plan.py', 'maintenance/host.py',
    'maintenance/coolify_host.py', 'maintenance/coolify_plan.py',
    'maintenance/coolify_rehearsal.py', 'maintenance/coolify_evidence.py',
    'maintenance/coolify_docker.py', 'maintenance/coolify_rehearsal_engine.py',
    'maintenance/coolify_systemd.py', 'maintenance/coolify_native_protocol.py',
    'maintenance/coolify_cleanup.py', 'maintenance/coolify_native_owner.py',
    'maintenance/coolify-public-manifest.json', 'maintenance/coolify_probe.php',
    'maintenance/coolify_startup.php', 'maintenance/coolify_startup.py', 'maintenance/coolify_startup_init',
    'maintenance/coolify_startup_middleware.php', 'maintenance/coolify_startup_nginx.conf',
    'maintenance/coolify_startup_site.conf',
    'maintenance/coolify_horizon_inspect.php',
    'systemd/sifto-coolify-native-controller.service')


class NativeEntrypointTests(unittest.TestCase):
    def test_n1_exact_closure_covers_all_native_modules_entrypoints_and_service(self):
        from maintenance import coolify_docker
        self.assertEqual(coolify_docker._SOURCE_NAMES, frozenset(SOURCES))

    def install(self):
        source = Path(__file__).resolve().parents[1]
        for name in SOURCES:
            self.assertTrue((source / name).is_file(), 'native source missing: ' + name)
        installed = Path('/opt/sifto-maintenance')
        installed.mkdir(mode=0o755, exist_ok=True)
        self.addCleanup(shutil.rmtree, installed)
        hashes = {}
        for name in SOURCES:
            target = installed / name
            target.parent.mkdir(mode=0o755, parents=True, exist_ok=True)
            shutil.copyfile(source / name, target)
            target.chmod(0o644)
            hashes[name] = hashlib.sha256(target.read_bytes()).hexdigest()
        manifest = Path('/etc/sifto-maintenance/coolify-native-sources.json')
        manifest.parent.mkdir(mode=0o700, exist_ok=True)
        manifest.write_text(json.dumps({'version': 1, 'sources': hashes}))
        manifest.chmod(0o600)
        self.addCleanup(manifest.unlink)
        return installed, manifest

    def run_entrypoint(self, installed, name, args=()):
        return subprocess.run([sys.executable, '-I', '-B', str(installed / name), *args],
            input=b'', stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            env={'PATH': '/usr/bin:/bin'}, timeout=10)

    def test_both_entrypoints_require_exact_source_manifest_before_any_local_import(self):
        installed, manifest = self.install()
        package = installed / 'maintenance/__init__.py'
        marker = Path('/tmp/PUBLIC-native-unvalidated-import')
        self.addCleanup(lambda: marker.unlink(missing_ok=True))
        package.write_text('from pathlib import Path\nPath("/tmp/PUBLIC-native-unvalidated-import").touch()\n')
        value = json.loads(manifest.read_text())
        value['sources']['maintenance/__init__.py'] = hashlib.sha256(package.read_bytes()).hexdigest()
        manifest.write_text(json.dumps(value))
        # The sentinel package is hash-authorized. Rejection must come from the
        # defective resource, before *any* local import can execute the sentinel.
        target = installed / 'maintenance/coolify_cleanup.py'
        target.chmod(0o666)
        for name in ('coolify_native_controller.py', 'coolify_native_engine.py'):
            result = self.run_entrypoint(installed, name)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout, b'')
            self.assertFalse(marker.exists())
            self.assertLessEqual(len(result.stderr), 80)

    def test_every_installed_resource_rejects_unsafe_metadata_before_authorized_package_import(self):
        installed, manifest = self.install()
        package = installed / 'maintenance/__init__.py'
        marker = Path('/tmp/PUBLIC-native-all-closure-import')
        self.addCleanup(lambda: marker.unlink(missing_ok=True))
        package.write_text('from pathlib import Path\nPath("/tmp/PUBLIC-native-all-closure-import").touch()\n')
        value = json.loads(manifest.read_text())
        value['sources']['maintenance/__init__.py'] = hashlib.sha256(package.read_bytes()).hexdigest()
        manifest.write_text(json.dumps(value))
        for name in SOURCES:
            target = installed / name
            original = target.read_bytes()
            entry = 'coolify_native_engine.py' if name == 'coolify_native_controller.py' else 'coolify_native_controller.py'
            for defect in ('missing', 'writable', 'nonroot', 'symlink', 'directory', 'fifo', 'hardlink'):
                with self.subTest(resource=name, defect=defect):
                    target.unlink()
                    link = installed / 'PUBLIC-hardlink'
                    if defect == 'symlink':
                        target.symlink_to(installed / 'maintenance/plan.py')
                    elif defect == 'directory':
                        target.mkdir()
                    elif defect == 'fifo':
                        os.mkfifo(target)
                    elif defect != 'missing':
                        target.write_bytes(original)
                        target.chmod(0o666 if defect == 'writable' else 0o644)
                        if defect == 'nonroot':
                            os.chown(target, 65534, 65534)
                        elif defect == 'hardlink':
                            os.link(target, link)
                    try:
                        result = self.run_entrypoint(installed, entry)
                        self.assertEqual(result.returncode, 1)
                        self.assertEqual(result.stderr, b'native bootstrap rejected\n')
                        self.assertEqual(result.stdout, b'')
                        self.assertFalse(marker.exists())
                    finally:
                        link.unlink(missing_ok=True)
                        if target.is_dir() and not target.is_symlink():
                            target.rmdir()
                        else:
                            target.unlink(missing_ok=True)
                        target.write_bytes(original)
                        target.chmod(0o644)

    def test_root_manifest_is_authority_and_rejects_extra_missing_or_boolean_schema(self):
        installed, manifest = self.install()
        original = json.loads(manifest.read_text())
        for defect in ('extra', 'missing', 'boolean'):
            value = json.loads(json.dumps(original))
            if defect == 'extra':
                value['sources']['PUBLIC-extra.py'] = 'a' * 64
            elif defect == 'missing':
                del value['sources']['maintenance/coolify_cleanup.py']
            else:
                value['version'] = True
            manifest.write_text(json.dumps(value))
            result = self.run_entrypoint(installed, 'coolify_native_controller.py')
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stderr, b'native bootstrap rejected\n')

    def test_native_entrypoints_ignore_poisoned_timestamp_valid_bytecode_and_write_none(self):
        installed, manifest = self.install()
        source = installed / 'maintenance/__init__.py'
        clean = source.read_bytes()
        old = source.stat()
        marker = Path('/tmp/PUBLIC-native-poisoned-cache')
        self.addCleanup(lambda: marker.unlink(missing_ok=True))
        malicious = b'from pathlib import Path; Path("/tmp/PUBLIC-native-poisoned-cache").touch()\n'
        # Equal source length and timestamp make the poison a valid legacy cache.
        clean = clean + b' ' * max(0, len(malicious) - len(clean))
        source.write_bytes(malicious.ljust(len(clean), b' '))
        os.utime(source, ns=(old.st_atime_ns, old.st_mtime_ns))
        py_compile.compile(str(source), doraise=True)
        source.write_bytes(clean)
        os.utime(source, ns=(old.st_atime_ns, old.st_mtime_ns))
        value = json.loads(manifest.read_text())
        value['sources']['maintenance/__init__.py'] = hashlib.sha256(clean).hexdigest()
        manifest.write_text(json.dumps(value))
        before = sorted(installed.rglob('*.pyc'))
        for name in ('coolify_native_controller.py', 'coolify_native_engine.py'):
            result = self.run_entrypoint(installed, name)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(result.stdout, b'')
            self.assertFalse(marker.exists())
        self.assertEqual(sorted(installed.rglob('*.pyc')), before)

    def test_no_argument_override_and_controller_service_exact_resource_caps(self):
        installed, _ = self.install()
        result = self.run_entrypoint(installed, 'coolify_native_controller.py', ('PUBLIC-path',))
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stderr, b'native bootstrap rejected\n')
        text = (installed / 'systemd/sifto-coolify-native-controller.service').read_text()
        for line in ('ExecStart=/usr/bin/python3 -I -B /opt/sifto-maintenance/coolify_native_controller.py',
                     'MemoryMax=64M', 'MemorySwapMax=0', 'CPUQuota=2.5%', 'TasksMax=16',
                     'KillMode=control-group', 'SendSIGKILL=yes', 'Restart=no',
                     'RuntimeDirectory=sifto-maintenance/coolify-native', 'RuntimeDirectoryMode=0700'):
            self.assertIn(line, text.splitlines())
        self.assertNotIn('[Install]', text)

    def test_reading_verified_source_may_update_atime_without_bootstrap_rejection(self):
        installed, _ = self.install()
        source = installed / 'maintenance/coolify_native_owner.py'
        os.utime(source, ns=(0, source.stat().st_mtime_ns))
        result = self.run_entrypoint(installed, 'coolify_native_controller.py')
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stderr, b'native request rejected\n')


if __name__ == '__main__':
    unittest.main()
