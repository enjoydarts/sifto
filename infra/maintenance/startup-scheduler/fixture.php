<?php
// PUBLIC synthetic scheduler events; stock S6/schedule:work/Process are unchanged.
declare(strict_types=1);
namespace App\Console;
require_once '/var/www/html/vendor/autoload.php';

class Kernel extends \Illuminate\Foundation\Console\Kernel {
    protected function commands(): void {}
    protected function schedule(\Illuminate\Console\Scheduling\Schedule $schedule): void {
        $mode = trim((string)@file_get_contents('/scheduler-state/case'));
        if (!in_array($mode, ['normal', 'detached', 'pipefull'], true)) { return; }
        $root = '/scheduler-state/'.$mode;
        if ($mode === 'detached') {
            $schedule->exec('php /scheduler/fixture.php background')->everyMinute()->runInBackground();
            return;
        }
        $schedule->call(static function () use ($root, $mode): void {
            file_put_contents($root.'/ready', 'PUBLIC');
            $deadline = hrtime(true) + 45000000000;
            while (!is_file($root.'/release')) {
                if (hrtime(true) >= $deadline) { throw new \RuntimeException('PUBLIC fixture gate timeout'); }
                usleep(10000);
            }
            if ($mode === 'pipefull') {
                // Far above the actual Process pipe capacity. No pipe is closed
                // by the observer; resuming the original owner drains it.
                for ($n = 0; $n < 256; $n++) { echo str_repeat('P', 65536); }
            }
            file_put_contents($root.'/first-finished', 'PUBLIC');
        })->name('public-first')->everyMinute();
        if ($mode === 'normal') {
            $schedule->call(static function () use ($root): void {
                file_put_contents($root.'/late', 'PUBLIC');
                $deadline = hrtime(true) + 45000000000;
                while (!is_file($root.'/release-late')) {
                    if (hrtime(true) >= $deadline) { throw new \RuntimeException('PUBLIC late gate timeout'); }
                    usleep(10000);
                }
                file_put_contents($root.'/done', 'PUBLIC');
            })->name('public-later-selected-before-barrier')->everyMinute();
        }
    }
}

if (($_SERVER['SCRIPT_FILENAME'] ?? '') === '/probe/startup.php') {
    register_shutdown_function(static function (): void {
        global $failure;
        if ($failure instanceof \Throwable) {
            fwrite(STDERR, json_encode(['public_failure_line' => $failure->getLine(),
                'callers' => array_column($failure->getTrace(), 'line')], JSON_THROW_ON_ERROR)."\n");
        }
    });
    return;
}
if (($_SERVER['SCRIPT_FILENAME'] ?? '') !== '/scheduler/fixture.php') { return; }
$mode = $argv[1] ?? '';
if ($mode === 'source') {
    echo json_encode(['php_bytes' => filesize('/usr/local/bin/php'), 'request_uid' => @fileowner('/probe/scheduler.request.json'),
        'symfony_process_sha256' => hash_file('sha256', '/var/www/html/vendor/symfony/process/Process.php')], JSON_THROW_ON_ERROR)."\n";
    foreach (['scheduler-worker', 'horizon', 's6rc-oneshot-runner', 's6rc-fdholder', 's6-linux-init-shutdownd'] as $role) {
        foreach (['run', 'dependencies.d', 'contents.d'] as $leaf) {
            $path = '/etc/s6-overlay/s6-rc.d/'.$role.'/'.$leaf;
            if (is_file($path)) { echo $role.'/'.$leaf."\n".file_get_contents($path)."\n"; }
            elseif (is_dir($path)) { echo $role.'/'.$leaf.':'.implode(',', scandir($path))."\n"; }
        }
    }
    exit;
}
if ($mode === 'topology') {
    foreach (glob('/proc/[0-9]*/cmdline') as $path) {
        echo json_encode([basename(dirname($path)), @readlink(dirname($path).'/exe'),
            str_replace("\0", '|', (string)@file_get_contents($path))], JSON_THROW_ON_ERROR)."\n";
    }
    exit;
}
if ($mode === 'component-phase') {
    // PUBLIC component fixture only. Its separate service/user-writable /probe
    // profile is explicitly rejected by the native factory's fixed selectors.
    if (posix_geteuid() !== 0 || posix_getegid() !== 0) { exit(1); }
    $request = ['version' => 1, 'phase' => 'scheduler-proof-held', 'generation' => bin2hex(random_bytes(16)),
        'deadline_ns' => hrtime(true) + 120000000000];
    $stream = fopen('/probe/scheduler.request.json', 'xb');
    $raw = json_encode($request, JSON_THROW_ON_ERROR);
    if (!is_resource($stream) || fwrite($stream, $raw) !== strlen($raw) || !fflush($stream) || !fsync($stream)) { exit(1); }
    fclose($stream); if (!chmod('/probe/scheduler.request.json', 0444)) { exit(1); } exit;
}
if ($mode === 'component-expire') {
    if (posix_geteuid() !== 0 || posix_getegid() !== 0) { exit(1); }
    $path = '/probe/scheduler.request.json';
    $request = json_decode(file_get_contents($path), true, 8, JSON_THROW_ON_ERROR);
    $request['deadline_ns'] = hrtime(true) - 1;
    if (!chmod($path, 0644) || file_put_contents($path, json_encode($request, JSON_THROW_ON_ERROR)) === false
        || !chmod($path, 0444)) { exit(1); } exit;
}
if ($mode === 'background') {
    file_put_contents('/scheduler-state/detached/ready', 'PUBLIC');
    $deadline = hrtime(true) + 45000000000;
    while (!is_file('/scheduler-state/detached/release')) {
        if (hrtime(true) >= $deadline) { exit(7); }
        usleep(10000);
    }
    file_put_contents('/scheduler-state/detached/done', 'PUBLIC'); exit(0);
}
$app = require '/var/www/html/bootstrap/app.php';
$app->make(\Illuminate\Contracts\Console\Kernel::class)->bootstrap();
if ($mode === 'prepare') {
    $mask = umask(0077);
    $stream = fopen('/probe/hmac.key', 'xb');
    if (!is_resource($stream) || fwrite($stream, str_repeat('P', 32)) !== 32 || !fflush($stream) || !fsync($stream)) { exit(1); }
    fclose($stream); umask($mask); exit;
}
$maintenance = $app->make(\Illuminate\Contracts\Foundation\MaintenanceMode::class);
if ($mode === 'hold') {
    $maintenance->activate(['except' => ['/api/health'], 'redirect' => null, 'retry' => null, 'refresh' => null,
        'secret' => null, 'status' => 503, 'template' => null]);
    if (!$maintenance->active()) { exit(1); } exit;
}
if ($mode === 'unhold') { $maintenance->deactivate(); if ($maintenance->active()) { exit(1); } exit; }
exit(1);
