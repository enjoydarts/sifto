<?php
// PUBLIC source evidence only; never bootstrap an application.
declare(strict_types=1);
require '/startup/inspect-source.php';
foreach (['vendor/laravel/horizon/src/SupervisorProcess.php',
    'vendor/laravel/horizon/src/Events/MasterSupervisorLooped.php',
    'vendor/laravel/horizon/src/Events/SupervisorLooped.php',
    'vendor/laravel/framework/src/Illuminate/Queue/Worker.php'] as $file) {
    $bytes = file_get_contents('/var/www/html/'.$file);
    if (!is_string($bytes) || strlen($bytes) > 2097152) { exit(1); }
    echo json_encode(['file' => $file, 'sha256' => hash('sha256', $bytes), 'source' => $bytes], JSON_THROW_ON_ERROR)."\n";
}
