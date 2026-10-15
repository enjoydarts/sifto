<?php
// Fixed PUBLIC authenticated fault cases. Never mounted or installed by native code.
declare(strict_types=1);
require '/var/www/html/vendor/autoload.php';
$app = require '/var/www/html/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
function demand(bool $ok): void { if (!$ok) { throw new RuntimeException('PUBLIC fault assertion'); } }
function canonical(array $a): string { return json_encode($a, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES); }
function mac(string $d, array $a): string { return hash_hmac('sha256', $d."\0".canonical($a), str_repeat('P', 32)); }
set_exception_handler(static function (Throwable $e): never {
    fwrite(STDERR, json_encode(['public_fault_line' => $e->getLine(), 'callers' => array_column($e->getTrace(), 'line')])."\n"); exit(1);
});
$class = App\SiftoMaintenance\InspectSupervisor::class;
$context = new ReflectionMethod($class, 'context');
$identities = new ReflectionMethod($class, 'identities');
$redis = app('redis')->connection('horizon');
$path = '/var/www/html/storage/app/startup-horizon-drain.state';
$mask = umask(0077);
$lock = fopen('/var/www/html/storage/app/startup-horizon-drain.lock', 'x+b');
demand(is_resource($lock));
foreach (['foreign', 'hmac', 'stale', 'expired', 'reordered', 'config', 'source', 'duplicate', 'no-ack', 'foreign-batch'] as $case) {
    demand(flock($lock, LOCK_EX | LOCK_NB));
    $ids = $identities->invoke(null, str_repeat('P', 32));
    $queue = 'commands:master:'.$ids['master_name'];
    demand($redis->llen($queue) === 0);
    $children = trim(file_get_contents('/proc/'.$ids['supervisor']['pid'].'/task/'.$ids['supervisor']['pid'].'/children'));
    demand(ctype_digit($children));
    $worker = file_get_contents('/proc/'.$children.'/stat');
    $r = ['run' => bin2hex(random_bytes(32)), 'expires' => hrtime(true) + 8000000000,
        'context' => $context->invoke(null, str_repeat('P', 32)), 'identities' => $ids];
    if ($case === 'stale') { $r['identities']['master']['start'] .= '0'; }
    if ($case === 'expired') { $r['expires'] = hrtime(true) - 1; }
    if ($case === 'config') { $r['context']['config'] = str_repeat('0', 64); }
    if ($case === 'source') { $r['context']['source'] = str_repeat('0', 64); }
    $s = ['request' => $r, 'phase' => $case === 'reordered' ? 'prepared' : 'master-queued',
        'original' => ['master' => null, 'supervisor' => null], 'intents' => [], 'receipt' => null, 'view' => null];
    $options = ['drain_role' => 'master', 'run' => $case === 'foreign' ? str_repeat('0', 64) : $r['run'],
        'request_hmac' => mac('drain-master', $r)];
    if ($case === 'hmac') { $options['request_hmac'] = str_repeat('0', 64); }
    $bytes = canonical(['data' => $s, 'hmac' => mac('drain-state', $s)])."\n";
    $out = fopen($path, is_file($path) ? 'r+b' : 'x+b');
    demand(ftruncate($out, 0) && fwrite($out, $bytes) === strlen($bytes) && fflush($out) && fsync($out)); fclose($out);
    $command = canonical(['command' => $class, 'options' => $options]);
    $batch = [$command];
    if ($case === 'duplicate') { $batch[] = $command; }
    if ($case === 'foreign-batch') {
        $batch[] = canonical(['command' => Laravel\Horizon\SupervisorCommands\Pause::class, 'options' => []]);
    }
    // One atomic PUBLIC fixture insertion establishes the exact stock local batch.
    $redis->rpush($queue, ...$batch);
    if ($case !== 'no-ack') { flock($lock, LOCK_UN); }
    $deadline = hrtime(true) + 5000000000;
    do {
        $current = json_decode(file_get_contents($path), true)['data'];
        if ($redis->llen($queue) === 0 && ($case === 'no-ack' || $current['phase'] === 'held')) { break; }
        usleep(100000);
    } while (hrtime(true) < $deadline);
    if ($case === 'no-ack') {
        demand($current['phase'] === 'master-queued' && $redis->llen($queue) === 0);
        flock($lock, LOCK_UN);
        $failed = false;
        try { App\SiftoMaintenance\InspectSupervisor::drainObserve(hrtime(true) + 5000000000); }
        catch (Throwable $e) { $failed = true; }
        demand($failed); $current = json_decode(file_get_contents($path), true)['data'];
    }
    demand($current['phase'] === 'held' && $current['intents'] === [] && $current['receipt'] === null
        && $current['original'] === ['master' => null, 'supervisor' => null] && $redis->llen($queue) === 0
        && trim(file_get_contents('/proc/'.$ids['supervisor']['pid'].'/task/'.$ids['supervisor']['pid'].'/children')) === $children);
    $parts = explode(' ', substr($worker, strrpos($worker, ') ') + 2));
    $now = file_get_contents('/proc/'.$children.'/stat');
    $now = explode(' ', substr($now, strrpos($now, ') ') + 2));
    demand($parts[19] === $now[19] && !in_array($now[0], ['Z', 'X'], true));
    echo json_encode(['public_fault' => $case, 'phase' => 'held', 'control_intents' => 0,
        'same_live_worker' => true, 'stock_batch_deferred' => true], JSON_THROW_ON_ERROR)."\n";
}
fclose($lock); umask($mask);
