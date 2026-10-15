<?php
// PUBLIC fixture only. Finite clone operations; never native startup authority.
declare(strict_types=1);
error_reporting(0);
ini_set('display_errors', '0');
ini_set('log_errors', '0');
ob_start();

function check(bool $value): void {
    if (!$value) { throw new RuntimeException('fixture_gate'); }
}

function result(array $value, int $status = 0): never {
    while (ob_get_level() > 0) { ob_end_clean(); }
    echo json_encode($value, JSON_THROW_ON_ERROR)."\n";
    exit($status);
}

function inventory(): string {
    $rows = [];
    foreach (['/var/www/html/storage/app/ssh', '/data/coolify/ssl', '/data/coolify/proxy'] as $root) {
        check(is_dir($root) && !is_link($root));
        $iterator = new RecursiveIteratorIterator(new RecursiveDirectoryIterator($root,
            FilesystemIterator::SKIP_DOTS), RecursiveIteratorIterator::SELF_FIRST);
        foreach ($iterator as $entry) {
            check(count($rows) < 32 && !$entry->isLink());
            $info = lstat($entry->getPathname());
            check(is_array($info) && ($entry->isDir() || $entry->isFile() && $info['nlink'] === 1 && $info['size'] <= 65536));
            $rows[] = [$entry->getPathname(), $info['mode'], $info['uid'], $info['gid'],
                $entry->isDir() ? null : hash_file('sha256', $entry->getPathname())];
        }
    }
    foreach (['/var/www/html/.env', '/etc/s6-overlay/s6-rc.d/init-script/up',
        '/var/www/html/app/Http/Middleware/PreventRequestsDuringMaintenance.php',
        '/etc/nginx/nginx.conf', '/etc/nginx/conf.d/default.conf'] as $path) {
        check(is_file($path) && !is_link($path));
        $rows[] = [$path, hash_file('sha256', $path)];
    }
    sort($rows);
    return hash('sha256', json_encode($rows, JSON_THROW_ON_ERROR));
}

function queueInventory(): string {
    $redis = Illuminate\Support\Facades\Redis::connection();
    return hash('sha256', serialize([
        $redis->lrange('queues:public-startup', 0, -1),
        $redis->zrange('queues:public-startup:delayed', 0, -1),
        $redis->zscore('queues:public-startup:delayed', 'PUBLIC DELAYED FIXTURE'),
        $redis->zrange('queues:public-startup:reserved', 0, -1),
        $redis->zscore('queues:public-startup:reserved', 'PUBLIC RESERVED FIXTURE'),
    ]));
}

$stage = 'action';
try {
    check($argc === 2 && in_array($argv[1], ['source', 'maintenance-guard', 'setup', 'hold', 'ready', 'observe'], true));
    if ($argv[1] === 'ready') {
        $stage = 'http_starting';
        $context = stream_context_create(['http' => ['ignore_errors' => true, 'timeout' => 1]]);
        check(is_string(@file_get_contents('http://127.0.0.1:8080/api/health', false, $context)));
        check(isset($http_response_header[0]) && preg_match('/^HTTP\/1\.[01] 200 /', $http_response_header[0]) === 1);
        result(['ok' => true, 'category' => 'http_ready']);
    }
    if ($argv[1] === 'source') {
        $stage = 'image_source';
        $expected = [
            '/etc/s6-overlay/s6-rc.d/init-script/up' => 'd5c0542b30ffe6581edd5a434cac53bac9aa70de0e628986fb90be1cb4574bbe',
            '/var/www/html/app/Http/Middleware/PreventRequestsDuringMaintenance.php' => '4a4445b3e4ac17a03594e5b84e0b8ff9d24bead5790162e44e4d5832297aa994',
            '/etc/s6-overlay/s6-rc.d/horizon/run' => '296dfda050b4d25c32fd9a858216a9a492148fd6ef9960644e08f32f7caf5e31',
            '/etc/s6-overlay/s6-rc.d/scheduler-worker/run' => '9034571c9f328e694720f9c66c461016e96a3d932ade29f5d2498754677d906c',
            '/etc/nginx/nginx.conf.template' => '1e1d6561af14f68b0146232cc7ee069abe68658bd62b29d5a9c8f35111402295',
            '/etc/nginx/site-opts.d/http.conf' => '5beded419f31b70c52063ecaf7820c57ad37babbfa5b1d5cb861c4ffe4478b34',
            '/usr/local/etc/php-fpm.conf' => 'b6b8add8db04ec8217b1d3704da775a6836d8ac8a596f58a5e180f9784fc5a32',
        ];
        foreach ($expected as $path => $digest) {
            check(realpath($path) === $path && hash_file('sha256', $path) === $digest);
        }
        check(!file_exists('/etc/nginx/nginx.conf') && readlink('/var/run') === '../run');
        result(['ok' => true, 'category' => 'fixed_image_source', 'files' => count($expected)]);
    }
    require '/var/www/html/vendor/autoload.php';
    if ($argv[1] === 'maintenance-guard') {
        $stage = 'stock_webhook_exception';
        $reflection = new ReflectionClass(App\Http\Middleware\PreventRequestsDuringMaintenance::class);
        $middleware = $reflection->newInstanceWithoutConstructor();
        $method = $reflection->getMethod('inExceptArray');
        check(!$method->invoke($middleware, Illuminate\Http\Request::create('/webhooks/public-startup')));
        check($method->invoke($middleware, Illuminate\Http\Request::create('/api/health')));
        result(['ok' => true, 'category' => 'health_only_exception']);
    }
    $stage = 'bootstrap';
    $app = require '/var/www/html/bootstrap/app.php';
    $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
    $redis = Illuminate\Support\Facades\Redis::connection();
    if ($argv[1] === 'setup') {
        $stage = 'synthetic_schema';
        check(Illuminate\Support\Facades\Artisan::call('migrate', ['--force' => true]) === 0);
        $stage = 'synthetic_settings';
        check(!Illuminate\Support\Facades\DB::table('instance_settings')->exists());
        Illuminate\Support\Facades\DB::table('instance_settings')->insert(['id' => 0, 'is_auto_update_enabled' => false]);
        $stage = 'synthetic_inventory';
        check(mkdir('/var/www/html/storage/app/ssh/keys', 0700, true));
        check(mkdir('/var/www/html/storage/app/ssh/mux', 0700, true));
        foreach (['keys/unrelated-public-fixture', 'mux/public-fixture-marker'] as $name) {
            $path = '/var/www/html/storage/app/ssh/'.$name;
            check(file_put_contents($path, "PUBLIC INERT FIXTURE\n", LOCK_EX) === 21);
            check(chmod($path, 0600));
        }
        $redis->set('public-startup-no-init', 'unchanged');
        $redis->rpush('queues:public-startup', 'PUBLIC PENDING FIXTURE');
        $redis->zadd('queues:public-startup:delayed', 2000000000, 'PUBLIC DELAYED FIXTURE');
        $redis->zadd('queues:public-startup:reserved', 2000000000, 'PUBLIC RESERVED FIXTURE');
        $redis->set('public-startup-inventory', inventory());
        $redis->set('public-startup-queue', queueInventory());
        $migrations = Illuminate\Support\Facades\DB::table('migrations')->orderBy('migration')->pluck('migration')->all();
        check(count($migrations) === 435);
        $redis->set('public-startup-old-migrations', json_encode($migrations, JSON_THROW_ON_ERROR));
        result(['ok' => true, 'category' => 'synthetic_setup', 'migrations' => 435]);
    }
    $stage = 'barrier';
    $manager = $app->make(Illuminate\Contracts\Foundation\MaintenanceMode::class);
    if ($argv[1] === 'hold') {
        check(!$manager->active());
        $manager->activate(['except' => [], 'redirect' => null, 'retry' => null,
            'refresh' => null, 'secret' => null, 'status' => 503, 'template' => null]);
        $redis->set('public-startup-barrier', hash('sha256', serialize($manager->data())));
        result(['ok' => true, 'category' => 'synthetic_barrier_held']);
    }
    check($manager->active());
    $data = $manager->data();
    check(($data['status'] ?? null) === 503 && array_key_exists('secret', $data) && $data['secret'] === null
        && array_key_exists('template', $data) && $data['template'] === null
        && array_key_exists('redirect', $data) && $data['redirect'] === null);
    check(hash('sha256', serialize($data)) === $redis->get('public-startup-barrier'));
    check(Illuminate\Support\Facades\Redis::connection('cache')->ttl(
        config('cache.prefix').'illuminate:foundation:down') === -1);
    $stage = 'unchanged_inventory';
    check(inventory() === $redis->get('public-startup-inventory'));
    $stage = 'unchanged_queue_fixture';
    check(queueInventory() === $redis->get('public-startup-queue'));
    $stage = 'init_sentinel';
    check($redis->get('public-startup-no-init') === 'unchanged');
    $stage = 'autoupdate';
    check(Illuminate\Support\Facades\DB::table('instance_settings')->where('id', 0)->value('is_auto_update_enabled') === false);
    $stage = 'literal_controls';
    $env = file_get_contents('/var/www/html/.env');
    foreach (['HORIZON_ENABLED', 'SCHEDULER_ENABLED', 'MIGRATION_ENABLED', 'SEEDER_ENABLED', 'AUTOUPDATE', 'AUTORUN_ENABLED', 'APP_DEBUG'] as $name) {
        check(preg_match_all('/^'.preg_quote($name, '/').'=(.*)$/m', $env, $matches) === 1);
        check($matches[1] === ['false'] && getenv($name) === 'false');
    }
    check(getenv('DISABLE_DEFAULT_CONFIG') === 'true' && getenv('SSL_MODE') === 'off');
    $stage = 'runtime_isolation';
    check(posix_geteuid() === 9999 && posix_getegid() === 9999);
    $status = file_get_contents('/proc/self/status');
    check(preg_match('/^CapEff:\s+0+$/m', $status) === 1 && preg_match('/^NoNewPrivs:\s+1$/m', $status) === 1);
    check(!file_exists('/var/run/docker.sock') && !file_exists('/run/docker.sock'));
    $options = [];
    $readonlyPaths = ['/', '/var/www/html/.env', '/etc/s6-overlay/s6-rc.d/init-script/up',
        '/var/www/html/app/Http/Middleware/PreventRequestsDuringMaintenance.php',
        '/etc/nginx/nginx.conf', '/etc/nginx/conf.d/default.conf',
        '/data/coolify/ssl/public-fixture.txt', '/data/coolify/proxy/public-fixture.txt'];
    foreach (file('/proc/mounts', FILE_IGNORE_NEW_LINES) as $mount) {
        $parts = explode(' ', $mount);
        if (in_array($parts[1], array_merge($readonlyPaths, ['/run', '/tmp']), true)) {
            $options[$parts[1]] = explode(',', $parts[3]);
        }
    }
    foreach ($readonlyPaths as $path) { check(in_array('ro', $options[$path], true)); }
    foreach (['/run', '/tmp'] as $path) {
        check(in_array('nosuid', $options[$path], true) && in_array('nodev', $options[$path], true));
    }
    check(!in_array('noexec', $options['/run'], true) && in_array('noexec', $options['/tmp'], true));
    $stage = 'migration_repository';
    $version = config('constants.coolify.version');
    check(in_array($version, ['4.4.2', '4.4.6'], true));
    $files = array_map(fn ($path) => basename($path, '.php'), glob('/var/www/html/database/migrations/*.php'));
    $repository = Illuminate\Support\Facades\DB::table('migrations')->orderBy('migration')->pluck('migration')->all();
    sort($files); sort($repository);
    check($files === $repository && count($repository) === ($version === '4.4.2' ? 435 : 438));
    $before = json_decode($redis->get('public-startup-old-migrations'), true, 512, JSON_THROW_ON_ERROR);
    $expectedDelta = $version === '4.4.2' ? [] : [
        '2026_10_09_092903_normalize_ipv6_server_ips',
        '2026_10_09_115444_remove_duplicate_redis_username_environment_variables',
        '2026_10_09_124641_remove_duplicate_nixpacks_node_version_environment_variables',
    ];
    check(array_values(array_diff($repository, $before)) === $expectedDelta && array_diff($before, $repository) === []);
    $stage = 'http';
    $responses = [];
    foreach (['/api/health', '/', '/webhooks/public-startup', '/livewire/update', '/terminal'] as $path) {
        $stream = stream_context_create(['http' => ['method' => $path === '/api/health' ? 'GET' : 'POST',
            'ignore_errors' => true, 'timeout' => 3, 'header' => 'Cookie: laravel_maintenance=old-invalid-cookie']]);
        $body = @file_get_contents('http://127.0.0.1:8080'.$path, false, $stream);
        check(is_string($body) && isset($http_response_header[0]) &&
            preg_match('/^HTTP\/1\.[01] ([0-9]{3}) /', $http_response_header[0], $match) === 1);
        $responses[$path] = (int) $match[1];
        check((int) $match[1] === ($path === '/api/health' ? 200 : 503));
    }
    result(['ok' => true, 'category' => 'functional_held', 'version' => $version,
        'maintenance' => true, 'barrier_ttl' => -1, 'inventory_unchanged' => true,
        'queue_fixture_unchanged' => true, 'no_init_or_seeder' => true, 'autoupdate_disabled' => true,
        'read_only' => true, 'caps_empty' => true, 'no_docker_socket' => true,
        'migrations' => count($repository), 'http' => $responses,
        'native_startup_authorized' => false]);
} catch (Throwable $error) {
    result(['ok' => false, 'category' => 'startup_fixture_failed', 'stage' => $stage], 1);
}
