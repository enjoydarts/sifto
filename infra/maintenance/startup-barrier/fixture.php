<?php
// PUBLIC synthetic evidence only; no private host data or native authority.
declare(strict_types=1);
error_reporting(0);
ini_set('display_errors', '0');
ini_set('log_errors', '0');
ob_start();
function demand(bool $condition): void { if (!$condition) { throw new RuntimeException('barrier_fixture'); } }
function emit(array $value, int $status = 0): never {
    while (ob_get_level()) { ob_end_clean(); }
    echo json_encode($value, JSON_THROW_ON_ERROR)."\n"; exit($status);
}
function invoke(string $operation, array $extra = []): array {
    $pipes = [];
    $child = proc_open(array_merge(['php', '/probe/startup.php', $operation], $extra),
        [0 => ['file', '/dev/null', 'r'], 1 => ['pipe', 'w'], 2 => ['pipe', 'w']], $pipes);
    demand(is_resource($child));
    $out = stream_get_contents($pipes[1]); $err = stream_get_contents($pipes[2]);
    fclose($pipes[1]); fclose($pipes[2]); $status = proc_close($child);
    demand($err === '' && is_string($out) && strlen($out) <= 8192);
    $value = json_decode($out, true, 32, JSON_THROW_ON_ERROR);
    if ($status !== 0) {
        while (ob_get_level()) { ob_end_clean(); }
        echo json_encode(['operation' => $operation, 'status' => $status,
            'category' => $value['category'] ?? 'missing_category'], JSON_THROW_ON_ERROR)."\n"; ob_start();
    }
    return [$status, $value];
}
function put(string $path, string $bytes, int $mode = 0600): void {
    demand(file_put_contents($path, $bytes) === strlen($bytes) && chmod($path, $mode));
}
function files(): array {
    $result = [];
    foreach (['maintenance.php', 'down'] as $name) {
        $path = '/var/www/html/storage/framework/'.$name;
        clearstatcache(true, $path); $stat = @lstat($path);
        $result[$name] = $stat === false ? null : [array_intersect_key($stat,
            array_flip(['mode', 'uid', 'gid', 'size', 'mtime'])), file_get_contents($path)];
    }
    return $result;
}
function clean(Redis $client, string $key): void {
    demand(chmod('/var/www/html/storage/framework', 0755));
    foreach (['/var/www/html/storage/framework/maintenance.php', '/var/www/html/storage/framework/down',
        '/var/www/html/storage/framework/.startup-barrier-maintenance', '/var/www/html/storage/framework/.startup-barrier-down',
        '/var/www/html/storage/app/startup-barrier.snapshot', '/var/www/html/storage/framework/.public-barrier-link'] as $path) {
        clearstatcache(true, $path);
        if (@lstat($path) !== false) { demand(unlink($path)); }
    }
    $client->del($key);
    put('/probe/hmac.key', str_repeat('P', 32));
}
function plain(): array {
    return ['except' => ['/api/health'], 'redirect' => null, 'retry' => null,
        'refresh' => null, 'secret' => null, 'status' => 503, 'template' => null];
}
function caseResult(string $case, bool $ok): void {
    demand($ok);
    while (ob_get_level()) { ob_end_clean(); }
    echo json_encode(['ok' => true, 'case' => $case], JSON_THROW_ON_ERROR)."\n"; ob_start();
}
function http(string $path, string $method, ?string $cookie, ?string $expectedLocation = null): int {
    $context = stream_context_create(['http' => ['method' => $method, 'ignore_errors' => true,
        'follow_location' => 0, 'timeout' => 3,
        'header' => $cookie === null ? '' : 'Cookie: laravel_maintenance='.$cookie]]);
    demand(is_string(@file_get_contents('http://127.0.0.1:8080'.$path, false, $context))
        && isset($http_response_header[0]) && preg_match('/^HTTP\/1\.[01] ([0-9]{3}) /', $http_response_header[0], $match) === 1);
    $locations = [];
    foreach ($http_response_header as $header) {
        demand(preg_match('/^Set-Cookie:\s*laravel_maintenance=/i', $header) !== 1);
        if (preg_match('/^Location:\s*(.*)$/i', $header, $location) === 1) { $locations[] = $location[1]; }
    }
    // Only cookie-before supplies the fixed local login URL. Every held check
    // retains the default requirement of no Location header whatsoever.
    demand($locations === ($expectedLocation === null ? [] : [$expectedLocation]));
    return (int)$match[1];
}
function inert(Redis $cache, string $cachePrefix): string {
    $redis = Illuminate\Support\Facades\Redis::connection()->client();
    $value = [$cache->get($cachePrefix.'public-barrier-unrelated'),
        $redis->lrange('queues:public-startup', 0, -1),
        $redis->zrange('queues:public-startup:delayed', 0, -1, true),
        $redis->zrange('queues:public-startup:reserved', 0, -1, true)];
    foreach (['/var/www/html/storage/app/ssh/keys', '/var/www/html/storage/app/ssh/mux'] as $root) {
        $entries = [];
        if (is_dir($root)) {
            foreach (scandir($root) as $name) {
                if ($name === '.' || $name === '..') { continue; }
                $path = $root.'/'.$name; $info = lstat($path);
                demand(is_array($info) && is_file($path) && !is_link($path) && $info['nlink'] === 1 && count($entries) < 32);
                $entries[$name] = [array_intersect_key($info, array_flip(['dev', 'ino', 'mode', 'nlink', 'uid', 'gid', 'size', 'mtime', 'ctime'])), hash_file('sha256', $path)];
            }
        }
        $value[] = $entries;
    }
    foreach (['/var/www/html/.env', '/etc/s6-overlay/s6-rc.d/init-script/up',
        '/var/www/html/app/Http/Middleware/PreventRequestsDuringMaintenance.php', '/etc/nginx/nginx.conf',
        '/etc/nginx/conf.d/default.conf', '/data/coolify/ssl/public-fixture.txt', '/data/coolify/proxy/public-fixture.txt'] as $path) {
        $value[] = hash_file('sha256', $path);
    }
    return hash('sha256', serialize($value));
}
$stage = 'bootstrap';
try {
    demand($argc === 2);
    require '/var/www/html/vendor/autoload.php';
    $app = require '/var/www/html/bootstrap/app.php';
    $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
    demand(file_put_contents('/probe/hmac.key', str_repeat('P', 32)) === 32 && chmod('/probe/hmac.key', 0600));
    $store = Illuminate\Support\Facades\Cache::store('redis')->getStore();
    $client = $store->connection()->client();
    $key = $store->getPrefix().'illuminate:foundation:down';
    $physical = $client->getOption(Redis::OPT_PREFIX).$key;
    $default = Illuminate\Support\Facades\Redis::connection()->client();
    if ($argv[1] === 'ready') {
        $stage = 'http_readiness'; demand(http('/api/health', 'GET', null) === 200);
        emit(['ok' => true, 'category' => 'http_ready']);
    }
    if ($argv[1] === 'alternate-prefix') {
        $stage = 'alternate_prefix'; clean($client, $key);
        putenv('CACHE_PREFIX=PUBLIC_ALTERNATE_CACHE_'); putenv('REDIS_PREFIX=PUBLIC_ALTERNATE_REDIS_');
        [$status, $value] = invoke('barrier-enter');
        demand($status === 0 && $value['key_state'] === 'persistent');
        demand($client->rawCommand('GET', 'PUBLIC_ALTERNATE_REDIS_PUBLIC_ALTERNATE_CACHE_illuminate:foundation:down') === serialize(plain()));
        [$status, $value] = invoke('barrier-restore');
        demand($status === 0 && $value['key_state'] === 'absent'
            && $client->rawCommand('GET', 'PUBLIC_ALTERNATE_REDIS_PUBLIC_ALTERNATE_CACHE_illuminate:foundation:down') === false);
        clean($client, $key);
        emit(['ok' => true, 'category' => 'effective_prefixes_bound_without_fixture_whitelist']);
    }
    if ($argv[1] === 'stage-preflight') {
        $stage = 'foreign_stage_preflight'; clean($client, $key);
        put('/var/www/html/storage/framework/.startup-barrier-down', 'PUBLIC FOREIGN STAGE');
        [$status, $value] = invoke('barrier-enter');
        demand($status === 1 && !file_exists('/var/www/html/storage/app/startup-barrier.snapshot') && $client->get($key) === false);
        clean($client, $key); emit(['ok' => true, 'category' => 'foreign_stage_rejected_before_write']);
    }
    if ($argv[1] === 'identity-denied') {
        $stage = 'actual_redis_restart_identity';
        $before = files(); $rawBefore = $client->get($key);
        [$status, $value] = invoke('barrier-restore');
        demand($status === 1 && $value['category'] === 'barrier_snapshot_invalid'
            && files() === $before && $client->get($key) === $rawBefore);
        emit(['ok' => true, 'category' => 'actual_redis_replacement_rejected_without_write']);
    }
    if ($argv[1] === 'cache-delete-preflight') {
        $stage = 'intervening_cache_delete'; clean($client, $key);
        [$status, ] = invoke('barrier-enter'); demand($status === 0);
        $heldFiles = files(); $heldSnapshot = file_get_contents('/var/www/html/storage/app/startup-barrier.snapshot');
        demand($client->del($key) === 1);
        [$status, ] = invoke('barrier-restore');
        demand($status === 1 && $client->get($key) === false && files() === $heldFiles
            && file_get_contents('/var/www/html/storage/app/startup-barrier.snapshot') === $heldSnapshot);
        clean($client, $key); emit(['ok' => true, 'category' => 'intervening_cache_delete_rejected_without_write']);
    }
    if ($argv[1] === 'oversized-cas') {
        $stage = 'oversized_actual_cas'; clean($client, $key);
        demand(filesize('/probe/startup.php') <= 131072);
        $parts = explode("const STARTUP_BARRIER_CAS = <<<'LUA'\n", file_get_contents('/probe/startup.php'));
        demand(count($parts) === 2);
        $tail = explode("\nLUA;", $parts[1], 2); demand(count($tail) === 2 && strlen($tail[0]) <= 4096);
        // Execute the exact source-owned fixed script, with a PUBLIC oversized
        // current/expected string. There is no product hook or policy switch.
        $oversized = str_repeat('P', 65537); demand($client->set($key, $oversized));
        $beforeFiles = files();
        $changed = $client->rawCommand('EVAL', $tail[0], '1', $physical, 'string', $oversized, '-1',
            'string', serialize(plain()), '-1');
        demand($changed === 0 && $client->get($key) === $oversized && files() === $beforeFiles);
        foreach (['barrier-observe', 'barrier-enter'] as $operation) {
            [$status, $value] = invoke($operation);
            demand($status === 1 && $value['category'] === 'barrier_state_invalid'
                && !file_exists('/var/www/html/storage/app/startup-barrier.snapshot')
                && $client->get($key) === $oversized && files() === $beforeFiles);
        }
        clean($client, $key);
        emit(['ok' => true, 'category' => 'actual_fixed_cas_oversized_string_rejected_without_write', 'bounded_raw_rejection' => true]);
    }
    if ($argv[1] === 'expiry-bounds') {
        $stage = 'expiry_source_scripts'; clean($client, $key);
        demand(filesize('/probe/startup.php') <= 131072); $scripts = [];
        foreach (['READ', 'CAS'] as $name) {
            $parts = explode("const STARTUP_BARRIER_".$name." = <<<'LUA'\n", file_get_contents('/probe/startup.php'));
            demand(count($parts) === 2); $tail = explode("\nLUA;", $parts[1], 2);
            demand(count($tail) === 2 && strlen($tail[0]) <= 4096); $scripts[$name] = $tail[0];
        }
        $raw = serialize(plain()); $before = files(); $physical = $client->getOption(Redis::OPT_PREFIX).$key;
        $stage = 'expiry_exact_supported_ceiling'; $ceiling = 9007199254740991;
        demand($client->rawCommand('SET', $physical, $raw, 'PXAT', (string)$ceiling) === true);
        demand($client->rawCommand('PEXPIRETIME', $physical) === $ceiling);
        [$status, $value] = invoke('barrier-observe'); demand($status === 0 && $value['key_state'] === 'expiring');
        [$status, $value] = invoke('barrier-enter'); demand($status === 0 && $client->rawCommand('PEXPIRETIME', $physical) === -1);
        [$status, $value] = invoke('barrier-restore');
        // Direct RESP integer is the oracle, never the rounded Lua observation.
        demand($status === 0 && $client->rawCommand('PEXPIRETIME', $physical) === $ceiling
            && $client->rawCommand('GET', $physical) === $raw && files() === $before);
        caseResult('expiry_exact_supported_ceiling', true);
        foreach ([9007199254740992, 9007199254740993] as $deadline) {
            clean($client, $key); $stage = 'expiry_unsupported_observe';
            demand($client->rawCommand('SET', $physical, $raw, 'PXAT', (string)$deadline) === true);
            demand($client->rawCommand('PEXPIRETIME', $physical) === $deadline);
            foreach (['barrier-observe', 'barrier-enter'] as $operation) {
                [$status, $value] = invoke($operation);
                demand($status === 1 && $value['category'] === 'barrier_state_invalid'
                    && !file_exists('/var/www/html/storage/app/startup-barrier.snapshot')
                    && files() === $before && $client->rawCommand('GET', $physical) === $raw
                    && $client->rawCommand('PEXPIRETIME', $physical) === $deadline);
            }
            $stage = 'expiry_unsupported_fixed_scripts';
            demand($client->rawCommand('EVAL', $scripts['READ'], '1', $physical) === ['invalid']);
            demand($client->rawCommand('EVAL', $scripts['CAS'], '1', $physical,
                'string', $raw, (string)$deadline, 'string', $raw, '-1') === 0);
            demand($client->rawCommand('GET', $physical) === $raw
                && $client->rawCommand('PEXPIRETIME', $physical) === $deadline && files() === $before);
            demand($client->rawCommand('SET', $physical, $raw) === true);
            demand($client->rawCommand('EVAL', $scripts['CAS'], '1', $physical,
                'string', $raw, (string)$deadline, 'string', $raw, '-1') === 0);
            demand($client->rawCommand('EVAL', $scripts['CAS'], '1', $physical,
                'string', $raw, '-1', 'string', $raw, (string)$deadline) === 0);
            demand($client->rawCommand('GET', $physical) === $raw && $client->rawCommand('PEXPIRETIME', $physical) === -1
                && files() === $before && !file_exists('/var/www/html/storage/app/startup-barrier.snapshot'));
            caseResult($deadline === 9007199254740992 ? 'expiry_above_ceiling' : 'expiry_above_ceiling_rounded', true);
        }
        clean($client, $key);
        emit(['ok' => true, 'category' => 'fixed_exact_expiry_domain', 'maximum_deadline_ms' => $ceiling,
            'direct_resp_oracle' => true, 'cases' => 3]);
    }
    if ($argv[1] === 'prepare-cookie') {
        $stage = 'public_cookie_setup'; clean($client, $key);
        demand($client->set($store->getPrefix().'public-barrier-unrelated', 'PUBLIC UNRELATED CACHE'));
        $original = plain(); $original['secret'] = 'PUBLIC-OLD-SECRET';
        demand($client->set($key, serialize($original)));
        $stale = $original; $stale['template'] = 'PUBLIC STALE MAINTENANCE TEMPLATE';
        put('/var/www/html/storage/framework/maintenance.php', file_get_contents('/var/www/html/vendor/laravel/framework/src/Illuminate/Foundation/Console/stubs/maintenance-mode.stub'), 0644);
        put('/var/www/html/storage/framework/down', json_encode($stale, JSON_PRETTY_PRINT | JSON_THROW_ON_ERROR), 0644);
        demand(touch('/var/www/html/storage/framework/maintenance.php', 1700000000)
            && touch('/var/www/html/storage/framework/down', 1700000001));
        $cookie = Illuminate\Foundation\Http\MaintenanceModeBypassCookie::create('PUBLIC-OLD-SECRET')->getValue();
        demand(Illuminate\Foundation\Http\MaintenanceModeBypassCookie::isValid($cookie, 'PUBLIC-OLD-SECRET'));
        $decoded = json_decode(base64_decode($cookie, true), true, 8, JSON_THROW_ON_ERROR);
        demand($decoded['expires_at'] > time() + 43000 && $decoded['expires_at'] <= time() + 43200
            && hash_equals(hash_hmac('sha256', (string)$decoded['expires_at'], 'PUBLIC-OLD-SECRET'), $decoded['mac']));
        demand($default->set('public-barrier-cookie', $cookie)
            && $default->set('public-barrier-original', json_encode(['raw' => base64_encode($client->get($key)), 'files' => files()], JSON_THROW_ON_ERROR))
            && $default->set('public-barrier-inert', inert($client, $store->getPrefix())));
        emit(['ok' => true, 'category' => 'public_valid_cookie_and_stale_files_prepared', 'cookie_uses_payload_secret' => true]);
    }
    if (in_array($argv[1], ['cookie-before', 'cookie-enter', 'cookie-held', 'cookie-restore', 'target-restore-denied'], true)) {
        $stage = $argv[1]; $cookie = $default->get('public-barrier-cookie');
        demand(is_string($cookie) && Illuminate\Foundation\Http\MaintenanceModeBypassCookie::isValid($cookie, 'PUBLIC-OLD-SECRET'));
        if ($argv[1] === 'cookie-before') {
            demand(http('/PUBLIC-undefined-startup-barrier', 'GET', null) === 503);
            $valid = http('/PUBLIC-undefined-startup-barrier', 'GET', $cookie, 'http://127.0.0.1:8080/login');
            // The pinned routes end with an unauthenticated local-login
            // fallback. Reaching its 302 proves bypass, without following it.
            demand($valid === 302);
            emit(['ok' => true, 'category' => 'real_valid_cookie_before_plain', 'no_cookie_status' => 503,
                'same_valid_cookie_status' => $valid, 'exact_local_login_location' => true]);
        }
        if ($argv[1] === 'cookie-restore') {
            [$status, $value] = invoke('barrier-restore'); demand($status === 0);
            $original = json_decode($default->get('public-barrier-original'), true, 16, JSON_THROW_ON_ERROR);
            demand(files() === $original['files'] && base64_encode($client->get($key)) === $original['raw']
                && inert($client, $store->getPrefix()) === $default->get('public-barrier-inert'));
            emit(['ok' => true, 'category' => 'pretarget_exact_cookie_and_files_restored', 'observation' => $value]);
        }
        if ($argv[1] === 'target-restore-denied') {
            $before = files(); $rawBefore = $client->get($key);
            [$status, $value] = invoke('barrier-restore');
            demand($status === 1 && files() === $before && $client->get($key) === $rawBefore);
            emit(['ok' => true, 'category' => 'target_cannot_restore_old_snapshot', 'native_final_release_available' => false]);
        }
        [$status, $value] = invoke($argv[1] === 'cookie-enter' ? 'barrier-enter' : 'barrier-observe');
        demand($status === 0 && $value['key_state'] === 'persistent' && $value['payload_profile'] === 'plain503'
            && $value['maintenance_file_state'] === 'absent' && $value['down_file_state'] === 'absent');
        if ($argv[1] === 'cookie-enter') {
            demand($default->set('public-barrier-common-identity', json_encode(array_intersect_key($value,
                array_flip(['redis_identity_hmac', 'cache_prefix_hmac', 'redis_prefix_hmac', 'original_state_hmac', 'current_state_hmac'])), JSON_THROW_ON_ERROR)));
        }
        demand(json_decode($default->get('public-barrier-common-identity'), true, 8, JSON_THROW_ON_ERROR)
            === array_intersect_key($value, array_flip(['redis_identity_hmac', 'cache_prefix_hmac', 'redis_prefix_hmac', 'original_state_hmac', 'current_state_hmac'])));
        demand(inert($client, $store->getPrefix()) === $default->get('public-barrier-inert'));
        $http = ['health' => http('/api/health', 'GET', $cookie),
            'same_valid_cookie' => http('/PUBLIC-undefined-startup-barrier', 'GET', $cookie)];
        demand($http === ['health' => 200, 'same_valid_cookie' => 503]);
        foreach (['/', '/webhooks/public-startup', '/livewire/update', '/terminal'] as $path) {
            demand(($http[$path] = http($path, 'POST', $cookie)) === 503);
        }
        emit(['ok' => true, 'category' => 'real_plain_barrier_held', 'selected_version' => config('constants.coolify.version'),
            'http' => $http, 'inert_inventory_unchanged' => true, 'observation' => $value,
            'native_final_release_available' => false]);
    }
    if ($argv[1] === 'raw-profile') {
        $stage = 'raw_profile';
        demand($client instanceof Redis && $client->getDBNum() === 1
            && $client->getOption(Redis::OPT_SERIALIZER) === Redis::SERIALIZER_NONE
            && $client->getOption(Redis::OPT_COMPRESSION) === Redis::COMPRESSION_NONE);
        demand(hash_file('sha256', '/var/www/html/vendor/laravel/framework/src/Illuminate/Foundation/Console/stubs/maintenance-mode.stub')
            === 'a8dc7e2db39e3e41b20ccb5709ce726679ef69d7a4c78de4a7d63b6665e08289');
        demand($client->exists($key) === 0);
        $app->maintenanceMode()->activate(['except' => ['/api/health'], 'redirect' => null, 'retry' => null,
            'refresh' => null, 'secret' => null, 'status' => 503, 'template' => null]);
        $raw = $client->rawCommand('GET', $physical);
        demand(is_string($raw) && $raw === serialize($app->maintenanceMode()->data()));
        $probe = $client->rawCommand('EVAL', "return {redis.call('GET',KEYS[1]),redis.call('PTTL',KEYS[1]),redis.call('TYPE',KEYS[1]).ok}", '1', $physical);
        demand($probe === [$raw, -1, 'string'] && $client->rawCommand('PEXPIRETIME', $physical) === -1);
        demand($client->rawCommand('GET', $client->getOption(Redis::OPT_PREFIX).$physical) === false);
        demand($client->del($key) === 1);
        emit(['ok' => true, 'category' => 'fixed_raw_redis_profile', 'database' => 1,
            'serializer_none' => true, 'compression_none' => true, 'eval_raw_physical_key' => true,
            'cache_activate_persistent' => true, 'absolute_pexpiretime_supported' => true,
            'stock_stub_sha256' => 'a8dc7e2db39e3e41b20ccb5709ce726679ef69d7a4c78de4a7d63b6665e08289']);
    }
    if (in_array($argv[1], ['matrix', 'progress-matrix', 'expiry-matrix'], true)) {
        $framework = '/var/www/html/storage/framework';
        $snapshot = '/var/www/html/storage/app/startup-barrier.snapshot';
        demand($client->set($store->getPrefix().'public-barrier-unrelated', 'PUBLIC UNRELATED CACHE'));
        $unchanged = inert($client, $store->getPrefix());
        $cases = ['up', 'persistent', 'secret', 'ttl-remaining', 'ttl-expired', 'stale-files',
            'unknown-php', 'symlink', 'parent-symlink', 'hardlink', 'unsafe-mode', 'oversized', 'bad-down',
            'object-cache', 'list-cache', 'oversized-cache', 'wrong-hmac-key', 'corrupt-snapshot', 'wrong-snapshot-hmac',
            'snapshot-mode', 'snapshot-link', 'replay', 'cache-intervening', 'cache-delete-intervening', 'file-intervening',
            'prefix-drift', 'cache-prefix-drift', 'db-drift', 'store-drift', 'driver-drift',
            'storage-drift', 'unexpected-stage', 'snapshot-oversized', 'snapshot-symlink',
            'unrecorded-restore-stage', 'truncated-progress', 'extra-argument',
            'restoration-stage-maintenance', 'restoration-stage-both', 'interrupted-before-files',
            'reordered-stage-progress', 'interrupted-pre-cas', 'interrupted-post-cas',
            'restore-intent-pre-cas', 'restore-intent-post-cas', 'reordered-progress'];
        if ($argv[1] === 'progress-matrix') {
            $cases = ['reordered-stage-progress', 'interrupted-pre-cas', 'interrupted-post-cas',
                'restore-intent-pre-cas', 'restore-intent-post-cas', 'reordered-progress'];
        }
        if ($argv[1] === 'expiry-matrix') { $cases = ['up', 'persistent', 'ttl-remaining', 'ttl-expired']; }
        foreach ($cases as $case) {
            $stage = $case; clean($client, $key); $original = plain();
            $raw = null;
            if (in_array($case, ['persistent', 'secret', 'ttl-remaining', 'ttl-expired', 'stale-files'], true)) {
                if ($case === 'secret') { $original['secret'] = 'PUBLIC-OLD-SECRET'; }
                $raw = serialize($original);
                if (str_starts_with($case, 'ttl-')) { demand($client->psetex($key, $case === 'ttl-expired' ? 800 : 10000, $raw)); }
                else { demand($client->set($key, $raw)); }
            }
            if (in_array($case, ['stale-files', 'restoration-stage-maintenance', 'restoration-stage-both', 'reordered-stage-progress'], true)) {
                $original['template'] = 'PUBLIC STALE TEMPLATE'; $original['secret'] = 'PUBLIC-STALE-SECRET';
                $original['redirect'] = '/PUBLIC-STALE';
                put($framework.'/maintenance.php', file_get_contents('/var/www/html/vendor/laravel/framework/src/Illuminate/Foundation/Console/stubs/maintenance-mode.stub'), 0644);
                put($framework.'/down', json_encode($original, JSON_PRETTY_PRINT | JSON_THROW_ON_ERROR), 0644);
                demand(touch($framework.'/maintenance.php', 1700000000) && touch($framework.'/down', 1700000001));
            }
            if ($case === 'unknown-php') { put($framework.'/maintenance.php', '<?php exit;'); }
            if ($case === 'symlink') { demand(symlink('/probe/hmac.key', $framework.'/maintenance.php')); }
            if ($case === 'parent-symlink') {
                demand(rename($framework, '/var/www/html/storage/.public-framework-original')
                    && symlink('/var/www/html/storage/.public-framework-original', $framework));
            }
            if ($case === 'hardlink') { put($framework.'/down', json_encode(plain())); demand(link($framework.'/down', $framework.'/.public-barrier-link')); }
            if ($case === 'unsafe-mode') { put($framework.'/down', json_encode(plain()), 0666); }
            if ($case === 'oversized') { put($framework.'/down', str_repeat('P', 65537)); }
            if ($case === 'bad-down') { put($framework.'/down', '{"secret":null,"secret":"PUBLIC"}'); }
            if ($case === 'object-cache') { demand($client->set($key, serialize(new stdClass))); }
            if ($case === 'list-cache') { demand($client->rpush($key, 'PUBLIC') === 1); }
            if ($case === 'oversized-cache') { demand($client->set($key, str_repeat('P', 65537))); }
            if ($case === 'unexpected-stage') { put($framework.'/.startup-barrier-down', 'PUBLIC FOREIGN STAGE'); }
            if ($case === 'interrupted-before-files') {
                put($framework.'/maintenance.php', file_get_contents('/var/www/html/vendor/laravel/framework/src/Illuminate/Foundation/Console/stubs/maintenance-mode.stub'));
                demand(chmod($framework, 0500));
            }
            $before = files();
            $beforeRaw = $case === 'list-cache' ? null : $client->get($key);
            $stage = $case.'-enter'; [$status, $value] = invoke('barrier-enter');
            if (in_array($case, ['unknown-php', 'symlink', 'parent-symlink', 'hardlink', 'unsafe-mode', 'oversized', 'bad-down', 'object-cache', 'list-cache', 'oversized-cache', 'unexpected-stage'], true)) {
                demand($status === 1 && !file_exists($snapshot) && files() === $before);
                demand($case === 'list-cache' ? $client->lrange($key, 0, -1) === ['PUBLIC'] : $client->get($key) === $beforeRaw);
                if ($case === 'parent-symlink') { demand(unlink($framework) && rename('/var/www/html/storage/.public-framework-original', $framework)); }
                demand(inert($client, $store->getPrefix()) === $unchanged); caseResult($case, true); continue;
            }
            demand($status === ($case === 'interrupted-before-files' ? 1 : 0));
            if ($status === 0) {
                demand($value['payload_profile'] === 'plain503' && $value['key_state'] === 'persistent'
                    && $value['snapshot_state'] === 'held' && files() === ['maintenance.php' => null, 'down' => null]
                    && $client->pttl($key) === -1 && $client->get($key) === serialize(plain()));
            } else { demand($client->get($key) === serialize(plain()) && files() === $before); }
            if ($case === 'ttl-expired') { usleep(1100000); }
            if ($case === 'wrong-hmac-key') { put('/probe/hmac.key', str_repeat('Q', 32)); }
            if ($case === 'corrupt-snapshot') { put($snapshot, 'PUBLIC CORRUPT SNAPSHOT'); }
            if ($case === 'wrong-snapshot-hmac') {
                $lines = explode("\n", rtrim(file_get_contents($snapshot), "\n"));
                $data = json_decode($lines[0], true, 32, JSON_THROW_ON_ERROR); $data['hmac'] = str_repeat('0', 64);
                $lines[0] = json_encode($data, JSON_THROW_ON_ERROR); put($snapshot, implode("\n", $lines)."\n");
            }
            if ($case === 'snapshot-mode') { demand(chmod($snapshot, 0644)); }
            if ($case === 'snapshot-link') { demand(link($snapshot, $framework.'/.public-barrier-link')); }
            if ($case === 'snapshot-oversized') { put($snapshot, str_repeat('P', 524289)); }
            if ($case === 'snapshot-symlink') { demand(rename($snapshot, $framework.'/.public-barrier-link') && symlink($framework.'/.public-barrier-link', $snapshot)); }
            if ($case === 'storage-drift') { demand(chmod($framework, 0700)); }
            if ($case === 'unrecorded-restore-stage') {
                put($framework.'/.startup-barrier-maintenance', file_get_contents('/var/www/html/vendor/laravel/framework/src/Illuminate/Foundation/Console/stubs/maintenance-mode.stub'));
            }
            if ($case === 'truncated-progress') { demand(file_put_contents($snapshot, '{', FILE_APPEND) === 1); }
            if (str_starts_with($case, 'restoration-stage-') || $case === 'reordered-stage-progress') {
                // Synthetic authenticated intermediate state of the finite
                // two-file protocol; no product hook or trusted phase grant.
                $originalRecord = json_decode(explode("\n", file_get_contents($snapshot))[0], true, 16, JSON_THROW_ON_ERROR)['data'];
                foreach (in_array($case, ['restoration-stage-both', 'reordered-stage-progress'], true) ? ['maintenance', 'down'] : ['maintenance'] as $name) {
                    $record = $originalRecord['files'][$name]; $temporary = $framework.'/.startup-barrier-'.$name;
                    put($temporary, base64_decode($record['bytes'], true), $record['identity']['mode'] & 07777);
                    demand(touch($temporary, $record['identity']['mtime'])); clearstatcache(true, $temporary);
                    $identity = array_intersect_key(lstat($temporary), array_flip(['dev', 'ino', 'mode', 'nlink', 'uid', 'gid', 'size', 'mtime']));
                    $data = ['nonce' => $originalRecord['nonce'], 'file' => $name, 'identity' => $identity];
                    $hmac = hash_hmac('sha256', "barrier-snapshot\0".json_encode($data, JSON_THROW_ON_ERROR), str_repeat('P', 32));
                    demand(file_put_contents($snapshot, json_encode(['data' => $data, 'hmac' => $hmac], JSON_THROW_ON_ERROR)."\n", FILE_APPEND) > 0);
                    if ($name === 'maintenance') { demand(rename($temporary, $framework.'/maintenance.php')); }
                }
                if ($case === 'reordered-stage-progress') {
                    $lines = explode("\n", rtrim(file_get_contents($snapshot), "\n"));
                    demand(count($lines) === 4); [$lines[2], $lines[3]] = [$lines[3], $lines[2]];
                    put($snapshot, implode("\n", $lines)."\n");
                }
            }
            if (in_array($case, ['interrupted-pre-cas', 'interrupted-post-cas', 'restore-intent-pre-cas', 'restore-intent-post-cas', 'reordered-progress'], true)) {
                // Source-owned synthetic boundaries of this finite protocol.
                // Real permission-induced post-CAS failure is tested separately.
                $lines = explode("\n", rtrim(file_get_contents($snapshot), "\n")); demand(count($lines) === 2);
                $nonce = json_decode($lines[0], true, 16, JSON_THROW_ON_ERROR)['data']['nonce'];
                $data = ['nonce' => $nonce, 'phase' => 'release-ready'];
                $ready = json_encode(['data' => $data, 'hmac' => hash_hmac('sha256', "barrier-snapshot\0".json_encode($data, JSON_THROW_ON_ERROR), str_repeat('P', 32))], JSON_THROW_ON_ERROR);
                if (str_starts_with($case, 'interrupted-')) { put($snapshot, $lines[0]."\n"); }
                elseif ($case === 'reordered-progress') { put($snapshot, $lines[0]."\n".$ready."\n".$lines[1]."\n"); }
                else { put($snapshot, implode("\n", $lines)."\n".$ready."\n"); }
                if (in_array($case, ['interrupted-pre-cas', 'restore-intent-post-cas'], true)) { demand($client->del($key) === 1); }
                if (str_starts_with($case, 'interrupted-')) {
                    [$observed, $preparing] = invoke('barrier-observe');
                    demand($observed === 0 && $preparing['snapshot_state'] === 'preparing');
                }
            }
            if ($case === 'cache-intervening') { demand($client->set($key, serialize($original + ['PUBLIC' => true]))); }
            if ($case === 'cache-delete-intervening') { demand($client->del($key) === 1); }
            if ($case === 'file-intervening') { put($framework.'/down', json_encode(plain())); }
            $env = match ($case) {'prefix-drift' => 'REDIS_PREFIX=PUBLIC_DRIFT_', 'cache-prefix-drift' => 'CACHE_PREFIX=PUBLIC_DRIFT_',
                'db-drift' => 'REDIS_CACHE_DB=0', 'store-drift' => 'APP_MAINTENANCE_STORE=array',
                'driver-drift' => 'APP_MAINTENANCE_DRIVER=file', default => null};
            if ($env !== null) { putenv($env); }
            $heldFiles = files(); $heldRaw = $client->get($key);
            $stage = $case.'-restore'; [$restored, $after] = invoke('barrier-restore', $case === 'extra-argument' ? ['final_success=true'] : []);
            if ($env !== null) { putenv(explode('=', $env)[0].'='.match ($case) {
                'prefix-drift' => 'sifto_public_fixture_database_', 'cache-prefix-drift' => 'sifto_public_fixture_cache_',
                'db-drift' => '1', 'driver-drift' => 'cache', default => 'redis'}); }
            if (in_array($case, ['wrong-hmac-key', 'corrupt-snapshot', 'wrong-snapshot-hmac', 'snapshot-mode', 'snapshot-link',
                'cache-intervening', 'cache-delete-intervening', 'file-intervening', 'prefix-drift', 'cache-prefix-drift', 'db-drift', 'store-drift', 'driver-drift',
                'storage-drift', 'snapshot-oversized', 'snapshot-symlink', 'unrecorded-restore-stage', 'truncated-progress', 'extra-argument',
                'reordered-stage-progress', 'reordered-progress'], true)) {
                demand($restored === 1 && files() === $heldFiles && $client->get($key) === $heldRaw);
                demand(inert($client, $store->getPrefix()) === $unchanged); caseResult($case, true); continue;
            }
            demand($restored === 0 && $after['snapshot_state'] === 'restored' && files() === $before);
            if ($case === 'ttl-expired') { demand($client->get($key) === false && $client->pttl($key) === -2); }
            elseif ($case === 'ttl-remaining') { demand($client->get($key) === $beforeRaw && $client->pttl($key) > 0 && $client->pttl($key) < 10000); }
            else { demand($client->get($key) === $beforeRaw && $client->pttl($key) === ($beforeRaw === false ? -2 : -1)); }
            [$replay, ] = invoke('barrier-restore'); demand($replay === 1);
            [$reenter, ] = invoke('barrier-enter'); demand($reenter === 1);
            demand(inert($client, $store->getPrefix()) === $unchanged); caseResult($case, true);
        }
        clean($client, $key);
        $events = file_get_contents('/sys/fs/cgroup/memory.events'); $peak = trim(file_get_contents('/sys/fs/cgroup/memory.peak'));
        demand(preg_match('/^oom_kill 0$/m', $events) === 1 && ctype_digit($peak) && (int)$peak <= 134217728);
        emit(['ok' => true, 'category' => 'public_barrier_matrix', 'cases' => count($cases),
            'probe_memory_peak_bytes' => (int)$peak, 'probe_memory_limit_bytes' => 134217728, 'oom_kill' => 0]);
    }
    demand($argv[1] === 'absent-red');
    $stage = 'missing_barrier_mode';
    [$status, $value] = invoke('barrier-observe');
    demand($status === 0 && $value['key_state'] === 'absent');
    emit(['ok' => true, 'category' => 'absent_barrier_observed']);
} catch (Throwable $failure) { emit(['ok' => false, 'category' => 'public_barrier_fixture_failed', 'stage' => $stage], 1); }
