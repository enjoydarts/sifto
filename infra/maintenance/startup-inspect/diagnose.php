<?php
// PUBLIC fixture diagnosis only. Fixed line/function names; no exception text.
register_shutdown_function(static function (): void {
    global $failure, $startupDeadline, $argv;
    if ($failure instanceof Throwable) {
        fwrite(STDERR, json_encode(['public_failure_line' => $failure->getLine(),
            'functions' => array_map(static fn ($f): array => ['function' => $f['function'] ?? '',
                'line' => $f['line'] ?? 0], $failure->getTrace())], JSON_THROW_ON_ERROR)."\n");
        if (in_array($argv[1] ?? '', ['horizon-drain-enter', 'horizon-drain-observe'], true)) {
            $path = '/var/www/html/storage/app/startup-horizon-drain.state';
            if (is_file($path)) {
                $s = json_decode(file_get_contents($path), true, 64, JSON_THROW_ON_ERROR)['data'];
                $now = hrtime(true);
                fwrite(STDERR, json_encode(['public_phase' => $s['phase'],
                    'request_remaining_ms' => intdiv($s['request']['expires'] - $now, 1000000),
                    'view_remaining_ms' => isset($s['view']['expires']) ? intdiv($s['view']['expires'] - $now, 1000000) : null,
                    'intents' => $s['intents']], JSON_THROW_ON_ERROR)."\n");
            }
        }
    }
    elseif (in_array($argv[1] ?? '', ['horizon-inspect-request', 'horizon-drain-enter'], true)) {
        $leaf = $argv[1] === 'horizon-drain-enter' ? 'drain' : 'inspect';
        $state = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-'.$leaf.'.state'), true, 64, JSON_THROW_ON_ERROR);
        $bound = is_int($startupDeadline) && $state['data']['request']['expires'] <= $startupDeadline;
        fwrite(STDERR, json_encode(['public_request_bound_to_original_30s_deadline' => $bound], JSON_THROW_ON_ERROR)."\n");
        if (!$bound) { exit(1); }
    }
});
