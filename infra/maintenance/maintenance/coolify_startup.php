<?php
// Fixed clone-only SSH producer. Native dispatch/registration remains closed.
// No identifier, path, ciphertext, plaintext or exception crosses stdout/stderr.
// Fixed-stage precondition: owned keys/mux directories already exist as 0700,
// with no concurrent application writer. Future native staging must create
// these directories; this producer never creates or repairs existing parents.
declare(strict_types=1);
error_reporting(0);
ini_set('display_errors', '0');
ini_set('log_errors', '0');
ob_start();

const STARTUP_KEYS = '/var/www/html/storage/app/ssh/keys';
const STARTUP_MUX = '/var/www/html/storage/app/ssh/mux';
const STARTUP_MAX_ENTRIES = 1024;
const STARTUP_MAX_KEY = 65536;
const STARTUP_MAX_TOTAL = 8388608;
$startupDeadline = hrtime(true) + 30000000000;

function startupGate(bool $condition): void {
    global $startupDeadline;
    if (!$condition || hrtime(true) > $startupDeadline) { throw new RuntimeException('startup_gate'); }
}

function startupResult(array $value, int $status): never {
    while (ob_get_level() > 0) { ob_end_clean(); }
    echo json_encode($value, JSON_THROW_ON_ERROR)."\n";
    exit($status);
}

function startupIdentity(array $info): array {
    $result = [];
    // Reading may update atime; never chmod/chown/touch an existing leaf.
    foreach (['dev', 'ino', 'mode', 'nlink', 'uid', 'gid', 'size', 'mtime', 'ctime'] as $name) {
        startupGate(isset($info[$name]) && is_int($info[$name])); $result[$name] = $info[$name];
    }
    return $result;
}

function startupParents(string $path): array {
    $result = [];
    for ($parent = dirname($path); ; $parent = dirname($parent)) {
        clearstatcache(true, $parent); $info = lstat($parent);
        startupGate(is_array($info) && ($info['mode'] & 0170000) === 0040000 && realpath($parent) === $parent);
        // Directory entries change during exclusive creates; directory identity,
        // ownership and permission bits must remain stable, including ancestors.
        $result[$parent] = array_intersect_key(startupIdentity($info), array_flip(['dev', 'ino', 'mode', 'uid', 'gid']));
        if ($parent === '/') { break; }
    }
    return $result;
}

function startupRead(string $path, int $limit, bool $strictMode = true): array {
    $parents = startupParents($path);
    clearstatcache(true, $path); $before = lstat($path);
    startupGate(is_array($before)); $identity = startupIdentity($before);
    startupGate(($before['mode'] & 0170000) === 0100000 && $before['nlink'] === 1
        && $before['uid'] === posix_geteuid() && $before['gid'] === posix_getegid()
        && (!$strictMode || ($before['mode'] & 07777) === 0600)
        && $before['size'] >= 0 && $before['size'] <= $limit);
    $stream = fopen($path, 'rb'); startupGate(is_resource($stream));
    try {
        startupGate(startupIdentity(fstat($stream)) === $identity);
        $bytes = stream_get_contents($stream, $limit + 1);
        startupGate(is_string($bytes) && strlen($bytes) === $before['size']);
        startupGate(startupIdentity(fstat($stream)) === $identity);
        clearstatcache(true, $path); $after = lstat($path);
        startupGate(is_array($after) && startupIdentity($after) === $identity && startupParents($path) === $parents);
        return [$identity, $bytes];
    } finally { fclose($stream); }
}

function startupInventory(string $root, bool $mux): array {
    clearstatcache(true, $root); $rootInfo = lstat($root);
    startupGate(is_array($rootInfo) && $rootInfo['mode'] === (0040000 | 0700)
        && $rootInfo['uid'] === posix_geteuid() && $rootInfo['gid'] === posix_getegid());
    $parents = startupParents($root.'/fixed-leaf');
    $directory = opendir($root); startupGate(is_resource($directory));
    $entries = []; $total = 0;
    try {
        while (($name = readdir($directory)) !== false) {
            if ($name === '.' || $name === '..') { continue; }
            startupGate(count($entries) < STARTUP_MAX_ENTRIES && strlen($name) <= 255);
            $path = $root.'/'.$name; clearstatcache(true, $path); $info = lstat($path);
            startupGate(is_array($info)); $identity = startupIdentity($info);
            if ($mux) {
                // Sockets have no file content to open. All mux entries are
                // observed as metadata only; no connect, unlink or chmod.
                startupGate(in_array($info['mode'] & 0170000, [0100000, 0140000], true)
                    && $info['uid'] === posix_geteuid() && $info['nlink'] === 1);
                $entries[$name] = [$identity];
            } else {
                [$identity, $bytes] = startupRead($path, STARTUP_MAX_KEY, false);
                $total += strlen($bytes); startupGate($total <= STARTUP_MAX_TOTAL);
                $entries[$name] = [$identity, hash('sha256', $bytes)]; unset($bytes);
            }
        }
    } finally { closedir($directory); }
    startupGate(startupParents($root.'/fixed-leaf') === $parents);
    ksort($entries, SORT_STRING); return $entries;
}

function startupHmac(string $domain, mixed $value, string $key): string {
    return hash_hmac('sha256', $domain."\0".json_encode($value, JSON_THROW_ON_ERROR), $key);
}

function startupCreate(string $path, string $plain, array $parents): array {
    global $startupCategory;
    $startupCategory = 'ssh_write_open_failed';
    startupGate(startupParents($path) === $parents);
    // Laravel converts an expected ENOENT warning into an exception. Suppress
    // only this absence probe; exclusive fopen still rejects every collision.
    clearstatcache(true, $path); startupGate(@lstat($path) === false);
    $oldMask = umask(0077);
    try { $stream = fopen($path, 'x+b'); } finally { umask($oldMask); }
    startupGate(is_resource($stream));
    try {
        $opened = fstat($stream); startupGate(is_array($opened));
        startupGate(($opened['mode'] & 0170000) === 0100000 && ($opened['mode'] & 07777) === 0600
            && $opened['uid'] === posix_geteuid() && $opened['gid'] === posix_getegid()
            && $opened['nlink'] === 1 && $opened['size'] === 0);
        $startupCategory = 'ssh_write_content_failed';
        $offset = 0;
        while ($offset < strlen($plain)) {
            startupGate(startupParents($path) === $parents);
            $written = fwrite($stream, substr($plain, $offset));
            startupGate(is_int($written) && $written > 0); $offset += $written;
        }
        $startupCategory = 'ssh_write_sync_failed';
        startupGate(fflush($stream) && fsync($stream));
        $startupCategory = 'ssh_write_identity_failed';
        $after = fstat($stream); startupGate(is_array($after));
        foreach (['dev', 'ino', 'mode', 'uid', 'gid', 'nlink'] as $field) { startupGate($after[$field] === $opened[$field]); }
        startupGate($after['size'] === strlen($plain) && fseek($stream, 0) === 0);
        $stored = stream_get_contents($stream, STARTUP_MAX_KEY + 1);
        startupGate(is_string($stored) && hash_equals($plain, $stored));
        $identity = startupIdentity($after);
        clearstatcache(true, $path); $final = lstat($path);
        startupGate(is_array($final) && startupIdentity($final) === $identity
            && startupIdentity(fstat($stream)) === $identity && startupParents($path) === $parents);
        return [$identity, hash('sha256', $stored)];
    } finally { fclose($stream); }
}

// Clone-only PRETARGET primitives. These arguments are literal dispatch modes,
// never a trusted engine phase or authority to release a post-target barrier.
const STARTUP_BARRIER_SNAPSHOT = '/var/www/html/storage/app/startup-barrier.snapshot';
const STARTUP_BARRIER_STUB = 'a8dc7e2db39e3e41b20ccb5709ce726679ef69d7a4c78de4a7d63b6665e08289';
const STARTUP_BARRIER_READ = <<<'LUA'
local t=redis.call('TYPE',KEYS[1]).ok
if t~='none' and t~='string' then return {'invalid'} end
if t=='string' and redis.call('STRLEN',KEYS[1])>65536 then return {'invalid'} end
local deadline=redis.call('PEXPIRETIME',KEYS[1])
if deadline>9007199254740991 then return {'invalid'} end
local time=redis.call('TIME')
return {t,redis.call('GET',KEYS[1]) or '',redis.call('PTTL',KEYS[1]),tonumber(time[1])*1000+math.floor(tonumber(time[2])/1000),deadline}
LUA;
const STARTUP_BARRIER_CAS = <<<'LUA'
local t=redis.call('TYPE',KEYS[1]).ok
if t~='none' and t~='string' then return 0 end
if t=='string' and redis.call('STRLEN',KEYS[1])>65536 then return 0 end
local deadline=redis.call('PEXPIRETIME',KEYS[1])
if deadline>9007199254740991 or tonumber(ARGV[3])>9007199254740991 or tonumber(ARGV[6])>9007199254740991 then return 0 end
local time=redis.call('TIME')
local now=tonumber(time[1])*1000+math.floor(tonumber(time[2])/1000)
local matches=(t==ARGV[1] and deadline==tonumber(ARGV[3]) and (t=='none' or redis.call('GET',KEYS[1])==ARGV[2]))
if not matches then return 0 end
if ARGV[4]=='none' or (tonumber(ARGV[6])>0 and tonumber(ARGV[6])<=now) then redis.call('DEL',KEYS[1])
elseif tonumber(ARGV[6])==-1 then redis.call('SET',KEYS[1],ARGV[5])
else redis.call('SET',KEYS[1],ARGV[5],'PXAT',ARGV[6]) end
return 1
LUA;

function startupBarrierPlain(): array {
    return ['except' => ['/api/health'], 'redirect' => null, 'retry' => null,
        'refresh' => null, 'secret' => null, 'status' => 503, 'template' => null];
}

function startupBarrierPayload(mixed $data): void {
    startupGate(is_array($data) && array_keys($data) === array_keys(startupBarrierPlain()));
    startupGate(is_array($data['except']) && array_is_list($data['except']) && count($data['except']) <= 16);
    foreach ($data['except'] as $item) { startupGate(is_string($item) && strlen($item) <= 256); }
    foreach (['redirect', 'secret', 'template'] as $name) {
        startupGate($data[$name] === null || is_string($data[$name]) && strlen($data[$name]) <= ($name === 'template' ? 32768 : 1024));
    }
    foreach (['retry', 'refresh'] as $name) {
        startupGate($data[$name] === null || is_int($data[$name]) && $data[$name] >= 0 && $data[$name] <= 2147483647
            || is_string($data[$name]) && preg_match('/\A[0-9]{1,10}\z/', $data[$name]) === 1);
    }
    startupGate(is_int($data['status']) && $data['status'] >= 100 && $data['status'] <= 599);
}

function startupBarrierRaw(Redis $client, string $physical, ?int &$now = null): array {
    $value = $client->rawCommand('EVAL', STARTUP_BARRIER_READ, '1', $physical);
    startupGate(is_array($value) && count($value) === 5 && in_array($value[0], ['none', 'string'], true)
        && is_string($value[1]) && strlen($value[1]) <= 65536);
    foreach ([2, 3, 4] as $index) { startupGate(is_int($value[$index])); }
    startupGate($value[3] >= 1700000000000 && $value[3] <= 4102444800000);
    $now = $value[3];
    if ($value[0] === 'none') { startupGate($value[1] === '' && $value[2] === -2 && $value[4] === -2); }
    else {
        startupGate(($value[2] === -1 && $value[4] === -1) || ($value[2] >= 0 && $value[4] >= $value[3]));
        // Never use CacheBasedMaintenanceMode::data()/RedisStore::get() here:
        // normal RedisStore deserialization permits object construction.
        $data = unserialize($value[1], ['allowed_classes' => false, 'max_depth' => 8]);
        startupBarrierPayload($data); startupGate(serialize($data) === $value[1]);
    }
    return ['type' => $value[0], 'raw' => base64_encode($value[1]), 'deadline' => $value[4]];
}

function startupBarrierFile(string $name): array {
    $path = '/var/www/html/storage/framework/'.($name === 'maintenance' ? 'maintenance.php' : 'down');
    startupParents($path); clearstatcache(true, $path);
    if (@lstat($path) === false) { return ['identity' => null, 'bytes' => null]; }
    [$identity, $bytes] = startupRead($path, 65536, false);
    startupGate(in_array($identity['mode'] & 07777, [0600, 0644], true));
    if ($name === 'maintenance') { startupGate(hash('sha256', $bytes) === STARTUP_BARRIER_STUB); }
    else {
        $data = json_decode($bytes, true, 8, JSON_THROW_ON_ERROR); startupBarrierPayload($data);
        // These are the two fixed Laravel writer encodings. Exact round-trip
        // rejects duplicate JSON keys without interpreting arbitrary PHP.
        startupGate($bytes === json_encode($data, JSON_THROW_ON_ERROR)
            || $bytes === json_encode($data, JSON_PRETTY_PRINT | JSON_THROW_ON_ERROR));
    }
    return ['identity' => $identity, 'bytes' => base64_encode($bytes)];
}

function startupBarrierStorage(): array {
    $result = [];
    foreach (['/var/www/html/storage', '/var/www/html/storage/app', '/var/www/html/storage/framework'] as $path) {
        $parents = startupParents($path.'/fixed-leaf'); $identity = $parents[$path];
        startupGate($identity['uid'] === 9999 && $identity['gid'] === 9999 && ($identity['mode'] & 0022) === 0);
        $result[] = $identity;
    }
    return $result;
}

function startupBarrierSemantic(array $files): array {
    foreach ($files as &$file) {
        if ($file['identity'] !== null) {
            // Recreated files cannot retain inode or ctime. Restoration proves
            // bytes and restorable mode/uid/gid/mtime, not original inode/ctime.
            $file['identity'] = array_intersect_key($file['identity'], array_flip(['mode', 'uid', 'gid', 'size', 'mtime']));
        }
    }
    unset($file); return $files;
}

function startupBarrierStagedIdentity(array $identity): array {
    // Rename updates ctime. A recorded stage authenticates the same inode and
    // all other metadata plus exact content, never an invented original ctime.
    unset($identity['ctime']); return $identity;
}

function startupBarrierSyncDirectory(string $path): void {
    $stream = fopen(dirname($path), 'r'); startupGate(is_resource($stream));
    try { startupGate(fsync($stream)); } finally { fclose($stream); }
}

function startupBarrierWrite($stream, string $bytes): void {
    startupGate(is_resource($stream) && strlen($bytes) <= 524288);
    for ($offset = 0; $offset < strlen($bytes); $offset += $written) {
        $written = fwrite($stream, substr($bytes, $offset)); startupGate(is_int($written) && $written > 0);
    }
    startupGate(fflush($stream) && fsync($stream));
}

function startupBarrierEnvelope(array $data, string $key): string {
    return json_encode(['data' => $data, 'hmac' => startupHmac('barrier-snapshot', $data, $key)], JSON_THROW_ON_ERROR)."\n";
}

function startupBarrierSnapshot(string $key): ?array {
    clearstatcache(true, STARTUP_BARRIER_SNAPSHOT);
    if (@lstat(STARTUP_BARRIER_SNAPSHOT) === false) { startupParents(STARTUP_BARRIER_SNAPSHOT); return null; }
    [$identity, $bytes] = startupRead(STARTUP_BARRIER_SNAPSHOT, 524288);
    startupGate(str_ends_with($bytes, "\n")); $lines = explode("\n", rtrim($bytes, "\n"));
    startupGate(count($lines) >= 1 && count($lines) <= 6);
    $records = [];
    foreach ($lines as $line) {
        $value = json_decode($line, true, 16, JSON_THROW_ON_ERROR);
        startupGate(is_array($value) && array_keys($value) === ['data', 'hmac'] && is_array($value['data'])
            && is_string($value['hmac']) && hash_equals(startupHmac('barrier-snapshot', $value['data'], $key), $value['hmac'])
            && startupBarrierEnvelope($value['data'], $key) === $line."\n");
        $records[] = $value['data'];
    }
    $original = array_shift($records);
    startupGate(array_keys($original) === ['version', 'source_version', 'source_sha256', 'nonce', 'storage', 'redis_identity', 'intent', 'redis', 'files']
        && $original['version'] === 1 && in_array($original['source_version'], ['4.4.2', '4.4.6'], true)
        && $original['source_sha256'] === hash_file('sha256', __FILE__)
        && is_string($original['nonce']) && preg_match('/\A[a-f0-9]{64}\z/', $original['nonce']) === 1
        && $original['storage'] === startupBarrierStorage()
        && $original['intent'] === ['horizon' => false, 'scheduler' => false]);
    $staged = []; $entered = false; $releaseReady = false; $done = false;
    foreach ($records as $record) {
        startupGate(!$done && ($record['nonce'] ?? null) === $original['nonce']);
        if (array_keys($record) === ['nonce', 'phase']) {
            if ($record['phase'] === 'entered') {
                startupGate(!$entered && !$releaseReady && $staged === []); $entered = true;
            } else {
                startupGate($record['phase'] === 'release-ready' && !$releaseReady); $releaseReady = true;
            }
        } elseif (array_keys($record) === ['nonce', 'restored'] && $record['restored'] === true) {
            startupGate($releaseReady); $done = true;
        }
        else {
            startupGate($entered && !$releaseReady && array_keys($record) === ['nonce', 'file', 'identity']
                && in_array($record['file'], ['maintenance', 'down'], true) && is_array($record['identity'])
                && !array_key_exists($record['file'], $staged)
                && ($record['file'] !== 'maintenance' || !array_key_exists('down', $staged)));
            $staged[$record['file']] = $record['identity'];
        }
    }
    return ['identity' => $identity, 'original' => $original, 'staged' => $staged,
        'entered' => $entered, 'release_ready' => $releaseReady, 'done' => $done];
}

function startupBarrierRestoreFile(string $name, array $original, ?array $staged, $snapshot, string $nonce, string $key): void {
    $current = startupBarrierFile($name);
    if ($current === $original) { return; }
    $path = '/var/www/html/storage/framework/'.($name === 'maintenance' ? 'maintenance.php' : 'down');
    $temporary = '/var/www/html/storage/framework/.startup-barrier-'.$name;
    if ($original['identity'] === null) { startupGate($current === $original && $staged === null); return; }
    if ($staged !== null) {
        if ($current['identity'] !== null && startupBarrierStagedIdentity($current['identity']) === $staged
            && startupBarrierSemantic([$current]) === startupBarrierSemantic([$original])) { return; }
        startupGate($current['identity'] === null);
        [$identity, $bytes] = startupRead($temporary, 65536, false);
        startupGate(startupBarrierStagedIdentity($identity) === $staged && base64_encode($bytes) === $original['bytes']);
    } else {
        startupGate($current['identity'] === null && @lstat($temporary) === false);
        $oldMask = umask(0077);
        try { $stream = fopen($temporary, 'x+b'); } finally { umask($oldMask); }
        startupGate(is_resource($stream));
        try {
            startupBarrierWrite($stream, base64_decode($original['bytes'], true));
            startupGate(chmod($temporary, $original['identity']['mode'] & 07777));
            startupGate(touch($temporary, $original['identity']['mtime']));
            startupGate(fflush($stream) && fsync($stream)); $identity = startupIdentity(fstat($stream));
            startupGate($identity['uid'] === $original['identity']['uid'] && $identity['gid'] === $original['identity']['gid']
                && $identity['nlink'] === 1 && $identity['size'] === $original['identity']['size']);
        } finally { fclose($stream); }
        startupBarrierSyncDirectory($temporary);
        startupBarrierWrite($snapshot, startupBarrierEnvelope(['nonce' => $nonce, 'file' => $name,
            'identity' => startupBarrierStagedIdentity($identity)], $key));
    }
    startupGate(startupBarrierFile($name)['identity'] === null && rename($temporary, $path));
    startupBarrierSyncDirectory($path);
    $current = startupBarrierFile($name);
    startupGate($current['identity'] !== null && startupBarrierStagedIdentity($current['identity']) === startupBarrierStagedIdentity($identity)
        && startupBarrierSemantic([$current]) === startupBarrierSemantic([$original]));
}

function startupBarrierContext(string $hmacKey): array {
    global $startupCategory;
    $startupCategory = 'barrier_environment_invalid';
    startupGate(config('app.maintenance.driver') === 'cache' && config('app.maintenance.store') === 'redis'
        && config('cache.stores.redis.driver') === 'redis' && config('cache.stores.redis.connection') === 'cache'
        && config('database.redis.client') === 'phpredis' && empty(config('database.redis.clusters'))
        && config('database.redis.options.cluster') === 'redis');
    // clone_env derives these from authenticated APP_NAME when not explicitly
    // configured. Observe the effective bounded prefixes; never whitelist a
    // PUBLIC fixture value as execution authority. Snapshot identity binds both.
    foreach ([config('cache.prefix'), config('database.redis.options.prefix')] as $prefix) {
        startupGate(is_string($prefix) && preg_match('/\A[a-zA-Z0-9_.:-]{0,192}\z/', $prefix) === 1);
    }
    $config = config('database.redis.cache');
    startupGate(is_array($config) && $config['host'] === '127.0.0.1' && (string)$config['port'] === '6379'
        && (string)$config['database'] === '1' && $config['password'] === 'rehearsal-only-password'
        && empty($config['url']) && empty($config['username']));
    $store = Illuminate\Support\Facades\Cache::store('redis')->getStore();
    startupGate(get_class($store) === Illuminate\Cache\RedisStore::class
        && get_class($store->connection()) === Illuminate\Redis\Connections\PhpRedisConnection::class);
    $client = $store->connection()->client();
    startupGate(get_class($client) === Redis::class && $client->getDBNum() === 1
        && $client->getOption(Redis::OPT_SERIALIZER) === Redis::SERIALIZER_NONE
        && $client->getOption(Redis::OPT_COMPRESSION) === Redis::COMPRESSION_NONE
        && $client->getOption(Redis::OPT_PREFIX) === config('database.redis.options.prefix')
        && $store->getPrefix() === config('cache.prefix'));
    $server = $client->info('server');
    startupGate(is_array($server) && is_string($server['run_id'] ?? null)
        && preg_match('/\A[a-f0-9]{40}\z/', $server['run_id']) === 1);
    $redisIdentity = startupHmac('barrier-redis', [$server['run_id'], $config, $store->getPrefix(),
        $client->getOption(Redis::OPT_PREFIX), $client->getDBNum(), Redis::SERIALIZER_NONE, Redis::COMPRESSION_NONE], $hmacKey);
    startupGate(hash_file('sha256', '/var/www/html/vendor/laravel/framework/src/Illuminate/Foundation/Console/stubs/maintenance-mode.stub') === STARTUP_BARRIER_STUB);
    $middleware = (new ReflectionClass(App\Http\Middleware\PreventRequestsDuringMaintenance::class))->newInstanceWithoutConstructor();
    startupGate($middleware->getExcludedPaths() === ['/api/health']);
    $storage = startupBarrierStorage();
    $physical = $client->getOption(Redis::OPT_PREFIX).$store->getPrefix().'illuminate:foundation:down';
    return [$client, $physical, $redisIdentity, $storage, $store, $config, $server['run_id']];
}

function startupBarrier(string $operation, string $version, string $hmacKey): never {
    global $startupCategory;
    [$client, $physical, $redisIdentity, $storage, $store] = startupBarrierContext($hmacKey);
    $startupCategory = 'barrier_state_invalid';
    $raw = startupBarrierRaw($client, $physical);
    $files = ['maintenance' => startupBarrierFile('maintenance'), 'down' => startupBarrierFile('down')];
    $startupCategory = 'barrier_snapshot_invalid';
    $snapshot = startupBarrierSnapshot($hmacKey);
    if ($snapshot !== null) { startupGate($snapshot['original']['redis_identity'] === $redisIdentity); }
    foreach (['maintenance', 'down'] as $name) {
        $temporary = '/var/www/html/storage/framework/.startup-barrier-'.$name;
        clearstatcache(true, $temporary); $stageInfo = @lstat($temporary);
        $staged = $snapshot['staged'][$name] ?? null;
        if ($staged === null) { startupGate($stageInfo === false); }
        else {
            $before = $snapshot['original']['files'][$name]; startupGate($before['identity'] !== null);
            if ($stageInfo === false) {
                startupGate($files[$name]['identity'] !== null && startupBarrierStagedIdentity($files[$name]['identity']) === $staged
                    && startupBarrierSemantic([$files[$name]]) === startupBarrierSemantic([$before]));
            } else {
                [$stageIdentity, $stageBytes] = startupRead($temporary, 65536, false);
                startupGate($files[$name]['identity'] === null && startupBarrierStagedIdentity($stageIdentity) === $staged
                    && base64_encode($stageBytes) === $before['bytes']
                    && startupBarrierSemantic([['identity' => $stageIdentity, 'bytes' => base64_encode($stageBytes)]]) === startupBarrierSemantic([$before]));
            }
        }
    }
    $plain = ['type' => 'string', 'raw' => base64_encode(serialize(startupBarrierPlain())), 'deadline' => -1];
    if ($operation === 'barrier-enter') {
        startupGate($snapshot === null);
        $original = ['version' => 1, 'source_version' => $version, 'source_sha256' => hash_file('sha256', __FILE__),
            'nonce' => bin2hex(random_bytes(32)), 'storage' => $storage, 'redis_identity' => $redisIdentity,
            'intent' => ['horizon' => false, 'scheduler' => false], 'redis' => $raw, 'files' => $files];
        $oldMask = umask(0077);
        try { $stream = fopen(STARTUP_BARRIER_SNAPSHOT, 'x+b'); } finally { umask($oldMask); }
        startupGate(is_resource($stream));
        try {
            startupGate(flock($stream, LOCK_EX | LOCK_NB));
            $info = fstat($stream);
            startupGate($info['mode'] === (0100000 | 0600) && $info['nlink'] === 1 && $info['uid'] === 9999 && $info['gid'] === 9999);
            startupBarrierWrite($stream, startupBarrierEnvelope($original, $hmacKey));
            startupBarrierSyncDirectory(STARTUP_BARRIER_SNAPSHOT);
            $startupCategory = 'barrier_enter_conflict';
            startupGate(startupBarrierStorage() === $storage && startupBarrierRaw($client, $physical) === $raw
                && startupBarrierFile('maintenance') === $files['maintenance'] && startupBarrierFile('down') === $files['down']);
            startupGate($client->rawCommand('EVAL', STARTUP_BARRIER_CAS, '1', $physical,
                $raw['type'], base64_decode($raw['raw'], true), (string)$raw['deadline'],
                $plain['type'], base64_decode($plain['raw'], true), '-1') === 1);
            // Durable completion distinguishes a held run from an interrupted
            // pre-CAS snapshot. A later intervening DEL is never adopted as up.
            startupBarrierWrite($stream, startupBarrierEnvelope(['nonce' => $original['nonce'], 'phase' => 'entered'], $hmacKey));
            foreach (['maintenance', 'down'] as $name) {
                startupGate(startupBarrierRaw($client, $physical) === $plain && startupBarrierFile($name) === $files[$name]);
                if ($files[$name]['identity'] !== null) {
                    $path = '/var/www/html/storage/framework/'.($name === 'maintenance' ? 'maintenance.php' : 'down');
                    startupGate(unlink($path)); startupBarrierSyncDirectory($path);
                }
            }
        } finally { fclose($stream); }
        $snapshot = startupBarrierSnapshot($hmacKey);
    } elseif ($operation === 'barrier-restore') {
        startupGate($snapshot !== null && !$snapshot['done'] && $snapshot['original']['source_version'] === $version);
        $original = $snapshot['original'];
        $stream = fopen(STARTUP_BARRIER_SNAPSHOT, 'r+b'); startupGate(is_resource($stream));
        try {
            startupGate(flock($stream, LOCK_EX | LOCK_NB) && startupIdentity(fstat($stream)) === $snapshot['identity']);
            startupGate(fseek($stream, 0, SEEK_END) === 0);
            $startupCategory = 'barrier_restore_conflict';
            $originalRaw = $original['redis'];
            // A crash after the final Redis CAS may be completed only with
            // both files already authenticated as original or staged below.
            $originalNow = $originalRaw;
            startupGate(startupBarrierRaw($client, $physical, $redisNow) === $raw);
            if ($originalRaw['deadline'] > 0 && $originalRaw['deadline'] <= $redisNow) {
                $originalNow = ['type' => 'none', 'raw' => '', 'deadline' => -2];
            }
            startupGate($raw === $plain || (!$snapshot['entered'] || $snapshot['release_ready']) && $raw === $originalNow);
            foreach (['maintenance', 'down'] as $name) {
                $current = $files[$name]; $before = $original['files'][$name]; $staged = $snapshot['staged'][$name] ?? null;
                startupGate($current === $before || $current['identity'] === null
                    || $staged !== null && $current['identity'] !== null && startupBarrierStagedIdentity($current['identity']) === $staged
                        && startupBarrierSemantic([$current]) === startupBarrierSemantic([$before]));
                if ($before['identity'] === null) { startupGate($current === $before && $staged === null); }
                if (!$snapshot['entered']) { startupGate($current === $before); }
                if ($snapshot['release_ready']) {
                    startupGate($current === $before || $staged !== null && $current['identity'] !== null
                        && startupBarrierStagedIdentity($current['identity']) === $staged
                        && startupBarrierSemantic([$current]) === startupBarrierSemantic([$before]));
                }
                if ($raw !== $plain) {
                    startupGate($current === $before || $staged !== null && $current['identity'] !== null
                        && startupBarrierStagedIdentity($current['identity']) === $staged);
                }
                // Never adopt unrecorded stage files or overwrite unknown
                // file identities, even if their contents look legitimate.
                $temporary = '/var/www/html/storage/framework/.startup-barrier-'.$name;
                clearstatcache(true, $temporary);
                startupGate($staged !== null || @lstat($temporary) === false);
            }
            foreach (['maintenance', 'down'] as $name) {
                startupGate(startupBarrierRaw($client, $physical) === $raw && startupBarrierStorage() === $storage);
                startupBarrierRestoreFile($name, $original['files'][$name], $snapshot['staged'][$name] ?? null,
                    $stream, $original['nonce'], $hmacKey);
            }
            startupGate(startupBarrierSemantic(['maintenance' => startupBarrierFile('maintenance'), 'down' => startupBarrierFile('down')])
                === startupBarrierSemantic($original['files']));
            if (!$snapshot['release_ready']) {
                // This is private finite PRETARGET compensation progress, not
                // final-success admission or a caller-supplied release grant.
                startupBarrierWrite($stream, startupBarrierEnvelope(['nonce' => $original['nonce'], 'phase' => 'release-ready'], $hmacKey));
            }
            startupGate($client->rawCommand('EVAL', STARTUP_BARRIER_CAS, '1', $physical,
                $raw['type'], base64_decode($raw['raw'], true), (string)$raw['deadline'],
                $originalRaw['type'], base64_decode($originalRaw['raw'], true), (string)$originalRaw['deadline']) === 1);
            startupBarrierWrite($stream, startupBarrierEnvelope(['nonce' => $original['nonce'], 'restored' => true], $hmacKey));
        } finally { fclose($stream); }
        $snapshot = startupBarrierSnapshot($hmacKey);
    }
    $raw = startupBarrierRaw($client, $physical, $redisNow);
    $files = ['maintenance' => startupBarrierFile('maintenance'), 'down' => startupBarrierFile('down')];
    $plainHeld = $raw === $plain && $files['maintenance']['identity'] === null && $files['down']['identity'] === null;
    if ($operation === 'barrier-enter') {
        startupGate($snapshot !== null && $snapshot['entered'] && !$snapshot['release_ready'] && !$snapshot['done'] && $plainHeld);
    } elseif ($operation === 'barrier-restore') {
        $expected = $snapshot['original']['redis'];
        if ($expected['deadline'] > 0 && $expected['deadline'] <= $redisNow) {
            $expected = ['type' => 'none', 'raw' => '', 'deadline' => -2];
        }
        startupGate($snapshot['done'] && $raw === $expected
            && startupBarrierSemantic($files) === startupBarrierSemantic($snapshot['original']['files']));
    }
    $state = ['redis' => $raw, 'files' => startupBarrierSemantic($files)];
    $keyState = $raw['type'] === 'none' ? 'absent' : ($raw['deadline'] === -1 ? 'persistent' : 'expiring');
    $originalState = $snapshot === null ? $state : ['redis' => $snapshot['original']['redis'], 'files' => startupBarrierSemantic($snapshot['original']['files'])];
    startupResult(['version' => 1, 'operation' => $operation, 'selected_version' => $version,
        'driver' => 'cache', 'store' => 'redis', 'connection' => 'cache', 'database' => 1,
        'redis_identity_hmac' => $redisIdentity, 'cache_prefix_hmac' => startupHmac('barrier-cache-prefix', $store->getPrefix(), $hmacKey),
        'redis_prefix_hmac' => startupHmac('barrier-redis-prefix', $client->getOption(Redis::OPT_PREFIX), $hmacKey),
        'logical_key' => 'illuminate:foundation:down', 'key_state' => $keyState, 'ttl_state' => $keyState,
        'payload_profile' => $raw['type'] === 'none' ? 'absent' : ($raw['raw'] === $plain['raw'] ? 'plain503' : 'original'),
        'maintenance_file_state' => $files['maintenance']['identity'] === null ? 'absent' : 'stock',
        'down_file_state' => $files['down']['identity'] === null ? 'absent' : 'original',
        'snapshot_state' => $snapshot === null ? 'absent' : ($snapshot['done'] ? 'restored'
            : ($snapshot['staged'] !== [] || $snapshot['release_ready'] ? 'restoring' : ($snapshot['entered'] && $plainHeld ? 'held' : 'preparing'))),
        'original_state_hmac' => startupHmac('barrier-state', $originalState, $hmacKey),
        'current_state_hmac' => startupHmac('barrier-state', $state, $hmacKey)], 0);
}

// Passive S1 only. No pause, signal, queue access, HTTP session inspection or
// lifecycle authority. The S6 commands below are literal read-only commands.
const STARTUP_RUNTIME_LONGRUNS = ['php-fpm', 'nginx', 'reverb', 'terminal-server', 'horizon', 'scheduler-worker'];
const STARTUP_RUNTIME_ONESHOTS = ['fix-attrs', 'legacy-cont-init', 'db-migration', 'init-seeder', 'init-script', 'legacy-services'];

function startupRuntimeFile(string $path, int $limit): array {
    clearstatcache(true, $path); $before = lstat($path);
    startupGate(is_array($before) && ($before['mode'] & 0170000) === 0100000
        && $before['nlink'] === 1 && $before['size'] >= 0 && $before['size'] <= $limit);
    $identity = startupIdentity($before);
    $stream = fopen($path, 'rb'); startupGate(is_resource($stream));
    try {
        startupGate(startupIdentity(fstat($stream)) === $identity);
        $bytes = stream_get_contents($stream, $limit + 1);
        clearstatcache(true, $path); $after = lstat($path);
        startupGate(is_string($bytes) && strlen($bytes) === $before['size'] && is_array($after)
            && startupIdentity($after) === $identity && startupIdentity(fstat($stream)) === $identity);
        return [$identity, $bytes];
    } finally { fclose($stream); }
}

function startupRuntimeEnvironment(): array {
    startupParents('/var/www/html/.env');
    [$identity, $bytes] = startupRuntimeFile('/var/www/html/.env', 65536);
    // Same narrow assignment/quote grammar as canonical clone_env/parse_env.
    // Worker switches additionally retain the exact unquoted S6 grep literals.
    startupGate(strpbrk($bytes, "\0\r\\$`") === false);
    $values = []; $literals = []; $lines = explode("\n", $bytes); startupGate(count($lines) <= 4096);
    foreach ($lines as $line) {
        if (trim($line) === '' || str_starts_with(ltrim($line), '#')) { continue; }
        startupGate(preg_match('/\A([A-Z][A-Z0-9_]*)=(.*)\z/D', $line, $match) === 1);
        startupGate(!array_key_exists($match[1], $values)); $value = $match[2]; $literals[$match[1]] = $value;
        if (str_starts_with($value, "'") || str_starts_with($value, '"')) {
            startupGate(strlen($value) >= 2 && substr($value, -1) === $value[0]); $value = substr($value, 1, -1);
        }
        startupGate(strpbrk($value, "'\"") === false && preg_match('/[\x00-\x1f\x7f]/', $value) === 0);
        $values[$match[1]] = $value;
    }
    foreach (['HORIZON_ENABLED', 'SCHEDULER_ENABLED'] as $name) {
        startupGate(isset($literals[$name]) && in_array($literals[$name], ['true', 'false'], true));
    }
    $configKeys = ['HORIZON_ENABLED' => 'constants.horizon.is_horizon_enabled',
        'SCHEDULER_ENABLED' => 'constants.horizon.is_scheduler_enabled',
        'MIGRATION_ENABLED' => 'constants.migration.is_migration_enabled',
        'SEEDER_ENABLED' => 'constants.seeder.is_seeder_enabled', 'AUTOUPDATE' => 'constants.coolify.autoupdate'];
    $flags = [];
    foreach ($configKeys as $name => $configKey) {
        startupGate(isset($values[$name]) && in_array($values[$name], ['true', 'false'], true)
            && getenv($name) === $values[$name] && config($configKey) === ($values[$name] === 'true'));
        if (!in_array($name, ['HORIZON_ENABLED', 'SCHEDULER_ENABLED'], true)) { startupGate($values[$name] === 'false'); }
        $flags[$name] = $values[$name] === 'true';
    }
    foreach (['APP_DEBUG' => 'false', 'AUTORUN_ENABLED' => 'false', 'DISABLE_DEFAULT_CONFIG' => 'true',
            'APP_MAINTENANCE_DRIVER' => 'cache', 'APP_MAINTENANCE_STORE' => 'redis', 'SSL_MODE' => 'off'] as $name => $expected) {
        startupGate(($values[$name] ?? null) === $expected && getenv($name) === $expected);
    }
    startupGate(config('app.maintenance.driver') === 'cache' && config('app.maintenance.store') === 'redis');
    $horizon = config('horizon.environments.production');
    startupGate(is_array($horizon) && count($horizon) <= 32
        && strlen(json_encode($horizon, JSON_THROW_ON_ERROR)) <= 32768);
    // Snapshot effective options, without claiming queue idleness or consulting
    // heartbeat/job/session data. No force=true configuration is adopted.
    foreach ($horizon as $options) {
        startupGate(is_array($options) && ($options['force'] ?? false) === false);
    }
    return [$flags, [$identity, hash('sha256', $bytes), $flags, $horizon]];
}

function startupRuntimeSource(): array {
    // Stock S6 intentionally uses these two symlinks. Authenticate their exact
    // targets here; startupParents/SSH/barrier strictness is never relaxed.
    startupGate(is_link('/run/s6-rc') && preg_match('/\As6-rc:s6-rc-init:[A-Za-z0-9]{6}\z/D', readlink('/run/s6-rc')) === 1);
    $live = '/run/'.readlink('/run/s6-rc');
    startupParents($live.'/servicedirs/fixed-leaf');
    $hashes = ['php-fpm' => 'be640dfc6586b1fac2e070fc559f96eb7a39f6de756762eb86066b277a99d074',
        'nginx' => '3c65b0ec5af86d534dc7ac5f17da83ab8b91edfaacee220ec129a1310e460df1',
        'reverb' => 'dca10063b2d8b43e6c691d5da37d65d1bbf161cc1ea5fa4b853ee9ae819df174',
        'terminal-server' => '8af137efcd342f2002e74a33c67feadea39ca795be706ad68c71d70826a1ec0b',
        'horizon' => '296dfda050b4d25c32fd9a858216a9a492148fd6ef9960644e08f32f7caf5e31',
        'scheduler-worker' => '9034571c9f328e694720f9c66c461016e96a3d932ade29f5d2498754677d906c'];
    $source = [];
    foreach ($hashes as $name => $expected) {
        $definition = '/etc/s6-overlay/s6-rc.d/'.$name;
        startupParents($definition.'/run');
        [$original, $bytes] = startupRuntimeFile($definition.'/run', 4096);
        startupGate(hash('sha256', $bytes) === $expected);
        [, $type] = startupRuntimeFile($definition.'/type', 16); startupGate(trim($type) === 'longrun');
        $link = '/run/service/'.$name;
        startupGate(is_link($link) && readlink($link) === '/run/s6-rc/servicedirs/'.$name
            && realpath($link) === $live.'/servicedirs/'.$name);
        $compiled = $live.'/servicedirs/'.$name;
        startupParents($compiled.'/run');
        [$copied, $run] = startupRuntimeFile($compiled.'/run', 4096);
        startupGate($run === $bytes && $copied['uid'] === 9999 && $copied['gid'] === 9999
            && ($copied['mode'] & 0022) === 0);
        $notify = [];
        foreach ([$definition, $compiled] as $root) {
            clearstatcache(true, $root.'/notification-fd');
            if (in_array($name, ['php-fpm', 'nginx'], true)) {
                [$info, $fd] = startupRuntimeFile($root.'/notification-fd', 8);
                startupGate($fd === ($root === $definition ? '3' : "3\n")); $notify[] = $info;
            } else { startupGate(@lstat($root.'/notification-fd') === false); }
        }
        $source[$name] = [$original, $copied, $expected, $notify];
    }
    return [$live, $source];
}

function startupSchedulerRequest(): array {
    [$identity, $bytes] = startupRuntimeFile('/probe/scheduler.request.json', 2048);
    startupGate($identity['uid'] === 0 && $identity['gid'] === 0 && ($identity['mode'] & 07777) === 0444);
    $request = json_decode($bytes, true, 8, JSON_THROW_ON_ERROR);
    startupGate(is_array($request) && array_keys($request) === ['version', 'phase', 'generation', 'deadline_ns']
        && json_encode($request, JSON_THROW_ON_ERROR) === $bytes && $request['version'] === 1
        && $request['phase'] === 'scheduler-proof-held' && is_string($request['generation'])
        && preg_match('/\A[a-f0-9]{32}\z/D', $request['generation']) === 1
        && is_int($request['deadline_ns']) && $request['deadline_ns'] > hrtime(true)
        && $request['deadline_ns'] - hrtime(true) <= 1200000000000);
    return [$identity, hash('sha256', $bytes), $request];
}

function startupSchedulerUncached(): void {
    foreach (['/var/www/html/bootstrap/cache/config.php', '/tmp/rehearsal-startup-uncached-config.php'] as $path) {
        clearstatcache(true, $path); startupGate(@lstat($path) === false);
    }
}

function startupSchedulerS6(): string {
    // Only this proc_open-owned read-only tool may be cleaned up on expiry.
    $process = proc_open(['/command/s6-svstat', '-o', 'up,wantedup,pid', '/run/service/scheduler-worker'],
        [0 => ['file', '/dev/null', 'r'], 1 => ['pipe', 'w'], 2 => ['pipe', 'w']], $pipes);
    startupGate(is_resource($process));
    $out = ''; $err = ''; $limit = hrtime(true) + 100000000;
    try {
        stream_set_blocking($pipes[1], false); stream_set_blocking($pipes[2], false);
        do {
            $out .= stream_get_contents($pipes[1], 257); $err .= stream_get_contents($pipes[2], 257);
            startupGate(strlen($out) <= 256 && strlen($err) <= 256 && hrtime(true) < $limit);
            $state = proc_get_status($process);
            if (!$state['running']) { break; }
            usleep(1000);
        } while (true);
        $out .= stream_get_contents($pipes[1], 257); $err .= stream_get_contents($pipes[2], 257);
        startupGate($state['exitcode'] === 0 && $err === ''
            && preg_match('/\Atrue true [1-9][0-9]{0,9}\n\z/D', $out) === 1);
        return $out;
    } finally {
        $state = proc_get_status($process);
        if ($state['running']) { proc_terminate($process, 9); }
        fclose($pipes[1]); fclose($pipes[2]); proc_close($process);
    }
}

function startupSchedulerCheck(string $key, ?array $created = null): never {
    global $startupCategory;
    $startupCategory = 'scheduler_context_invalid';
    [$projectionIdentity, $bytes] = startupRead('/probe/scheduler.context.json', 131072);
    $projection = json_decode($bytes, true, 32, JSON_THROW_ON_ERROR);
    startupGate(is_array($projection) && array_keys($projection) === ['data', 'hmac']
        && json_encode($projection, JSON_THROW_ON_ERROR) === $bytes && is_array($projection['data'])
        && is_string($projection['hmac']) && hash_equals(startupHmac('scheduler-context', $projection['data'], $key), $projection['hmac']));
    $data = $projection['data'];
    startupGate($created === null || $data === $created);
    startupGate(array_keys($data) === ['version', 'selected_version', 'request', 'environment', 'helper', 'source', 'storage', 's6', 'cache']
        && $data['version'] === 1 && in_array($data['selected_version'], ['4.4.2', '4.4.6'], true)
        && startupSchedulerRequest() === $data['request']);
    startupSchedulerUncached();
    $environment = startupRuntimeFile('/var/www/html/.env', 65536);
    $helper = startupRuntimeFile(__FILE__, 131072);
    startupGate([$environment[0], hash('sha256', $environment[1])] === $data['environment']
        && [$helper[0], hash('sha256', $helper[1])] === $data['helper']
        && startupRuntimeSource() === $data['source'] && startupBarrierStorage() === $data['storage']);
    startupGate(is_string($data['s6']) && startupSchedulerS6() === $data['s6']);
    $cache = $data['cache'];
    startupGate(is_array($cache) && array_keys($cache) === ['config', 'prefix', 'physical', 'run_id']
        && is_array($cache['config']) && $cache['config']['host'] === '127.0.0.1'
        && (string)$cache['config']['port'] === '6379' && (string)$cache['config']['database'] === '1'
        && $cache['config']['password'] === 'rehearsal-only-password'
        && is_string($cache['prefix']) && preg_match('/\A[a-zA-Z0-9_.:-]{0,192}\z/D', $cache['prefix']) === 1
        && is_string($cache['physical']) && strlen($cache['physical']) <= 512
        && str_starts_with($cache['physical'], $cache['prefix'])
        && str_ends_with($cache['physical'], 'illuminate:foundation:down')
        && is_string($cache['run_id']) && preg_match('/\A[a-f0-9]{40}\z/D', $cache['run_id']) === 1);
    $client = new Redis;
    startupGate($client->connect('127.0.0.1', 6379, 0.1, null, 0, 0.1)
        && $client->auth($cache['config']['password']) && $client->select(1)
        && $client->setOption(Redis::OPT_PREFIX, $cache['prefix']));
    $server = $client->info('server');
    startupGate(is_array($server) && ($server['run_id'] ?? null) === $cache['run_id']);
    $plain = ['type' => 'string', 'raw' => base64_encode(serialize(startupBarrierPlain())), 'deadline' => -1];
    startupGate(startupBarrierRaw($client, $cache['physical']) === $plain
        && startupBarrierFile('maintenance')['identity'] === null && startupBarrierFile('down')['identity'] === null);
    // Post-I/O current-cache and immutable-context barrier. This path never
    // loads Laravel or accepts a caller path/command/configuration projection.
    startupGate(startupSchedulerRequest() === $data['request']
        && startupRuntimeFile('/var/www/html/.env', 65536) === $environment
        && startupRuntimeFile(__FILE__, 131072) === $helper && startupRuntimeSource() === $data['source']
        && startupBarrierStorage() === $data['storage'] && startupBarrierRaw($client, $cache['physical']) === $plain
        && startupRead('/probe/scheduler.context.json', 131072) === [$projectionIdentity, $bytes]
        && startupBarrierFile('maintenance')['identity'] === null && startupBarrierFile('down')['identity'] === null
        && startupSchedulerS6() === $data['s6']);
    startupSchedulerUncached(); startupGate(hrtime(true) < $data['request'][2]['deadline_ns']);
    startupResult(['version' => 1, 'operation' => 'scheduler-context',
        'selected_version' => $data['selected_version'], 'generation' => $data['request'][2]['generation'],
        'context_hmac' => $projection['hmac']], 0);
}

function startupSchedulerContext(string $version, string $key): never {
    global $startupCategory;
    $startupCategory = 'scheduler_context_invalid';
    $environment = startupRuntimeEnvironment();
    startupGate($environment[0]['SCHEDULER_ENABLED'] && !$environment[0]['HORIZON_ENABLED']);
    $source = startupRuntimeSource();
    [$client, $physical, $redisIdentity, $storage, $store, $config, $runId] = startupBarrierContext($key);
    $plain = ['type' => 'string', 'raw' => base64_encode(serialize(startupBarrierPlain())), 'deadline' => -1];
    $files = ['maintenance' => startupBarrierFile('maintenance'), 'down' => startupBarrierFile('down')];
    startupGate(startupBarrierRaw($client, $physical) === $plain
        && $files['maintenance']['identity'] === null && $files['down']['identity'] === null);
    startupSchedulerUncached();
    $startupCategory = 'scheduler_context_invalid';
    $envFile = startupRuntimeFile('/var/www/html/.env', 65536);
    $helper = startupRuntimeFile(__FILE__, 131072);
    $projection = ['version' => 1, 'selected_version' => $version, 'request' => startupSchedulerRequest(),
        'environment' => [$envFile[0], hash('sha256', $envFile[1])], 'helper' => [$helper[0], hash('sha256', $helper[1])],
        'source' => $source, 'storage' => $storage, 's6' => startupSchedulerS6(),
        'cache' => ['config' => $config, 'prefix' => $client->getOption(Redis::OPT_PREFIX), 'physical' => $physical, 'run_id' => $runId]];
    // Re-read all context after I/O; no live process inventory/paused assertion
    // is borrowed from the passive S1 selector, and no operation releases it.
    startupGate(startupRuntimeEnvironment() === $environment && startupRuntimeSource() === $source
        && startupBarrierStorage() === $storage && startupBarrierRaw($client, $physical) === $plain
        && startupBarrierFile('maintenance') === $files['maintenance'] && startupBarrierFile('down') === $files['down']);
    startupGate(true);
    $bytes = json_encode(['data' => $projection, 'hmac' => startupHmac('scheduler-context', $projection, $key)], JSON_THROW_ON_ERROR);
    startupGate(strlen($bytes) <= 131072);
    startupCreate('/probe/scheduler.context.json', $bytes, startupParents('/probe/scheduler.context.json'));
    startupSchedulerCheck($key, $projection);
}

function startupRuntimeProcesses(): array {
    $paths = glob('/proc/[0-9]*/stat'); startupGate(is_array($paths) && count($paths) <= 256);
    $rows = [];
    foreach ($paths as $path) {
        $pid = (int)basename(dirname($path));
        if ($pid === getmypid()) { continue; }
        $before = @file_get_contents($path, false, null, 0, 4097);
        if ($before === false) { continue; }
        startupGate(strlen($before) <= 4096
            && preg_match('/\A([0-9]+) \((.*)\) (.*)\n?\z/sD', $before, $match) === 1 && (int)$match[1] === $pid);
        $fields = explode(' ', trim($match[3]));
        startupGate(count($fields) >= 20 && ctype_digit($fields[1]) && ctype_digit($fields[19]));
        $status = @file_get_contents('/proc/'.$pid.'/status', false, null, 0, 8193);
        if ($status === false) { continue; }
        startupGate(strlen($status) <= 8192 && preg_match('/^Uid:\s+([0-9]+)\s+([0-9]+)\s+([0-9]+)\s+([0-9]+)$/m', $status, $uids) === 1);
        if ((int)$uids[2] !== 9999) { continue; }
        startupGate(array_slice($uids, 1) === ['9999', '9999', '9999', '9999']);
        $argv = @file_get_contents('/proc/'.$pid.'/cmdline', false, null, 0, 8193);
        $exe = @readlink('/proc/'.$pid.'/exe'); $info = @stat('/proc/'.$pid.'/exe');
        $after = @file_get_contents($path, false, null, 0, 4097);
        if ($after === false) { continue; }
        // A persistent zombie/unreadable application executable remains owned
        // and unclassified. Never silently drop it from a "stable" snapshot.
        startupGate(($argv === false || strlen($argv) <= 8192) && ($exe === false || strlen($exe) <= 512));
        // state/CPU counters can change while reading; starttime/parent/argv/exe
        // form identity. Whole membership is captured again after S6 reads.
        startupGate(preg_match('/\A[0-9]+ \((.*)\) (.*)\n?\z/sD', $after, $second) === 1);
        $afterFields = explode(' ', trim($second[2]));
        startupGate(($afterFields[1] ?? null) === $fields[1] && ($afterFields[19] ?? null) === $fields[19]);
        $rows[$pid] = ['pid' => $pid, 'parent' => (int)$fields[1], 'start' => (int)$fields[19],
            'exe' => $exe === false ? '' : $exe, 'argv' => $argv === false ? '' : $argv,
            'executable' => $info === false ? null : startupIdentity($info)];
    }
    ksort($rows, SORT_NUMERIC); return $rows;
}

function startupRuntime(string $version, string $key): never {
    global $startupCategory, $startupDeadline;
    $startupCategory = 'runtime_environment_invalid';
    [$flags, $configBefore] = startupRuntimeEnvironment();
    $startupCategory = 'runtime_source_invalid'; $sourceBefore = startupRuntimeSource();
    $startupCategory = 'runtime_process_invalid'; $before = startupRuntimeProcesses();
    $startupCategory = 'runtime_s6_invalid';
    // No caller-controlled command, path, service name or option is accepted.
    $commands = [
        'active' => ['/command/s6-rc', '-a', 'list'],
        'php-fpm' => ['/command/s6-svstat', '-o', 'up,wantedup,ready,paused,pid', '/run/service/php-fpm'],
        'nginx' => ['/command/s6-svstat', '-o', 'up,wantedup,ready,paused,pid', '/run/service/nginx'],
        'reverb' => ['/command/s6-svstat', '-o', 'up,wantedup,ready,paused,pid', '/run/service/reverb'],
        'terminal-server' => ['/command/s6-svstat', '-o', 'up,wantedup,ready,paused,pid', '/run/service/terminal-server'],
        'horizon' => ['/command/s6-svstat', '-o', 'up,wantedup,ready,paused,pid', '/run/service/horizon'],
        'scheduler-worker' => ['/command/s6-svstat', '-o', 'up,wantedup,ready,paused,pid', '/run/service/scheduler-worker'],
    ];
    $outputs = [];
    foreach ($commands as $name => $command) {
        startupGate(true);
        $process = proc_open($command, [0 => ['file', '/dev/null', 'r'], 1 => ['pipe', 'w'], 2 => ['pipe', 'w']], $pipes);
        if (!is_resource($process)) { startupGate(false); }
        try {
            startupGate(true);
            stream_set_blocking($pipes[1], false); stream_set_blocking($pipes[2], false);
            $out = ''; $err = ''; $exit = -1;
            do {
                $out .= stream_get_contents($pipes[1], 4097); $err .= stream_get_contents($pipes[2], 4097);
                startupGate(strlen($out) <= 4096 && strlen($err) <= 4096);
                $state = proc_get_status($process);
                if (!$state['running']) { $exit = $state['exitcode']; break; }
                // Only the observer's own read-only tool may be terminated on
                // budget expiry. No application process is ever signalled.
                if (hrtime(true) > $startupDeadline) { proc_terminate($process, 9); startupGate(false); }
                usleep(1000);
            } while (true);
            $out .= stream_get_contents($pipes[1], 4097); $err .= stream_get_contents($pipes[2], 4097);
            startupGate(strlen($out) <= 4096 && strlen($err) <= 4096);
            $outputs[$name] = [$exit, $out, $err];
        } finally {
            // Reap only this proc_open-owned read-only tool. Deadline/output
            // gates can throw before the loop's explicit timeout branch.
            $state = proc_get_status($process);
            if ($state['running']) { proc_terminate($process, 9); }
            fclose($pipes[1]); fclose($pipes[2]); proc_close($process);
        }
    }
    $startupCategory = 'runtime_process_invalid'; $after = startupRuntimeProcesses();
    $startupCategory = 'runtime_source_invalid'; startupGate(startupRuntimeSource() === $sourceBefore);
    $startupCategory = 'runtime_environment_invalid'; startupGate(startupRuntimeEnvironment() === [$flags, $configBefore]);
    $startupCategory = 'runtime_s6_invalid';
    [$exit, $out, $err] = $outputs['active']; $active = null;
    $known = array_merge(STARTUP_RUNTIME_ONESHOTS, STARTUP_RUNTIME_LONGRUNS, ['s6rc-fdholder', 's6rc-oneshot-runner']);
    if ($exit === 0) {
        startupGate($err === '' && $out !== '' && str_ends_with($out, "\n"));
        $active = explode("\n", substr($out, 0, -1));
        startupGate(count(array_unique($active)) === count($active));
        foreach ($active as $name) { startupGate(in_array($name, $known, true)); }
    } else {
        startupGate($out === '' && $exit === 111 && $err === "s6-rc: fatal: unable to take locks: Resource busy\n");
    }
    $rows = []; $roots = [];
    foreach (STARTUP_RUNTIME_ONESHOTS as $name) {
        $rows[] = ['name' => $name, 'kind' => 'oneshot',
            'state' => $active === null ? 'unknown' : (in_array($name, $active, true) ? 'active' : 'not-active'),
            'readiness' => 'unsupported', 'role' => 'none', 'pid' => 0, 'start_time_ticks' => 0, 'identity_hmac' => null];
    }
    foreach (STARTUP_RUNTIME_LONGRUNS as $name) {
        [$exit, $out, $err] = $outputs[$name];
        $row = ['name' => $name, 'kind' => 'longrun', 'state' => 'unknown', 'readiness' => 'unknown',
            'role' => 'none', 'pid' => 0, 'start_time_ticks' => 0, 'identity_hmac' => null];
        if ($exit === 1 && $out === '' && $err === 's6-svstat: fatal: unable to read status for /run/service/'.$name.": s6-supervise not running\n") {
            $row['state'] = 'missing';
        } elseif ($exit === 0) {
            startupGate($err === '' && preg_match('/\A(true|false) (true|false) (true|false) (true|false) (-1|[0-9]+)\n\z/D', $out, $m) === 1);
            $pid = (int)$m[5];
            if ($m[1] === 'false') { startupGate($pid === -1 && $m[3] === 'false'); $row['state'] = 'down'; }
            else {
                startupGate($pid > 0 && $pid <= 4194304);
                $process = $before[$pid] ?? null;
                if ($process !== null && ($after[$pid] ?? null) === $process) {
                    $row['pid'] = $pid; $row['start_time_ticks'] = $process['start'];
                    $row['identity_hmac'] = startupHmac('runtime-process', $process, $key);
                    $row['role'] = 'unknown'; $row['state'] = 'unstable';
                    $parent = $before[$process['parent']] ?? null;
                    $supervised = $parent !== null && ($after[$process['parent']] ?? null) === $parent
                        && $parent['exe'] === '/package/admin/s6-2.13.2.0/command/s6-supervise'
                        && $parent['argv'] === "s6-supervise\0".$name."\0" && $parent['parent'] === 1;
                    $expected = match ($name) {
                        'php-fpm' => $process['exe'] === '/usr/local/sbin/php-fpm'
                            && rtrim($process['argv'], "\0") === 'php-fpm: master process (/usr/local/etc/php-fpm.conf)',
                        'nginx' => $process['exe'] === '/usr/sbin/nginx' && rtrim($process['argv'], "\0") === 'nginx: master process nginx',
                        'reverb' => $process['exe'] === '/usr/local/bin/php' && $process['argv'] === "php\0artisan\0reverb:start\0--host=0.0.0.0\0--port=6001\0",
                        'terminal-server' => $process['exe'] === '/usr/bin/node' && $process['argv'] === "node\0/terminal/terminal-server.js\0",
                        'horizon' => $flags['HORIZON_ENABLED'] && $process['exe'] === '/usr/local/bin/php' && $process['argv'] === "php\0artisan\0horizon\0",
                        'scheduler-worker' => $flags['SCHEDULER_ENABLED'] && $process['exe'] === '/usr/local/bin/php' && $process['argv'] === "php\0artisan\0schedule:work\0",
                    };
                    $disabled = in_array($name, ['horizon', 'scheduler-worker'], true)
                        && !$flags[$name === 'horizon' ? 'HORIZON_ENABLED' : 'SCHEDULER_ENABLED']
                        && $process['exe'] === '/bin/busybox' && $process['argv'] === "sleep\0infinity\0";
                    if ($supervised && ($expected || $disabled) && $active !== null && in_array($name, $active, true)
                            && $m[2] === 'true' && $m[4] === 'false') {
                        $row['state'] = 'up'; $row['role'] = $disabled ? 'disabled-sleep' : $name;
                        $row['readiness'] = in_array($name, ['php-fpm', 'nginx'], true)
                            ? ($m[3] === 'true' ? 'notified' : 'not-notified') : 'unsupported';
                        $roots[$pid] = $name;
                    }
                } else { $row['state'] = 'unstable'; }
            }
        }
        $rows[] = $row;
    }
    // Classify the whole application UID namespace conservatively. Descendants
    // are membership facts, never evidence of idle/drained queue or terminal.
    $unclassified = 0;
    foreach ($after as $pid => $process) {
        $cursor = $pid; $seen = []; $classified = false;
        while (isset($after[$cursor]) && !isset($seen[$cursor])) {
            if (isset($roots[$cursor])) { $classified = true; break; }
            $seen[$cursor] = true; $cursor = $after[$cursor]['parent'];
        }
        if (!$classified) {
            $exe = $process['exe'];
            // These are infrastructure processes only when actual executable
            // and S6 ancestry agree. Unknown application execs are counted.
            $supervisorArgv = array_map(fn ($name) => "s6-supervise\0".$name."\0",
                array_merge(STARTUP_RUNTIME_LONGRUNS, ['s6rc-fdholder', 's6rc-oneshot-runner', 's6-linux-init-shutdownd']));
            $classified = ($pid === 1 && $exe === '/package/admin/s6-2.13.2.0/command/s6-svscan'
                    && $process['argv'] === "/package/admin/s6/command/s6-svscan\0-d4\0--\0/run/service\0")
                || ($process['parent'] === 1 && $exe === '/package/admin/s6-2.13.2.0/command/s6-supervise'
                    && in_array($process['argv'], $supervisorArgv, true))
                || (isset($after[$process['parent']]) && $after[$process['parent']]['exe'] === '/package/admin/s6-2.13.2.0/command/s6-supervise'
                    && in_array($after[$process['parent']]['argv'], $supervisorArgv, true)
                    && in_array($exe, ['/package/admin/s6-2.13.2.0/command/s6-ipcserverd',
                        '/package/admin/s6-linux-init-1.1.3.0/command/s6-linux-init-shutdownd'], true));
        }
        if (!$classified) { $unclassified++; }
    }
    startupGate(true);
    $result = ['version' => 1, 'operation' => 'runtime-observe', 'selected_version' => $version,
        'horizon_enabled' => $flags['HORIZON_ENABLED'], 'scheduler_enabled' => $flags['SCHEDULER_ENABLED'],
        'services' => $rows, 'membership_state' => $before !== $after ? 'changed' : ($unclassified > 0 ? 'unknown' : 'stable'),
        'owned_process_count' => count($after), 'unclassified_count' => $unclassified,
        'source_hmac' => startupHmac('runtime-source', $sourceBefore, $key),
        'config_hmac' => startupHmac('runtime-config', $configBefore, $key),
        'membership_hmac_before' => startupHmac('runtime-membership', $before, $key),
        'membership_hmac_after' => startupHmac('runtime-membership', $after, $key)];
    startupGate(strlen(json_encode($result, JSON_THROW_ON_ERROR)) <= 8192); startupResult($result, 0);
}

$startupCategory = 'ssh_input_invalid';
$startupDb = null;
try {
    startupGate(PHP_SAPI === 'cli' && ($argc === 2 && in_array($argv[1], ['ssh-reconstruct', 'barrier-observe', 'barrier-enter', 'barrier-restore', 'runtime-observe', 'scheduler-context', 'scheduler-context-check', 'horizon-inspect-request', 'horizon-inspect-observe', 'horizon-drain-enter', 'horizon-drain-observe', 'horizon-restore'], true)
            || $argc === 7 && $argv[1] === 'horizon-restore-worker')
        && realpath(__FILE__) === '/probe/startup.php'
        && posix_geteuid() === 9999 && posix_getegid() === 9999);
    $startupCategory = 'ssh_hmac_invalid';
    [, $startupHmacKey] = startupRead('/probe/hmac.key', 32);
    startupGate(strlen($startupHmacKey) === 32);
    if ($argv[1] === 'scheduler-context-check') { startupSchedulerCheck($startupHmacKey); }
    $startupCategory = 'ssh_environment_invalid';
    putenv('APP_CONFIG_CACHE=/tmp/rehearsal-startup-uncached-config.php');
    require '/var/www/html/vendor/autoload.php';
    $startupApp = require '/var/www/html/bootstrap/app.php';
    $startupApp->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
    $startupVersion = config('constants.coolify.version');
    startupGate(in_array($startupVersion, ['4.4.2', '4.4.6'], true)
        && config('database.default') === 'pgsql' && config('app.env') === 'production'
        && config('app.debug') === false && is_string(getenv('APP_KEY'))
        && hash_equals(getenv('APP_KEY'), (string)config('app.key')));
    foreach (['HORIZON_ENABLED', 'SCHEDULER_ENABLED', 'MIGRATION_ENABLED', 'SEEDER_ENABLED', 'AUTOUPDATE'] as $flag) {
        if (($argv[1] === 'runtime-observe' || $argv[1] === 'scheduler-context' || str_starts_with($argv[1], 'horizon-')) && in_array($flag, ['HORIZON_ENABLED', 'SCHEDULER_ENABLED'], true)) {
            startupGate(in_array(getenv($flag), ['true', 'false'], true));
        } else { startupGate(getenv($flag) === 'false'); }
    }
    $startupConnection = config('database.connections.pgsql');
    startupGate(is_array($startupConnection) && $startupConnection['host'] === '127.0.0.1'
        && (string)$startupConnection['port'] === '5432' && $startupConnection['database'] === 'coolify'
        && $startupConnection['username'] === 'coolify' && $startupConnection['password'] === 'rehearsal-only-password'
        && empty($startupConnection['url']) && empty($startupConnection['read']) && empty($startupConnection['write']));
    if (str_starts_with($argv[1], 'horizon-')) {
        $startupCategory = 'horizon_inspection_held';
        startupRuntimeEnvironment();
        $startupHelper = '/var/www/html/app/SiftoMaintenance/InspectSupervisor.php';
        [, $startupHelperBytes] = startupRead($startupHelper, 131072);
        startupGate(hash('sha256', $startupHelperBytes) === 'eef76cd9881407dd35a4e16c1d476373a001c06fae384ddbbc9c6728ade15b20');
        if ($argv[1] === 'horizon-drain-enter') {
            startupResult(App\SiftoMaintenance\InspectSupervisor::drainEnter($startupDeadline), 0);
        }
        if ($argv[1] === 'horizon-drain-observe') {
            startupResult(App\SiftoMaintenance\InspectSupervisor::drainObserve($startupDeadline), 0);
        }
        if ($argv[1] === 'horizon-restore') {
            startupResult(App\SiftoMaintenance\InspectSupervisor::restore($startupDeadline), 0);
        }
        if ($argv[1] === 'horizon-restore-worker') {
            App\SiftoMaintenance\InspectSupervisor::restoreWorker(array_slice($argv, 2));
        }
        $startupResult = $argv[1] === 'horizon-inspect-request'
            ? App\SiftoMaintenance\InspectSupervisor::request($startupDeadline) : App\SiftoMaintenance\InspectSupervisor::observe();
        startupGate(true);
        startupResult($startupResult, 0);
    }
    if ($argv[1] === 'runtime-observe') { startupRuntime($startupVersion, $startupHmacKey); }
    if ($argv[1] === 'scheduler-context') { startupSchedulerContext($startupVersion, $startupHmacKey); }
    if ($argv[1] !== 'ssh-reconstruct') { startupBarrier($argv[1], $startupVersion, $startupHmacKey); }
    $startupCategory = 'ssh_database_invalid';
    $startupDb = Illuminate\Support\Facades\DB::connection();
    startupGate($startupDb->transactionLevel() === 0);
    $startupDb->beginTransaction();
    $startupDb->statement('SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY');
    $startupDb->statement("SET LOCAL statement_timeout = '5000ms'");
    $startupDb->statement("SET LOCAL lock_timeout = '1000ms'");
    $startupIdentity = (array)$startupDb->selectOne('SELECT current_database() AS database, current_user AS username, host(inet_server_addr()) AS address, inet_server_port() AS port, (SELECT oid FROM pg_database WHERE datname = current_database()) AS database_oid');
    startupGate($startupIdentity['database'] === 'coolify' && $startupIdentity['username'] === 'coolify'
        && $startupIdentity['address'] === '127.0.0.1' && $startupIdentity['port'] === 5432);
    // Reject aggregate ciphertext size BEFORE the cursor materializes any row.
    $startupBounds = $startupDb->selectOne('SELECT count(*) AS rows, coalesce(sum(octet_length(private_key)), 0) AS bytes, coalesce(max(octet_length(private_key)), 0) AS largest FROM private_keys');
    startupGate((int)$startupBounds->rows <= STARTUP_MAX_ENTRIES && (int)$startupBounds->bytes <= STARTUP_MAX_TOTAL
        && (int)$startupBounds->largest <= 131072);
    $startupPrototype = new App\Models\PrivateKey;
    startupGate($startupPrototype->getTable() === 'private_keys' && ($startupPrototype->getCasts()['private_key'] ?? null) === 'encrypted');
    $startupRows = []; $startupPlainBytes = 0;
    $startupCategory = 'ssh_decrypt_failed';
    foreach ($startupPrototype->newQuery()->withoutGlobalScopes()->select(['id', 'uuid', 'private_key'])->orderBy('id')->toBase()->cursor() as $record) {
        startupGate(count($startupRows) < STARTUP_MAX_ENTRIES);
        $row = (array)$record;
        startupGate(is_int($row['id']) && $row['id'] >= 0 && is_string($row['uuid'])
            && preg_match('/\A[a-zA-Z0-9_-]{1,128}\z/', $row['uuid']) === 1
            && is_string($row['private_key']) && strlen($row['private_key']) > 0 && strlen($row['private_key']) <= 131072);
        $model = (new App\Models\PrivateKey)->setRawAttributes($row, true);
        // Only the encrypted cast is accessed. No serialization, appended
        // public_key accessor, events, save/store/seeder or remote connection.
        $plain = $model->getAttribute('private_key');
        startupGate(is_string($plain) && strlen($plain) > 0 && strlen($plain) <= STARTUP_MAX_KEY);
        $startupPlainBytes += strlen($plain); startupGate($startupPlainBytes <= STARTUP_MAX_TOTAL);
        $name = 'ssh_key@'.$row['uuid']; startupGate(!array_key_exists($name, $startupRows));
        $path = STARTUP_KEYS.'/'.$name;
        startupGate($model->getKeyLocation() === $path);
        $startupRows[$name] = ['id' => $row['id'], 'cipher_hmac' => startupHmac('cipher', $row['private_key'], $startupHmacKey), 'plain' => $plain];
        unset($model, $plain, $row, $record);
    }
    startupGate(count($startupRows) === (int)$startupBounds->rows);
    $startupCategory = 'ssh_destination_invalid';
    $startupParents = startupParents(STARTUP_KEYS.'/fixed-leaf');
    $startupExisting = startupInventory(STARTUP_KEYS, false);
    $startupMux = startupInventory(STARTUP_MUX, true);
    // Validate inventory serialization before any missing destination is made.
    $startupExistingHmac = startupHmac('existing', $startupExisting, $startupHmacKey);
    $startupMuxHmac = startupHmac('mux', $startupMux, $startupHmacKey);
    $startupMatched = 0; $startupMissing = [];
    // Validate EVERY existing destination and EVERY DB row before first create.
    foreach ($startupRows as $name => $row) {
        if (array_key_exists($name, $startupExisting)) {
            [$info, $bytes] = startupRead(STARTUP_KEYS.'/'.$name, STARTUP_MAX_KEY);
            startupGate(hash_equals($row['plain'], $bytes)); $startupMatched++; unset($bytes);
        } else { $startupMissing[] = $name; }
    }
    startupGate(count($startupExisting) + count($startupMissing) <= STARTUP_MAX_ENTRIES);
    $startupDestinationBytes = array_sum(array_map(fn ($entry): int => $entry[0]['size'], $startupExisting));
    foreach ($startupMissing as $name) { $startupDestinationBytes += strlen($startupRows[$name]['plain']); }
    startupGate($startupDestinationBytes <= STARTUP_MAX_TOTAL);
    startupGate(startupInventory(STARTUP_KEYS, false) === $startupExisting && startupInventory(STARTUP_MUX, true) === $startupMux);
    $startupDbDigest = startupHmac('database', [$startupIdentity, $startupVersion, array_map(
        fn ($row): array => [$row['id'], $row['cipher_hmac'], startupHmac('plain', $row['plain'], $startupHmacKey)], $startupRows)], $startupHmacKey);
    $startupCreated = [];
    $startupCategory = 'ssh_write_failed';
    foreach ($startupMissing as $name) {
        $startupCreated[$name] = startupCreate(STARTUP_KEYS.'/'.$name, $startupRows[$name]['plain'], $startupParents);
    }
    $startupCategory = 'ssh_write_inventory_failed';
    $startupAfter = startupInventory(STARTUP_KEYS, false);
    $startupMuxAfter = startupInventory(STARTUP_MUX, true);
    startupGate(count($startupAfter) === count($startupExisting) + count($startupCreated));
    foreach ($startupExisting as $name => $entry) { startupGate(($startupAfter[$name] ?? null) === $entry); }
    foreach ($startupCreated as $name => $entry) { startupGate(($startupAfter[$name] ?? null) === $entry); }
    startupGate($startupMuxAfter === $startupMux);
    $startupDb->rollBack(); $startupDb = null;
    $startupPreservedAfter = array_intersect_key($startupAfter, $startupExisting);
    startupResult(['version' => 1, 'operation' => 'ssh-reconstruct', 'selected_version' => $startupVersion,
        'rows_checked' => count($startupRows), 'keys_created' => count($startupCreated), 'keys_matched' => $startupMatched,
        'unrelated_key_count' => count(array_diff_key($startupExisting, $startupRows)), 'mux_entry_count' => count($startupMux),
        'db_identity_hmac' => $startupDbDigest,
        'existing_keys_hmac_before' => $startupExistingHmac,
        'existing_keys_hmac_after' => startupHmac('existing', $startupPreservedAfter, $startupHmacKey),
        'mux_hmac_before' => $startupMuxHmac,
        'mux_hmac_after' => startupHmac('mux', $startupMuxAfter, $startupHmacKey),
        'created_inventory_hmac' => startupHmac('created', $startupCreated, $startupHmacKey)], 0);
} catch (Throwable $failure) {
    if ($startupDb !== null) { try { $startupDb->rollBack(); } catch (Throwable $ignored) {} }
    // A failed exclusive create can leave ONLY its own partial leaf. Existing
    // files are never removed/repaired; a subsequent invocation rejects it.
    $startupOperation = isset($argv[1]) && in_array($argv[1], ['barrier-observe', 'barrier-enter', 'barrier-restore', 'runtime-observe', 'scheduler-context', 'scheduler-context-check', 'horizon-inspect-request', 'horizon-inspect-observe', 'horizon-drain-enter', 'horizon-drain-observe', 'horizon-restore', 'horizon-restore-worker'], true) ? $argv[1] : 'ssh-reconstruct';
    startupResult(['version' => 1, 'operation' => $startupOperation, 'category' => $startupCategory], 1);
}
