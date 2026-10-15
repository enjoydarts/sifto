<?php
// Output boundary: only static categories, counts and HMAC of identifiers.
declare(strict_types=1);
error_reporting(0);
ini_set('display_errors', '0');
ob_start();

function gate(bool $condition): void {
    if (!$condition) { throw new RuntimeException('probe_gate'); }
}

function hmacStatIdentity(array $info): array {
    $identity = [];
    foreach (['dev', 'ino', 'size', 'mode', 'nlink', 'uid', 'gid', 'mtime', 'ctime'] as $field) {
        gate(array_key_exists($field, $info) && is_int($info[$field]));
        $identity[$field] = $info[$field];
    }
    return $identity;
}

function readHmacKey(string $path): string {
    gate(function_exists('posix_geteuid') && function_exists('posix_getegid'));
    $uid = posix_geteuid(); $gid = posix_getegid();
    gate(is_int($uid) && $uid > 0 && is_int($gid) && $gid > 0);
    clearstatcache(true, $path);
    $before = lstat($path);
    gate(is_array($before) && realpath($path) === $path);
    $identity = hmacStatIdentity($before);
    gate(($before['mode'] & 0170000) === 0100000 && ($before['mode'] & 07777) === 0600 && $before['size'] === 32 && $before['nlink'] === 1);
    $nonowner = $identity; unset($nonowner['uid'], $nonowner['gid']);
    // A fresh bind may briefly report root ownership. Only that exact pair may
    // converge; no other metadata may change and no key FD is opened meanwhile.
    for ($attempt = 0; $before['uid'] === 0 && $before['gid'] === 0; $attempt++) {
        gate($attempt < 8);
        usleep(250000);
        clearstatcache(true, $path);
        $before = lstat($path);
        gate(is_array($before) && realpath($path) === $path);
        $identity = hmacStatIdentity($before);
        $observedNonowner = $identity; unset($observedNonowner['uid'], $observedNonowner['gid']);
        gate($observedNonowner === $nonowner);
    }
    gate($before['uid'] === $uid && $before['gid'] === $gid);
    $stream = fopen($path, 'rb');
    gate(is_resource($stream) && get_resource_type($stream) === 'stream');
    try {
        $opened = fstat($stream);
        gate(is_array($opened) && hmacStatIdentity($opened) === $identity);
        $payload = fread($stream, 33);
        gate(is_string($payload) && strlen($payload) === 32);
        $after = fstat($stream);
        gate(is_array($after) && hmacStatIdentity($after) === $identity);
        clearstatcache(true, $path);
        $final = lstat($path);
        gate(is_array($final) && hmacStatIdentity($final) === $identity && realpath($path) === $path);
        return $payload;
    } finally {
        if (is_resource($stream)) { fclose($stream); }
    }
}

function activityLogSourceProfile(): array {
    // Public read-only file derivation is byte-identical in both pinned images.
    // Spatie 4.12.3, source 2a2024fcac05628b0d1bfdbb1b94dda8b0661dc0.
    // First three migrations use config indirection; last two use literal SQL.
    // Sources are bound as bytes only, never loaded or executed by this helper.
    return [
        'vendor/spatie/laravel-activitylog/config/activitylog.php' => 'fb0514535d5c9d3ea053f9ddc170800da14b80affb49935499ada54e12881357',
        'vendor/spatie/laravel-activitylog/src/ActivitylogServiceProvider.php' => 'a7ac294472f243c31d81f6109df8be42f7dde308e3eb3c7a1f491e89ac387bcd',
        'vendor/spatie/laravel-activitylog/src/Models/Activity.php' => 'd7796edc45ced67125a8bfd39b8400a68a8667544c246d0e61fc624e1407f466',
        'database/migrations/2023_03_20_112410_create_activity_log_table.php' => 'b638449880f6c6d4caf48fa2b33c02aded5c08f6ce97115b4de368e3544aaef8',
        'database/migrations/2023_03_20_112411_add_event_column_to_activity_log_table.php' => '7b939413d1f49e769caf46448f96ca9be0ff3c146dedfb83703184d352578c26',
        'database/migrations/2023_03_20_112412_add_batch_uuid_column_to_activity_log_table.php' => 'ebc57f53396fa2e3dda9ba7678c95183ca12412e5c7815c0e0b324ef33207865',
        'database/migrations/2024_11_11_125366_add_index_to_activity_log.php' => '9653d2bf6906ef87844ec888b3ac6e5dbde89698a8e051bbcf1e43188ba42e90',
        'database/migrations/2025_06_26_131350_optimize_activity_log_indexes.php' => 'c683865a772f1d2d3b8169fe1f6aedae28da43833013834510c530d13b4ce896',
    ];
}

function activityLogContext(): array {
    $root = '/var/www/html/'; $sources = [];
    foreach (activityLogSourceProfile() as $relative => $digest) {
        $path = $root.$relative;
        gate(realpath($path) === $path);
        $sources[$relative] = hash_file('sha256', $path);
    }
    $path = $root.'vendor/composer/installed.json';
    gate(realpath($path) === $path && filesize($path) <= 4 * 1024 * 1024);
    $installed = json_decode(file_get_contents($path), true, 64, JSON_THROW_ON_ERROR);
    $packages = $installed['packages'] ?? $installed;
    gate(is_array($packages) && array_is_list($packages) && count($packages) <= 10000);
    $matches = array_values(array_filter($packages, fn ($package): bool => is_array($package) && ($package['name'] ?? null) === 'spatie/laravel-activitylog'));
    gate(count($matches) === 1);
    return ['package' => array_intersect_key($matches[0], array_flip(['name', 'version', 'source'])), 'sources' => $sources, 'config' => [
        'table_name' => config('activitylog.table_name'),
        'database_connection' => config('activitylog.database_connection'),
        'activity_model' => config('activitylog.activity_model'),
    ]];
}

function trustedTableCatalog(mixed $manifestTables, mixed $coreTables, mixed $migrations, mixed $version, mixed $expectedMigrations, mixed $activityContext): array {
    gate(in_array($version, ['4.4.2', '4.4.6'], true));
    // Sorted list digests authenticate the immutable b37 manifest's 77/59 tables
    // and full selected 435/438 migration lists; caller flags cannot grant tables.
    $migrationProfiles = ['4.4.2' => 'bd4d2bdad3d09f147f6a2726f021176d18b2dc62816effe700a09678e5fedbfe', '4.4.6' => '258df8c9e851c19fa53d10f9b8346de9a3add4d119c270352c8893c85815bf9a'];
    foreach ([[$manifestTables, 77, 'de39285ac19d1bd5206e5e4b8a28c02eb2cb0b056e0da18faaee8fb5ccd9ec9e'], [$coreTables, 59, '033f368f01c7433379ecef3ce2569e5993aa3dc653d0f762431e11bd9ae1e001'], [$expectedMigrations, $version === '4.4.2' ? 435 : 438, $migrationProfiles[$version]]] as [$profile, $size, $digest]) {
        gate(is_array($profile) && array_is_list($profile) && count($profile) === $size);
        foreach ($profile as $name) { gate(is_string($name)); }
        sort($profile, SORT_STRING);
        gate(hash('sha256', json_encode($profile, JSON_THROW_ON_ERROR)) === $digest);
    }
    validateMigrationRepository($migrations, $expectedMigrations, $expectedMigrations, $version);
    gate(is_array($activityContext) && count($activityContext) === 3 && count(array_diff(array_keys($activityContext), ['package', 'sources', 'config'])) === 0);
    gate(($activityContext['package'] ?? null) === ['name' => 'spatie/laravel-activitylog', 'version' => '4.12.3', 'source' => ['type' => 'git', 'url' => 'https://github.com/spatie/laravel-activitylog.git', 'reference' => '2a2024fcac05628b0d1bfdbb1b94dda8b0661dc0']]);
    $sources = $activityContext['sources'] ?? null; $profile = activityLogSourceProfile();
    gate(is_array($sources) && count($sources) === count($profile) && count(array_diff(array_keys($sources), array_keys($profile))) === 0);
    foreach ($profile as $path => $hash) { gate(($sources[$path] ?? null) === $hash); }
    $config = $activityContext['config'] ?? null;
    gate(is_array($config) && count($config) === 3 && count(array_diff(array_keys($config), ['table_name', 'database_connection', 'activity_model'])) === 0);
    gate(($config['table_name'] ?? null) === 'activity_log' && array_key_exists('database_connection', $config) && in_array($config['database_connection'], [null, 'pgsql'], true) && ($config['activity_model'] ?? null) === 'Spatie\\Activitylog\\Models\\Activity');
    $required = array_merge($coreTables, ['migrations', 'activity_log']);
    $allowed = array_merge($manifestTables, ['migrations', 'activity_log']);
    // Fixed retired catalog SHA256 44798287f34608bdb47ce9f4f4643026602ec1c7515b2d2aa40d7b4dd4460e85.
    // 2018 public source SHA256 685119f4a2ae53a57cae6a43f875660f5f1cd8e830d3f6cd12b227c279172881.
    // Only these three proven literals; retired source is never executed.
    if (in_array('2018_08_08_100000_create_telescope_entries_table', $migrations, true)) {
        $telescope = ['telescope_entries', 'telescope_entries_tags', 'telescope_monitoring'];
        $allowed = array_merge($allowed, $telescope); $required = array_merge($required, $telescope);
    }
    return ['allowed' => $allowed, 'required' => $required];
}

function coreTableDiagnostics(mixed $actualTables, mixed $manifestTables, mixed $coreTables, mixed $migrations = null, mixed $version = null, mixed $expectedMigrations = null, mixed $activityContext = null): array {
    $unavailable = ['query_ok' => false, 'db_count' => 0, 'nonstring_count' => 0, 'invalid_name_count' => 0, 'duplicate_count' => 0, 'unknown_count' => 0, 'missing_core_count' => 0, 'migration_table_present' => false];
    // Only the frozen public profile can classify queried names; no names leave this helper.
    foreach ([[$manifestTables, 77], [$coreTables, 59]] as [$public, $expectedCount]) {
        if (!is_array($public) || !array_is_list($public) || count($public) !== $expectedCount) { return $unavailable; }
        foreach ($public as $name) {
            if (!is_string($name) || preg_match('/\A[a-z_][a-z0-9_]{0,127}\z/', $name) !== 1) { return $unavailable; }
        }
        if (count(array_unique($public, SORT_STRING)) !== count($public)) { return $unavailable; }
    }
    if (in_array('migrations', $manifestTables, true) || count(array_diff($coreTables, $manifestTables)) !== 0) { return $unavailable; }
    try { $catalog = trustedTableCatalog($manifestTables, $coreTables, $migrations, $version, $expectedMigrations, $activityContext); }
    catch (Throwable $failure) { return $unavailable; }
    if (!is_array($actualTables) || !array_is_list($actualTables) || count($actualTables) > 1000) { return $unavailable; }
    $stats = $unavailable; $validNames = [];
    $stats['query_ok'] = true; $stats['db_count'] = count($actualTables);
    foreach ($actualTables as $name) {
        if (!is_string($name)) { $stats['nonstring_count']++; }
        elseif (preg_match('/\A[a-z_][a-z0-9_]{0,127}\z/', $name) !== 1) { $stats['invalid_name_count']++; }
        else { $validNames[] = $name; }
    }
    $distinctNames = array_unique($validNames, SORT_STRING);
    $stats['duplicate_count'] = count($validNames) - count($distinctNames);
    $stats['unknown_count'] = count(array_diff($distinctNames, $catalog['allowed']));
    $stats['missing_core_count'] = count(array_diff($coreTables, $distinctNames));
    $stats['migration_table_present'] = in_array('migrations', $distinctNames, true);
    return $stats;
}

function tableCandidateClassification(mixed $actualTables, mixed $manifestTables, mixed $coreTables, mixed $migrations, mixed $stats, mixed $version, mixed $expectedMigrations, mixed $activityContext = null): array {
    $unavailable = ['query_ok' => false, 'telescope_candidate_count' => 0, 'framework_candidate_count' => 0, 'passkey_candidate_count' => 0, 'unsupported_count' => 0, 'telescope_repository_present' => false];
    try {
        $catalog = trustedTableCatalog($manifestTables, $coreTables, $migrations, $version, $expectedMigrations, $activityContext);
        $observed = coreTableDiagnostics($actualTables, $manifestTables, $coreTables, $migrations, $version, $expectedMigrations, $activityContext);
        gate($observed === $stats && $observed['query_ok'] && $observed['nonstring_count'] === 0 && $observed['invalid_name_count'] === 0 && $observed['duplicate_count'] === 0);
        $result = $unavailable; $result['query_ok'] = true;
        // Public diagnostic candidates only, never table allowances or deployment proof.
        // Telescope: retired catalog 44798287f34608bdb47ce9f4f4643026602ec1c7515b2d2aa40d7b4dd4460e85,
        // blob 700a83f0958543ed173a466ddfbeb0833103baf5, SHA256 685119f4a2ae53a57cae6a43f875660f5f1cd8e830d3f6cd12b227c279172881.
        // Passkeys: public publish-only package evidence SHA256
        // 0c34835680b23b7e8e606a65cd7dee4bb833746b638f8bc01042ffe0aca56f3e.
        $telescope = ['telescope_entries', 'telescope_entries_tags', 'telescope_monitoring'];
        $framework = ['cache', 'cache_locks', 'jobs', 'job_batches', 'notifications'];
        foreach (array_diff($actualTables, $catalog['allowed']) as $name) {
            if (in_array($name, $telescope, true)) { $result['telescope_candidate_count']++; }
            elseif (in_array($name, $framework, true)) { $result['framework_candidate_count']++; }
            elseif ($name === 'passkeys') { $result['passkey_candidate_count']++; }
            else { $result['unsupported_count']++; }
        }
        $result['telescope_repository_present'] = in_array('2018_08_08_100000_create_telescope_entries_table', $migrations, true);
        return $result;
    } catch (Throwable $failure) { return $unavailable; }
}

function validateCoreTableInventory(mixed $actualTables, array $manifestTables, array $coreTables, mixed $migrations = null, mixed $version = null, mixed $expectedMigrations = null, mixed $activityContext = null): void {
    gate(is_array($actualTables) && array_is_list($actualTables) && count($actualTables) <= 1000);
    foreach ($actualTables as $table) {
        gate(is_string($table) && preg_match('/\A[a-z_][a-z0-9_]{0,127}\z/', $table) === 1);
    }
    gate(count(array_unique($actualTables, SORT_STRING)) === count($actualTables));
    // The framework repository is required independently of application model tables.
    $catalog = trustedTableCatalog($manifestTables, $coreTables, $migrations, $version, $expectedMigrations, $activityContext);
    gate(count(array_diff($actualTables, $catalog['allowed'])) === 0
        && count(array_diff($catalog['required'], $actualTables)) === 0);
}

function digestRows(array $rows, string $key): string {
    usort($rows, fn (array $a, array $b): int => $a[0] <=> $b[0]);
    return hash_hmac('sha256', json_encode($rows, JSON_THROW_ON_ERROR), $key);
}

function primaryColumns(string $table): array {
    if ($table === 'team_user') { return ['id', 'team_id', 'user_id']; }
    if ($table === 'taggables') { return ['tag_id', 'taggable_id', 'taggable_type']; }
    $keys = Illuminate\Support\Facades\DB::select('SELECT a.attname AS name FROM pg_index i CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY k(attnum, position) JOIN pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=k.attnum WHERE i.indisprimary AND i.indrelid=?::regclass ORDER BY k.position', [$table]);
    $columns = array_map(fn (object $row): string => (string)$row->name, $keys);
    gate(count($columns) > 0 && count($columns) <= 16);
    foreach ($columns as $column) { gate(preg_match('/\A[a-z0-9_]{1,128}\z/', $column) === 1); }
    return $columns;
}

function protectedSshFile(string $path): string {
    for ($parent = dirname($path); $parent !== '/'; $parent = dirname($parent)) { gate(!is_link($parent) && is_dir($parent)); }
    clearstatcache(true, $path);
    $before = lstat($path);
    gate(is_array($before) && ($before['mode'] & 0170000) === 0100000 && ($before['mode'] & 0077) === 0 && $before['nlink'] === 1 && $before['uid'] === posix_geteuid() && $before['size'] > 0 && $before['size'] <= 1048576 && !is_link($path));
    $stream = fopen($path, 'rb'); gate(is_resource($stream));
    try {
        $opened = fstat($stream);
        $fields = ['dev', 'ino', 'size', 'mode', 'nlink', 'uid', 'gid', 'mtime', 'ctime'];
        foreach ($fields as $field) { gate($opened[$field] === $before[$field]); }
        $bytes = stream_get_contents($stream, 1048577);
        $after = fstat($stream); clearstatcache(true, $path); $final = lstat($path);
        foreach ($fields as $field) { gate($after[$field] === $before[$field] && $final[$field] === $before[$field]); }
        gate(is_string($bytes) && strlen($bytes) === $before['size'] && !is_link($path));
        return $bytes;
    } finally { fclose($stream); }
}

function typedPlain(mixed $value, string $cast): bool {
    return match ($cast) {
        'encrypted', 'encrypted:string', 'fortify:string' => is_string($value),
        'encrypted:array' => is_array($value),
        'encrypted:collection' => $value instanceof Illuminate\Support\Collection,
        'encrypted:object' => is_object($value),
        default => false,
    };
}

function correctedEncryptedCasts(mixed $name, mixed $declared, mixed $sourceHash, mixed $version, mixed $expectedMigrations): array {
    gate(is_string($name) && preg_match('/\\A[A-Za-z][A-Za-z0-9]*\\z/', $name) === 1);
    gate(is_string($sourceHash) && preg_match('/\\A[0-9a-f]{64}\\z/', $sourceHash) === 1);
    gate(in_array($version, ['4.4.2', '4.4.6'], true) && is_array($declared));
    foreach ($declared as $attribute => $cast) {
        gate(is_string($attribute) && preg_match('/\\A[a-z0-9_]+\\z/', $attribute) === 1);
        gate(in_array($cast, ['encrypted', 'encrypted:string', 'encrypted:array', 'encrypted:collection', 'encrypted:object'], true));
    }
    ksort($declared);
    if (in_array($name, ['LocalFileVolume', 'InstanceSettings', 'Server'], true)) {
        $migrationProfiles = ['4.4.2' => 'bd4d2bdad3d09f147f6a2726f021176d18b2dc62816effe700a09678e5fedbfe', '4.4.6' => '258df8c9e851c19fa53d10f9b8346de9a3add4d119c270352c8893c85815bf9a'];
        gate(is_array($expectedMigrations) && array_is_list($expectedMigrations) && count($expectedMigrations) === ($version === '4.4.2' ? 435 : 438));
        foreach ($expectedMigrations as $migration) { gate(is_string($migration)); }
        sort($expectedMigrations, SORT_STRING);
        gate(hash('sha256', json_encode($expectedMigrations, JSON_THROW_ON_ERROR)) === $migrationProfiles[$version]);
        if ($name === 'LocalFileVolume') {
            gate($sourceHash === 'f0bc1eb914bcb8a009c372a71babd87e4f4f2eb29737b4a1fa9b041f878ab468');
            gate($declared === ['content' => 'encrypted', 'fs_path' => 'encrypted', 'mount_path' => 'encrypted']);
            // The latter two entries are comments in this exact public source.
            return ['content' => 'encrypted'];
        }
        if ($name === 'Server') {
            $serverSources = ['4.4.2' => 'adc7e42d8b6d665afcb7d4f35752d8ba06c81edeaa6afd8a99b40a20a3b3eaeb', '4.4.6' => '2f9b1de185caf15b1cb804bfcfa5c66c9e5e54650d4521b59c55885fdb17e7d4'];
            gate($sourceHash === $serverSources[$version]);
            gate($declared === ['logdrain_axiom_api_key' => 'encrypted', 'logdrain_newrelic_license_key' => 'encrypted']);
            return $declared;
        }
        gate($sourceHash === 'a72d4ebbcbe9acf9443dd8dfa7ec2f82f51f08fdf45d67450a94ea4c7a2524d1');
        gate($declared === array_fill_keys(['domain_connect_private_key', 'resend_api_key', 'sentinel_token', 'smtp_from_address', 'smtp_from_name', 'smtp_host', 'smtp_password', 'smtp_recipients', 'smtp_username'], 'encrypted'));
    }
    return $declared;
}

function persistedEncryptedAttributes(mixed $name, mixed $declared, mixed $sourceHash, mixed $version, mixed $expectedMigrations, mixed $presence): array {
    $attributes = correctedEncryptedCasts($name, $declared, $sourceHash, $version, $expectedMigrations);
    if ($name === 'User') { $attributes['two_factor_secret'] = 'fortify:string'; $attributes['two_factor_recovery_codes'] = 'fortify:string'; }
    gate(is_array($presence) && count($presence) === count($attributes) && count(array_diff(array_keys($presence), array_keys($attributes))) === 0);
    foreach ($attributes as $attribute => $cast) {
        gate(is_bool($presence[$attribute]));
        if ($name === 'Server' && in_array($attribute, ['logdrain_axiom_api_key', 'logdrain_newrelic_license_key'], true) && $presence[$attribute] === false) {
            // Exact pinned Server declarations; current DDL adds these to server_settings only.
            // Keep each present Server attribute selected, decrypted and counted.
            unset($attributes[$attribute]); continue;
        }
        if ($name === 'InstanceSettings' && $attribute === 'sentinel_token' && $presence[$attribute] === false) {
            // Pinned current DDL adds sentinel_token only to server_settings.
            // Declared cast inventory remains nine; a present column stays covered.
            unset($attributes[$attribute]); continue;
        }
        gate($presence[$attribute] === true);
    }
    return $attributes;
}

function predictedIpv6Changes(): array {
    // Exact pure semantics from the frozen target migration/helpers, also usable on the old image.
    $counts = ['servers' => 0, 'sentinel' => 0];
    foreach (Illuminate\Support\Facades\DB::table('servers')->select(['id', 'ip', 'hetzner_server_id'])->where('ip', 'like', '%:%')->cursor() as $server) {
        $ip = (string)$server->ip;
        if (filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV6) === false) { continue; }
        $normalized = inet_ntop(inet_pton(trim($ip, '[]')));
        $reachable = (bool)Illuminate\Support\Facades\DB::table('server_settings')->where('server_id', $server->id)->value('is_reachable');
        if ($server->hetzner_server_id !== null && str_ends_with($ip, '::64') && !$reachable) {
            $network = substr($ip, 0, -strlen('::64')).'::';
            if (filter_var($network, FILTER_VALIDATE_IP, FILTER_FLAG_IPV6) !== false) { $normalized = inet_ntop(inet_pton($network) | inet_pton('::1')); }
        }
        $duplicate = Illuminate\Support\Facades\DB::table('servers')->whereNull('deleted_at')->where('id', '!=', $server->id)->where('ip', $normalized)->exists();
        if ($normalized !== $ip && !$duplicate) { $counts['servers']++; }
    }
    foreach (Illuminate\Support\Facades\DB::table('server_settings')->select(['sentinel_custom_url'])->where('sentinel_custom_url', 'like', '%://%:%:%')->cursor() as $setting) {
        $url = (string)$setting->sentinel_custom_url;
        if (preg_match('#\A(https?)://([0-9A-Fa-f:.]+):(\d+)(/.*)?\z#', $url, $matches) === 1 && filter_var($matches[2], FILTER_VALIDATE_IP, FILTER_FLAG_IPV6) !== false) { $counts['sentinel']++; }
    }
    return $counts;
}

function networkStubFlags(): array {
    return ['tunl0' => ['NOARP'], 'gre0' => ['NOARP'], 'gretap0' => ['BROADCAST', 'MULTICAST'], 'erspan0' => ['BROADCAST', 'MULTICAST'], 'ip_vti0' => ['NOARP'], 'ip6_vti0' => ['NOARP'], 'sit0' => ['NOARP'], 'ip6tnl0' => ['NOARP'], 'ip6gre0' => ['NOARP']];
}

function networkCommandResult(string $stdout, string $stderr, int $exit, bool $timedOut): string {
    gate(!$timedOut && $exit === 0 && $stderr === '' && strlen($stdout) <= 65536);
    return $stdout;
}

function networkCommandOutputs(): array {
    $outputs = [];
    // Fixed image-owned argv only. The restored application environment never reaches a child.
    foreach ([['/sbin/ip', 'link', 'show'], ['/sbin/ip', 'address', 'show']] as $command) {
        $deadline = hrtime(true) + 2000000000;
        $process = proc_open($command, [0 => ['file', '/dev/null', 'r'], 1 => ['pipe', 'w'], 2 => ['pipe', 'w']], $pipes, '/', [], ['bypass_shell' => true]);
        gate(is_resource($process));
        $stdout = ''; $stderr = ''; $exit = -1; $timedOut = false; $running = true;
        try {
            gate(stream_set_blocking($pipes[1], false) && stream_set_blocking($pipes[2], false));
            while (true) {
                foreach ([1, 2] as $channel) {
                    $bytes = stream_get_contents($pipes[$channel], 65537 - min(strlen($channel === 1 ? $stdout : $stderr), 65536));
                    gate(is_string($bytes));
                    if ($channel === 1) { $stdout .= $bytes; } else { $stderr .= $bytes; }
                }
                $state = proc_get_status($process); gate(is_array($state));
                $running = $state['running'];
                if (hrtime(true) >= $deadline) { $timedOut = true; break; }
                if (strlen($stdout) > 65536 || strlen($stderr) > 65536) { break; }
                if (!$running) {
                    $exit = $state['exitcode'];
                    // Drain bytes written between the read and the child-exit observation.
                    $stdout .= stream_get_contents($pipes[1], 65537 - strlen($stdout));
                    $stderr .= stream_get_contents($pipes[2], 65537 - strlen($stderr));
                    break;
                }
                usleep(1000);
            }
        } finally {
            if ($running) { proc_terminate($process, 9); }
            fclose($pipes[1]); fclose($pipes[2]);
            $closed = proc_close($process); if ($exit < 0) { $exit = $closed; }
        }
        $outputs[$command[1]] = networkCommandResult($stdout, $stderr, $exit, $timedOut);
    }
    return $outputs;
}

function parseNetworkView(string $output, bool $addresses): array {
    gate($output !== '' && strlen($output) <= 65536 && !str_contains($output, "\r") && !str_contains($output, "\0") && str_ends_with($output, "\n") && !str_ends_with($output, "\n\n"));
    $stubs = networkStubFlags(); $view = []; $indices = []; $current = null; $lifetime = false;
    foreach (explode("\n", substr($output, 0, -1)) as $line) {
        // BusyBox emits a trailing space on the loopback IPv6 address line.
        $line = rtrim($line, ' ');
        if (preg_match('/\A([1-9][0-9]{0,9}): ([a-z][a-z0-9_]*(?:@NONE)?): <([A-Z_]+(?:,[A-Z_]+)*)> (.+)\z/', $line, $matches) === 1) {
            gate(!$lifetime && ($current === null || $view[$current]['link']));
            $parts = explode('@', $matches[2]); $name = $parts[0];
            gate($name === 'lo' || isset($stubs[$name]));
            gate(count($parts) === 1 || ($name !== 'lo' && isset($stubs[$name]) && $parts[1] === 'NONE'));
            gate(!isset($view[$name]) && !isset($indices[$matches[1]]));
            $metadata = preg_split('/\s+/', $matches[4]); $seen = [];
            gate(count($metadata) % 2 === 0 && $metadata[0] === 'mtu');
            for ($i = 0; $i < count($metadata); $i += 2) {
                [$key, $value] = [$metadata[$i], $metadata[$i + 1]];
                gate(!isset($seen[$key])); $seen[$key] = true;
                if (in_array($key, ['mtu', 'qlen', 'group'], true)) { gate(preg_match('/\A[0-9]{1,10}\z/', $value) === 1); }
                elseif ($key === 'qdisc') { gate(preg_match('/\A[a-z0-9_]{1,32}\z/', $value) === 1); }
                elseif ($key === 'state') { gate(in_array($value, $name === 'lo' ? ['UP', 'UNKNOWN'] : ['DOWN', 'UNKNOWN'], true)); }
                elseif ($key === 'mode') { gate($value === 'DEFAULT'); }
                else { gate(false); }
            }
            $flags = explode(',', $matches[3]); gate(count($flags) === count(array_unique($flags))); sort($flags);
            $view[$name] = ['index' => $matches[1], 'flags' => $flags, 'addresses' => [], 'link' => false];
            $indices[$matches[1]] = true; $current = $name;
        } elseif (preg_match('/\A\s+link\/[a-z][a-z0-9_]* \S+(?: \S+ \S+)*\z/', $line) === 1 || ($current === 'ip6gre0' && preg_match('/\A\s+link\/\[823\] \S+(?: \S+ \S+)*\z/', $line) === 1)) {
            gate($current !== null && !$view[$current]['link'] && !$lifetime);
            $view[$current]['link'] = true;
        } elseif (preg_match('/\A\s+(inet6?) (\S+) scope host(?: lo)?\z/', $line, $matches) === 1) {
            gate($addresses && $current !== null && $view[$current]['link'] && !$lifetime);
            $value = $matches[1].' '.$matches[2];
            gate($current === 'lo' && in_array($value, ['inet 127.0.0.1/8', 'inet6 ::1/128'], true) && !in_array($value, $view[$current]['addresses'], true));
            $view[$current]['addresses'][] = $value; $lifetime = true;
        } elseif ($line === '       valid_lft forever preferred_lft forever') {
            gate($addresses && $lifetime); $lifetime = false;
        } else { gate(false); }
    }
    gate(!$lifetime);
    foreach ($view as $facts) { gate($facts['link']); }
    return $view;
}

function networkInterfaces(string $devices, string $link, string $address): bool {
    gate($devices !== '' && strlen($devices) <= 65536 && !str_contains($devices, "\r") && !str_contains($devices, "\0"));
    $lines = explode("\n", rtrim($devices, "\n"));
    gate(count($lines) >= 3 && preg_match('/\A\s*Inter-\|\s*Receive\s*\|\s*Transmit\s*\z/', $lines[0]) === 1);
    $columns = explode('|', $lines[1]);
    gate(count($columns) === 3 && trim($columns[0]) === 'face' && preg_split('/\s+/', trim($columns[1])) === ['bytes', 'packets', 'errs', 'drop', 'fifo', 'frame', 'compressed', 'multicast'] && preg_split('/\s+/', trim($columns[2])) === ['bytes', 'packets', 'errs', 'drop', 'fifo', 'colls', 'carrier', 'compressed']);
    $names = [];
    foreach (array_slice($lines, 2) as $line) {
        gate(preg_match('/\A\s*([a-z][a-z0-9_]*):\s+([0-9]+(?:\s+[0-9]+){15})\s*\z/', $line, $matches) === 1 && !isset($names[$matches[1]]));
        $names[$matches[1]] = true;
    }
    $stubs = networkStubFlags(); $observed = array_keys($names); sort($observed);
    $complete = array_merge(['lo'], array_keys($stubs)); sort($complete);
    gate($observed === ['lo'] || $observed === $complete);
    $links = parseNetworkView($link, false); $addresses = parseNetworkView($address, true);
    foreach ([$links, $addresses] as $view) { $keys = array_keys($view); sort($keys); gate($keys === $observed); }
    foreach ($observed as $name) {
        $expected = $name === 'lo' ? ['LOOPBACK', 'UP', 'LOWER_UP'] : $stubs[$name]; sort($expected);
        gate($links[$name]['flags'] === $expected && $addresses[$name]['flags'] === $expected && $links[$name]['index'] === $addresses[$name]['index']);
        $assigned = $addresses[$name]['addresses']; sort($assigned);
        gate($assigned === ($name === 'lo' ? ['inet 127.0.0.1/8', 'inet6 ::1/128'] : []));
    }
    return true;
}

function networkRoutes(string $ipv4, string $ipv6): bool {
    gate($ipv4 !== '' && strlen($ipv4) <= 65536 && strlen($ipv6) <= 65536 && !str_contains($ipv4.$ipv6, "\0") && !str_contains($ipv4.$ipv6, "\r"));
    $rows = explode("\n", rtrim($ipv4, "\n"));
    gate(preg_split('/\s+/', trim(array_shift($rows))) === ['Iface', 'Destination', 'Gateway', 'Flags', 'RefCnt', 'Use', 'Metric', 'Mask', 'MTU', 'Window', 'IRTT']);
    foreach ($rows as $line) {
        $fields = preg_split('/\s+/', trim($line));
        gate(count($fields) === 11 && $fields[0] === 'lo');
        foreach ([1, 2, 7] as $field) { gate(preg_match('/\A[0-9A-Fa-f]{8}\z/', $fields[$field]) === 1); }
        gate(preg_match('/\A[0-9A-Fa-f]{4}\z/', $fields[3]) === 1);
        foreach ([4, 5, 6, 8, 9, 10] as $field) { gate(preg_match('/\A[0-9]+\z/', $fields[$field]) === 1); }
        $default = $fields[1] === '00000000' && $fields[7] === '00000000';
        gate(!$default || ((hexdec($fields[3]) & 0x200) !== 0 && $fields[2] === '00000000'));
    }
    foreach ($ipv6 === '' ? [] : explode("\n", rtrim($ipv6, "\n")) as $line) {
        $fields = preg_split('/\s+/', trim($line));
        gate(count($fields) === 10 && $fields[9] === 'lo');
        foreach ([0, 2, 4] as $field) { gate(preg_match('/\A[0-9A-Fa-f]{32}\z/', $fields[$field]) === 1); }
        foreach ([1, 3] as $field) { gate(preg_match('/\A[0-9A-Fa-f]{2}\z/', $fields[$field]) === 1 && hexdec($fields[$field]) <= 128); }
        foreach ([5, 6, 7, 8] as $field) { gate(preg_match('/\A[0-9A-Fa-f]{8}\z/', $fields[$field]) === 1); }
        $default = $fields[0] === str_repeat('0', 32) && $fields[1] === '00';
        // Linux's null /0 routes are allowed only with RTF_REJECT and a zero gateway.
        gate(!$default || ((hexdec($fields[8]) & 0x200) !== 0 && $fields[4] === str_repeat('0', 32)));
    }
    return true;
}

function isolatedNetwork(): bool {
    $devices = file_get_contents('/proc/net/dev', false, null, 0, 65537);
    $ipv4 = file_get_contents('/proc/net/route', false, null, 0, 65537);
    $ipv6 = file_get_contents('/proc/net/ipv6_route', false, null, 0, 65537);
    gate(is_string($devices) && is_string($ipv4) && is_string($ipv6));
    $views = networkCommandOutputs();
    gate(networkInterfaces($devices, $views['link'], $views['address']) && networkRoutes($ipv4, $ipv6));
    $socket = @stream_socket_client('tcp://1.1.1.1:443', $errno, $message, 2, STREAM_CLIENT_CONNECT);
    if (is_resource($socket)) { fclose($socket); return false; }
    return true;
}

function httpGate(string $path, int $port, bool $literal = false): bool {
    $context = stream_context_create(['http' => ['timeout' => 3, 'ignore_errors' => true, 'follow_location' => 0]]);
    $body = @file_get_contents('http://127.0.0.1:'.$port.$path, false, $context);
    $status = $http_response_header[0] ?? '';
    gate(is_string($body) && strlen($body) <= 4 * 1024 * 1024 && preg_match('#^HTTP/\S+ 200(?: |$)#', $status) === 1);
    if ($literal) { gate($body === 'OK'); }
    if ($path === '/login') { gate(str_contains($body, '<form') || str_contains($body, 'wire:submit')); }
    return true;
}

function mainlineRetiredMigrations(): array {
    // Inert repository metadata only, never source files to load or execute.
    // Public first-parent catalog at old revision 76d3386a4d69f894258210ea265717a8fb8cd778:
    // mainline-retired-migration-candidates.json SHA256
    // 44798287f34608bdb47ce9f4f4643026602ec1c7515b2d2aa40d7b4dd4460e85.
    return [
        '2018_08_08_100000_create_telescope_entries_table',
        '2019_08_19_000000_create_failed_jobs_table',
        '2023_03_20_112810_create_jobs_table',
        '2023_03_20_112813_create_coolify_instance_settings_table',
        '2023_03_20_112813_create_instance_settings_table',
        '2023_03_24_203356_create_private_keyables_table',
        '2023_03_27_083620_create_databases_table',
        '2023_03_27_085022_create_standalone_dockers_table',
        '2023_03_27_085023_create_swarm_dockers_table',
        '2023_03_28_083722_create_gits_table',
        '2023_03_28_100003_create_git_deploy_keys_table',
        '2023_06_19_084152_add_git_related_private_key',
        '2025_10_10_120000_create_cloud_init_scripts_table',
        '2025_10_10_120000_create_webhook_notification_settings_table',
        '2025_10_10_120001_populate_webhook_notification_settings_for_existing_teams',
        '2025_10_10_120002_create_cloud_init_scripts_table',
        '2025_10_10_120002_create_webhook_notification_settings_table',
        '2025_11_25_000001_create_webhook_notification_settings_table',
        '2025_11_25_000002_create_cloud_init_scripts_table',
        '2026_09_28_093936_remove_stored_commands_from_finished_activities',
        '2026_09_28_194652_add_next_run_at_to_scheduled_jobs',
    ];
}

function validateMigrationRepository(mixed $rows, mixed $known, mixed $expected, mixed $version): void {
    $stats = migrationDiagnostics($rows, $known, $expected, $version);
    gate($stats['query_ok'] && $stats['image_manifest_match'] && $stats['pending_count'] === 0 && $stats['nonstring_count'] === 0 && $stats['duplicate_count'] === 0);
    // Every current migration remains required. Only fixed retired mainline rows may remain.
    gate(count(array_diff($rows, $expected, mainlineRetiredMigrations())) === 0);
}

function migrationDiagnostics(mixed $rows, mixed $known, mixed $expected, mixed $version): array {
    gate(in_array($version, ['4.4.2', '4.4.6'], true));
    foreach ([$expected, $known === null ? [] : $known] as $public) {
        gate(is_array($public) && array_is_list($public) && count($public) <= 10000);
        foreach ($public as $name) { gate(is_string($name) && preg_match('/\A[0-9]{4}_[0-9]{2}_[0-9]{2}_[0-9]{6}_[A-Za-z0-9_]{1,180}\z/', $name) === 1); }
        gate(count($public) === count(array_unique($public, SORT_STRING)));
    }
    $stats = ['query_ok' => false, 'db_count' => 0, 'nonstring_count' => 0, 'duplicate_count' => 0, 'unknown_count' => 0, 'pending_count' => 0, 'image_manifest_match' => false, 'selected_version' => $version];
    if ($rows === null) { gate($known === null); return $stats; }
    gate(is_array($rows) && array_is_list($rows) && count($rows) <= 10000 && is_array($known));
    $strings = [];
    foreach ($rows as $row) {
        if (is_string($row)) { gate(strlen($row) <= 1024); $strings[] = $row; }
        else { $stats['nonstring_count']++; }
    }
    sort($known); sort($expected);
    $stats['query_ok'] = true;
    $stats['db_count'] = count($rows);
    $stats['duplicate_count'] = count($strings) - count(array_unique($strings, SORT_STRING));
    $stats['unknown_count'] = count(array_diff($strings, $expected));
    $stats['pending_count'] = count(array_diff($expected, $strings));
    $stats['image_manifest_match'] = $known === $expected;
    return $stats;
}

function migrationClassification(mixed $rows, mixed $expected, mixed $stats): array {
    $unavailable = ['query_ok' => false, 'historical_candidate_count' => 0, 'target_candidate_count' => 0, 'unsupported_count' => 0, 'mainline_candidate_count' => 0];
    try {
        $version = is_array($stats) ? ($stats['selected_version'] ?? null) : null;
        $validated = probeFailureRecord('migrations', $stats, $version)['migration_diagnostics'];
        gate($validated === $stats && $validated['query_ok'] && $validated['nonstring_count'] === 0);
        // Reuse the original bounded public-list/row validation and require matching counts.
        $observed = migrationDiagnostics($rows, $expected, $expected, $version);
        gate($observed['query_ok']);
        foreach (['db_count', 'nonstring_count', 'duplicate_count', 'unknown_count', 'pending_count'] as $key) { gate($observed[$key] === $validated[$key]); }
        // Static diagnostic-only candidates from public ancestry of old revision
        // 76d3386a4d69f894258210ea265717a8fb8cd778, absent from both fixed image lists.
        // Includes unreleased side branches; never migration gate allowances.
        // Public metadata SHA256: 312d4c7f1d889faea61b82ce37b573166d358a95332caf52f32a7ccc4f136d21.
        $historical = [
            '2018_08_08_100000_create_telescope_entries_table',
            '2019_08_19_000000_create_failed_jobs_table',
            '2023_03_20_112810_create_jobs_table',
            '2023_03_20_112813_create_coolify_instance_settings_table',
            '2023_03_20_112813_create_instance_settings_table',
            '2023_03_24_203356_create_private_keyables_table',
            '2023_03_27_075445_create_environmentables_table',
            '2023_03_27_075445_create_environmentsables_table',
            '2023_03_27_083620_create_databases_table',
            '2023_03_27_083622_create_deployments_table',
            '2023_03_27_085022_create_standalone_dockers_table',
            '2023_03_27_085023_create_swarm_dockers_table',
            '2023_03_28_083722_create_gits_table',
            '2023_03_28_100003_create_git_deploy_keys_table',
            '2023_05_09_093548_create_github_events_applications_table',
            '2023_06_19_084152_add_git_related_private_key',
            '2023_08_07_142950_create_postgresqls_table',
            '2023_08_07_142950_create_standalone_postgres_table',
            '2023_08_07_142951_add_description_field_to_applications_table',
            '2023_08_07_142952_remove_foreignId_environment_variables',
            '2023_08_07_142954_add_readonly_localpersistentvolumes',
            '2023_08_08_114038_add_port_mappings_to_standalone_postgresqls',
            '2023_08_09_154921_change_postgres_password_of_standalone_postgresqls',
            '2023_08_22_071061_add_dockercompose_to_applications_table',
            '2023_09_23_111815_add_destination_to_services_table',
            '2024_02_25_222150_add_disabled_server_due_to_overflow',
            '2024_06_07_151615_add_git_to_services',
            '2024_06_18_105947_disable_server_metrics_by_default',
            '2024_06_25_184323_remove_docker_compose_pr',
            '2024_08_05_142659_add_auto_update_frequency_and_update_check_frequency',
            '2024_08_05_142659_add_new_version_available_to_instance_settings_table',
            '2024_08_12_155023_add_timezone_to_server_settings_table',
            '2024_08_21_165435_add_redis_username_to_standalone_redis_table',
            '2024_09_16_094127_add_ssh_key_fingerprint_to_private_keys_table',
            '2024_09_16_160709_add_ssh_sftp_fields_to_private_keys_table',
            '2024_09_16_170001_populate_ssh_keys_and_clear_mux_directory',
            '2024_09_16_170001_populate_ssh_keys_directory',
            '2024_09_29_163625_add_base_url_to_oauth_settings_table',
            '2024_10_16_120026_encrypt_existing_redis_passwords',
            '2024_11_12_213200_add_slack_notifications_to_teams_table',
            '2024_11_19_000001_add_uuid_to_cloud_provider_tokens',
            '2025_01_16_130238_add_deployment_queue_finished_at',
            '2025_01_27_102616_add_ssl_fields_to_standalone_postgresqls',
            '2025_10_10_120000_create_cloud_init_scripts_table',
            '2025_10_10_120000_create_webhook_notification_settings_table',
            '2025_10_10_120001_populate_webhook_notification_settings_for_existing_teams',
            '2025_10_10_120002_create_cloud_init_scripts_table',
            '2025_10_10_120002_create_webhook_notification_settings_table',
            '2025_11_12_131253_add_traefik_outdated_to_discord_notification_settings',
            '2025_11_12_131253_add_traefik_outdated_to_pushover_notification_settings',
            '2025_11_12_131253_add_traefik_outdated_to_slack_notification_settings',
            '2025_11_12_131253_add_traefik_outdated_to_telegram_notification_settings',
            '2025_11_12_131253_add_traefik_outdated_to_webhook_notification_settings',
            '2025_11_19_115504_create_github_runner_sources_table',
            '2025_11_25_000001_create_webhook_notification_settings_table',
            '2025_11_25_000002_create_cloud_init_scripts_table',
            '2025_11_26_000000_add_build_cache_settings_to_application_settings',
            '2025_12_08_000000_add_performance_indexes',
            '2026_03_27_000000_add_max_restart_count_to_applications_and_databases',
            '2026_06_04_050157_create_v5_projects_table',
            '2026_06_16_130649_create_v5_clusters_table',
            '2026_06_16_130649_v5_create_clusters_table',
            '2026_06_16_130650_create_v5_servers_table',
            '2026_06_16_130650_v5_create_servers_table',
            '2026_06_16_131229_add_cluster_id_to_v5_servers_table',
            '2026_06_16_132000_make_v5_server_private_key_nullable',
            '2026_06_16_204644_v5_add_wireguard_cli_configuration_to_clusters_and_servers',
            '2026_06_17_165112_v5_add_builder_cpu_quota_to_servers_table',
            '2026_06_17_172845_add_status_check_fields_to_v5_servers_table',
            '2026_06_19_140000_v5_create_applications_table',
            '2026_06_19_141231_add_canvas_position_to_v5_servers_table',
            '2026_06_19_142000_v5_create_resource_connections_table',
            '2026_06_19_173933_add_caddy_ingress_status_to_v5_servers_table',
            '2026_06_19_182231_create_container_statuses_table',
            '2026_06_20_072818_v5_add_ingress_routing_to_applications_table',
            '2026_07_05_215736_v5_add_status_lookup_indexes',
            '2026_07_05_215736_v5_make_servers_uuid_not_null',
            '2026_07_05_222616_v5_add_status_observed_at_columns',
            '2026_07_05_222940_v5_add_coold_version_to_servers_table',
            '2026_07_06_090000_v5_convert_resource_connection_morphs_to_aliases',
            '2026_07_06_090100_v5_convert_server_capabilities_to_booleans',
            '2026_07_06_100000_v5_add_agent_token_jti_to_servers_table',
            '2026_07_06_100100_v5_create_revoked_agent_tokens_table',
            '2026_07_06_110000_v5_add_agent_token_expires_at_to_servers_table',
            '2026_07_06_130000_v5_add_missing_uuid_columns',
            '2026_07_15_125721_add_retention_limits_to_scheduled_volume_backups_table',
            '2026_08_07_185535_add_avatar_storage_settings_to_instance_settings_table',
            '2026_09_28_093936_remove_stored_commands_from_finished_activities',
            '2026_09_28_194652_add_next_run_at_to_scheduled_jobs',
            'new_services',
        ];
        $mainline = mainlineRetiredMigrations();
        $target = ['2026_10_09_092903_normalize_ipv6_server_ips', '2026_10_09_115444_remove_duplicate_redis_username_environment_variables', '2026_10_09_124641_remove_duplicate_nixpacks_node_version_environment_variables'];
        $classification = array_replace($unavailable, ['query_ok' => true]);
        foreach ($rows as $name) {
            gate(is_string($name));
            if (in_array($name, $expected, true)) { continue; }
            $key = in_array($name, $historical, true) ? 'historical_candidate_count' : (in_array($name, $target, true) ? 'target_candidate_count' : 'unsupported_count');
            $classification[$key]++;
            if ($key === 'historical_candidate_count' && in_array($name, $mainline, true)) { $classification['mainline_candidate_count']++; }
        }
        gate($classification['historical_candidate_count'] + $classification['target_candidate_count'] + $classification['unsupported_count'] === $validated['unknown_count']);
        gate($classification['mainline_candidate_count'] <= $classification['historical_candidate_count']);
        return $classification;
    } catch (Throwable $failure) { return $unavailable; }
}

function decryptionDiagnostics(mixed $diagnostics): array {
    $safe = ['available' => false, 'checkpoint' => 'unavailable', 'model_ordinal' => 0, 'checked' => 0, 'nonempty' => 0, 'missing' => 0, 'failures' => 0];
    $checkpoints = ['unavailable', 'model_construct', 'model_table', 'cast_type', 'cast_inventory', 'encrypted_column', 'row_query', 'raw_string', 'decrypt_value', 'plaintext_type', 'fortify_json', 'fortify_codes', 'ssh_uuid', 'ssh_file', 'summary'];
    if (!is_array($diagnostics) || count($diagnostics) !== count($safe) || count(array_diff(array_keys($diagnostics), array_keys($safe))) !== 0) { return $safe; }
    if (!is_bool($diagnostics['available']) || !in_array($diagnostics['checkpoint'], $checkpoints, true) || !is_int($diagnostics['model_ordinal']) || $diagnostics['model_ordinal'] < 0 || $diagnostics['model_ordinal'] > 68) { return $safe; }
    foreach (['checked', 'nonempty', 'missing', 'failures'] as $key) {
        if (!is_int($diagnostics[$key]) || $diagnostics[$key] < 0 || $diagnostics[$key] > 10000000) { return $safe; }
    }
    if (!$diagnostics['available']) {
        if ($diagnostics['checkpoint'] !== 'unavailable' || $diagnostics['model_ordinal'] !== 0 || $diagnostics['checked'] !== 0 || $diagnostics['nonempty'] !== 0 || $diagnostics['missing'] !== 0 || $diagnostics['failures'] !== 0) { return $safe; }
    } elseif ($diagnostics['checkpoint'] === 'unavailable' || $diagnostics['model_ordinal'] < 1 || $diagnostics['nonempty'] > $diagnostics['checked'] || $diagnostics['failures'] > $diagnostics['checked']) { return $safe; }
    // SSH can fail after nonempty increments; the two counts may overlap.
    foreach (array_keys($safe) as $key) { $safe[$key] = $diagnostics[$key]; }
    return $safe;
}

function probeFailureRecord(string $stage, mixed $stats, mixed $version, mixed $classification = null, mixed $coreCheckpoint = null, mixed $tableInventoryStats = null, mixed $tableInventoryClassification = null, mixed $decryptionDiagnostic = null): array {
    $allowedStages = ['arguments', 'network', 'bootstrap', 'environment', 'database', 'migrations', 'core_data', 'migration_prediction', 'decryption', 'wrong_key', 'ssh', 'http_health', 'reverb', 'login', 'version'];
    $category = in_array($stage, $allowedStages, true) ? 'probe_'.$stage : 'probe_failed';
    $record = ['ok' => false, 'category' => $category];
    if ($category === 'probe_decryption') {
        $record['decryption_diagnostics'] = decryptionDiagnostics($decryptionDiagnostic);
        return $record;
    }
    if ($category === 'probe_core_data') {
        $allowedCheckpoints = ['unavailable', 'model_inventory', 'model_source', 'model_class', 'model_construct', 'model_table', 'table_query', 'table_inventory', 'table_count', 'migration_count', 'core_table', 'primary_columns', 'uuid_column', 'row_query', 'row_identity', 'row_bound', 'core_digest'];
        $record['core_checkpoint'] = in_array($coreCheckpoint, $allowedCheckpoints, true) ? $coreCheckpoint : 'unavailable';
        if ($record['core_checkpoint'] === 'table_inventory') {
            $safe = ['query_ok' => false, 'db_count' => 0, 'nonstring_count' => 0, 'invalid_name_count' => 0, 'duplicate_count' => 0, 'unknown_count' => 0, 'missing_core_count' => 0, 'migration_table_present' => false];
            $valid = is_array($tableInventoryStats) && count($tableInventoryStats) === count($safe) && count(array_diff(array_keys($tableInventoryStats), array_keys($safe))) === 0;
            if ($valid) {
                $valid = is_bool($tableInventoryStats['query_ok']) && is_bool($tableInventoryStats['migration_table_present']);
                foreach (['db_count', 'nonstring_count', 'invalid_name_count', 'duplicate_count', 'unknown_count', 'missing_core_count'] as $key) {
                    $valid = $valid && is_int($tableInventoryStats[$key]) && $tableInventoryStats[$key] >= 0 && $tableInventoryStats[$key] <= 1000;
                }
                if ($valid) {
                    $validCount = $tableInventoryStats['db_count'] - $tableInventoryStats['nonstring_count'] - $tableInventoryStats['invalid_name_count'];
                    $valid = $validCount >= 0 && $tableInventoryStats['duplicate_count'] <= $validCount && $tableInventoryStats['unknown_count'] <= $validCount - $tableInventoryStats['duplicate_count'] && $tableInventoryStats['missing_core_count'] <= 59;
                    if ($tableInventoryStats['migration_table_present']) { $valid = $valid && $validCount - $tableInventoryStats['duplicate_count'] >= 1; }
                    if (!$tableInventoryStats['query_ok']) {
                        $valid = $valid && !$tableInventoryStats['migration_table_present'] && array_sum(array_intersect_key($tableInventoryStats, array_flip(['db_count', 'nonstring_count', 'invalid_name_count', 'duplicate_count', 'unknown_count', 'missing_core_count']))) === 0;
                    }
                }
            }
            // Copy the exact typed projection only; queried names and arbitrary values never escape.
            if ($valid) { foreach (array_keys($safe) as $key) { $safe[$key] = $tableInventoryStats[$key]; } }
            $record['table_inventory_diagnostics'] = $safe;
            $safeClassification = ['query_ok' => false, 'telescope_candidate_count' => 0, 'framework_candidate_count' => 0, 'passkey_candidate_count' => 0, 'unsupported_count' => 0, 'telescope_repository_present' => false];
            $validClassification = $valid && $safe['query_ok'] && is_array($tableInventoryClassification) && count($tableInventoryClassification) === count($safeClassification) && count(array_diff(array_keys($tableInventoryClassification), array_keys($safeClassification))) === 0;
            if ($validClassification) {
                $validClassification = $tableInventoryClassification['query_ok'] === true && is_bool($tableInventoryClassification['telescope_repository_present']);
                foreach (['telescope_candidate_count' => 3, 'framework_candidate_count' => 5, 'passkey_candidate_count' => 1, 'unsupported_count' => 1000] as $key => $bound) {
                    $validClassification = $validClassification && is_int($tableInventoryClassification[$key]) && $tableInventoryClassification[$key] >= 0 && $tableInventoryClassification[$key] <= $bound;
                }
                if ($validClassification) { $validClassification = $tableInventoryClassification['telescope_candidate_count'] + $tableInventoryClassification['framework_candidate_count'] + $tableInventoryClassification['passkey_candidate_count'] + $tableInventoryClassification['unsupported_count'] === $safe['unknown_count']; }
            }
            if ($validClassification) { foreach (array_keys($safeClassification) as $key) { $safeClassification[$key] = $tableInventoryClassification[$key]; } }
            $record['table_inventory_classification'] = $safeClassification;
        }
        return $record;
    }
    if ($category !== 'probe_migrations') { return $record; }
    $selected = in_array($version, ['4.4.2', '4.4.6'], true) ? $version : 'unavailable';
    $safe = ['query_ok' => false, 'db_count' => 0, 'nonstring_count' => 0, 'duplicate_count' => 0, 'unknown_count' => 0, 'pending_count' => 0, 'image_manifest_match' => false, 'selected_version' => $selected];
    $valid = is_array($stats) && count($stats) === count($safe) && count(array_diff(array_keys($stats), array_keys($safe))) === 0;
    if ($valid) {
        $valid = is_bool($stats['query_ok']) && is_bool($stats['image_manifest_match']) && $stats['selected_version'] === $selected && $selected !== 'unavailable';
        foreach (['db_count', 'nonstring_count', 'duplicate_count', 'unknown_count', 'pending_count'] as $key) {
            $valid = $valid && is_int($stats[$key]) && $stats[$key] >= 0 && $stats[$key] <= 10000;
        }
        if ($valid) {
            $valid = $stats['nonstring_count'] + $stats['duplicate_count'] <= $stats['db_count'] && $stats['unknown_count'] <= $stats['db_count'] - $stats['nonstring_count'];
            if (!$stats['query_ok']) { $valid = $valid && !$stats['image_manifest_match'] && array_sum(array_intersect_key($stats, array_flip(['db_count', 'nonstring_count', 'duplicate_count', 'unknown_count', 'pending_count']))) === 0; }
        }
    }
    // Copy only this fixed typed projection, never an arbitrary exception or queried row.
    if ($valid) { foreach (array_keys($safe) as $key) { $safe[$key] = $stats[$key]; } }
    $record['migration_diagnostics'] = $safe;
    $safeClassification = ['query_ok' => false, 'historical_candidate_count' => 0, 'target_candidate_count' => 0, 'unsupported_count' => 0, 'mainline_candidate_count' => 0];
    $validClassification = $valid && $safe['query_ok'] && is_array($classification) && count($classification) === count($safeClassification) && count(array_diff(array_keys($classification), array_keys($safeClassification))) === 0;
    if ($validClassification) {
        $validClassification = $classification['query_ok'] === true;
        foreach (['historical_candidate_count', 'target_candidate_count', 'unsupported_count', 'mainline_candidate_count'] as $key) { $validClassification = $validClassification && is_int($classification[$key]) && $classification[$key] >= 0 && $classification[$key] <= 10000; }
        if ($validClassification) { $validClassification = $classification['historical_candidate_count'] + $classification['target_candidate_count'] + $classification['unsupported_count'] === $safe['unknown_count'] && $classification['mainline_candidate_count'] <= $classification['historical_candidate_count']; }
    }
    if ($validClassification) { foreach (array_keys($safeClassification) as $key) { $safeClassification[$key] = $classification[$key]; } }
    $record['migration_classification'] = $safeClassification;
    return $record;
}

$stage = 'arguments';
$decryptionDiagnostic = null;
try {
    $mode = $argv[1] ?? '';
    gate(in_array($mode, ['baseline', 'verify', 'network'], true) && count($argv) === 2);
    $stage = 'network';
    $network = isolatedNetwork();
    gate($network);
    if ($mode === 'network') {
        while (ob_get_level() > 0) { ob_end_clean(); }
        echo json_encode(['ok' => true, 'category' => 'network_verified', 'network' => true], JSON_THROW_ON_ERROR)."\n";
        exit(0);
    }
    $stage = 'bootstrap';
    $hmacKey = readHmacKey('/probe/hmac.key');
    // Each phase has a fresh owned cache directory. Probe bypasses any image config cache.
    putenv('APP_CONFIG_CACHE=/tmp/rehearsal-uncached-config.php');
    require '/var/www/html/vendor/autoload.php';
    $app = require '/var/www/html/bootstrap/app.php';
    $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
    $stage = 'environment';
    gate(is_string(getenv('APP_KEY')) && hash_equals(getenv('APP_KEY'), (string) config('app.key')));
    gate(config('app.env') === 'production' && config('app.debug') === false);
    gate(config('queue.default') === 'redis' && config('database.default') === 'pgsql');
    $connection = config('database.connections.pgsql');
    gate($connection['host'] === '127.0.0.1' && (string)$connection['port'] === '5432' && $connection['database'] === 'coolify' && $connection['username'] === 'coolify' && $connection['password'] === 'rehearsal-only-password' && empty($connection['url']) && empty($connection['read']) && empty($connection['write']));
    foreach (['HORIZON_ENABLED', 'SCHEDULER_ENABLED', 'AUTOUPDATE'] as $flag) { gate(getenv($flag) === 'false'); }
    $stage = 'database';
    $dbResult = Illuminate\Support\Facades\DB::selectOne('SELECT 1 AS literal');
    gate((int)$dbResult->literal === 1);
    $version = (string)config('constants.coolify.version');
    gate(in_array($version, ['4.4.2', '4.4.6'], true));
    $manifestBytes = file_get_contents('/probe/public-manifest.json');
    gate(is_string($manifestBytes) && hash('sha256', $manifestBytes) === 'b37aff8b86a8ff20372ef5fb7e9bba40ba7b192f899b649dce0c9d0daa61e276');
    $manifest = json_decode($manifestBytes, true, 64, JSON_THROW_ON_ERROR);
    $stage = 'migrations';
    $migrationClassification = ['query_ok' => false, 'historical_candidate_count' => 0, 'target_candidate_count' => 0, 'unsupported_count' => 0, 'mainline_candidate_count' => 0];
    $migrationStats = migrationDiagnostics(null, null, $manifest['migrations'][$version], $version);
    $migrations = Illuminate\Support\Facades\DB::table('migrations')->orderBy('migration')->pluck('migration')->all();
    $known = array_map(fn (string $path): string => basename($path, '.php'), glob('/var/www/html/database/migrations/*.php') ?: []);
    sort($known); sort($migrations);
    $migrationStats = migrationDiagnostics($migrations, $known, $manifest['migrations'][$version], $version);
    $migrationClassification = migrationClassification($migrations, $manifest['migrations'][$version], $migrationStats);
    validateMigrationRepository($migrations, $known, $manifest['migrations'][$version], $version);
    $stage = 'core_data';
    $coreCheckpoint = 'unavailable';
    $tableInventoryStats = coreTableDiagnostics(null, null, null);
    $tableInventoryClassification = tableCandidateClassification(null, null, null, null, null, null, null);
    $runtimeTables = ['audit_events', 'docker_cleanup_executions', 'github_runner_executions', 'notification_throttles', 'scheduled_database_backup_executions', 'scheduled_job_deliveries', 'scheduled_job_states', 'scheduled_task_executions', 'scheduled_volume_backup_executions', 'user_changelog_reads', 'application_deployment_queues', 'jobs', 'failed_jobs', 'job_batches', 'cache', 'cache_locks', 'sessions', 'notifications', 'migrations', 'activity_log', 'telescope_entries', 'telescope_entries_tags', 'telescope_monitoring'];
    $coreTables = array_values(array_diff(array_unique(array_merge(array_values($manifest['model_tables']['4.4.2']), ['team_user', 'taggables'])), $runtimeTables)); sort($coreTables);
    $coreCheckpoint = 'model_inventory';
    $actualModels = array_map(fn (string $path): string => basename($path, '.php'), glob('/var/www/html/app/Models/*.php') ?: []); sort($actualModels);
    $expectedModels = array_keys($manifest['models'][$version]); sort($expectedModels); gate($actualModels === $expectedModels);
    $models = [];
    foreach ($manifest['models'][$version] as $name => $sourceHash) {
        $path = '/var/www/html/app/Models/'.$name.'.php';
        $coreCheckpoint = 'model_source';
        gate(realpath($path) === $path && hash_file('sha256', $path) === $sourceHash);
        if ($name === 'BaseModel') { continue; }
        $class = 'App\\Models\\'.$name;
        $coreCheckpoint = 'model_class';
        gate(class_exists($class) && !(new ReflectionClass($class))->isAbstract());
        $coreCheckpoint = 'model_construct';
        $model = new $class;
        $coreCheckpoint = 'model_table';
        gate($model->getTable() === $manifest['model_tables'][$version][$name]);
        $models[$name] = $class;
    }
    $coreCheckpoint = 'table_query';
    $actualTables = array_map(fn (object $row): mixed => $row->tablename, Illuminate\Support\Facades\DB::select("SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname='public' ORDER BY tablename"));
    $coreCheckpoint = 'table_inventory';
    $activityContext = activityLogContext();
    $tableInventoryStats = coreTableDiagnostics($actualTables, $manifest['tables'], $coreTables, $migrations, $version, $manifest['migrations'][$version], $activityContext);
    $tableInventoryClassification = tableCandidateClassification($actualTables, $manifest['tables'], $coreTables, $migrations, $tableInventoryStats, $version, $manifest['migrations'][$version], $activityContext);
    validateCoreTableInventory($actualTables, $manifest['tables'], $coreTables, $migrations, $version, $manifest['migrations'][$version], $activityContext);
    $tableCoverage = ['known' => count($actualTables), 'unknown' => 0, 'tables' => []];
    $coreCheckpoint = 'table_count';
    foreach ($actualTables as $table) { $tableCoverage['tables'][$table] = ['count' => Illuminate\Support\Facades\DB::table($table)->count(), 'classification' => in_array($table, $coreTables, true) ? 'core' : 'runtime']; }
    $coreCheckpoint = 'migration_count';
    gate($tableCoverage['tables']['migrations']['count'] === count($migrations));
    $core = []; $seeder = ['users' => [], 'teams' => [], 'servers' => [], 'private_keys' => []];
    foreach ($coreTables as $table) {
        $coreCheckpoint = 'core_table';
        gate(Illuminate\Support\Facades\Schema::hasTable($table));
        $coreCheckpoint = 'primary_columns';
        $primary = primaryColumns($table); $columns = $primary;
        $coreCheckpoint = 'uuid_column';
        $hasUuid = Illuminate\Support\Facades\Schema::hasColumn($table, 'uuid');
        if ($hasUuid) { $columns[] = 'uuid'; }
        $rows = [];
        $coreCheckpoint = 'row_query';
        $query = Illuminate\Support\Facades\DB::table($table)->select($columns);
        foreach ($primary as $column) { $query->orderBy($column); }
        $coreCheckpoint = 'row_query';
        foreach ($query->cursor() as $row) {
            $coreCheckpoint = 'row_identity';
            $tuple = array_map(fn (string $column): string => (string)$row->$column, $primary);
            $identity = [count($tuple) === 1 && $primary[0] === 'id' ? (int)$tuple[0] : $tuple, $hasUuid ? (string)$row->uuid : ''];
            $rows[] = $identity;
            if (array_key_exists($table, $seeder) && (int)$row->id <= 1) {
                $coreCheckpoint = 'core_digest';
                $seeder[$table][] = ['id' => (int)$row->id, 'uuid_digest' => digestRows([$identity], $hmacKey)];
            }
            $coreCheckpoint = 'row_bound';
            gate(count($rows) <= 1000000);
            $coreCheckpoint = 'row_query';
        }
        $coreCheckpoint = 'core_digest';
        $core[$table] = ['count' => count($rows), 'digest' => digestRows($rows, $hmacKey)];
    }
    // Match the frozen migrations: latest updated_at, then latest id survives.
    $stage = 'migration_prediction';
    $deleteIds = []; $survivors = [];
    $groups = [];
    $envRows = Illuminate\Support\Facades\DB::table('environment_variables')
        ->select(['id', 'uuid', 'resourceable_type', 'resourceable_id', 'key', 'is_preview', 'updated_at'])
        ->orderByDesc('updated_at')->orderByDesc('id')->get();
    gate($envRows->count() <= 1000000);
    foreach ($envRows as $row) {
        $redis = $row->resourceable_type === App\Models\StandaloneRedis::class && $row->key === 'REDIS_USERNAME';
        $node = $row->resourceable_type === App\Models\Application::class && $row->key === 'NIXPACKS_NODE_VERSION';
        if (!$redis && !$node) { continue; }
        $group = ($redis ? 'redis:' : 'node:').$row->resourceable_id.($node ? ':'.(int)$row->is_preview : '');
        if (isset($groups[$group])) { $deleteIds[(int)$row->id] = true; }
        else { $groups[$group] = true; $survivors[] = [(int)$row->id, (string)$row->uuid]; }
    }
    $expected = [];
    foreach ($envRows as $row) { if (!isset($deleteIds[(int)$row->id])) { $expected[] = [(int)$row->id, (string)$row->uuid]; } }
    $deletions = ['surplus' => count($deleteIds), 'survivor_digest' => digestRows($survivors, $hmacKey), 'expected_digest' => digestRows($expected, $hmacKey), 'expected_count' => count($expected)];
    $ipv6Affected = predictedIpv6Changes();
    $stage = 'decryption';
    $decrypt = ['checked' => 0, 'nonempty' => 0, 'missing' => 0, 'failures' => 0];
    $decryptionCheckpoint = 'unavailable'; $decryptionOrdinal = 0; $decryptionFirstFailure = null;
    $decryptModels = [];
    $negative = null; $sshChecked = 0; $sshGood = true;
    foreach ($models as $name => $class) {
        // The frozen, validated manifest foreach order is the sorted PUBLIC 68-model order.
        $decryptionOrdinal++;
        $decryptionCheckpoint = 'model_construct';
        $prototype = new $class;
        $decryptionCheckpoint = 'model_table';
        $table = $prototype->getTable();
        gate(Illuminate\Support\Facades\Schema::hasTable($table));
        $attributes = [];
        $decryptionCheckpoint = 'cast_inventory';
        foreach ($prototype->getCasts() as $attribute => $cast) {
            if (is_string($cast) && str_starts_with($cast, 'encrypted')) {
                $decryptionCheckpoint = 'cast_type';
                gate(in_array($cast, ['encrypted', 'encrypted:string', 'encrypted:array', 'encrypted:collection', 'encrypted:object'], true));
                $attributes[$attribute] = $cast;
            }
        }
        $declaredCasts = $manifest['encrypted_casts'][$version][$name];
        $expectedCasts = correctedEncryptedCasts($name, $declaredCasts, $manifest['models'][$version][$name], $version, $manifest['migrations'][$version]);
        $decryptionCheckpoint = 'cast_inventory';
        ksort($attributes); ksort($expectedCasts); gate($attributes === $expectedCasts);
        // Fortify uses Crypt::decrypt on these raw User fields, without model encrypted casts.
        if ($name === 'User') { $attributes['two_factor_secret'] = 'fortify:string'; $attributes['two_factor_recovery_codes'] = 'fortify:string'; }
        $presence = [];
        foreach ($attributes as $attribute => $cast) { $decryptionCheckpoint = 'encrypted_column'; $presence[$attribute] = Illuminate\Support\Facades\Schema::hasColumn($table, $attribute); }
        $attributes = persistedEncryptedAttributes($name, $declaredCasts, $manifest['models'][$version][$name], $version, $manifest['migrations'][$version], $presence);
        $decryptModels[$name] = ['checked' => 0, 'failures' => 0, 'columns' => count($attributes)];
        if (!$attributes) { continue; }
        $decryptionCheckpoint = 'row_query';
        $columns = array_unique(array_merge(primaryColumns($table), array_keys($attributes), $name === 'PrivateKey' ? ['uuid'] : []));
        // toBase avoids retrieved model events. Hydration below uses raw attributes, without jobs.
        $query = $prototype->newQuery()->withoutGlobalScopes()->select($columns)->toBase();
        foreach ($query->cursor() as $record) {
            $decryptionCheckpoint = 'model_construct';
            $model = (new $class)->setRawAttributes((array)$record, true);
            foreach ($attributes as $attribute => $cast) {
                $decryptionCheckpoint = 'raw_string';
                $raw = $model->getRawOriginal($attribute);
                if ($raw === null || $raw === '') { $decrypt['missing']++; continue; }
                $decrypt['checked']++;
                $decryptModels[$name]['checked']++;
                try {
                    gate(is_string($raw));
                    $decryptionCheckpoint = 'decrypt_value';
                    $plain = $cast === 'fortify:string' ? Laravel\Fortify\Fortify::currentEncrypter()->decrypt($raw) : $model->getAttribute($attribute);
                    $decryptionCheckpoint = 'plaintext_type';
                    gate(typedPlain($plain, $cast));
                    if ($name === 'User' && $attribute === 'two_factor_recovery_codes') {
                        $decryptionCheckpoint = 'fortify_json';
                        $codes = json_decode($plain, true, 64, JSON_THROW_ON_ERROR);
                        $decryptionCheckpoint = 'fortify_codes';
                        gate(is_array($codes) && array_is_list($codes) && count($codes) <= 100);
                        foreach ($codes as $code) { gate(is_string($code) && strlen($code) > 0 && strlen($code) <= 1024); }
                        unset($codes, $code);
                    }
                    if ((is_string($plain) && $plain !== '') || (is_array($plain) && count($plain) > 0) || is_object($plain)) { $decrypt['nonempty']++; }
                    if ($negative === null) { $negative = $raw; }
                    if ($class === App\Models\PrivateKey::class && $attribute === 'private_key' && $mode === 'verify') {
                        $decryptionCheckpoint = 'ssh_uuid';
                        $uuid = (string)$model->getRawOriginal('uuid');
                        gate(preg_match('/\A[a-zA-Z0-9_-]{1,128}\z/', $uuid) === 1);
                        $path = '/var/www/html/storage/app/ssh/keys/ssh_key@'.$uuid;
                        $decryptionCheckpoint = 'ssh_file';
                        $stored = protectedSshFile($path);
                        $sshChecked++;
                        $sshGood = $sshGood && is_string($stored) && hash_equals($plain, $stored);
                        unset($stored);
                    }
                    unset($plain);
                } catch (Throwable $failure) {
                    if ($decryptionFirstFailure === null) { $decryptionFirstFailure = ['checkpoint' => $decryptionCheckpoint, 'model_ordinal' => $decryptionOrdinal]; }
                    $decrypt['failures']++; $decryptModels[$name]['failures']++;
                }
            }
            $decryptionCheckpoint = 'row_query';
        }
    }
    $decryptionCheckpoint = 'summary';
    gate($decrypt['checked'] > 0 && $decrypt['nonempty'] > 0 && $decrypt['failures'] === 0 && is_string($negative));
    $stage = 'wrong_key';
    $wrongKey = false;
    try {
        $wrong = new Illuminate\Encryption\Encrypter(random_bytes(32), (string)config('app.cipher'));
        $wrong->decrypt($negative, false);
    } catch (Illuminate\Contracts\Encryption\DecryptException $expectedFailure) { $wrongKey = true; }
    gate($wrongKey);
    if ($mode === 'verify') {
        $stage = 'ssh';
        gate($sshGood && $sshChecked > 0);
        $stage = 'http_health';
        httpGate('/api/health', 8080, true);
        $stage = 'reverb';
        httpGate('/up', 6001);
        $stage = 'login';
        httpGate('/login', 8080);
    }
    $stage = 'version';
    $version = (string)config('constants.coolify.version');
    gate(in_array($version, ['4.4.2', '4.4.6'], true));
    $result = ['ok' => true, 'category' => 'verified', 'version' => $version, 'db' => true, 'no_pending' => true, 'network' => true, 'wrong_key' => $wrongKey, 'ssh' => $mode === 'verify' && $sshGood, 'decrypt' => $decrypt, 'decrypt_models' => $decryptModels, 'table_coverage' => $tableCoverage, 'ipv6_affected' => $ipv6Affected, 'migrations' => $migrations, 'core' => $core, 'deletions' => $deletions, 'seeder' => $seeder, 'ssh_checked' => $sshChecked];
    while (ob_get_level() > 0) { ob_end_clean(); }
    echo json_encode($result, JSON_THROW_ON_ERROR)."\n";
    exit(0);
} catch (Throwable $failure) {
    while (ob_get_level() > 0) { ob_end_clean(); }
    $decryptionDiagnostic = isset($decrypt) ? ['available' => true, 'checkpoint' => $decryptionFirstFailure['checkpoint'] ?? $decryptionCheckpoint ?? 'unavailable', 'model_ordinal' => $decryptionFirstFailure['model_ordinal'] ?? $decryptionOrdinal ?? 0, 'checked' => $decrypt['checked'], 'nonempty' => $decrypt['nonempty'], 'missing' => $decrypt['missing'], 'failures' => $decrypt['failures']] : null;
    echo json_encode(probeFailureRecord($stage, $migrationStats ?? null, $version ?? null, $migrationClassification ?? null, $coreCheckpoint ?? null, $tableInventoryStats ?? null, $tableInventoryClassification ?? null, $decryptionDiagnostic), JSON_THROW_ON_ERROR)."\n";
    exit(1);
}
