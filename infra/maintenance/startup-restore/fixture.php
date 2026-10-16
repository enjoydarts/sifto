<?php
// PUBLIC clone fixture; contains no production input.
declare(strict_types=1);
$mode = $argv[1] ?? '';
// Metadata/Redis fixtures must not add a fifth full Laravel worker footprint.
// The actual product producers and every real replacement still bootstrap normally.
if ($mode === 'restore' || str_starts_with($mode, 'fault-')) {
    require '/var/www/html/vendor/autoload.php';
    $app = require '/var/www/html/bootstrap/app.php';
    $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
}
set_exception_handler(static function (Throwable $e): never {
    fwrite(STDERR, json_encode(['public_restore_failure_line' => $e->getLine(),
        'calls' => array_column($e->getTrace(), 'line')], JSON_THROW_ON_ERROR)."\n");
    $path = '/var/www/html/storage/app/startup-horizon-drain.state';
    if (is_file($path)) {
        $s = json_decode(file_get_contents($path), true)['data'];
        fwrite(STDERR, json_encode(['public_phase' => $s['phase'],
            'restore_stage' => $s['restore']['stage'] ?? null, 'intents' => $s['restore']['intents'] ?? []], JSON_THROW_ON_ERROR)."\n");
    }
    if (is_file('/tmp/public-restore-child-failure.json')) { fwrite(STDERR, file_get_contents('/tmp/public-restore-child-failure.json')."\n"); }
    exit(1);
});
function check(bool $ok): void { if (!$ok) { throw new RuntimeException('PUBLIC assertion'); } }
function publicRedis(bool $horizon = false): Redis {
    $r = new Redis; check($r->connect('127.0.0.1', 6379, 2) && $r->auth('rehearsal-only-password') && $r->select(0));
    $r->setOption(Redis::OPT_PREFIX, $horizon ? 'sifto_public_restore_horizon:' : 'sifto_public_fixture_database_'); return $r;
}
function publicRows(): array {
    $r = publicRedis(true); $rows = [];
    foreach ($r->zRange('supervisors', 0, 31) as $name) { $rows[] = (object)$r->hGetAll('supervisor:'.$name); }
    return $rows;
}
function state(): array {
    $lock = fopen('/var/www/html/storage/app/startup-horizon-drain.lock', 'rb'); check(is_resource($lock));
    check(flock($lock, LOCK_SH));
    try { return json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-drain.state'), true, 64, JSON_THROW_ON_ERROR)['data']; }
    finally { fclose($lock); }
}
function queues(): array {
    $r = publicRedis(); return [$r->lrange('queues:public-drain', 0, 64),
        $r->zrange('queues:public-drain:delayed', 0, 64, true),
        $r->zrange('queues:public-drain:reserved', 0, 64, true)];
}
function owner(string $mode, bool $master = false): array {
    $row = publicRows()[0];
    $path = $master ? '/tmp/public-restore-master.json' : '/tmp/public-restore-owner.json';
    if (is_file($path)) { unlink($path); }
    $queue = 'commands:'.($master ? 'master:'.$row->master : $row->name);
    $r = publicRedis(true); check($r->llen($queue) === 0);
    $r->rpush($queue, json_encode(['command' => App\SiftoMaintenance\RestoreFixture::class, 'options' => ['mode' => $mode]], JSON_THROW_ON_ERROR));
    $end = hrtime(true) + ($mode === 'start' ? 15000000000 : 5000000000);
    do { if (is_file($path)) { return json_decode(file_get_contents($path), true, 64, JSON_THROW_ON_ERROR); } usleep(20000); } while (hrtime(true) < $end);
    throw new RuntimeException('PUBLIC owner observation timeout');
}
$mode = $argv[1] ?? '';
if ($mode === 'diagnostic') {
    $s = state(); $acks = [];
    foreach ($s['restore']['slots'] ?? [] as $i => $pool) { foreach ($pool as $slot => $worker) {
        $acks[] = [$i, $slot, isset($worker['ack']), is_dir('/proc/'.($worker['identity']['pid'] ?? 0))];
    } }
    echo json_encode(['public_phase' => $s['phase'], 'restore_stage' => $s['restore']['stage'] ?? null,
        'restore_intents' => $s['restore']['intents'] ?? [], 'slots' => $acks,
        'restore_expired' => isset($s['restore']) && hrtime(true) >= $s['restore']['request']['expires']], JSON_THROW_ON_ERROR)."\n";
    if (is_file('/tmp/public-restore-child-failure.json')) { echo file_get_contents('/tmp/public-restore-child-failure.json')."\n"; }
    exit;
}
if ($mode === 'install') {
    $end = hrtime(true) + 20000000000;
    do {
        $rows = publicRows();
        $row = count($rows) === 1 ? array_values($rows)[0] : null;
        $children = $row ? array_filter(explode(' ', trim(file_get_contents('/proc/'.$row->pid.'/task/'.$row->pid.'/children')))) : [];
        if (count($children) === 4) { break; } usleep(100000);
    } while (hrtime(true) < $end);
    check(count($children) === 4);
    foreach ($children as $pid) { check(str_contains(file_get_contents('/proc/'.$pid.'/cmdline'), "\0horizon:work\0")); }
    $mask = umask(0077);
    foreach (['/fixed/coolify_horizon_inspect.php' => '/var/www/html/app/SiftoMaintenance/InspectSupervisor.php',
        '/startup/SyntheticJob.php' => '/var/www/html/app/SiftoMaintenance/SyntheticJob.php',
        '/startup/RestoreFixture.php' => '/var/www/html/app/SiftoMaintenance/RestoreFixture.php'] as $from => $to) {
        $out = fopen($to, 'xb'); $bytes = file_get_contents($from);
        check(fwrite($out, $bytes) === strlen($bytes) && fflush($out) && fsync($out)); fclose($out);
    }
    $out = fopen('/probe/hmac.key', 'xb'); check(fwrite($out, str_repeat('P', 32)) === 32 && fflush($out) && fsync($out)); fclose($out); umask($mask);
    $before = owner('read'); check(count($before['pools']) === 2 && array_map(fn ($p) => count($p['workers']), $before['pools']) === [2, 2]);
    echo json_encode(['public_existing_workers' => 4, 'public_existing_pools' => 2,
        'source_sha256' => hash_file('sha256', '/fixed/coolify_horizon_inspect.php')], JSON_THROW_ON_ERROR)."\n"; exit;
}
if ($mode === 'start') {
    owner('start', true); $r = publicRedis(); check($r->get('public-drain:started') && !$r->get('public-drain:completed'));
    echo "PUBLIC actual existing master enqueued job; original worker active before held barrier and sentinels\n"; exit;
}
if ($mode === 'settle') {
    $r = publicRedis(); $end = hrtime(true) + 15000000000;
    do {
        if (!$r->get('public-drain:started') || $r->get('public-drain:completed')) { echo "PUBLIC job settled before cleanup\n"; exit; }
        usleep(100000);
    } while (hrtime(true) < $end);
    throw new RuntimeException('PUBLIC active job not settled');
}
if (in_array($mode, ['running', 'mixed', 'paused'], true)) {
    if ($mode === 'mixed') { owner('mixed'); }
    if ($mode === 'paused') { owner('paused', true); usleep(1500000); }
    $before = owner('read'); $master = owner('read', true);
    check($master['working'] === ($mode !== 'paused') && $before['working'] === ($mode !== 'paused')
        && array_column($before['pools'], 'working') === ($mode === 'paused' ? [false, false] : ($mode === 'mixed' ? [true, false] : [true, true])));
    file_put_contents('/tmp/public-restore-before.json', json_encode([$master, $before], JSON_THROW_ON_ERROR));
    $ages = []; foreach ($before['pools'] as $pool) { foreach ($pool['workers'] as $worker) { $ages[] = $worker['age_seconds']; } }
    echo json_encode(['public_saved_logical_intent' => $mode, 'original_worker_internal_pause_unobserved' => true,
        'original_worker_ages_seconds' => $ages], JSON_THROW_ON_ERROR)."\n"; exit;
}
if (($argv[1] ?? '') === 'restore') {
    check(method_exists(App\SiftoMaintenance\InspectSupervisor::class, 'restore'));
    echo json_encode(App\SiftoMaintenance\InspectSupervisor::restore(hrtime(true) + 30000000000), JSON_THROW_ON_ERROR)."\n";
    exit;
}
if ($mode === 'verify') {
    $s = state(); $r = publicRedis();
    check($s['phase'] === 'restored' && $s['restore']['stage'] === 'complete');
    check($r->get('public-drain:completed') === $r->get('public-drain:started'));
    $baseline = [['PUBLIC PENDING A', 'PUBLIC PENDING B'], ['PUBLIC DELAYED A' => 2000000001.0, 'PUBLIC DELAYED B' => 2000000002.0],
        ['PUBLIC RESERVED A' => 2000000003.0, 'PUBLIC RESERVED B' => 2000000004.0]];
    check(queues() === $baseline);
    [$beforeMaster, $before] = json_decode(file_get_contents('/tmp/public-restore-before.json'), true);
    $after = owner('read'); $afterMaster = owner('read', true);
    check($afterMaster === $beforeMaster && $after['object'] === $before['object'] && $after['working'] === $before['working']);
    $count = 0;
    foreach ($before['pools'] as $i => $pool) {
        check($after['pools'][$i]['object'] === $pool['object'] && $after['pools'][$i]['working'] === $pool['working']
            && $after['pools'][$i]['options_sha256'] === $pool['options_sha256'] && $after['pools'][$i]['terminating'] === 0);
        foreach ($pool['workers'] as $slot => $worker) {
            $new = $after['pools'][$i]['workers'][$slot]; $ack = $s['restore']['slots'][$i][$slot]['ack'];
            check($new['worker'] === $worker['worker'] && $new['live'] && !$new['native'] && $new['pid'] !== $worker['pid']
                && $ack['data']['identity']['pid'] === $new['pid'] && $ack['data']['paused'] === !$pool['working']); ++$count;
        }
    }
    check($count === 4); usleep(1200000); check(queues() === $baseline);
    file_put_contents('/tmp/public-restore-completion-queue.json', json_encode($baseline, JSON_THROW_ON_ERROR));
    echo "PUBLIC exact four original WorkerProcess slots/two pools/options/parent flags and four real WorkerStarting ACKs; held queue bytes/order/scores preserved\n"; exit;
}
if ($mode === 'expire') {
    $s = state(); $expires = $s['request']['expires'];
    do { usleep(100000); } while (hrtime(true) <= $expires + 1000000000);
    $after = state(); check($after['request'] === $s['request'] && $after['phase'] === 'held');
    echo "PUBLIC original H2 authority expired into held; original request unchanged\n"; exit;
}
if (str_starts_with($mode, 'fault-')) {
    $fault = substr($mode, 6); check(in_array($fault, ['generation', 'ticket', 'noack', 'partial', 'stale', 'expired'], true));
    $before = state(); $baseline = queues();
    if ($fault === 'stale') {
        // PUBLIC authenticated corruption fixture: stale original identity.
        $lock = fopen('/var/www/html/storage/app/startup-horizon-drain.lock', 'r+be'); check(flock($lock, LOCK_EX));
        $changed = $before; $changed['request']['identities']['supervisor']['start'] .= '0';
        $json = json_encode($changed, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES);
        $record = ['data' => $changed, 'hmac' => hash_hmac('sha256', "drain-state\0".$json, str_repeat('P', 32))];
        file_put_contents('/var/www/html/storage/app/startup-horizon-drain.state', json_encode($record, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES)."\n");
        fclose($lock);
    } else { file_put_contents('/tmp/public-restore-fault', $fault); }
    $failed = false;
    try { App\SiftoMaintenance\InspectSupervisor::restore(hrtime(true) + ($fault === 'expired' ? 800000000 : (in_array($fault, ['partial', 'noack'], true) ? 6500000000 : 20000000000))); }
    catch (RuntimeException $failure) { $failed = $failure->getMessage() === 'horizon_inspection_held'; }
    check($failed); $after = state(); check($after['phase'] === 'held' && $after['receipt'] === null && $after['restore']['stage'] === 'held');
    $actual = owner('read'); check(!$actual['working'] && array_column($actual['pools'], 'working') === [false, false]);
    $live = 0; $acks = 0;
    foreach ($actual['pools'] as $i => $pool) {
        check($pool['terminating'] === 0 && count($pool['workers']) === 2);
        foreach ($pool['workers'] as $slot => $worker) {
            check($worker['worker'] === $before['original']['supervisor']['pools'][$i]['workers'][$slot]['object']);
            $live += (int)$worker['live']; $acks += (int)isset($after['restore']['slots'][$i][$slot]['ack']);
        }
    }
    if ($fault === 'partial') { check($acks === 3 && $live === 4); }
    if ($fault === 'noack') { check($acks === 0 && $live === 4); }
    if (in_array($fault, ['stale', 'expired'], true)) { check($live === 0 && $acks === 0); }
    check(queues() === $baseline && app()->isDownForMaintenance());
    // No signal or stop. Delayed bootstrap children leave on their own expiry;
    // already ACKed replacements remain owned by the paused supervisor.
    if (in_array($fault, ['partial', 'noack'], true)) { usleep(4000000); }
    $later = owner('read'); check(!$later['working'] && queues() === $baseline);
    echo json_encode(['public_fault' => $fault, 'phase' => 'held', 'ready_at_failure' => $acks,
        'live_owned_at_failure' => $live, 'original_worker_objects_retained' => 4,
        'maintenance' => 'held', 'queue_preserved' => true, 'force_termination' => false], JSON_THROW_ON_ERROR)."\n"; exit;
}
if ($mode === 'native') {
    $s = state(); $end = hrtime(true) + 45000000000;
    do {
        $after = owner('read'); $all = true;
        foreach ($after['pools'] as $pool) { foreach ($pool['workers'] as $worker) { $all = $all && $worker['native'] && $worker['original_native'] && $worker['live']; } }
        if ($all) { break; } usleep(500000);
    } while (hrtime(true) < $end);
    check($all && hrtime(true) > $s['restore']['request']['expires']);
    $later = state(); $native = array_values(array_filter($later['restore']['intents'], fn ($i) => $i[0] === 'native-restart'));
    check(count($native) === 4 && $later['request'] === $s['request'] && $later['restore']['request'] === $s['restore']['request']);
    check(json_encode(queues(), JSON_THROW_ON_ERROR) === file_get_contents('/tmp/public-restore-completion-queue.json'));
    echo "PUBLIC four wrappers exited naturally at native maxTime; original closed native Process objects restarted after ticket expiry; queues preserved\n"; exit;
}
throw new RuntimeException('PUBLIC mode');
