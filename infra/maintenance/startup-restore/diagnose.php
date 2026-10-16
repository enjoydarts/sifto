<?php
// PUBLIC clone only; line/function trace without arguments or exception text.
if (($argv[1] ?? '') === 'horizon-restore-worker' && is_file('/tmp/public-restore-fault')) {
    $publicFault = trim(file_get_contents('/tmp/public-restore-fault'));
    $publicFirst = ($argv[4] ?? '') === '0' && ($argv[5] ?? '') === '0';
    if ($publicFirst && $publicFault === 'generation') { $argv[3] = str_repeat('0', 64); }
    if ($publicFirst && $publicFault === 'ticket') { $argv[6] = str_repeat('0', 64); }
    if ($publicFault === 'noack' || $publicFault === 'partial' && $publicFirst) { usleep(9000000); }
}
register_shutdown_function(static function (): void {
    global $failure, $argv;
    if (($argv[1] ?? '') === 'horizon-restore-worker' && $failure instanceof Throwable) {
        file_put_contents('/tmp/public-restore-child-failure.json', json_encode([
            'line' => $failure->getLine(), 'calls' => array_map(static fn ($f): array => [
                $f['function'] ?? '', $f['line'] ?? 0], $failure->getTrace())], JSON_THROW_ON_ERROR));
    }
});
