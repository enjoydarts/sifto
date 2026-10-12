"""Exercise Linux DAC with the same empty capability set as scan services."""
import ctypes
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest


def drop_capabilities():
    class Header(ctypes.Structure):
        _fields_ = [("version", ctypes.c_uint32), ("pid", ctypes.c_int)]

    class Data(ctypes.Structure):
        _fields_ = [("effective", ctypes.c_uint32), ("permitted", ctypes.c_uint32),
                    ("inheritable", ctypes.c_uint32)]

    libc = ctypes.CDLL(None, use_errno=True)
    if libc.capset(ctypes.byref(Header(0x20080522, 0)), (Data * 2)()) != 0:
        raise OSError(ctypes.get_errno(), "capset failed")
    if libc.prctl(38, 1, 0, 0, 0) != 0:  # PR_SET_NO_NEW_PRIVS
        raise OSError(ctypes.get_errno(), "PR_SET_NO_NEW_PRIVS failed")


def child_write(uid, directories):
    read_fd, write_fd = os.pipe()
    pid = os.fork()
    if pid == 0:
        os.close(read_fd)
        try:
            if uid:
                os.setgroups([])
                os.setgid(uid)
                os.setuid(uid)
            drop_capabilities()
            status = dict(line.split(":", 1) for line in Path("/proc/self/status").read_text().splitlines() if ":" in line)
            if int(status["CapEff"].strip(), 16) or int(status["CapPrm"].strip(), 16):
                raise AssertionError("Child retained capabilities")
            if status["NoNewPrivs"].strip() != "1":
                raise AssertionError("Child can gain new privileges")
            written = []
            for directory in directories:
                try:
                    (directory / f"uid-{uid}.json").write_text('{"written": true}\n')
                    written.append(True)
                except PermissionError:
                    written.append(False)
            result = {"uid": os.getuid(), "gid": os.getgid(), "written": written}
        except Exception as error:
            result = {"error": repr(error)}
        os.write(write_fd, json.dumps(result).encode())
        os.close(write_fd)
        os._exit(0)
    os.close(write_fd)
    with os.fdopen(read_fd) as stream:
        result = json.load(stream)
    _, status = os.waitpid(pid, 0)
    if status != 0:
        raise AssertionError("DAC fixture child failed")
    return result


class LinuxDACRegression(unittest.TestCase):
    def test_capless_caller_can_write_reports_and_cache_while_root_cannot(self):
        self.assertEqual(os.geteuid(), 0, "Fixture parent must run as root to establish directory ownership")
        temporary = Path(tempfile.mkdtemp(prefix="sifto-image-dac-", dir="/dev/shm"))
        self.addCleanup(shutil.rmtree, temporary)
        temporary.chmod(0o755)
        directories = [temporary / "reports", temporary / "cache"]
        for directory in directories:
            directory.mkdir(mode=0o755)
            os.chown(directory, 1001, 1001)
        denied = child_write(0, directories)
        allowed = child_write(1001, directories)
        self.assertEqual(denied.get("written"), [False, False], denied)
        self.assertEqual(allowed.get("written"), [True, True], allowed)
        self.assertEqual((allowed["uid"], allowed["gid"]), (1001, 1001))
        for directory in directories:
            self.assertEqual((directory.stat().st_uid, directory.stat().st_gid), (1001, 1001))
            self.assertEqual((directory / "uid-1001.json").stat().st_uid, 1001)
            self.assertEqual(directory.stat().st_mode & 0o777, 0o755)


if __name__ == "__main__":
    unittest.main()
