<?php
// PUBLIC exact restoreStep exception boundary with real Symfony processes.
// Synthetic owner/options/auth/storage; NOT application readiness evidence.
declare(strict_types=1);
require '/var/www/html/vendor/autoload.php';
function app(string $name): object { return new class { public function listen($name, $callback): void {} }; }
$source = file_get_contents('/fixed/coolify_horizon_inspect.php');
$a = strpos($source, '    private static function restoreStep(');
$b = strpos($source, '    private static function restoreChildren(', $a);
$method = substr($source, $a, $b - $a);
eval('final class PublicPartial {
    private const ROOT = "/var/www/html/";
    public static array $durable = [];
    private static function gate(bool $ok): void { if (!$ok) { throw new RuntimeException("PUBLIC gate"); } }
    private static function restoreAuth(array $s, string $key): void {}
    private static function mac(string $domain, array $value, string $key): string { return str_repeat("a", 64); }
    private static function proc(int $pid): array { return ["pid" => $pid, "parent" => getmypid(), "cgroup" => "PUBLIC"]; }
    private static function children(int $pid): array { return []; }
    private static function write(array $s, string $key, bool $create, bool $drain): void {
        self::$durable = $s;
        if ($s["phase"] === "restoring" && isset($s["restore"]["slots"][0][0]["identity"])) {
            throw new RuntimeException("PUBLIC injected durable write failure after first real spawn");
        }
    }
    private static function restoreIntent(array &$s, string $key, array $intent): void { $s["restore"]["intents"][] = $intent; self::write($s, $key, false, true); }
    private static function drainHold(array &$s, string $key): void { $s["phase"] = "held"; $s["restore"]["stage"] = "held"; self::$durable = $s; }
    public static function run(object $actor, array &$s, array &$retained, array &$replacements): void { self::restoreStep($actor, "supervisor", $s, "PUBLIC", $retained, $replacements); }
'.$method.'}');
$processes = []; $retainedWorkers = []; $savedWorkers = [];
for ($i = 0; $i < 2; ++$i) {
    $p = new Symfony\Component\Process\Process(['/usr/local/bin/php', '-r', 'exit(0);'], '/var/www/html');
    $p->setTimeout(null); if ($p->run() !== 0) { exit(1); }
    $worker = new Laravel\Horizon\WorkerProcess($p); $worker->handleOutputUsing(static function (): void {});
    $processes[$i] = $worker; $retainedWorkers[$i] = [$worker, $p];
    $savedWorkers[$i] = ['object' => spl_object_id($worker), 'process_object' => spl_object_id($p), 'identity' => ['pid' => 4194304]];
}
$options = new class($p->getCommandLine()) {
    public bool $force = false;
    public int $nice = 0;
    public function __construct(private string $command) {}
    public function toArray(): array { return ['force' => false]; }
    public function toWorkerCommand(): string { return $this->command; }
};
$pool = (object)['working' => false, 'options' => $options, 'terminatingProcesses' => [], 'processes' => $processes];
$actor = (object)['working' => false, 'options' => $options, 'processPools' => [$pool]];
$retained = [['pool' => $pool, 'workers' => $retainedWorkers]]; $replacements = [];
$s = ['phase' => 'restoring', 'request' => ['identities' => ['supervisor' => ['cgroup' => 'PUBLIC']]],
    'original' => ['supervisor' => ['object' => spl_object_id($actor), 'options' => $options->toArray(),
        'pools' => [['object' => spl_object_id($pool), 'options' => $options->toArray(), 'workers' => $savedWorkers]]]],
    'restore' => ['stage' => 'master-ready', 'request' => ['run' => str_repeat('b', 64), 'generation' => str_repeat('c', 64)],
        'intents' => [], 'slots' => []]];
PublicPartial::run($actor, $s, $retained, $replacements);
$live = $replacements[0][0]['process'];
$ok = $s['phase'] === 'held' && count($replacements[0]) === 1 && $live->isRunning()
    && $pool->processes[0]->process === $live && $pool->processes[1]->process === $retainedWorkers[1][1]
    && $retainedWorkers[0][1]->getExitCode() === 0 && $retainedWorkers[1][1]->getExitCode() === 0
    && !$actor->working && !$pool->working && $pool->terminatingProcesses === [];
$exit = $live->wait();
echo json_encode(['public_partial_spawn_held_owned_live' => $ok, 'natural_child_exit' => $exit,
    'signaled' => $live->hasBeenSignaled(), 'application_readiness_evidence' => false], JSON_THROW_ON_ERROR)."\n";
exit($ok && $exit === 0 && !$live->hasBeenSignaled() ? 0 : 1);
