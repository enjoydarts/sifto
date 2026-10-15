<?php
// PUBLIC fixture only: all keys, ciphertext, database and failure injection synthetic.
declare(strict_types=1);
error_reporting(0);
ini_set('display_errors', '0');
ini_set('log_errors', '0');
ob_start();
function demand(bool $condition): void { if (!$condition) { throw new RuntimeException('fixture_failed'); } }
function emit(array $value, int $status = 0): never {
    while (ob_get_level()) { ob_end_clean(); }
    echo json_encode($value, JSON_THROW_ON_ERROR)."\n"; exit($status);
}
function metadata(string $path): array {
    clearstatcache(true, $path); $value = lstat($path); demand(is_array($value));
    return array_intersect_key($value, array_flip(['dev', 'ino', 'mode', 'nlink', 'uid', 'gid', 'size', 'mtime', 'ctime']));
}
function snapshot(string $root): array {
    $result = [];
    foreach (scandir($root) as $name) {
        if ($name === '.' || $name === '..') { continue; }
        $path = $root.'/'.$name; $stat = metadata($path);
        // Keep the 128MiB fixture cap even when checking an 8MiB inventory
        // alongside the child Laravel runtime. Hash bytes without retaining
        // a second complete copy; metadata remains compared field for field.
        $digest = null;
        if (($stat['mode'] & 0170000) === 0100000) {
            $digest = hash_file('sha256', $path); demand(is_string($digest));
        }
        $result[$name] = [$stat, $digest];
    }
    return $result;
}
function cleanKeys(): void {
    foreach (['keys', 'mux'] as $leaf) {
        $dir = '/var/www/html/storage/app/ssh/'.$leaf;
        foreach (scandir($dir) as $name) { if ($name !== '.' && $name !== '..') {
            $path = $dir.'/'.$name;
            demand(is_dir($path) && !is_link($path) ? rmdir($path) : unlink($path));
        } }
    }
    Illuminate\Support\Facades\DB::table('private_keys')->delete();
}
function row(int $id, string $uuid, mixed $plain, bool $cipher = false): void {
    Illuminate\Support\Facades\DB::table('private_keys')->insert([
        'id' => $id, 'uuid' => $uuid, 'name' => 'PUBLIC fixture', 'team_id' => 0,
        'private_key' => $cipher ? $plain : Illuminate\Support\Facades\Crypt::encryptString($plain),
        'created_at' => now(), 'updated_at' => now(),
    ]);
}
function put(string $path, string $bytes, int $mode = 0600): void {
    demand(file_put_contents($path, $bytes) === strlen($bytes)); demand(chmod($path, $mode));
}
function invoke(bool $partial = false, array $extra = []): array {
    if ($partial) {
        // Inherited OS failure only; the product has no testing flag or hook.
        $pipes = [];
        $child = proc_open(['php', '/startup/ssh-fixture.php', 'partial-child'],
            [0 => ['file', '/dev/null', 'r'], 1 => ['pipe', 'w'], 2 => ['pipe', 'w']], $pipes);
    } else {
        $child = proc_open(array_merge(['php', '/probe/startup.php', 'ssh-reconstruct'], $extra),
            [0 => ['file', '/dev/null', 'r'], 1 => ['pipe', 'w'], 2 => ['pipe', 'w']], $pipes);
    }
    demand(is_resource($child));
    $stdout = stream_get_contents($pipes[1]); $stderr = stream_get_contents($pipes[2]);
    fclose($pipes[1]); fclose($pipes[2]); $status = proc_close($child);
    demand($stderr === '' && is_string($stdout) && strlen($stdout) <= 8192);
    $decoded = json_decode($stdout, true, 16, JSON_THROW_ON_ERROR); demand(is_array($decoded));
    return [$status, $decoded];
}
function observationSchema(array $value): void {
    demand(array_keys($value) === ['version', 'operation', 'selected_version', 'rows_checked', 'keys_created',
        'keys_matched', 'unrelated_key_count', 'mux_entry_count', 'db_identity_hmac', 'existing_keys_hmac_before',
        'existing_keys_hmac_after', 'mux_hmac_before', 'mux_hmac_after', 'created_inventory_hmac']);
    demand($value['version'] === 1 && $value['operation'] === 'ssh-reconstruct'
        && $value['selected_version'] === config('constants.coolify.version'));
    foreach (['rows_checked', 'keys_created', 'keys_matched', 'unrelated_key_count', 'mux_entry_count'] as $field) {
        demand(is_int($value[$field]) && $value[$field] >= 0 && $value[$field] <= 1024);
    }
    demand($value['keys_created'] + $value['keys_matched'] === $value['rows_checked']
        && $value['rows_checked'] + $value['unrelated_key_count'] <= 1024);
    foreach ($value as $field => $item) {
        if (str_contains($field, 'hmac')) { demand(is_string($item) && preg_match('/\A[a-f0-9]{64}\z/', $item) === 1); }
    }
}
$stage = 'bootstrap';
try {
    demand($argc === 2);
    if ($argv[1] === 'partial-child') {
        demand(posix_setrlimit(POSIX_RLIMIT_FSIZE, 1024, 1024));
        demand(pcntl_signal(SIGXFSZ, SIG_IGN));
        $argc = 2; $argv = ['/probe/startup.php', 'ssh-reconstruct'];
        require '/probe/startup.php';
        exit(99);
    }
    require '/var/www/html/vendor/autoload.php';
    $app = require '/var/www/html/bootstrap/app.php';
    $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
    if ($argv[1] === 'setup') {
        demand(Illuminate\Support\Facades\Artisan::call('migrate', ['--force' => true]) === 0);
        foreach (['keys', 'mux'] as $leaf) { demand(mkdir('/var/www/html/storage/app/ssh/'.$leaf, 0700, true)); }
        emit(['ok' => true, 'operation' => 'public-ssh-setup']);
    }
    demand($argv[1] === 'matrix');
    put('/probe/hmac.key', str_repeat('P', 32));
    $keys = '/var/www/html/storage/app/ssh/keys'; $mux = '/var/www/html/storage/app/ssh/mux';
    $cases = ['missing', 'matching', 'unrelated', 'mux-socket', 'collision', 'symlink', 'hardlink',
        'mode', 'keys-directory-mode', 'mux-directory-mode', 'badcipher', 'null', 'empty', 'decrypted-empty',
        'bad-uuid', 'duplicate-uuid', 'late-invalid', 'destination-total-limit', 'invalid-inventory-encoding',
        'partialwrite', 'extra-argument'];
    foreach ($cases as $case) {
        $stage = $case; cleanKeys(); $socket = null;
        $plain = $case === 'partialwrite' ? str_repeat('PUBLIC KEY DATA\n', 300) : "PUBLIC SYNTHETIC KEY\n";
        row(1, $case === 'bad-uuid' ? '../PUBLIC' : 'PUBLIC_ONE', $plain);
        if ($case === 'matching') { put($keys.'/ssh_key@PUBLIC_ONE', $plain); }
        if ($case === 'collision') { put($keys.'/ssh_key@PUBLIC_ONE', 'DIFFERENT PUBLIC BYTES'); }
        if ($case === 'symlink') { put($keys.'/public-unrelated', 'PUBLIC'); demand(symlink($keys.'/public-unrelated', $keys.'/ssh_key@PUBLIC_ONE')); }
        if ($case === 'hardlink') { put($keys.'/public-unrelated', $plain); demand(link($keys.'/public-unrelated', $keys.'/ssh_key@PUBLIC_ONE')); }
        if ($case === 'mode') { put($keys.'/ssh_key@PUBLIC_ONE', $plain, 0644); }
        if ($case === 'keys-directory-mode') { demand(chmod($keys, 0755)); }
        if ($case === 'mux-directory-mode') { demand(chmod($mux, 0755)); }
        if ($case === 'decrypted-empty') {
            Illuminate\Support\Facades\DB::table('private_keys')->delete(); row(1, 'PUBLIC_ONE', '');
        }
        if (in_array($case, ['badcipher', 'null', 'empty', 'late-invalid'], true)) {
            if ($case !== 'late-invalid') { Illuminate\Support\Facades\DB::table('private_keys')->delete(); }
            // PUBLIC corrupt-row coverage only; the fixed schema normally
            // refuses NULL before the producer can inspect it.
            if ($case === 'null') { Illuminate\Support\Facades\DB::statement('ALTER TABLE private_keys ALTER COLUMN private_key DROP NOT NULL'); }
            row(2, 'PUBLIC_TWO', $case === 'null' ? null : ($case === 'empty' ? '' : 'PUBLIC BAD CIPHER'), true);
        }
        if ($case === 'duplicate-uuid') {
            demand(Illuminate\Support\Facades\DB::table('information_schema.table_constraints')
                ->where('table_name', 'private_keys')->where('constraint_name', 'private_keys_uuid_unique')
                ->where('constraint_type', 'UNIQUE')->exists());
            // Intentionally corrupt only this synthetic fixture table so the
            // producer's duplicate-row defense is exercised independently.
            Illuminate\Support\Facades\DB::statement('ALTER TABLE private_keys DROP CONSTRAINT private_keys_uuid_unique');
            row(2, 'PUBLIC_ONE', $plain);
        }
        if ($case === 'destination-total-limit') {
            for ($index = 0; $index < 128; $index++) { put($keys.'/public-unrelated-'.$index, str_repeat('P', 65536)); }
        }
        if ($case === 'invalid-inventory-encoding') { put($keys.'/public-'.chr(255), 'PUBLIC'); }
        if ($case === 'unrelated' || $case === 'partialwrite') { put($keys.'/public-unrelated', 'PUBLIC UNRELATED'); }
        put($mux.'/public-marker', 'PUBLIC MUX MARKER');
        if ($case === 'mux-socket') {
            $socket = stream_socket_server('unix://'.$mux.'/public-socket', $errno, $error);
            demand(is_resource($socket));
        }
        $beforeKeys = snapshot($keys); $beforeMux = snapshot($mux);
        $dbBefore = serialize(Illuminate\Support\Facades\DB::table('private_keys')->orderBy('id')->get()->all());
        [$status, $output] = invoke($case === 'partialwrite', $case === 'extra-argument' ? ['PUBLIC EXTRA'] : []);
        emitCase($case, $status, $output);
        demand(serialize(Illuminate\Support\Facades\DB::table('private_keys')->orderBy('id')->get()->all()) === $dbBefore);
        demand(snapshot($mux) === $beforeMux);
        if (in_array($case, ['missing', 'matching', 'unrelated', 'mux-socket'], true)) {
            observationSchema($output);
            demand($status === 0 && ($output['rows_checked'] ?? null) === 1
                && ($output['keys_matched'] ?? null) === ($case === 'matching' ? 1 : 0)
                && ($output['keys_created'] ?? null) === ($case === 'matching' ? 0 : 1));
            demand(file_get_contents($keys.'/ssh_key@PUBLIC_ONE') === $plain);
            demand((metadata($keys.'/ssh_key@PUBLIC_ONE')['mode'] & 07777) === 0600);
            foreach ($beforeKeys as $name => $value) { demand(snapshot($keys)[$name] === $value); }
            demand($output['existing_keys_hmac_before'] === $output['existing_keys_hmac_after']
                && $output['mux_hmac_before'] === $output['mux_hmac_after']);
            demand($output['unrelated_key_count'] === ($case === 'unrelated' ? 1 : 0)
                && $output['mux_entry_count'] === ($case === 'mux-socket' ? 2 : 1));
        } else {
            demand($status === 1 && array_keys($output) === ['version', 'operation', 'category']
                && $output['version'] === 1 && $output['operation'] === 'ssh-reconstruct'
                && in_array($output['category'], ['ssh_input_invalid', 'ssh_hmac_invalid', 'ssh_environment_invalid',
                    'ssh_database_invalid', 'ssh_destination_invalid', 'ssh_decrypt_failed', 'ssh_write_failed',
                    'ssh_write_open_failed', 'ssh_write_content_failed', 'ssh_write_sync_failed',
                    'ssh_write_identity_failed', 'ssh_write_inventory_failed'], true));
            if ($case === 'partialwrite') {
                demand(is_file($keys.'/ssh_key@PUBLIC_ONE') && filesize($keys.'/ssh_key@PUBLIC_ONE') === 1024);
                demand(snapshot($keys)['public-unrelated'] === $beforeKeys['public-unrelated']);
            } else { demand(snapshot($keys) === $beforeKeys); }
        }
        if ($case === 'keys-directory-mode') { demand(chmod($keys, 0700)); }
        if ($case === 'mux-directory-mode') { demand(chmod($mux, 0700)); }
        if ($case === 'null') {
            Illuminate\Support\Facades\DB::table('private_keys')->delete();
            Illuminate\Support\Facades\DB::statement('ALTER TABLE private_keys ALTER COLUMN private_key SET NOT NULL');
        }
        if ($case === 'duplicate-uuid') {
            Illuminate\Support\Facades\DB::table('private_keys')->delete();
            Illuminate\Support\Facades\DB::statement('ALTER TABLE private_keys ADD CONSTRAINT private_keys_uuid_unique UNIQUE (uuid)');
        }
        if (is_resource($socket)) { fclose($socket); }
    }
    cleanKeys(); emit(['ok' => true, 'operation' => 'public-ssh-matrix', 'cases' => count($cases)]);
} catch (Throwable $failure) { emit(['ok' => false, 'category' => 'public_ssh_fixture_failed', 'stage' => $stage], 1); }
function emitCase(string $case, int $status, array $output): void {
    // Output is explicitly public, but keep the same product privacy boundary.
    $record = ['case' => $case, 'status' => $status];
    if ($status === 0) { $record['observation'] = $output; } else { $record['category'] = $output['category']; }
    $line = json_encode($record, JSON_THROW_ON_ERROR)."\n";
    while (ob_get_level()) { ob_end_clean(); }
    echo $line; ob_start();
}
