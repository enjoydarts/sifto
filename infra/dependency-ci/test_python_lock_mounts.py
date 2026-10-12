"""Check resolved Compose mounts and the generator's actual write boundaries."""
import json
import errno
from pathlib import Path
import sys
import tempfile
import unittest


CONFIG = json.load(sys.stdin)
ROOT = Path('/repository')
ALLOWED = {'/repository/worker', '/repository/infra/maintenance', '/repository/infra/audio-concat'}


class GeneratorMountTests(unittest.TestCase):
    def test_resolved_compose_keeps_repository_readonly_and_only_three_overlays_writable(self):
        locks = CONFIG['services']['locks']
        generator = CONFIG['services']['generate']
        self.assertTrue(locks['read_only'])
        self.assertTrue(generator['read_only'])
        self.assertEqual(len(locks['volumes']), 1)
        self.assertEqual(locks['volumes'][0]['target'], '/repository')
        self.assertTrue(locks['volumes'][0]['read_only'])
        self.assertEqual({volume['target'] for volume in generator['volumes'] if not volume.get('read_only', False)}, ALLOWED)
        root = next(volume for volume in generator['volumes'] if volume['target'] == '/repository')
        self.assertTrue(root['read_only'])
        self.assertEqual(root['source'], locks['volumes'][0]['source'])
        for volume in generator['volumes']:
            if volume['target'] in ALLOWED:
                relative = volume['target'].removeprefix('/repository/')
                self.assertEqual(Path(volume['source']), Path(root['source']) / relative)

    def test_actual_generator_refuses_writes_outside_three_directories(self):
        for directory in [ROOT, ROOT / 'api', ROOT / '.github', ROOT / 'infra/dependency-ci']:
            with self.subTest(directory=directory):
                with self.assertRaises(OSError) as refused:
                    # A failed protection check still cleans up its harmless probe.
                    with tempfile.TemporaryDirectory(prefix='.lock-mount-probe-', dir=directory):
                        pass
                self.assertEqual(refused.exception.errno, errno.EROFS)
        for directory in sorted(ALLOWED):
            with self.subTest(directory=directory):
                with tempfile.TemporaryDirectory(prefix='.lock-mount-probe-', dir=directory) as temporary:
                    probe = Path(temporary) / 'write-probe'
                    probe.write_text('mount verification only\n')
                    self.assertEqual(probe.read_text(), 'mount verification only\n')


if __name__ == '__main__':
    unittest.main()
