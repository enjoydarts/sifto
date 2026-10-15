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

$startupCategory = 'ssh_input_invalid';
$startupDb = null;
try {
    startupGate(PHP_SAPI === 'cli' && $argc === 2 && $argv[1] === 'ssh-reconstruct'
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
    startupResult(['version' => 1, 'operation' => 'ssh-reconstruct', 'category' => $startupCategory], 1);
}
