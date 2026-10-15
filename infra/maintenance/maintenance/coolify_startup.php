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

function startupBarrier(string $operation, string $version, string $hmacKey): never {
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

$startupCategory = 'ssh_input_invalid';
$startupDb = null;
try {
    startupGate(PHP_SAPI === 'cli' && $argc === 2 && in_array($argv[1], ['ssh-reconstruct', 'barrier-observe', 'barrier-enter', 'barrier-restore'], true)
        && realpath(__FILE__) === '/probe/startup.php'
        && posix_geteuid() === 9999 && posix_getegid() === 9999);
    $startupCategory = 'ssh_hmac_invalid';
    [, $startupHmacKey] = startupRead('/probe/hmac.key', 32);
    startupGate(strlen($startupHmacKey) === 32);
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
        startupGate(getenv($flag) === 'false');
    }
    $startupConnection = config('database.connections.pgsql');
    startupGate(is_array($startupConnection) && $startupConnection['host'] === '127.0.0.1'
        && (string)$startupConnection['port'] === '5432' && $startupConnection['database'] === 'coolify'
        && $startupConnection['username'] === 'coolify' && $startupConnection['password'] === 'rehearsal-only-password'
        && empty($startupConnection['url']) && empty($startupConnection['read']) && empty($startupConnection['write']));
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
    $startupOperation = isset($argv[1]) && in_array($argv[1], ['barrier-observe', 'barrier-enter', 'barrier-restore'], true) ? $argv[1] : 'ssh-reconstruct';
    startupResult(['version' => 1, 'operation' => $startupOperation, 'category' => $startupCategory], 1);
}
