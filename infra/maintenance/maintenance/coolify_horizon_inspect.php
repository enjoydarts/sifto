<?php
// Fixed PSR-4 receiver. Inspection only; no worker/control mutation capability.
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
    private const CLASS_FILE = '/var/www/html/app/SiftoMaintenance/InspectSupervisor.php';
    private const SOURCES = [
        'composer.json' => '84acb6071121ac9a8843f960c500ae3ed469287fdb9f4b7164975d37ae979593',
        'vendor/composer/ClassLoader.php' => 'd4b2313e562edf942838ec316d983c1a3459ca67da3e5658e861200b0b3693c2',
        'vendor/composer/autoload_real.php' => 'ad097449e5603cb3204b9e3d63e8bbf510984b4559196386f22d886190c1a06b',
        'vendor/composer/autoload_psr4.php' => '391ba6f01907760bb4564cfb1dc33b42d984c359f8589183639bb4475d3fb5fa',
        'vendor/symfony/process/Process.php' => '73c4e31230f9602768b73cda3451ed7ec0dddc20ddef6cc0d6795ea879600c39',
        'vendor/laravel/horizon/src/WorkerProcess.php' => 'b03707bb4eb455ce8b01b3dc744820d1bfd843ca86b481be94aa081b02299abc',
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
        $sources['helper'] = hash('sha256', self::read(self::CLASS_FILE, 32768));
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

    private static function locked(bool $create): mixed
    {
        self::parents(self::LOCK);
        $mask = umask(0077);
        try { $lock = fopen(self::LOCK, $create ? 'x+b' : 'r+b'); } finally { umask($mask); }
        self::gate(is_resource($lock));
        try {
            $s = fstat($lock); clearstatcache(true, self::LOCK);
            self::gate(is_array($s) && ($s['mode'] & 0177777) === 0100600 && $s['nlink'] === 1
                && $s['uid'] === 9999 && $s['gid'] === 9999 && $s['size'] === 0
                && lstat(self::LOCK) === $s && flock($lock, LOCK_EX | LOCK_NB));
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

    private static function write(array $state, string $key, bool $create = false): void
    {
        $bytes = self::json(['data' => $state, 'hmac' => self::mac('inspect-state', $state, $key)])."\n";
        self::gate(strlen($bytes) <= 32768); $parents = self::parents(self::LEAF);
        $before = $create ? null : lstat(self::LEAF);
        if (!$create) { self::read(self::LEAF, 32768); }
        $mask = umask(0077);
        try { $out = fopen(self::LEAF, $create ? 'x+b' : 'r+b'); } finally { umask($mask); }
        self::gate(is_resource($out));
        try {
            $opened = fstat($out);
            self::gate($create || self::identity($opened) === self::identity($before));
            self::gate(ftruncate($out, 0) && fwrite($out, $bytes) === strlen($bytes) && fflush($out) && fsync($out)
                && self::parents(self::LEAF) === $parents);
        } finally { fclose($out); }
        $dir = fopen(dirname(self::LEAF), 'r'); self::gate(is_resource($dir));
        try { self::gate(fsync($dir)); } finally { fclose($dir); }
        self::gate(self::read(self::LEAF, 32768) === $bytes);
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

    public function process($supervisor, $options): void
    {
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
