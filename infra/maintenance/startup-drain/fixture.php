<?php
declare(strict_types=1);
require '/var/www/html/vendor/autoload.php';
$app = require '/var/www/html/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
set_exception_handler(static function (Throwable $e): never {
    fwrite(STDERR, json_encode(['public_failure_line' => $e->getLine(),
        'callers' => array_column($e->getTrace(), 'line')], JSON_THROW_ON_ERROR)."\n");
    $path = '/var/www/html/storage/app/startup-horizon-drain.state';
    if (is_file($path)) {
        $s = json_decode(file_get_contents($path), true)['data'];
        $pid = $s['request']['identities']['supervisor']['pid'];
        fwrite(STDERR, json_encode(['public_phase' => $s['phase'], 'intents' => $s['intents'],
            'children' => trim(file_get_contents('/proc/'.$pid.'/task/'.$pid.'/children')),
            'job_started' => app('redis')->connection()->get('public-drain:started'),
            'job_completed' => app('redis')->connection()->get('public-drain:completed'), 'queues' => queues()], JSON_THROW_ON_ERROR)."\n");
    }
    exit(1);
});
function check(bool $ok): void { if (!$ok) { throw new RuntimeException('PUBLIC assertion'); } }
function queues(): array {
    $r = app('redis')->connection();
    return [$r->lrange('queues:public-drain', 0, 64),
        $r->zrange('queues:public-drain:delayed', 0, 64, ['withscores' => true]),
        $r->zrange('queues:public-drain:reserved', 0, 64, ['withscores' => true])];
}
$mode = $argv[1] ?? '';
$r = app('redis')->connection();
if (in_array($mode, ['start', 'start-long'], true)) {
    $rows = app(Laravel\Horizon\Contracts\SupervisorRepository::class)->all();
    check(count($rows) === 1 && array_values($rows)[0]->options['timeout'] > 12);
    $out = fopen('/var/www/html/app/SiftoMaintenance/SyntheticJob.php', 'xb');
    $bytes = file_get_contents('/startup/SyntheticJob.php');
    check(fwrite($out, $bytes) === strlen($bytes) && fflush($out) && fsync($out)); fclose($out);
    $maintenance = $app->make(Illuminate\Contracts\Foundation\MaintenanceMode::class);
    $maintenance->deactivate();
    app('queue')->connection('redis')->push(new App\SiftoMaintenance\SyntheticJob($mode === 'start-long'), '', 'public-drain');
    $deadline = hrtime(true) + 10000000000;
    do { if ($r->get('public-drain:started')) { break; } usleep(100000); } while (hrtime(true) < $deadline);
    check((bool)$r->get('public-drain:started') && !$r->get('public-drain:completed'));
    $maintenance->activate(['except' => ['/api/health'], 'redirect' => null, 'retry' => null,
        'refresh' => null, 'secret' => null, 'status' => 503, 'template' => null]);
    $r->rpush('queues:public-drain', 'PUBLIC PENDING A', 'PUBLIC PENDING B');
    $r->zadd('queues:public-drain:delayed', 2000000001, 'PUBLIC DELAYED A', 2000000002, 'PUBLIC DELAYED B');
    $r->zadd('queues:public-drain:reserved', 2000000003, 'PUBLIC RESERVED A', 2000000004, 'PUBLIC RESERVED B');
    check(count(queues()[2]) === 3);
    echo "PUBLIC actual job started before barrier; active reservation plus preserved sentinels\n"; exit;
}
if ($mode === 'bounded-enter') {
    check(method_exists(App\SiftoMaintenance\InspectSupervisor::class, 'drainEnter'));
    try { App\SiftoMaintenance\InspectSupervisor::drainEnter(hrtime(true) + 4000000000); }
    catch (RuntimeException $e) {
        check($e->getMessage() === 'horizon_inspection_held');
        check((bool)$r->get('public-drain:started') && !$r->get('public-drain:completed'));
        $state = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-drain.state'), true)['data'];
        echo json_encode(['public_timeout_phase' => $state['phase'], 'intents' => $state['intents']], JSON_THROW_ON_ERROR)."\n";
        check($state['phase'] === 'held' && in_array(['worker-term', 0, 0], $state['intents'], true));
        $pid = (int)$r->get('public-drain:started');
        check(is_dir('/proc/'.$pid) && str_contains(file_get_contents('/proc/'.$pid.'/cmdline'), "\0horizon:work\0"));
        echo "PUBLIC bounded producer held while long job survives\n"; exit;
    }
    throw new RuntimeException('PUBLIC expected held');
}
if (in_array($mode, ['verify', 'verify-long'], true)) {
    $deadline = hrtime(true) + 15000000000;
    $sent = false;
    do {
        $state = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-drain.state'), true, 64, JSON_THROW_ON_ERROR)['data'];
        $ids = $state['request']['identities'];
        $children = trim(file_get_contents('/proc/'.$ids['supervisor']['pid'].'/task/'.$ids['supervisor']['pid'].'/children'));
        if ($mode === 'verify-long' && $r->get('public-drain:completed') && !$sent) {
            // A held parent deliberately no longer monitors workers. Read the
            // retained handle after natural completion, reaping a possible zombie.
            $horizon = app('redis')->connection('horizon');
            check($horizon->llen('commands:'.$ids['name']) === 0);
            $horizon->rpush('commands:'.$ids['name'], json_encode(['command' => App\SiftoMaintenance\SyntheticJob::class, 'options' => []], JSON_THROW_ON_ERROR));
            $sent = true;
        }
        if ($r->get('public-drain:completed') && $children === '') { break; }
        usleep(100000);
    } while (hrtime(true) < $deadline);
    check($children === '' && $r->get('public-drain:completed') === $r->get('public-drain:started'));
    check(queues() === [['PUBLIC PENDING A', 'PUBLIC PENDING B'],
        ['PUBLIC DELAYED A' => 2000000001.0, 'PUBLIC DELAYED B' => 2000000002.0],
        ['PUBLIC RESERVED A' => 2000000003.0, 'PUBLIC RESERVED B' => 2000000004.0]]);
    $baseline = queues(); usleep(2200000);
    check(queues() === $baseline && trim(file_get_contents('/proc/'.$ids['supervisor']['pid'].'/task/'.$ids['supervisor']['pid'].'/children')) === '');
    check($state['phase'] === ($mode === 'verify' ? 'quiescent' : 'held'));
    if ($mode === 'verify') {
        check($state['view']['sequence'] === 2 && $state['view']['master'] >= $state['view']['requested']
            && $state['view']['supervisor'] >= $state['view']['requested']);
        echo "PUBLIC fresh ACK from both actual owners validates same retained handles, exit0, and paused flags\n";
        echo "PUBLIC job natural completion; no respawn; post-completion queue bytes/order/scores preserved\n"; exit;
    }
    $deadline = hrtime(true) + 5000000000;
    do { if (is_file('/tmp/public-drain-exits.json')) { break; } usleep(100000); } while (hrtime(true) < $deadline);
    check(is_file('/tmp/public-drain-exits.json'));
    $exit = json_decode(file_get_contents('/tmp/public-drain-exits.json'), true);
    check($exit === ['same_objects' => true, 'exit_codes' => [0], 'signaled' => [false], 'paused' => true]);
    echo json_encode($exit, JSON_THROW_ON_ERROR)."\n";
    echo "PUBLIC same supervisor retained; real job natural completion; no worker respawn; post-completion queue bytes/order/scores preserved\n"; exit;
}
if ($mode === 'settle') {
    $deadline = hrtime(true) + 15000000000;
    do {
        if (!$r->get('public-drain:started') || $r->get('public-drain:completed')) { echo "PUBLIC job settled before cleanup\n"; exit; }
        usleep(100000);
    } while (hrtime(true) < $deadline);
    throw new RuntimeException('PUBLIC job still active');
}
if ($mode === 'post-foreign') {
    $state = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-drain.state'), true)['data'];
    check($state['phase'] === 'quiescent');
    $queue = 'commands:master:'.$state['request']['identities']['master_name'];
    $horizon = app('redis')->connection('horizon'); check($horizon->llen($queue) === 0);
    $horizon->rpush($queue, json_encode(['command' => App\SiftoMaintenance\SyntheticJob::class, 'options' => []], JSON_THROW_ON_ERROR));
    $deadline = hrtime(true) + 3000000000;
    do { if (is_file('/tmp/public-drain-foreign-master')) { break; } usleep(10000); } while (hrtime(true) < $deadline);
    check(is_file('/tmp/public-drain-foreign-master'));
    $rejected = false;
    try { App\SiftoMaintenance\InspectSupervisor::drainObserve(hrtime(true) + 5000000000); }
    catch (RuntimeException $e) { $rejected = true; }
    check($rejected);
    // The foreign marker precedes the owner loop-end callback. A nonblocking
    // observer may reject while that callback still owns the lock. Wait only
    // for its durable rejection; do not repeat an observation or any control.
    $readLock = fopen('/var/www/html/storage/app/startup-horizon-drain.lock', 'rb');
    $deadline = hrtime(true) + 3000000000; $after = null;
    do {
        if (flock($readLock, LOCK_SH | LOCK_NB)) {
            $after = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-drain.state'), true)['data'];
            flock($readLock, LOCK_UN);
            if ($after['phase'] === 'held') { break; }
        }
        usleep(10000);
    } while (hrtime(true) < $deadline);
    fclose($readLock);
    check($after['phase'] === 'held' && $after['original'] === $state['original']);
    echo "PUBLIC immediate foreign master working change invalidates prior quiescence; original generation retained\n"; exit;
}
throw new RuntimeException('PUBLIC fixture mode');
