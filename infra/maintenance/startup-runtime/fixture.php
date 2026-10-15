<?php
// PUBLIC fixture setup only; observer is always a separate direct Docker exec.
declare(strict_types=1);
$fixtureStart = hrtime(true);
if ($argc === 2 && $argv[1] === 'extra-process') {
    echo "PUBLIC EXTRA PROCESS STARTED\n"; fflush(STDOUT);
    usleep(45000000); exit;
}
if ($argc === 2 && $argv[1] === 'source-runtime') {
    $rows = [];
    foreach (['php-fpm', 'nginx', 'reverb', 'terminal-server', 'horizon', 'scheduler-worker'] as $name) {
        $rows[$name] = ['original_sha256' => hash_file('sha256', '/etc/s6-overlay/s6-rc.d/'.$name.'/run'),
            'copied_sha256' => hash_file('sha256', '/run/s6-rc/servicedirs/'.$name.'/run')];
        if (in_array($name, ['php-fpm', 'nginx'], true)) {
            $rows[$name]['original_fd_hex'] = bin2hex(file_get_contents('/etc/s6-overlay/s6-rc.d/'.$name.'/notification-fd'));
            $rows[$name]['copied_fd_hex'] = bin2hex(file_get_contents('/run/s6-rc/servicedirs/'.$name.'/notification-fd'));
        }
    }
    echo json_encode($rows, JSON_THROW_ON_ERROR)."\n"; exit;
}
if ($argc === 2 && $argv[1] === 'runtime-observe') {
    // Test-only auto_prepend diagnosis; product execution remains a direct exec.
    register_shutdown_function(function () use ($fixtureStart): void {
        global $failure;
        if ($failure instanceof Throwable) {
            $trace = array_map(fn ($frame) => ['line' => $frame['line'] ?? 0,
                'function' => $frame['function'] ?? ''], $failure->getTrace());
            fwrite(STDERR, json_encode(['elapsed_ms' => (int)((hrtime(true) - $fixtureStart) / 1000000),
                'line' => $failure->getLine(), 'trace' => $trace], JSON_THROW_ON_ERROR)."\n");
        }
    });
    return;
}
if (PHP_SAPI !== 'cli' || $argc !== 2 || $argv[1] !== 'prepare-hmac'
    || posix_geteuid() !== 9999 || file_exists('/probe/hmac.key')) { exit(1); }
$mask = umask(0077);
$stream = fopen('/probe/hmac.key', 'xb');
if (!is_resource($stream) || fwrite($stream, str_repeat('P', 32)) !== 32
    || !fflush($stream) || !fclose($stream)) { exit(1); }
umask($mask);
echo "{\"ok\":true,\"category\":\"public_hmac_prepared\"}\n";
