<?php
// PUBLIC exact producer methods: retired phase rejects without corrupting H3.
declare(strict_types=1);
$source = file_get_contents('/fixed/coolify_horizon_inspect.php');
function extractMethod(string $source, string $start, string $end): string {
    $a = strpos($source, '    '.$start); $b = strpos($source, '    '.$end, $a);
    if ($a === false || $b === false) { throw new RuntimeException('PUBLIC extraction'); }
    return substr($source, $a, $b - $a);
}
$methods = extractMethod($source, 'public static function drainObserve(', 'private static function drainSeen(')
    .extractMethod($source, 'public static function restore(', 'private static function restoreStep(')
    .extractMethod($source, 'private static function drainArm(', 'private static function drainIntent(')
    .extractMethod($source, 'private static function drainHold(', 'public static function drainEnter(');
eval('final class PublicRetiredPhase {
    public static array $durable;
    public static bool $handoff = false;
    public static int $reads = 0;
    private static function gate(bool $ok): void { if (!$ok) { throw new RuntimeException("PUBLIC rejection"); } }
    private static function key(): string { return "PUBLIC"; }
    private static function locked(bool $create, bool $drain): mixed { return tmpfile(); }
    private static function drainState(string $key): array {
        if (self::$handoff && ++self::$reads === 2) { self::$durable = ["phase" => "restored", "restore" => ["stage" => "complete"], "receipt" => ["phase" => "restored"]]; }
        return self::$durable;
    }
    private static function drainAuth(array $s, string $key): void {}
    private static function children(int $pid): array { return []; }
    public static function arm(int $deadline): void { self::drainArm(new stdClass, []); }
    private static function write(array $s, string $key, bool $create, bool $drain): void { self::$durable = $s; }
'.$methods.'}');
$failed = false;
foreach (['drainObserve', 'restore', 'arm'] as $method) {
    $original = ['phase' => 'restored', 'restore' => ['stage' => 'complete'], 'receipt' => ['phase' => 'restored']];
    PublicRetiredPhase::$durable = $original; $rejected = false;
    try { PublicRetiredPhase::$method(hrtime(true) + 1000000000); } catch (RuntimeException $e) { $rejected = true; }
    $same = PublicRetiredPhase::$durable === $original;
    echo json_encode(['public_method' => $method, 'rejected' => $rejected, 'completed_handoff_unchanged' => $same], JSON_THROW_ON_ERROR)."\n";
    $failed = $failed || ($method !== 'arm' && !$rejected) || !$same;
}
PublicRetiredPhase::$durable = ['phase' => 'quiescent', 'request' => ['expires' => hrtime(true) + 3000000000,
    'identities' => ['supervisor' => ['pid' => 42]]], 'view' => null];
PublicRetiredPhase::$handoff = true; $rejected = false;
try { PublicRetiredPhase::drainObserve(hrtime(true) + 2000000000); } catch (RuntimeException $e) { $rejected = true; }
$same = PublicRetiredPhase::$durable === $original;
echo json_encode(['public_method' => 'late-observe-poll', 'rejected' => $rejected,
    'completed_handoff_unchanged' => $same], JSON_THROW_ON_ERROR)."\n";
$failed = $failed || !$rejected || !$same;
exit($failed ? 1 : 0);
