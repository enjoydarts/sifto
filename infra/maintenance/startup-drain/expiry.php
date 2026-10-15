<?php
// PUBLIC exact-method deadline regression. No APP, real I/O, worker or signal.
declare(strict_types=1);
$source = file_get_contents('/fixed/coolify_horizon_inspect.php');
function method(string $source, string $name, string $next): string {
    $start = strpos($source, '    private static function '.$name.'(');
    $end = strpos($source, '    '.$next, $start);
    if ($start === false || $end === false) { throw new RuntimeException('PUBLIC extraction'); }
    return substr($source, $start, $end - $start);
}
$methods = method($source, 'drainAuth', 'private static function drainPush(')
    .method($source, 'drainIntent', 'private static function drainStep(')
    .method($source, 'drainHold', 'public static function drainEnter(');
$fixture = <<<'PHP'
namespace PublicDrainExpiry;
function app(string $name): object {
    return new class {
        public function connection(string $name): object { return $this; }
        public function llen(string $name): int { Probe::io(str_starts_with($name, 'commands:master:') ? 'llen-master' : 'llen-supervisor'); return 0; }
    };
}
final class Probe {
    private static int $calls;
    private static string $delay;
    private static array $durable;
    private static int $deadline;
    private static array $injection;
    public static function io(string $point): void {
        if (self::$calls === 2 && self::$delay === $point) {
            self::$injection = ['delayed' => true, 'began_before_expiry' => hrtime(true) < self::$deadline];
            usleep(120000);
            self::$injection['ended_after_expiry'] = hrtime(true) >= self::$deadline;
        }
    }
    private static function gate(bool $value): void { if (!$value) { throw new \RuntimeException('PUBLIC rejected'); } }
    private static function context(string $key): array { ++self::$calls; self::io('context'); return ['public' => true]; }
    private static function proc(int $pid): array { self::io($pid === 101 ? 'proc-master' : 'proc-supervisor'); return ['pid' => $pid]; }
    private static function children(int $pid): array { self::io('children'); return [102]; }
    private static function write(array $state, string $key, bool $create, bool $drain): void { self::$durable = $state; }
    public static function run(string $delay, bool $view): array {
        self::$calls = 0; self::$delay = $delay; self::$durable = [];
        $start = hrtime(true); $expired = $start + 50000000;
        self::$deadline = $expired; self::$injection = [];
        $state = ['request' => ['expires' => $view ? $start + 1000000000 : $expired,
            'context' => ['public' => true], 'identities' => ['master' => ['pid' => 101],
                'supervisor' => ['pid' => 102], 'master_name' => 'public-master', 'name' => 'public-supervisor']],
            'phase' => $view ? 'observing' : 'supervisor-armed', 'view' => $view ? ['expires' => $expired] : null,
            'original' => ['working' => true, 'slot' => 0], 'receipt' => null, 'intents' => []];
        $original = $state['original'];
        $process = (object)['public' => true]; $worker = (object)['process' => $process];
        $pool = (object)['working' => true, 'processes' => [$worker]]; $retained = [$worker, $process];
        $controls = 0; $returned = false;
        try {
            self::drainIntent($state, 'PUBLIC', ['worker-term', 0, 0]); $returned = true;
            // An observation ACK or owner action must not be reached after expiry.
            $pool->working = false; ++$controls;
        } catch (\RuntimeException $expected) { self::drainHold($state, 'PUBLIC'); }
        $same = $pool->processes === [$retained[0]] && $worker->process === $retained[1];
        $ok = !$returned && $controls === 0 && $pool->working && $same
            && self::$durable['phase'] === 'held' && self::$durable['original'] === $original
            && self::$durable['intents'] === [['worker-term', 0, 0]] && self::$calls === 2
            && self::$injection === ['delayed' => true, 'began_before_expiry' => true, 'ended_after_expiry' => true]
            && hrtime(true) >= $expired;
        return ['delay' => $delay, 'deadline' => $view ? 'view' : 'request', 'ok' => $ok,
            'intent_returned' => $returned, 'subsequent_controls' => $controls,
            'same_retained_objects' => $same, 'phase' => self::$durable['phase'], 'injection' => self::$injection];
    }
PHP;
eval($fixture.$methods."\n}");
$ok = true;
foreach (['context', 'proc-master', 'proc-supervisor', 'children', 'llen-master', 'llen-supervisor'] as $delay) {
    foreach ([false, true] as $view) {
        $result = \PublicDrainExpiry\Probe::run($delay, $view); $ok = $result['ok'] && $ok;
        echo json_encode($result, JSON_THROW_ON_ERROR)."\n";
    }
}
echo json_encode(['candidate_sha256' => hash('sha256', $source),
    'extracted_methods_sha256' => hash('sha256', $methods)], JSON_THROW_ON_ERROR)."\n";
exit($ok ? 0 : 1);
