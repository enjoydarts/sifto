<?php
// PUBLIC synthetic integration; never a native source installer.
declare(strict_types=1);
error_reporting(E_ALL);
$mode = $argv[1] ?? '';
if ($mode === 'wait-receipt') {
    $deadline = hrtime(true) + 5000000000;
    do {
        $s = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-inspect.state'), true);
        $phase = $s['data']['phase'] ?? 'unknown';
        if (in_array($phase, ['complete', 'held'], true)) {
            echo json_encode(['public_receipt_phase' => $phase, 'rejection' => $s['data']['rejection'] ?? null], JSON_THROW_ON_ERROR)."\n";
            if ($phase === 'held') {
                $pid = $s['data']['request']['identities']['supervisor']['pid'];
                $children = trim(file_get_contents('/proc/'.$pid.'/task/'.$pid.'/children'));
                foreach (array_filter(explode(' ', $children)) as $child) {
                    fwrite(STDERR, json_encode(['public_worker_argv' => explode("\0", file_get_contents('/proc/'.$child.'/cmdline')),
                        'public_worker_exe' => readlink('/proc/'.$child.'/exe')], JSON_THROW_ON_ERROR)."\n");
                }
            }
            exit($phase === 'complete' ? 0 : 1);
        }
        usleep(100000);
    } while (hrtime(true) < $deadline);
    require '/var/www/html/vendor/autoload.php';
    $app = require '/var/www/html/bootstrap/app.php';
    $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
    $client = app('redis')->connection('horizon')->client();
    $prefix = $client->getOption(Redis::OPT_PREFIX);
    $logical = 'commands:'.$s['data']['request']['identities']['name'];
    fwrite(STDERR, json_encode(['public_receipt_phase' => $phase,
        'correct_queue_count' => $client->rawCommand('LLEN', $prefix.$logical),
        'double_prefix_queue_count' => $client->rawCommand('LLEN', $prefix.$prefix.$logical)], JSON_THROW_ON_ERROR)."\n"); exit(1);
}
require '/var/www/html/vendor/autoload.php';
$app = require '/var/www/html/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
set_exception_handler(static function (Throwable $failure): never {
    fwrite(STDERR, "PUBLIC fixture failed at line ".$failure->getLine()."\n"); exit(1);
});
function publicProcessIdentity(int $pid): array {
    $stat = file_get_contents('/proc/'.$pid.'/stat');
    $parts = explode(' ', substr($stat, strrpos($stat, ') ') + 2));
    if (in_array($parts[0], ['Z', 'X'], true)) { exit(1); }
    return [$pid, $parts[1], $parts[19], file_get_contents('/proc/'.$pid.'/cmdline')];
}
function publicInventory(object $row): array {
    $master = app(Laravel\Horizon\Contracts\MasterSupervisorRepository::class)->find($row->master);
    $children = array_values(array_filter(explode(' ', trim(file_get_contents('/proc/'.$row->pid.'/task/'.$row->pid.'/children')))));
    if (count($children) !== 1) { exit(1); }
    $queue = app('redis')->connection();
    return [publicProcessIdentity((int)$master->pid), publicProcessIdentity((int)$row->pid),
        publicProcessIdentity((int)$children[0]), $row->options,
        $queue->lrange('queues:public-inspection-empty', 0, 32),
        $queue->zrange('queues:public-inspection-empty:delayed', 0, 32, ['withscores' => true]),
        $queue->zrange('queues:public-inspection-empty:reserved', 0, 32, ['withscores' => true])];
}
function publicJson(array $value): string { return json_encode($value, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES); }
function publicMac(string $domain, array $value): string {
    return hash_hmac('sha256', $domain."\0".publicJson($value), str_repeat('P', 32));
}
if ($mode === 'hold') {
    $app->make(Illuminate\Contracts\Foundation\MaintenanceMode::class)->activate([
        'except' => ['/api/health'], 'redirect' => null, 'retry' => null,
        'refresh' => null, 'secret' => null, 'status' => 503, 'template' => null]);
    echo "PUBLIC maintenance held\n"; exit;
}
$repo = app(Laravel\Horizon\Contracts\SupervisorRepository::class);
$deadline = hrtime(true) + 20000000000;
do {
    $rows = $repo->all();
    if (count($rows) === 1) { break; }
    usleep(200000);
} while (hrtime(true) < $deadline);
if (count($rows) !== 1) { throw new RuntimeException('PUBLIC supervisor missing'); }
$row = array_values($rows)[0];
$master = app(Laravel\Horizon\Contracts\MasterSupervisorRepository::class)->find($row->master);
if ($mode === 'install') {
    // Horizon publishes an object heartbeat before the first monitor starts it.
    // The test waits for an actual existing child, never object-count readiness.
    do {
        $children = array_values(array_filter(explode(' ', trim(file_get_contents('/proc/'.$row->pid.'/task/'.$row->pid.'/children')))));
        if (count($children) === 1 && str_contains(file_get_contents('/proc/'.$children[0].'/cmdline'), "\0horizon:work\0")) { break; }
        usleep(100000);
    } while (hrtime(true) < $deadline);
    if (count($children) !== 1 || !str_contains(file_get_contents('/proc/'.$children[0].'/cmdline'), "\0horizon:work\0")) { exit(1); }
    if (!is_file('/fixed/coolify_horizon_inspect.php')) {
        fwrite(STDERR, "PUBLIC fixed inspection helper missing\n"); exit(1);
    }
    if (file_exists('/var/www/html/app/SiftoMaintenance/InspectSupervisor.php')) {
        throw new RuntimeException('PUBLIC helper preexisting');
    }
    $mask = umask(0077);
    $out = fopen('/var/www/html/app/SiftoMaintenance/InspectSupervisor.php', 'xb');
    $bytes = file_get_contents('/fixed/coolify_horizon_inspect.php');
    if (fwrite($out, $bytes) !== strlen($bytes) || !fflush($out) || !fsync($out)) { exit(1); }
    fclose($out);
    $key = fopen('/probe/hmac.key', 'xb');
    if (fwrite($key, str_repeat('P', 32)) !== 32 || !fflush($key) || !fsync($key)) { exit(1); }
    fclose($key); umask($mask);
    // Inert PUBLIC sentinels are never executable jobs. The held barrier and
    // force=false must preserve their order and scores without any reservation.
    $queue = app('redis')->connection();
    $queue->rpush('queues:public-inspection-empty', 'PUBLIC PENDING A', 'PUBLIC PENDING B');
    $queue->zadd('queues:public-inspection-empty:delayed', 2000000001, 'PUBLIC DELAYED A', 2000000002, 'PUBLIC DELAYED B');
    $queue->zadd('queues:public-inspection-empty:reserved', 2000000003, 'PUBLIC RESERVED A', 2000000004, 'PUBLIC RESERVED B');
    file_put_contents('/tmp/public-inspect-baseline.json', publicJson(publicInventory($row)));
    echo json_encode(['category' => 'public_installed_after_supervisor_started',
        'pid' => (int)$row->pid, 'worker_existed_before_install' => true,
        'source_sha256' => hash('sha256', $bytes)], JSON_THROW_ON_ERROR)."\n";
    exit;
}
if ($mode === 'verify-positive') {
    $current = publicJson(publicInventory($row));
    if ($current !== file_get_contents('/tmp/public-inspect-baseline.json')) { exit(1); }
    $bytes = file_get_contents('/var/www/html/storage/app/startup-horizon-inspect.state');
    $state = json_decode($bytes, true, 32, JSON_THROW_ON_ERROR);
    if ($state['data']['phase'] !== 'complete' || $state['data']['receipt']['live_worker_count'] !== 1
        || !hash_equals(publicMac('inspect-state', $state['data']), $state['hmac'])
        || $state['data']['request']['identities']['supervisor']['pid'] !== (int)$row->pid) { exit(1); }
    file_put_contents('/tmp/public-inspect-golden.json', $bytes);
    echo "PUBLIC same supervisor/worker PID-start/options/nonempty queue order+scores preserved\n"; exit;
}
if ($mode === 'negative-producer-expired') {
    $before = file_get_contents('/var/www/html/storage/app/startup-horizon-inspect.state');
    $caught = false;
    try { App\SiftoMaintenance\InspectSupervisor::request(hrtime(true) - 1); }
    catch (RuntimeException $failure) { $caught = $failure->getMessage() === 'horizon_inspection_held'; }
    if (!$caught || $before !== file_get_contents('/var/www/html/storage/app/startup-horizon-inspect.state')
        || app('redis')->connection('horizon')->llen('commands:'.$row->name) !== 0) { exit(1); }
    echo "PUBLIC expired internal producer budget denied without state/queue mutation\n"; exit;
}
if (in_array($mode, ['negative-stale', 'negative-expired', 'negative-foreign', 'negative-reordered',
        'negative-duplicate', 'negative-config', 'negative-no-ack'], true)) {
    if (publicJson(publicInventory($row)) !== file_get_contents('/tmp/public-inspect-baseline.json')) { exit(1); }
    $key = 'commands:'.$row->name; $redis = app('redis')->connection('horizon');
    if ($redis->llen($key) !== 0) { exit(1); }
    $lock = fopen('/var/www/html/storage/app/startup-horizon-inspect.lock', 'r+b');
    if (!flock($lock, LOCK_EX | LOCK_NB)) { exit(1); }
    $data = json_decode(file_get_contents('/tmp/public-inspect-golden.json'), true, 32, JSON_THROW_ON_ERROR)['data'];
    $data['request']['run'] = bin2hex(random_bytes(32));
    $data['request']['expires'] = hrtime(true) + 10000000000;
    $data['phase'] = 'queued'; $data['receipt'] = null; $data['rejection'] = null;
    if ($mode === 'negative-stale') { $data['request']['identities']['supervisor']['start'] .= '0'; }
    if ($mode === 'negative-expired') { $data['request']['expires'] = hrtime(true) - 1; }
    if ($mode === 'negative-reordered') { $data['phase'] = 'prepared'; }
    if ($mode === 'negative-config') { $data['request']['context']['config'] = str_repeat('0', 64); }
    $options = ['run' => $data['request']['run'], 'request_hmac' => publicMac('inspect-request', $data['request'])];
    if ($mode === 'negative-foreign') { $options['run'] = str_repeat('0', 64); }
    $command = json_encode(['command' => App\SiftoMaintenance\InspectSupervisor::class, 'options' => $options], JSON_THROW_ON_ERROR);
    $out = fopen('/var/www/html/storage/app/startup-horizon-inspect.state', 'r+b');
    $bytes = publicJson(['data' => $data, 'hmac' => publicMac('inspect-state', $data)])."\n";
    if (!ftruncate($out, 0) || fwrite($out, $bytes) !== strlen($bytes) || !fflush($out) || !fsync($out)) { exit(1); }
    fclose($out);
    $redis->rpush($key, $command);
    if ($mode === 'negative-duplicate') { $redis->rpush($key, $command); }
    if ($mode !== 'negative-no-ack') { fclose($lock); $lock = null; }
    $deadline = hrtime(true) + 5000000000;
    do {
        $value = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-inspect.state'), true, 32, JSON_THROW_ON_ERROR);
        if ($redis->llen($key) === 0 && ($mode === 'negative-no-ack' || $value['data']['phase'] === 'held')) { break; }
        usleep(100000);
    } while (hrtime(true) < $deadline);
    if (is_resource($lock)) { fclose($lock); }
    $expected = $mode === 'negative-no-ack' ? 'queued' : 'held';
    if ($redis->llen($key) !== 0 || $value['data']['phase'] !== $expected || $value['data']['receipt'] !== null
        || publicJson(publicInventory($row)) !== file_get_contents('/tmp/public-inspect-baseline.json')) { exit(1); }
    echo json_encode(['public_case' => $mode, 'phase' => $expected, 'own_queue_empty' => true,
        'same_supervisor_worker_preserved' => true, 'completion' => false], JSON_THROW_ON_ERROR)."\n"; exit;
}
throw new RuntimeException('PUBLIC fixture mode');
