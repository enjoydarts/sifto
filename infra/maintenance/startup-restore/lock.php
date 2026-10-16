<?php
// PUBLIC exact locking method regression; real Symfony child, no APP or signal.
declare(strict_types=1);
require '/var/www/html/vendor/autoload.php';
$source = file_get_contents('/fixed/coolify_horizon_inspect.php');
$start = strpos($source, '    private static function locked(');
$end = strpos($source, '    private static function state(', $start);
$method = substr($source, $start, $end - $start);
eval('final class PublicLock {
    private const LOCK = "/tmp/public-restore-lock";
    private const DRAIN_LOCK = self::LOCK;
    private static function parents(string $path): array { return []; }
    private static function gate(bool $ok): void { if (!$ok) { throw new RuntimeException("PUBLIC lock gate"); } }
    public static function acquire(): mixed { return self::locked(true, true); }
'.$method.'}');
$lock = PublicLock::acquire();
$child = new Symfony\Component\Process\Process(['/usr/local/bin/php', '-r',
    'usleep(300000); $h=fopen("/tmp/public-restore-lock","r+b"); exit(flock($h, LOCK_EX|LOCK_NB) ? 0 : 1);'], '/var/www/html');
$child->start(); fclose($lock); $exit = $child->wait();
echo json_encode(['public_child_acquired_after_parent_close' => $exit === 0,
    'actual_child_exit' => $exit], JSON_THROW_ON_ERROR)."\n";
exit($exit);
