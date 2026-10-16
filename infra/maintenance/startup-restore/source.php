<?php
// PUBLIC pinned-image source only; no application bootstrap.
declare(strict_types=1);
require '/startup/drain-source.php';
foreach (['vendor/laravel/horizon/src/Events/WorkerProcessRestarting.php',
    'vendor/laravel/horizon/src/Console/WorkCommand.php',
    'vendor/laravel/horizon/src/SupervisorOptions.php',
    'vendor/laravel/framework/src/Illuminate/Queue/Console/WorkCommand.php',
    'vendor/laravel/framework/src/Illuminate/Queue/Events/WorkerStarting.php',
    'vendor/laravel/framework/src/Illuminate/Queue/WorkerOptions.php',
    'vendor/symfony/console/Input/ArrayInput.php'] as $file) {
    $bytes = file_get_contents('/var/www/html/'.$file);
    if (!is_string($bytes) || strlen($bytes) > 2097152) { exit(1); }
    echo json_encode(['file' => $file, 'sha256' => hash('sha256', $bytes), 'source' => $bytes], JSON_THROW_ON_ERROR)."\n";
}
