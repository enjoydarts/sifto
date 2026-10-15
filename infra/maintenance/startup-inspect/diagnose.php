<?php
// PUBLIC fixture diagnosis only. Fixed line/function names; no exception text.
register_shutdown_function(static function (): void {
    global $failure, $startupDeadline, $argv;
    if ($failure instanceof Throwable) {
        fwrite(STDERR, json_encode(['public_failure_line' => $failure->getLine(),
            'functions' => array_map(static fn ($f): array => ['function' => $f['function'] ?? '',
                'line' => $f['line'] ?? 0], $failure->getTrace())], JSON_THROW_ON_ERROR)."\n");
    }
    elseif (($argv[1] ?? '') === 'horizon-inspect-request') {
        $state = json_decode(file_get_contents('/var/www/html/storage/app/startup-horizon-inspect.state'), true, 32, JSON_THROW_ON_ERROR);
        $bound = is_int($startupDeadline) && $state['data']['request']['expires'] <= $startupDeadline;
        fwrite(STDERR, json_encode(['public_request_bound_to_original_30s_deadline' => $bound], JSON_THROW_ON_ERROR)."\n");
        if (!$bound) { exit(1); }
    }
});
