<?php
// Fixed PSR-4 receiver. Clone-only bounded inspection and parent-owned graceful drain.
declare(strict_types=1);
namespace App\SiftoMaintenance;

use Laravel\Horizon\Contracts\MasterSupervisorRepository;
use Laravel\Horizon\Contracts\SupervisorRepository;
use Laravel\Horizon\Supervisor;
use RuntimeException;
use Throwable;

final class InspectSupervisor
{
    private const ROOT = '/var/www/html/';
    private const LEAF = '/var/www/html/storage/app/startup-horizon-inspect.state';
    private const LOCK = '/var/www/html/storage/app/startup-horizon-inspect.lock';
    private const DRAIN = '/var/www/html/storage/app/startup-horizon-drain.state';
    private const DRAIN_LOCK = '/var/www/html/storage/app/startup-horizon-drain.lock';
    private const CLASS_FILE = '/var/www/html/app/SiftoMaintenance/InspectSupervisor.php';
    private const SOURCES = [
        'composer.json' => '84acb6071121ac9a8843f960c500ae3ed469287fdb9f4b7164975d37ae979593',
        'vendor/composer/ClassLoader.php' => 'd4b2313e562edf942838ec316d983c1a3459ca67da3e5658e861200b0b3693c2',
        'vendor/composer/autoload_real.php' => 'ad097449e5603cb3204b9e3d63e8bbf510984b4559196386f22d886190c1a06b',
        'vendor/composer/autoload_psr4.php' => '391ba6f01907760bb4564cfb1dc33b42d984c359f8589183639bb4475d3fb5fa',
        'vendor/symfony/process/Process.php' => '73c4e31230f9602768b73cda3451ed7ec0dddc20ddef6cc0d6795ea879600c39',
        'vendor/laravel/horizon/src/WorkerProcess.php' => 'b03707bb4eb455ce8b01b3dc744820d1bfd843ca86b481be94aa081b02299abc',
        'vendor/laravel/horizon/src/SupervisorProcess.php' => '864b7b1677d0749ddadee63278cec4ae87232461dc032fc501f38c42ee1089c8',
        'vendor/laravel/framework/src/Illuminate/Queue/Worker.php' => '2fe2c748bf807b89d779856f106e41156fdcee90706a9c3fc8fe1a1190d41615',
        'vendor/laravel/horizon/src/Events/MasterSupervisorLooped.php' => '15b66a2238154eb8dd12982bf6f83c3d561347a991e4c9bcc069d4c3121c705e',
        'vendor/laravel/horizon/src/Events/SupervisorLooped.php' => '92a53968bf4a14636035c510f5738e48692f35de7f534c0ea6b509576d923af3',
        'vendor/laravel/horizon/src/Supervisor.php' => 'c35978f84178552423d6ca0b0573743ebe81d81c1c9fd3df0f214b501c269130',
        'vendor/laravel/horizon/src/ProcessPool.php' => 'c21a1ad2c1be9bc4164c4693e5c0ea25f416c6d1a609382dd53a63bd0637b001',
        'vendor/laravel/horizon/src/RedisHorizonCommandQueue.php' => '11d13043fafc4b95314b890e1d68f02d2f601b72c9287ef119ef812e664345b7',
        'vendor/laravel/horizon/src/Repositories/RedisSupervisorRepository.php' => '406cc977ca4d2abff014303d4b3750d9db0bbbfe4078b09e341478e7fec04e75',
        'vendor/laravel/horizon/src/Repositories/RedisMasterSupervisorRepository.php' => '102da315c3d218e50cb2bf48d7d7883701b61112d6e17047b198cade29a114d8',
        'vendor/laravel/horizon/src/MasterSupervisor.php' => '78f1fc7d45074ec3a66d011a5e253f893517161b20597f51ee773b2fa2da1010',
        'vendor/laravel/horizon/src/Console/SupervisorCommand.php' => '90c9896b514c8083c5f6a6cda6700801bef6321bf2afd04ec9036619ae24e251',
        'vendor/laravel/horizon/src/WorkerCommandString.php' => '8dc0451e03bab8d9fb7ad081db8c1ba3314a1a0c912b8354cfcafce891ccb32b',
        'vendor/laravel/horizon/src/SupervisorCommandString.php' => '60eb235637d78eb3929d220a74307f907b617babb9489563cb2e25ac657ae3ba',
        'vendor/laravel/horizon/src/QueueCommandString.php' => 'bd6ab668da8e9d96377e6109bc3cb8aa2735e2c4cb0b5289a5233edbb913735f',
        'vendor/laravel/horizon/src/ListensForSignals.php' => 'bf1cc43eab3442d557cbbd5ee707ac49986c2ce5038b18177c26e333820f46f8',
        'vendor/laravel/horizon/src/EventMap.php' => 'f236862dfbc665fc094aabc6e77508771b11ddb463dd4138f2130f15ea97fbfb',
        'vendor/laravel/horizon/src/Listeners/PruneTerminatingProcesses.php' => 'e741df8575bba656945f1b548226a0570247b0bfbdab2cdaf02bec3bf5488523',
        'vendor/laravel/horizon/src/Events/WorkerProcessRestarting.php' => '0d2f5ddc49267590d930181ce286f506e176e7a65ca02eed8e252a19fd1d9648',
        'vendor/laravel/horizon/src/Console/WorkCommand.php' => '3a7246e7c21a864c9eca6ae6275ca9b94bb9a010e79d8880600d544531837545',
        'vendor/laravel/horizon/src/SupervisorOptions.php' => 'b8b82415e0fbdecbe2f42f3dbe04a7410a95bec28babc1609f302017c87a1d3c',
        'vendor/laravel/framework/src/Illuminate/Queue/Console/WorkCommand.php' => 'ab7f19f45217a7f11d6cdbf07535411bace3396d29d937be68adb7a6ce811448',
        'vendor/laravel/framework/src/Illuminate/Queue/Events/WorkerStarting.php' => 'f35ac009c411b2c2f4254d2ca42953f2c9fd5cdc69de7f0cfacf0f5750c3f46d',
        'vendor/laravel/framework/src/Illuminate/Queue/WorkerOptions.php' => 'a296590b2df88890af4619d9ca046d7e159d91d90b1ef1176d7634cd7de140c1',
        'vendor/symfony/console/Input/ArrayInput.php' => '7bc4602b0bc7ff1bece6e1ab263573393d5322f515243a4a7d1fa16c1eed9a8f',
    ];

    private static function gate(bool $value): void
    {
        if (!$value) { throw new RuntimeException('horizon_inspection_held'); }
    }

    private static function json(array $value): string
    {
        return json_encode($value, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES);
    }

    private static function mac(string $domain, array $value, string $key): string
    {
        return hash_hmac('sha256', $domain."\0".self::json($value), $key);
    }

    private static function parents(string $path): array
    {
        $rows = [];
        for ($parent = dirname($path); ; $parent = dirname($parent)) {
            clearstatcache(true, $parent); $s = lstat($parent);
            self::gate(is_array($s) && ($s['mode'] & 0170000) === 0040000 && realpath($parent) === $parent);
            $rows[] = [$s['dev'], $s['ino'], $s['mode'], $s['uid'], $s['gid']];
            if ($parent === '/') { return $rows; }
        }
    }

    private static function identity(array $info): array
    {
        return array_intersect_key($info, array_flip(['dev', 'ino', 'mode', 'nlink', 'uid', 'gid', 'size', 'mtime', 'ctime']));
    }

    private static function read(string $path, int $limit, bool $private = true): string
    {
        $parents = self::parents($path); clearstatcache(true, $path); $s = lstat($path);
        self::gate(is_array($s) && ($s['mode'] & 0170000) === 0100000 && $s['nlink'] === 1
            && $s['size'] >= 0 && $s['size'] <= $limit
            && (!$private || ($s['mode'] & 07777) === 0600 && $s['uid'] === 9999 && $s['gid'] === 9999));
        $stream = fopen($path, 'rb'); self::gate(is_resource($stream));
        try {
            self::gate(self::identity(fstat($stream)) === self::identity($s));
            $bytes = stream_get_contents($stream, $limit + 1);
            self::gate(is_string($bytes) && strlen($bytes) === $s['size'] && self::identity(fstat($stream)) === self::identity($s));
            clearstatcache(true, $path);
            self::gate(self::identity(lstat($path)) === self::identity($s) && self::parents($path) === $parents);
            return $bytes;
        } finally { fclose($stream); }
    }

    private static function key(): string
    {
        self::gate(PHP_SAPI === 'cli' && posix_geteuid() === 9999 && posix_getegid() === 9999
            && realpath(__FILE__) === self::CLASS_FILE);
        $key = self::read('/probe/hmac.key', 32); self::gate(strlen($key) === 32); return $key;
    }

    private static function proc(int $pid): array
    {
        self::gate($pid > 1 && $pid <= 4194304);
        $root = '/proc/'.$pid.'/';
        $stat = file_get_contents($root.'stat', false, null, 0, 4097);
        self::gate(is_string($stat) && strlen($stat) <= 4096 && str_starts_with($stat, $pid.' ('));
        $parts = explode(' ', substr($stat, strrpos($stat, ') ') + 2));
        self::gate(count($parts) >= 22 && !in_array($parts[0], ['Z', 'X'], true));
        $argv = file_get_contents($root.'cmdline', false, null, 0, 16385);
        $group = file_get_contents($root.'cgroup', false, null, 0, 8193);
        $status = file_get_contents($root.'status', false, null, 0, 8193);
        self::gate(is_string($argv) && strlen($argv) <= 16384 && is_string($group) && strlen($group) <= 8192
            && is_string($status) && strlen($status) <= 8192
            && preg_match('/^Uid:\s+9999\s+9999\s+9999\s+9999$/m', $status) === 1
            && readlink($root.'exe') === '/usr/local/bin/php' && readlink($root.'cwd') === '/var/www/html');
        $again = file_get_contents($root.'stat', false, null, 0, 4097);
        $again = explode(' ', substr($again, strrpos($again, ') ') + 2));
        self::gate($again[19] === $parts[19] && $again[1] === $parts[1]);
        return ['pid' => $pid, 'parent' => (int)$parts[1], 'start' => $parts[19], 'argv' => $argv, 'cgroup' => $group];
    }

    private static function context(string $key): array
    {
        self::gate(in_array(config('constants.coolify.version'), ['4.4.2', '4.4.6'], true)
            && config('app.env') === 'production' && config('app.debug') === false
            && config('horizon.fast_termination') === false && getenv('HORIZON_ENABLED') === 'true'
            && config('constants.horizon.is_horizon_enabled') === true);
        foreach (['MIGRATION_ENABLED', 'SEEDER_ENABLED', 'AUTOUPDATE', 'SCHEDULER_ENABLED'] as $flag) {
            self::gate(getenv($flag) === 'false');
        }
        $env = self::read(self::ROOT.'.env', 65536, false);
        self::gate(substr_count($env, "\nHORIZON_ENABLED=true\n") === 1);
        $config = [config('horizon'), config('database.redis'), config('cache'), hash('sha256', $env)];
        self::gate(strlen(self::json($config)) <= 65536);
        foreach (config('horizon.environments.production') as $options) { self::gate(($options['force'] ?? false) === false); }
        $sources = self::SOURCES;
        $sources['vendor/composer/autoload_static.php'] = config('constants.coolify.version') === '4.4.2'
            ? 'bcfece87ee9e9083d8c349a06af61ef6c1da3d06967de68a1a8b6043640584af'
            : '73b83c07b71136e2fcf61f0eb9579717ce2450cf76dee0405b3e61bc18a5c61b';
        foreach ($sources as $file => $hash) {
            self::gate(realpath(self::ROOT.$file) === self::ROOT.$file && hash_file('sha256', self::ROOT.$file) === $hash);
        }
        $sources['helper'] = hash('sha256', self::read(self::CLASS_FILE, 131072));
        $sources['startup'] = hash('sha256', self::read('/probe/startup.php', 131072, false));
        self::gate(config('app.maintenance.driver') === 'cache' && config('app.maintenance.store') === 'redis'
            && config('database.redis.client') === 'phpredis' && config('cache.stores.redis.connection') === 'cache');
        $store = app('cache')->store('redis')->getStore(); $client = $store->connection()->client();
        self::gate(get_class($store) === \Illuminate\Cache\RedisStore::class && get_class($client) === \Redis::class
            && $client->getDBNum() === 1 && $client->getOption(\Redis::OPT_SERIALIZER) === \Redis::SERIALIZER_NONE
            && $client->getOption(\Redis::OPT_COMPRESSION) === \Redis::COMPRESSION_NONE
            && $client->getOption(\Redis::OPT_PREFIX) === config('database.redis.options.prefix')
            && $store->getPrefix() === config('cache.prefix'));
        // Never unserialize unknown cache bytes. Only this exact plain barrier is accepted.
        $logical = $store->getPrefix().'illuminate:foundation:down';
        $plain = serialize(['except' => ['/api/health'], 'redirect' => null, 'retry' => null,
            'refresh' => null, 'secret' => null, 'status' => 503, 'template' => null]);
        self::gate($client->type($logical) === \Redis::REDIS_STRING && $client->strlen($logical) <= 65536
            && $client->get($logical) === $plain && $client->pttl($logical) === -1);
        foreach (['maintenance.php', 'down'] as $file) {
            self::parents(self::ROOT.'storage/framework/'.$file);
            self::gate(@lstat(self::ROOT.'storage/framework/'.$file) === false);
        }
        $server = $client->info('server');
        self::gate(is_string($server['run_id'] ?? null) && preg_match('/\A[a-f0-9]{40}\z/', $server['run_id']) === 1);
        return ['version' => config('constants.coolify.version'), 'source' => self::mac('inspect-source', $sources, $key),
            'config' => self::mac('inspect-config', $config, $key),
            'barrier' => self::mac('inspect-barrier', [$server['run_id'], $plain, $logical, $client->getDBNum()], $key),
            'boot' => trim(file_get_contents('/proc/sys/kernel/random/boot_id', false, null, 0, 65))];
    }

    private static function identities(string $key): array
    {
        $redis = app('redis')->connection('horizon');
        self::gate(get_class($redis) === \Illuminate\Redis\Connections\PhpRedisConnection::class);
        $names = $redis->zrange('supervisors', 0, 32); self::gate(count($names) === 1);
        $name = $names[0]; self::gate(is_string($name) && preg_match('/\A[a-zA-Z0-9_.:-]{1,192}\z/', $name) === 1);
        $row = app(SupervisorRepository::class)->find($name);
        self::gate($row !== null && in_array($row->status, ['running', 'paused'], true)
            && strlen(self::json((array)$row)) <= 32768 && $row->name === $name
            && (float)$redis->zscore('supervisors', $name) >= time() - 29);
        $masters = $redis->zrange('masters', 0, 32); self::gate($masters === [$row->master]);
        $master = app(MasterSupervisorRepository::class)->find($row->master);
        self::gate($master !== null && $master->name === $row->master && $master->environment === 'production'
            && in_array($master->status, ['running', 'paused'], true) && $master->supervisors === [$name]
            && (float)$redis->zscore('masters', $master->name) >= time() - 14
            && ($row->options['force'] ?? null) === false);
        $parent = self::proc((int)$master->pid); $own = self::proc((int)$row->pid);
        self::gate($own['parent'] === $parent['pid'] && $own['cgroup'] === $parent['cgroup']
            && $own['cgroup'] === file_get_contents('/proc/self/cgroup', false, null, 0, 8193)
            && $parent['argv'] === "php\0artisan\0horizon\0"
            && str_contains($own['argv'], "\0horizon:supervisor\0".$name."\0")
            && ($row->options['parentId'] ?? null) === (string)$parent['pid']);
        return ['name' => $name, 'master_name' => $master->name, 'supervisor' => $own, 'master' => $parent,
            'options' => $row->options, 'supervisor_status' => $row->status, 'master_status' => $master->status];
    }

    private static function locked(bool $create, bool $drain = false): mixed
    {
        $path = $drain ? self::DRAIN_LOCK : self::LOCK;
        self::parents($path);
        $mask = umask(0077);
        // The parent must hold this lock THROUGH proc_open. Close-on-exec
        // prevents a replacement from inheriting its exclusive open description.
        try { $lock = fopen($path, $create ? 'x+be' : 'r+be'); } finally { umask($mask); }
        self::gate(is_resource($lock));
        try {
            $s = fstat($lock); clearstatcache(true, $path);
            self::gate(is_array($s) && ($s['mode'] & 0177777) === 0100600 && $s['nlink'] === 1
                && $s['uid'] === 9999 && $s['gid'] === 9999 && $s['size'] === 0
                && lstat($path) === $s && flock($lock, LOCK_EX | LOCK_NB));
            return $lock;
        } catch (Throwable $failure) { fclose($lock); throw $failure; }
    }

    private static function state(string $key): array
    {
        $bytes = self::read(self::LEAF, 32768);
        $record = json_decode($bytes, true, 32, JSON_THROW_ON_ERROR);
        self::gate(is_array($record) && array_keys($record) === ['data', 'hmac'] && is_array($record['data'])
            && is_string($record['hmac']) && hash_equals(self::mac('inspect-state', $record['data'], $key), $record['hmac'])
            && self::json($record)."\n" === $bytes);
        $s = $record['data'];
        self::gate(array_keys($s) === ['request', 'phase', 'receipt', 'rejection'] && is_array($s['request'])
            && array_keys($s['request']) === ['run', 'expires', 'context', 'identities']
            && is_string($s['request']['run']) && preg_match('/\A[a-f0-9]{64}\z/', $s['request']['run']) === 1
            && is_int($s['request']['expires']) && $s['request']['expires'] > 0
            && is_array($s['request']['context']) && is_array($s['request']['identities'])
            && in_array($s['phase'], ['prepared', 'queued', 'executing', 'complete', 'held'], true)
            && ($s['receipt'] === null || is_array($s['receipt']))
            && in_array($s['rejection'], [null, 'authentication', 'inventory', 'pool', 'handle', 'worker', 'membership', 'postvalidation', 'receipt'], true));
        return $s;
    }

    private static function write(array $state, string $key, bool $create = false, bool $drain = false): void
    {
        $path = $drain ? self::DRAIN : self::LEAF; $limit = $drain ? 131072 : 32768;
        $bytes = self::json(['data' => $state, 'hmac' => self::mac($drain ? 'drain-state' : 'inspect-state', $state, $key)])."\n";
        self::gate(strlen($bytes) <= $limit); $parents = self::parents($path);
        $before = $create ? null : lstat($path);
        if (!$create) { self::read($path, $limit); }
        $mask = umask(0077);
        try { $out = fopen($path, $create ? 'x+b' : 'r+b'); } finally { umask($mask); }
        self::gate(is_resource($out));
        try {
            $opened = fstat($out);
            self::gate($create || self::identity($opened) === self::identity($before));
            self::gate(ftruncate($out, 0) && fwrite($out, $bytes) === strlen($bytes) && fflush($out) && fsync($out)
                && self::parents($path) === $parents);
        } finally { fclose($out); }
        $dir = fopen(dirname($path), 'r'); self::gate(is_resource($dir));
        try { self::gate(fsync($dir)); } finally { fclose($dir); }
        self::gate(self::read($path, $limit) === $bytes);
    }

    public static function request(int $producerDeadline): array
    {
        // The fixed startup entrypoint supplies its original 30s deadline.
        // This is internal source data, never a caller CLI option or authority.
        self::gate(hrtime(true) < $producerDeadline);
        $key = self::key(); $lock = self::locked(true);
        try {
            self::gate(@lstat(self::LEAF) === false);
            $context = self::context($key); $ids = self::identities($key);
            $request = ['run' => bin2hex(random_bytes(32)), 'expires' => min($producerDeadline, hrtime(true) + 20000000000),
                'context' => $context, 'identities' => $ids];
            self::gate(hrtime(true) < $request['expires']);
            $state = ['request' => $request, 'phase' => 'prepared', 'receipt' => null, 'rejection' => null];
            self::write($state, $key, true);
            $options = ['run' => $request['run'], 'request_hmac' => self::mac('inspect-request', $request, $key)];
            $command = json_encode(['command' => self::class, 'options' => $options], JSON_THROW_ON_ERROR);
            $client = app('redis')->connection('horizon')->client();
            self::gate(get_class($client) === \Redis::class && $client->getOption(\Redis::OPT_SERIALIZER) === \Redis::SERIALIZER_NONE);
            $physical = $client->getOption(\Redis::OPT_PREFIX).'commands:'.$ids['name'];
            $state['phase'] = 'queued'; self::write($state, $key);
            self::gate(hrtime(true) < $request['expires']);
            // One exact push; never adopt, flush, pop or retry foreign/ambiguous work.
            $sent = $client->rawCommand('EVAL', "if redis.call('LLEN',KEYS[1])~=0 then return 0 end redis.call('RPUSH',KEYS[1],ARGV[1]); return 1", '1', $physical, $command);
            if ($sent !== 1) { $state['phase'] = 'held'; self::write($state, $key); self::gate(false); }
            return ['version' => 1, 'operation' => 'horizon-inspect-request', 'phase' => 'queued'];
        } finally { fclose($lock); }
    }

    public static function observe(): array
    {
        $key = self::key(); $lock = self::locked(false); $state = null;
        try {
            $state = self::state($key); $request = $state['request'];
            self::gate($state['phase'] === 'complete' && hrtime(true) <= $request['expires']
                && self::context($key) === $request['context'] && self::identities($key) === $request['identities']
                && app('redis')->connection('horizon')->llen('commands:'.$request['identities']['name']) === 0);
            return $state['receipt'];
        } catch (Throwable $failure) {
            // Once an observation is ambiguous it cannot later turn into ACK
            // merely because a foreign/popped queue became empty again.
            if (is_array($state)) {
                try {
                    $state['phase'] = 'held'; $state['receipt'] = null; $state['rejection'] = 'authentication';
                    self::write($state, $key);
                } catch (Throwable $ignored) {}
            }
            throw $failure;
        } finally { fclose($lock); }
    }

    private static function drainState(string $key): array
    {
        $bytes = self::read(self::DRAIN, 131072);
        $record = json_decode($bytes, true, 64, JSON_THROW_ON_ERROR);
        self::gate(is_array($record) && array_keys($record) === ['data', 'hmac'] && is_array($record['data'])
            && is_string($record['hmac']) && hash_equals(self::mac('drain-state', $record['data'], $key), $record['hmac'])
            && self::json($record)."\n" === $bytes);
        $s = $record['data'];
        self::gate(in_array(array_keys($s), [['request', 'phase', 'original', 'intents', 'receipt', 'view'],
                ['request', 'phase', 'original', 'intents', 'receipt', 'view', 'restore']], true)
            && is_array($s['request']) && array_keys($s['request']) === ['run', 'expires', 'context', 'identities']
            && is_string($s['request']['run']) && preg_match('/\A[a-f0-9]{64}\z/', $s['request']['run']) === 1
            && is_int($s['request']['expires']) && $s['request']['expires'] > 0
            && is_array($s['request']['context']) && is_array($s['request']['identities'])
            && in_array($s['phase'], ['prepared', 'master-queued', 'master-armed', 'supervisor-queued',
                'supervisor-armed', 'waiting', 'workers-exited', 'quiescent', 'observing', 'held', 'restoring', 'restored'], true)
            && is_array($s['original']) && array_keys($s['original']) === ['master', 'supervisor']
            && is_array($s['intents']) && count($s['intents']) <= 100
            && ($s['receipt'] === null || is_array($s['receipt']))
            && ($s['view'] === null || is_array($s['view']) && array_keys($s['view']) === ['sequence', 'requested', 'expires', 'master', 'supervisor']
                && is_int($s['view']['sequence']) && $s['view']['sequence'] >= 1 && $s['view']['sequence'] <= 8
                && is_int($s['view']['expires']) && $s['view']['expires'] <= $s['request']['expires']
                && is_int($s['view']['requested']) && is_int($s['view']['master']) && is_int($s['view']['supervisor'])));
        return $s;
    }

    private static function children(int $pid): array
    {
        // This finite profile supports one task per PHP owner/worker. Otherwise
        // leader-only children would miss children created by another task.
        $dir = opendir('/proc/'.$pid.'/task'); self::gate(is_resource($dir)); $tasks = [];
        try {
            for ($i = 0; $i < 4 && ($task = readdir($dir)) !== false; ++$i) { $tasks[] = $task; }
        } finally { closedir($dir); }
        sort($tasks, SORT_STRING);
        self::gate($tasks === ['.', '..', (string)$pid]);
        $bytes = file_get_contents('/proc/'.$pid.'/task/'.$pid.'/children', false, null, 0, 4097);
        self::gate(is_string($bytes) && strlen($bytes) <= 4096 && preg_match('/\A(?:[0-9]+ ?)*\z/', $bytes) === 1);
        $children = $bytes === '' ? [] : array_map('intval', explode(' ', trim($bytes)));
        self::gate(count($children) <= 64 && count(array_unique($children)) === count($children));
        sort($children); return $children;
    }

    private static function drainAuth(array $s, string $key): void
    {
        $r = $s['request']; $ids = $r['identities'];
        self::gate(hrtime(true) < $r['expires'] && self::context($key) === $r['context']
            && self::proc($ids['master']['pid']) === $ids['master']
            && self::proc($ids['supervisor']['pid']) === $ids['supervisor']
            && self::children($ids['master']['pid']) === [$ids['supervisor']['pid']]);
        self::gate($s['phase'] !== 'observing' || hrtime(true) < $s['view']['expires']);
        $redis = app('redis')->connection('horizon');
        self::gate($redis->llen('commands:master:'.$ids['master_name']) === 0
            && $redis->llen('commands:'.$ids['name']) === 0);
        // Authentication I/O may cross either original deadline. Recheck after
        // the final read before returning to an owner mutation or observation.
        $now = hrtime(true);
        self::gate($now < $r['expires'] && ($s['phase'] !== 'observing' || $now < $s['view']['expires']));
    }

    private static function drainPush(array $s, string $role, string $key): void
    {
        self::gate(in_array($role, ['master', 'supervisor'], true));
        $r = $s['request']; $ids = $r['identities'];
        $options = ['drain_role' => $role, 'run' => $r['run'], 'request_hmac' => self::mac('drain-'.$role, $r, $key)];
        $command = self::json(['command' => self::class, 'options' => $options]);
        $client = app('redis')->connection('horizon')->client();
        self::gate(get_class($client) === \Redis::class && $client->getOption(\Redis::OPT_SERIALIZER) === \Redis::SERIALIZER_NONE
            && hrtime(true) < $r['expires']);
        $queue = $role === 'master' ? 'master:'.$ids['master_name'] : $ids['name'];
        $physical = $client->getOption(\Redis::OPT_PREFIX).'commands:'.$queue;
        self::gate($client->rawCommand('EVAL', "if redis.call('LLEN',KEYS[1])~=0 then return 0 end redis.call('RPUSH',KEYS[1],ARGV[1]); return 1", '1', $physical, $command) === 1);
    }

    private static function drainHold(?array $s, ?string $key): void
    {
        if (is_array($s) && is_string($key)) {
            try {
                $s['phase'] = 'held'; $s['receipt'] = null;
                if (isset($s['restore'])) { $s['restore']['stage'] = 'held'; }
                self::write($s, $key, false, true);
            }
            catch (Throwable $ignored) {}
        }
    }

    public static function drainEnter(int $producerDeadline): array
    {
        self::gate(hrtime(true) < $producerDeadline);
        $key = self::key(); $lock = self::locked(true, true); $s = null;
        try {
            self::gate(@lstat(self::DRAIN) === false);
            $r = ['run' => bin2hex(random_bytes(32)), 'expires' => min($producerDeadline - 500000000, hrtime(true) + 20000000000),
                'context' => self::context($key), 'identities' => self::identities($key)];
            $s = ['request' => $r, 'phase' => 'prepared', 'original' => ['master' => null, 'supervisor' => null],
                'intents' => [], 'receipt' => null, 'view' => null];
            self::drainAuth($s, $key); self::write($s, $key, true, true);
            $s['phase'] = 'master-queued'; self::write($s, $key, false, true);
            self::drainPush($s, 'master', $key);
        } catch (Throwable $failure) { self::drainHold($s, $key); throw $failure; }
        finally { fclose($lock); }
        // No requeue: a missing ACK expires into durable held state.
        for ($poll = 0; $poll < 250 && hrtime(true) < $producerDeadline; ++$poll) {
            usleep(100000); $lock = null; $ready = false;
            try { $lock = self::locked(false, true); } catch (Throwable $busy) { continue; }
            try {
                $s = self::drainState($key);
                if (isset($s['restore'])) { $s = null; self::gate(false); }
                if ($s['phase'] === 'held' || hrtime(true) >= $r['expires']) {
                    self::drainHold($s, $key); self::gate(false);
                }
                if ($s['phase'] === 'quiescent') {
                    self::drainAuth($s, $key);
                    self::gate(self::children($r['identities']['supervisor']['pid']) === []);
                    $ready = true;
                }
            } catch (Throwable $failure) { self::drainHold($s, $key); throw $failure; }
            finally { fclose($lock); }
            if ($ready) { return self::drainObserve($producerDeadline); }
        }
        // Receivers independently enforce the same expiry even if the producer
        // cannot reacquire the nonblocking lock. They never restore or force-stop.
        self::gate(false);
    }

    public static function drainObserve(int $producerDeadline): array
    {
        self::gate(hrtime(true) < $producerDeadline);
        $key = self::key(); $lock = self::locked(false, true); $s = null;
        try {
            $s = self::drainState($key);
            // This authority has been handed off. A retired H2 caller cannot
            // invalidate H3's distinct request or its completed native policy.
            if (isset($s['restore'])) { $s = null; self::gate(false); }
            self::gate($s['phase'] === 'quiescent');
            self::drainAuth($s, $key);
            self::gate(self::children($s['request']['identities']['supervisor']['pid']) === []);
            $sequence = ($s['view']['sequence'] ?? 0) + 1; self::gate($sequence <= 8);
            $s['view'] = ['sequence' => $sequence, 'requested' => hrtime(true),
                'expires' => min($s['request']['expires'], $producerDeadline - 500000000), 'master' => 0, 'supervisor' => 0];
            self::gate($s['view']['requested'] < $s['view']['expires']);
            $s['phase'] = 'observing'; self::write($s, $key, false, true);
        } catch (Throwable $failure) {
            // Empty stock command queues cannot repair a lost acknowledgement.
            self::drainHold($s, $key); throw $failure;
        } finally { fclose($lock); }
        // Existing loop-end listeners ACK actual objects after this generation.
        // No control command, extra queue push, requeue or renewed run expiry.
        for ($poll = 0; $poll < 250 && hrtime(true) < $producerDeadline; ++$poll) {
            usleep(100000); $lock = null;
            try { $lock = self::locked(false, true); } catch (Throwable $busy) { continue; }
            try {
                $s = self::drainState($key);
                if (isset($s['restore'])) { $s = null; self::gate(false); }
                self::drainAuth($s, $key);
                self::gate(in_array($s['phase'], ['observing', 'quiescent'], true)
                    && $s['view']['sequence'] === $sequence && hrtime(true) < $s['view']['expires']);
                if ($s['phase'] === 'quiescent') {
                    self::gate($s['view']['master'] >= $s['view']['requested']
                        && $s['view']['supervisor'] >= $s['view']['requested']);
                    return $s['receipt'];
                }
            } catch (Throwable $failure) { self::drainHold($s, $key); throw $failure; }
            finally { fclose($lock); }
        }
        self::gate(false);
    }

    private static function drainSeen(array &$s, string $key, string $role): void
    {
        if ($s['phase'] !== 'observing' || $s['view'][$role] !== 0) { return; }
        $s['view'][$role] = hrtime(true);
        if ($s['view']['master'] >= $s['view']['requested'] && $s['view']['supervisor'] >= $s['view']['requested']) {
            $s['phase'] = 'quiescent';
        }
        self::write($s, $key, false, true);
    }

    private static function drainArm(object $actor, array $options): void
    {
        $lock = null; $s = null; $key = null;
        try {
            $key = self::key(); $lock = self::locked(false, true); $s = self::drainState($key);
            if (isset($s['restore'])) { $s = null; self::gate(false); }
            $role = $options['drain_role']; $r = $s['request']; $ids = $r['identities'];
            self::gate(in_array($role, ['master', 'supervisor'], true)
                && $options === ['drain_role' => $role, 'run' => $r['run'], 'request_hmac' => self::mac('drain-'.$role, $r, $key)]
                && $s['phase'] === $role.'-queued'
                && get_class($actor) === ($role === 'master' ? \Laravel\Horizon\MasterSupervisor::class : Supervisor::class)
                && getmypid() === $ids[$role]['pid']
                && $actor->name === ($role === 'master' ? $ids['master_name'] : $ids['name']));
            self::drainAuth($s, $key);
            $s['phase'] = $role.'-armed'; self::write($s, $key, false, true);
            $retained = []; $replacements = []; $finished = false;
            $event = $role === 'master' ? \Laravel\Horizon\Events\MasterSupervisorLooped::class : \Laravel\Horizon\Events\SupervisorLooped::class;
            // Invoke after stock pending() has finished its entire local batch.
            // A duplicate changes phase to held before any control is executed.
            app('events')->listen($event, static function ($event) use ($actor, $role, $r, &$retained, &$replacements, &$finished): void {
                $actual = $role === 'master' ? $event->master : $event->supervisor;
                if ($actual === $actor && !$finished) { self::drainStep($actor, $role, $r['run'], $retained, $replacements, $finished); }
            });
        } catch (Throwable $failure) { if (is_resource($lock)) { self::drainHold($s, $key); } }
        finally { if (is_resource($lock)) { fclose($lock); } }
    }

    private static function drainIntent(array &$s, string $key, array $intent): void
    {
        self::drainAuth($s, $key);
        $s['intents'][] = $intent; self::write($s, $key, false, true);
        self::drainAuth($s, $key);
    }

    private static function drainStep(object $actor, string $role, string $run, array &$retained, array &$replacements, bool &$finished): void
    {
        $lock = null; $s = null; $key = null;
        try {
            $key = self::key(); $lock = self::locked(false, true); $s = self::drainState($key);
            // Authenticated phase handoff; H2's request and expiry stay immutable.
            // The same owner closures keep every original closed handle alive.
            if ($s['request']['run'] === $run && isset($s['restore'])) {
                self::restoreStep($actor, $role, $s, $key, $retained, $replacements);
                $finished = $s['phase'] === 'restored' && $s['restore']['stage'] === 'complete'; return;
            }
            if ($s['request']['run'] !== $run || $s['phase'] === 'held') { return; }
            self::drainAuth($s, $key); $ids = $s['request']['identities'];
            if ($role === 'master') {
                self::gate(count($actor->supervisors) === 1);
                $child = $actor->supervisors->first();
                self::gate(get_class($child) === \Laravel\Horizon\SupervisorProcess::class
                    && get_class($child->process) === \Symfony\Component\Process\Process::class
                    && $child->name === $ids['name'] && $child->process->isRunning()
                    && $child->process->getPid() === $ids['supervisor']['pid'] && !$child->dead);
                if ($s['phase'] === 'master-armed') {
                    self::gate(is_bool($actor->working) && ($actor->working ? 'running' : 'paused') === $ids['master_status']);
                    $retained = [$child, $child->process];
                    $s['original']['master'] = ['working' => $actor->working, 'object' => spl_object_id($actor),
                        'supervisor_object' => spl_object_id($child), 'process_object' => spl_object_id($child->process),
                        'options' => $child->options->toArray(), 'count' => count($actor->supervisors)];
                    self::drainIntent($s, $key, ['master-monitor-pause']);
                    $actor->working = false;
                    $s['phase'] = 'supervisor-queued'; self::write($s, $key, false, true);
                    self::drainPush($s, 'supervisor', $key);
                } else {
                    self::gate($actor->working === false && $retained === [$child, $child->process]
                        && $child->options->toArray() === $s['original']['master']['options']);
                    if ($s['phase'] === 'workers-exited') {
                        self::gate(self::children($ids['supervisor']['pid']) === []);
                        $row = app(SupervisorRepository::class)->find($ids['name']);
                        self::gate($row !== null && $row->status === 'paused' && $row->options === $ids['options']);
                        $s['phase'] = 'quiescent'; self::write($s, $key, false, true);
                    }
                    self::drainSeen($s, $key, 'master');
                }
                return;
            }
            self::gate($actor->options->toArray() === $ids['options'] && $actor->options->force === false);
            if ($s['phase'] === 'supervisor-armed') {
                self::gate(is_bool($actor->working) && ($actor->working ? 'running' : 'paused') === $ids['supervisor_status']
                    && count($actor->processPools) >= 1 && count($actor->processPools) <= 32);
                $original = ['working' => $actor->working, 'object' => spl_object_id($actor),
                    'options' => $actor->options->toArray(), 'pools' => []];
                $pids = []; $count = 0;
                foreach ($actor->processPools as $poolIndex => $pool) {
                    self::gate(is_int($poolIndex) && get_class($pool) === \Laravel\Horizon\ProcessPool::class
                        && is_bool($pool->working) && $pool->terminatingProcesses === [] && $pool->options->force === false
                        && count($pool->processes) >= 1 && count($pool->processes) <= 64);
                    $entry = ['working' => $pool->working, 'object' => spl_object_id($pool),
                        'options' => $pool->options->toArray(), 'workers' => []];
                    $retained[$poolIndex] = ['pool' => $pool, 'workers' => []];
                    foreach ($pool->processes as $slot => $worker) {
                        self::gate(is_int($slot) && ++$count <= 64 && get_class($worker) === \Laravel\Horizon\WorkerProcess::class
                            && get_class($worker->process) === \Symfony\Component\Process\Process::class && $worker->process->isRunning());
                        $pid = $worker->process->getPid(); self::gate(is_int($pid)); $proc = self::proc($pid);
                        self::gate($proc['parent'] === getmypid() && $proc['cgroup'] === $ids['supervisor']['cgroup']
                            && str_contains($proc['argv'], "\0horizon:work\0")
                            && !in_array('--force', explode("\0", $proc['argv']), true)
                            && str_contains($proc['argv'], "\0--supervisor=".$ids['name']."\0")
                            && self::children($pid) === []);
                        $status = file_get_contents('/proc/'.$pid.'/status', false, null, 0, 8193);
                        self::gate(is_string($status) && strlen($status) <= 8192
                            && preg_match('/^SigCgt:\s+([0-9a-f]{16})$/m', $status, $mask) === 1
                            && (hexdec(substr($mask[1], -8)) & (1 << (SIGTERM - 1))) !== 0);
                        $entry['workers'][$slot] = ['object' => spl_object_id($worker),
                            'process_object' => spl_object_id($worker->process), 'identity' => $proc];
                        $retained[$poolIndex]['workers'][$slot] = [$worker, $worker->process]; $pids[] = $pid;
                    }
                    $original['pools'][$poolIndex] = $entry;
                }
                sort($pids); self::gate(self::children(getmypid()) === $pids);
                $s['original']['supervisor'] = $original;
                self::drainIntent($s, $key, ['supervisor-monitor-pause']); $actor->working = false;
                foreach ($actor->processPools as $poolIndex => $pool) {
                    self::drainIntent($s, $key, ['pool-monitor-pause', $poolIndex]); $pool->working = false;
                    foreach ($pool->processes as $slot => $worker) {
                        $proc = $original['pools'][$poolIndex]['workers'][$slot]['identity'];
                        self::gate($worker->process->isRunning() && $worker->process->getPid() === $proc['pid']
                            && self::proc($proc['pid']) === $proc);
                        self::drainIntent($s, $key, ['worker-term', $poolIndex, $slot]);
                        // Parent-owned live handle only. Never scale, markForTermination,
                        // restart, stop, prune or remove a process from the pool.
                        $worker->terminate();
                    }
                }
                $s['phase'] = 'waiting'; self::write($s, $key, false, true);
            }
            if (!in_array($s['phase'], ['waiting', 'workers-exited', 'quiescent', 'observing'], true)) { return; }
            self::gate($actor->working === false && count($actor->processPools) === count($retained));
            $live = []; $exited = 0;
            foreach ($actor->processPools as $poolIndex => $pool) {
                self::gate(isset($retained[$poolIndex]) && $retained[$poolIndex]['pool'] === $pool
                    && $pool->working === false && $pool->terminatingProcesses === []
                    && $pool->options->toArray() === $s['original']['supervisor']['pools'][$poolIndex]['options']
                    && array_keys($pool->processes) === array_keys($retained[$poolIndex]['workers']));
                foreach ($pool->processes as $slot => $worker) {
                    self::gate($retained[$poolIndex]['workers'][$slot] === [$worker, $worker->process]);
                    $proc = $s['original']['supervisor']['pools'][$poolIndex]['workers'][$slot]['identity'];
                    if ($worker->process->isRunning()) {
                        self::gate($worker->process->getPid() === $proc['pid'] && self::proc($proc['pid']) === $proc
                            && self::children($proc['pid']) === []); $live[] = $proc['pid'];
                    } else {
                        self::gate($worker->process->getExitCode() === 0 && !$worker->process->hasBeenSignaled()
                            && !is_dir('/proc/'.$proc['pid'])); ++$exited;
                    }
                }
            }
            sort($live); self::gate(self::children(getmypid()) === $live);
            if ($live === []) {
                self::drainAuth($s, $key);
                if (in_array($s['phase'], ['quiescent', 'observing'], true)) {
                    self::drainSeen($s, $key, 'supervisor'); return;
                }
                $s['receipt'] = ['version' => 1, 'operation' => 'horizon-drain', 'selected_version' => $s['request']['context']['version'],
                    'phase' => 'quiescent', 'maintenance' => 'held', 'force' => false, 'restoration' => 'required',
                    'pool_count' => count($retained), 'worker_object_count' => $exited, 'exited_worker_count' => $exited,
                    'live_worker_count' => 0, 'terminating_count' => 0, 'master_status' => 'paused', 'supervisor_status' => 'paused',
                    'run_hmac' => self::mac('drain-run', [$run], $key), 'original_hmac' => self::mac('drain-original', $s['original'], $key),
                    'membership_hmac' => self::mac('drain-membership', [$ids['master'], $ids['supervisor'], []], $key),
                    'source_hmac' => $s['request']['context']['source'], 'config_hmac' => $s['request']['context']['config']];
                $s['phase'] = 'workers-exited'; self::write($s, $key, false, true);
            }
        } catch (Throwable $failure) { if (is_resource($lock)) { self::drainHold($s, $key); } }
        finally { if (is_resource($lock)) { fclose($lock); } }
    }

    private static function restoreAuth(array $s, string $key): void
    {
        $r = $s['restore']['request']; $ids = $s['request']['identities'];
        self::gate(array_keys($r) === ['run', 'generation', 'expires', 'drain_run', 'original_hmac']
            && (string)$s['original']['supervisor']['options']['nice'] === '0'
            && preg_match('/\A[a-f0-9]{64}\z/', $r['run']) === 1
            && preg_match('/\A[a-f0-9]{64}\z/', $r['generation']) === 1
            && is_int($r['expires']) && hrtime(true) < $r['expires']
            && $r['drain_run'] === $s['request']['run']
            && hash_equals(self::mac('restore-original', $s['original'], $key), $r['original_hmac'])
            && hash_equals(self::mac('restore-request', $r, $key), $s['restore']['request_hmac'])
            && $s['phase'] === 'restoring'
            && in_array($s['restore']['stage'], ['requested', 'master-ready', 'spawned', 'supervisor-ready'], true)
            && self::context($key) === $s['request']['context']
            && self::proc($ids['master']['pid']) === $ids['master']
            && self::proc($ids['supervisor']['pid']) === $ids['supervisor']
            && self::children($ids['master']['pid']) === [$ids['supervisor']['pid']]);
        $redis = app('redis')->connection('horizon');
        self::gate($redis->llen('commands:master:'.$ids['master_name']) === 0
            && $redis->llen('commands:'.$ids['name']) === 0 && hrtime(true) < $r['expires']);
    }

    private static function restoreIntent(array &$s, string $key, array $intent): void
    {
        self::restoreAuth($s, $key);
        self::gate(count($s['restore']['intents']) < 256);
        $s['restore']['intents'][] = $intent; self::write($s, $key, false, true);
        self::restoreAuth($s, $key);
    }

    public static function restore(int $producerDeadline): array
    {
        self::gate(hrtime(true) < $producerDeadline);
        $key = self::key(); $lock = null; $s = null;
        // H3 alone permits a bounded wait BEFORE creating its one request.
        // This neither retries H2 observation nor renews any control authority.
        $acquireBy = min($producerDeadline - 500000000, hrtime(true) + 2000000000);
        do {
            try { $lock = self::locked(false, true); break; } catch (Throwable $busy) { usleep(10000); }
        } while (hrtime(true) < $acquireBy);
        self::gate(is_resource($lock));
        try {
            $s = self::drainState($key);
            // A new request may inspect expired H2's retained handles. It never
            // renews H2 or derives success from its old observation receipt.
            if (isset($s['restore'])) { $s = null; self::gate(false); }
            self::gate(in_array($s['phase'], ['held', 'quiescent'], true)
                && is_array($s['original']['master']) && is_array($s['original']['supervisor']));
            $r = ['run' => bin2hex(random_bytes(32)), 'generation' => bin2hex(random_bytes(32)),
                'expires' => min($producerDeadline - 500000000, hrtime(true) + 20000000000),
                'drain_run' => $s['request']['run'], 'original_hmac' => self::mac('restore-original', $s['original'], $key)];
            $s['restore'] = ['request' => $r, 'request_hmac' => self::mac('restore-request', $r, $key),
                'stage' => 'requested', 'intents' => [], 'slots' => [], 'supervisor_seen' => 0];
            $s['phase'] = 'restoring'; $s['receipt'] = null;
            self::restoreAuth($s, $key); self::write($s, $key, false, true);
        } catch (Throwable $failure) { self::drainHold($s, $key); throw $failure; }
        finally { fclose($lock); }
        for ($poll = 0; $poll < 250 && hrtime(true) < $producerDeadline; ++$poll) {
            usleep(100000); $lock = null;
            try { $lock = self::locked(false, true); } catch (Throwable $busy) { continue; }
            try {
                $s = self::drainState($key);
                self::gate($s['restore']['request'] === $r && hrtime(true) < $r['expires']);
                if ($s['phase'] === 'restored' && $s['restore']['stage'] === 'complete') {
                    self::gate(self::context($key) === $s['request']['context'] && hrtime(true) < $r['expires']);
                    return $s['receipt'];
                }
                self::restoreAuth($s, $key);
            } catch (Throwable $failure) { self::drainHold($s, $key); throw $failure; }
            finally { fclose($lock); }
        }
        self::gate(false);
    }

    private static function restoreStep(object $actor, string $role, array &$s, string $key,
        array &$retained, array &$replacements): void
    {
        if ($s['restore']['stage'] === 'complete' && $s['phase'] === 'restored') { return; }
        try {
            self::restoreAuth($s, $key); $ids = $s['request']['identities'];
            $original = $s['original'][$role]; $stage = $s['restore']['stage'];
            self::gate(spl_object_id($actor) === $original['object']);
            if ($role === 'master') {
                $child = $actor->supervisors->first();
                self::gate(count($actor->supervisors) === 1 && $retained === [$child, $child->process]
                    && $actor->working === false && $child->process->isRunning() && !$child->dead
                    && $child->process->getPid() === $ids['supervisor']['pid']
                    && $child->options->toArray() === $original['options']);
                if ($stage === 'requested') {
                    self::restoreIntent($s, $key, ['master-handoff']);
                    $s['restore']['stage'] = 'master-ready'; self::write($s, $key, false, true);
                } elseif ($stage === 'supervisor-ready') {
                    self::restoreChildren($s, $key);
                    self::gate($s['restore']['supervisor_seen'] > 0);
                    self::restoreIntent($s, $key, ['master-restore-last']);
                    // Never continue(): it propagates CONT and loses mixed intent.
                    $actor->working = $original['working'];
                    $s['restore']['stage'] = 'complete'; $s['phase'] = 'restored';
                    $count = array_sum(array_map('count', $s['restore']['slots']));
                    $s['receipt'] = ['version' => 1, 'operation' => 'horizon-restore',
                        'selected_version' => $s['request']['context']['version'], 'phase' => 'restored',
                        'maintenance' => 'held', 'force' => false, 'pool_count' => count($s['restore']['slots']),
                        'worker_object_count' => $count, 'ready_worker_count' => $count, 'terminating_count' => 0,
                        'master_status' => $original['working'] ? 'running' : 'paused',
                        'supervisor_status' => $s['original']['supervisor']['working'] ? 'running' : 'paused',
                        'run_hmac' => self::mac('restore-run', $s['restore']['request'], $key),
                        'original_hmac' => $s['restore']['request']['original_hmac'],
                        'readiness_hmac' => self::mac('restore-readiness', $s['restore']['slots'], $key),
                        'source_hmac' => $s['request']['context']['source'], 'config_hmac' => $s['request']['context']['config']];
                    self::write($s, $key, false, true);
                }
                return;
            }
            self::gate($actor->options->toArray() === $original['options'] && $actor->options->force === false
                && count($actor->processPools) === count($retained) && count($retained) === count($original['pools']));
            if ($stage === 'requested') { self::gate($actor->working === false); return; }
            if ($stage === 'master-ready') {
                self::gate($actor->working === false && $replacements === []);
                // Validate ALL original children before replacing even one handle.
                foreach ($actor->processPools as $i => $pool) {
                    $saved = $original['pools'][$i];
                    self::gate($retained[$i]['pool'] === $pool && spl_object_id($pool) === $saved['object']
                        && $pool->working === false && $pool->terminatingProcesses === []
                        && $pool->options->toArray() === $saved['options'] && $pool->options->force === false
                        && (string)$pool->options->nice === '0'
                        && array_keys($pool->processes) === array_keys($saved['workers']));
                    foreach ($pool->processes as $slot => $worker) {
                        $old = $saved['workers'][$slot]; $native = $retained[$i]['workers'][$slot][1];
                        self::gate($retained[$i]['workers'][$slot] === [$worker, $worker->process]
                            && spl_object_id($worker) === $old['object'] && spl_object_id($native) === $old['process_object']
                            && !$native->isRunning() && $native->getExitCode() === 0 && !$native->hasBeenSignaled()
                            && !is_dir('/proc/'.$old['identity']['pid'])
                            && $native->getCommandLine() === $pool->options->toWorkerCommand()
                            && $native->getWorkingDirectory() === '/var/www/html' && $native->getTimeout() === null);
                    }
                }
                self::gate(self::children(getmypid()) === []);
                self::restoreIntent($s, $key, ['all-original-handles-closed']);
                // Keep the native policy in the supervisor, not in an expiring argv.
                app('events')->listen(\Laravel\Horizon\Events\WorkerProcessRestarting::class,
                    static function ($event) use ($actor, &$retained, &$replacements): void {
                        self::restoreNative($event->process, $actor, $retained, $replacements);
                    });
                foreach ($actor->processPools as $i => $pool) {
                    foreach ($pool->processes as $slot => $worker) {
                        $ticket = self::mac('restore-slot', [$s['restore']['request'], $i, $slot], $key);
                        $r = $s['restore']['request'];
                        $process = new \Symfony\Component\Process\Process(['/usr/local/bin/php', '/probe/startup.php',
                            'horizon-restore-worker', $r['run'], $r['generation'], (string)$i, (string)$slot, $ticket], self::ROOT);
                        $process->setTimeout(null)->disableOutput();
                        // Retain BEFORE start. Exceptions never destroy a live child.
                        $replacements[$i][$slot] = ['process' => $process, 'native' => false, 'run' => $r['run']];
                        $s['restore']['slots'][$i][$slot] = ['ticket' => $ticket, 'identity' => null, 'ack' => null];
                        self::restoreIntent($s, $key, ['spawn', $i, $slot]);
                        $worker->process = $process; $worker->start($worker->output);
                        self::gate($process->isRunning() && is_int($process->getPid()));
                        $identity = self::proc($process->getPid());
                        self::gate($identity['parent'] === getmypid() && $identity['cgroup'] === $ids['supervisor']['cgroup']);
                        $s['restore']['slots'][$i][$slot]['identity'] = $identity;
                        self::write($s, $key, false, true);
                    }
                }
                $s['restore']['stage'] = 'spawned'; self::write($s, $key, false, true); return;
            }
            self::gate(in_array($stage, ['spawned', 'supervisor-ready'], true));
            foreach ($actor->processPools as $i => $pool) {
                $saved = $original['pools'][$i];
                self::gate($retained[$i]['pool'] === $pool && $pool->terminatingProcesses === []
                    && $pool->options->toArray() === $saved['options']
                    && array_keys($pool->processes) === array_keys($saved['workers']));
                foreach ($pool->processes as $slot => $worker) {
                    self::gate($retained[$i]['workers'][$slot][0] === $worker
                        && $worker->process === $replacements[$i][$slot]['process'] && $worker->process->isRunning()
                        && $worker->process->getPid() === $s['restore']['slots'][$i][$slot]['identity']['pid']);
                }
                self::gate($pool->working === ($stage === 'spawned' ? false : $saved['working']));
            }
            self::gate($actor->working === ($stage === 'spawned' ? false : $original['working']));
            $ready = self::restoreChildren($s, $key, false);
            if (!$ready) { return; }
            if ($stage === 'spawned') {
                foreach ($actor->processPools as $i => $pool) {
                    self::restoreIntent($s, $key, ['pool-restore', $i]); $pool->working = $original['pools'][$i]['working'];
                }
                self::restoreIntent($s, $key, ['supervisor-restore']); $actor->working = $original['working'];
                $s['restore']['stage'] = 'supervisor-ready';
            }
            $s['restore']['supervisor_seen'] = hrtime(true); self::write($s, $key, false, true);
        } catch (Throwable $failure) {
            // Keep all objects; no stop/prune/scale, no release and no retry-to-success.
            $actor->working = false;
            if ($role === 'supervisor') { foreach ($actor->processPools as $pool) { $pool->working = false; } }
            self::drainHold($s, $key);
        }
    }

    private static function restoreChildren(array $s, string $key, bool $requireReady = true): bool
    {
        $pids = []; $ready = true; $r = $s['restore']['request'];
        foreach ($s['original']['supervisor']['pools'] as $i => $pool) {
            self::gate(array_keys($s['restore']['slots'][$i]) === array_keys($pool['workers']));
            foreach ($pool['workers'] as $slot => $old) {
                $entry = $s['restore']['slots'][$i][$slot]; $identity = $entry['identity'];
                self::gate(is_array($identity) && self::proc($identity['pid']) === $identity && self::children($identity['pid']) === []);
                $pids[] = $identity['pid'];
                if ($entry['ack'] === null) { $ready = false; continue; }
                $ack = $entry['ack'];
                self::gate($ack['data'] === ['run' => $r['run'], 'generation' => $r['generation'],
                    'pool' => $i, 'slot' => $slot, 'identity' => $identity,
                    'paused' => !$pool['working'], 'options_hmac' => self::mac('restore-options', $pool['options'], $key)]
                    && hash_equals(self::mac('restore-worker-ready', $ack['data'], $key), $ack['hmac']));
            }
        }
        sort($pids); self::gate(self::children($s['request']['identities']['supervisor']['pid']) === $pids
            && (!$requireReady || $ready)); return $ready;
    }

    public static function restoreWorker(array $args): never
    {
        self::gate(count($args) === 5 && preg_match('/\A[a-f0-9]{64}\z/', $args[0]) === 1
            && preg_match('/\A[a-f0-9]{64}\z/', $args[1]) === 1
            && preg_match('/\A(?:0|[1-9][0-9]?)\z/', $args[2]) === 1 && (int)$args[2] < 32
            && preg_match('/\A(?:0|[1-9][0-9]?)\z/', $args[3]) === 1 && (int)$args[3] < 64
            && preg_match('/\A[a-f0-9]{64}\z/', $args[4]) === 1);
        $key = self::key(); $i = (int)$args[2]; $slot = (int)$args[3];
        $deadline = hrtime(true) + 15000000000; $lock = null;
        do {
            try { $lock = self::locked(false, true); break; } catch (Throwable $busy) { usleep(10000); }
        } while (hrtime(true) < $deadline);
        self::gate(is_resource($lock));
        try {
            $s = self::drainState($key); self::restoreAuth($s, $key); $r = $s['restore']['request'];
            self::gate($r['run'] === $args[0] && $r['generation'] === $args[1]
                && hash_equals(self::mac('restore-slot', [$r, $i, $slot], $key), $args[4])
                && $s['restore']['stage'] === 'spawned'
                && $s['restore']['slots'][$i][$slot]['identity'] === self::proc(getmypid())
                && posix_getppid() === $s['request']['identities']['supervisor']['pid']);
            $options = $s['original']['supervisor']['pools'][$i]['options'];
        } finally { fclose($lock); }
        $worker = app('queue.worker');
        self::gate(get_class($worker) === \Illuminate\Queue\Worker::class && $worker->shouldQuit === false);
        $worker->paused = true;
        app('events')->listen(\Illuminate\Queue\Events\WorkerStarting::class,
            static function ($event) use ($worker, $args, $key, $i, $slot, $r, $options): void {
                $lock = null;
                do {
                    try { $lock = self::locked(false, true); break; } catch (Throwable $busy) { usleep(10000); }
                } while (hrtime(true) < $r['expires']);
                self::gate(is_resource($lock));
                try {
                    $s = self::drainState($key); self::restoreAuth($s, $key);
                    self::gate($s['restore']['request'] === $r && $s['restore']['stage'] === 'spawned'
                        && $s['restore']['slots'][$i][$slot]['ticket'] === $args[4]
                        && $s['restore']['slots'][$i][$slot]['identity'] === self::proc(getmypid())
                        && $s['restore']['slots'][$i][$slot]['ack'] === null
                        && app('queue.worker') === $worker && $worker->paused === true && $worker->shouldQuit === false
                        && $event->connectionName === $options['connection'] && $event->queue === $options['queue']
                        && $event->workerOptions->force === false && pcntl_signal_get_handler(SIGTERM) instanceof \Closure
                        && pcntl_signal_get_handler(SIGUSR2) instanceof \Closure);
                    foreach (['name' => 'workersName', 'backoff' => 'backoff', 'memory' => 'memory', 'timeout' => 'timeout',
                        'sleep' => 'sleep', 'maxTries' => 'maxTries', 'maxJobs' => 'maxJobs', 'maxTime' => 'maxTime', 'rest' => 'rest'] as $field => $saved) {
                        self::gate((string)$event->workerOptions->{$field} === (string)$options[$saved]);
                    }
                    $identity = self::proc(getmypid());
                    self::restoreIntent($s, $key, ['worker-ready', $i, $slot]);
                    $worker->paused = !$s['original']['supervisor']['pools'][$i]['working'];
                    $data = ['run' => $r['run'], 'generation' => $r['generation'], 'pool' => $i, 'slot' => $slot,
                        'identity' => $identity, 'paused' => $worker->paused,
                        'options_hmac' => self::mac('restore-options', $options, $key)];
                    $s['restore']['slots'][$i][$slot]['ack'] = ['data' => $data, 'hmac' => self::mac('restore-worker-ready', $data, $key)];
                    self::gate(hrtime(true) < $r['expires']);
                    self::write($s, $key, false, true); self::restoreAuth($s, $key);
                } catch (Throwable $failure) { $worker->paused = true; throw $failure; }
                finally { fclose($lock); }
            });
        $input = ['connection' => $options['connection'], '--name' => $options['workersName'], '--supervisor' => $options['name']];
        foreach (['backoff' => 'backoff', 'max-time' => 'maxTime', 'max-jobs' => 'maxJobs', 'memory' => 'memory',
            'queue' => 'queue', 'sleep' => 'sleep', 'timeout' => 'timeout', 'tries' => 'maxTries', 'rest' => 'rest'] as $flag => $field) {
            $input['--'.$flag] = (string)$options[$field];
        }
        // Fixed real command with structured input; never parse or execute caller argv.
        $command = app(\Laravel\Horizon\Console\WorkCommand::class); $command->setLaravel(app());
        self::gate(get_class($command) === \Laravel\Horizon\Console\WorkCommand::class
            && (new \ReflectionProperty(\Illuminate\Queue\Console\WorkCommand::class, 'worker'))->getValue($command) === $worker);
        while (ob_get_level() > 0) { ob_end_clean(); }
        exit($command->run(new \Symfony\Component\Console\Input\ArrayInput($input), new \Symfony\Component\Console\Output\NullOutput()));
    }

    private static function restoreNative(object $worker, object $actor, array &$retained, array &$replacements): void
    {
        foreach ($replacements as $i => &$slots) {
            foreach ($slots as $slot => &$entry) {
                if ($retained[$i]['workers'][$slot][0] !== $worker || $entry['native']) { continue; }
                // Permanent supervisor-local provenance, not renewed startup authority.
                // WorkerProcess emits this only AFTER its owned handle has stopped.
                $key = self::key(); $lock = null; $s = null;
                try {
                    // Completed owner-loop listeners have retired. A short
                    // read/observation collision must not break native restart.
                    $acquireBy = hrtime(true) + 2000000000;
                    do {
                        try { $lock = self::locked(false, true); break; } catch (Throwable $busy) { usleep(10000); }
                    } while (hrtime(true) < $acquireBy);
                    self::gate(is_resource($lock));
                    $s = self::drainState($key); $native = $retained[$i]['workers'][$slot][1];
                    self::gate($s['phase'] === 'restored' && $s['restore']['stage'] === 'complete'
                        && $entry['run'] === $s['restore']['request']['run']
                        && $actor->processPools[$i] === $retained[$i]['pool']
                        && $actor->processPools[$i]->processes[$slot] === $worker
                        && $worker->process === $entry['process'] && !$worker->process->isRunning()
                        && $worker->process->getExitCode() === 0 && !$worker->process->hasBeenSignaled()
                        && !$native->isRunning() && $native->getExitCode() === 0 && !$native->hasBeenSignaled());
                    $s['restore']['intents'][] = ['native-restart', $i, $slot]; self::write($s, $key, false, true);
                    $worker->process = $native; $entry['native'] = true;
                } catch (Throwable $failure) {
                    $actor->working = false; foreach ($actor->processPools as $pool) { $pool->working = false; }
                    if (is_resource($lock)) { self::drainHold($s, $key); }
                    throw new RuntimeException('horizon_restoration_required');
                } finally { if (is_resource($lock)) { fclose($lock); } }
                return;
            }
        }
    }

    public function process($supervisor, $options): void
    {
        if (is_array($options) && array_key_exists('drain_role', $options)) {
            self::drainArm($supervisor, $options); return;
        }
        // Do not throw to Horizon's raw exception logger or reveal private names.
        $lock = null; $state = null; $key = null; $stage = 'authentication';
        try {
            $key = self::key(); $lock = self::locked(false); $state = self::state($key);
            $request = $state['request'];
            self::gate(get_class($supervisor) === Supervisor::class && $state['phase'] === 'queued'
                && hrtime(true) <= $request['expires'] && is_array($options)
                && $options === ['run' => $request['run'], 'request_hmac' => self::mac('inspect-request', $request, $key)]
                && self::context($key) === $request['context'] && self::identities($key) === $request['identities']);
            $ids = $request['identities'];
            self::gate(getmypid() === $ids['supervisor']['pid'] && $supervisor->name === $ids['name']
                && $supervisor->options->toArray() === $ids['options'] && $supervisor->options->force === false
                && ($supervisor->working ? 'running' : 'paused') === $ids['supervisor_status']
                && app('redis')->connection('horizon')->llen('commands:'.$ids['name']) === 0);
            $state['phase'] = 'executing'; self::write($state, $key);
            $stage = 'inventory';
            $members = []; $objects = 0;
            self::gate(count($supervisor->processPools) >= 1 && count($supervisor->processPools) <= 32);
            foreach ($supervisor->processPools as $pool) {
                $stage = 'pool';
                // The accessor prunes and can force-stop. Inspect the PUBLIC property only.
                self::gate(get_class($pool) === \Laravel\Horizon\ProcessPool::class && $pool->terminatingProcesses === []
                    && $pool->options->force === false && count($pool->processes) >= 1 && count($pool->processes) <= 64);
                foreach ($pool->processes as $worker) {
                    $stage = 'handle';
                    self::gate(++$objects <= 64 && get_class($worker) === \Laravel\Horizon\WorkerProcess::class
                        && get_class($worker->process) === \Symfony\Component\Process\Process::class
                        && $worker->process->isRunning());
                    $pid = $worker->process->getPid(); self::gate(is_int($pid) && !isset($members[$pid]));
                    $stage = 'worker'; $child = self::proc($pid);
                    $stage = 'membership';
                    self::gate($child['parent'] === getmypid() && $child['cgroup'] === $ids['supervisor']['cgroup']
                        && str_contains($child['argv'], "\0horizon:work\0")
                        && str_contains($child['argv'], "\0--supervisor=".$ids['name']."\0")
                        && $worker->process->getPid() === $pid);
                    $members[$pid] = $child;
                }
            }
            $stage = 'postvalidation';
            self::gate(self::context($key) === $request['context'] && self::identities($key) === $ids
                && hrtime(true) <= $request['expires']);
            foreach ($members as $pid => $child) { self::gate(self::proc($pid) === $child); }
            $stage = 'receipt';
            $state['receipt'] = ['version' => 1, 'operation' => 'horizon-inspect', 'selected_version' => $request['context']['version'],
                'phase' => 'complete', 'supervisor_status' => $ids['supervisor_status'], 'master_status' => $ids['master_status'],
                'pool_count' => count($supervisor->processPools), 'worker_object_count' => $objects,
                'live_worker_count' => count($members), 'terminating_count' => 0, 'maintenance' => 'held', 'force' => false,
                'run_hmac' => self::mac('inspect-run', [$request['run']], $key),
                'supervisor_hmac' => self::mac('inspect-supervisor', [$ids['name'], $ids['supervisor']], $key),
                'master_hmac' => self::mac('inspect-master', [$ids['master_name'], $ids['master']], $key),
                'source_hmac' => $request['context']['source'], 'config_hmac' => $request['context']['config'],
                'membership_hmac' => self::mac('inspect-membership', $members, $key)];
            $state['phase'] = 'complete'; self::write($state, $key);
        } catch (Throwable $failure) {
            if (is_resource($lock) && is_array($state) && is_string($key)) {
                try { $state['phase'] = 'held'; $state['receipt'] = null; $state['rejection'] = $stage; self::write($state, $key); } catch (Throwable $ignored) {}
            }
        } finally { if (is_resource($lock)) { fclose($lock); } }
    }
}
